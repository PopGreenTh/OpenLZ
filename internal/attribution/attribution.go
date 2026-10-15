// Package attribution maps cost lines onto the Project and Biz cost hierarchies with a single
// portable SQL query (plain CTEs, no engine-specific syntax: it runs unchanged on embedded DuckDB
// and on PostgreSQL, the database Directus uses).
//
// Inputs are three tables, created by Schema:
//
//	node      every valid Project path prefix, one row per node, with the Biz codes its levels reference
//	evidence  Cloud-to-Project links (kind override|rg|account|region|lz, key, node_id)
//	lines     the cost lines to attribute (cloud values, project tags, biz tags)
//	alias     per level, invalid tag value -> correct code (hier project|biz, lvl 1..5)
//
// Resolution order for each line:
//
//	A. Alias: an invalid tag value is replaced by the code the alias table maps it to, and the
//	   line is flagged "alias" (a repaired tag is not "tag-corrected").
//	0. Resource override: an explicit resource -> node mapping always wins.
//	1. Tag anchor: every valid path is scored against the line's Project tags; deeper tags weigh
//	   more (1,2,4,8,16), so the most specific tag wins and a stale upper tag is corrected and flagged.
//	2. Evidence refines the anchor, most specific first: resource group, biz tag, account's environment, account's app, region.
//	   Evidence that contradicts what is already known is ignored and flagged ("name!"), never applied.
//	3. LZ evidence is a last resort, used only when nothing else is known (an LZ hosts many projects).
//	4. Common ancestor: a level survives only if every remaining candidate agrees on it.
//	5. Missing levels are padded with <LEVEL>-UNSPEC, or <LEVEL>-UNALLOC when nothing resolved.
//
// The result is always built from real tree nodes, never by joining independently resolved codes,
// so it cannot produce a combination that is not in the tree.
package attribution

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// Schema creates the input tables.
const Schema = `
CREATE TABLE alias (hier TEXT, lvl INT, v TEXT, code TEXT);
CREATE TABLE node (node_id INT, lvl INT, p TEXT, a TEXT, e TEXT, f TEXT, fn TEXT, bu TEXT, cc TEXT, own TEXT, pl TEXT, cust TEXT);
CREATE TABLE evidence (kind TEXT, k TEXT, node_id INT);
CREATE TABLE lines (line_id INT, resource_id TEXT, rg TEXT, account TEXT, region TEXT, lz TEXT,
  p TEXT, a TEXT, e TEXT, f TEXT, fn TEXT, bu_tag TEXT, cc_tag TEXT, own_tag TEXT, pl_tag TEXT, cust_tag TEXT);`

// Result is the attribution of one line.
type Result struct {
	LineID      int
	ProjectHash string // |Project|App|Env|Feature|Function|
	BizHash     string // |BU|CostCenter|OwnerTeam|Product|Customer|
	// Trail lists what was used: "override", "tags", evidence classes ("rg", "biz", "account",
	// "region", "lz"), a trailing "!" on a class that was ignored because it contradicted the
	// anchor, and "tag-corrected" when a tag disagrees with the final path.
	Trail string
}

// Resolve runs the query on a database that already holds the node, evidence and lines tables.
func Resolve(ctx context.Context, db *sql.DB) ([]Result, error) {
	rows, err := db.QueryContext(ctx, SQL())
	if err != nil {
		return nil, fmt.Errorf("attribution query: %w", err)
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.LineID, &r.ProjectHash, &r.BizHash, &r.Trail); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

var (
	pathCols = []string{"p", "a", "e", "f", "fn"}
	bizCols  = []string{"bu", "cc", "own", "pl", "cust"}
)

// SQL returns the portable attribution query.
func SQL() string {
	var b strings.Builder
	b.WriteString("WITH\n-- A: alias table repairs invalid tag values.\n")
	b.WriteString(aliasCTEs() + ",\n")
	b.WriteString("-- 1: tag anchor. Score every valid path; deeper tags weigh more.\n")
	b.WriteString("anchor_score AS (SELECT l.line_id, n.node_id, " + tagScore() + " AS score FROM lines l CROSS JOIN node n),\n")
	b.WriteString("anchor_max AS (SELECT line_id, MAX(score) AS ms FROM anchor_score WHERE score > 0 GROUP BY line_id),\n")
	b.WriteString("s0 AS (SELECT a.line_id, a.node_id FROM anchor_score a JOIN anchor_max m ON m.line_id = a.line_id AND a.score = m.ms),\n")
	b.WriteString("-- 2: evidence refines the anchor, most specific class first.\n")

	evidence := func(kind, col string) string {
		return fmt.Sprintf("SELECT l.line_id, e.node_id FROM lines l JOIN evidence e ON e.kind = '%s' AND e.k = l.%s", kind, col)
	}
	biz := "SELECT l.line_id, n.node_id FROM lines l JOIN node n ON " +
		"(n.lvl = 1 AND n.bu = l.bu_tag) OR (n.lvl = 2 AND n.cc = l.cc_tag) OR (n.lvl = 3 AND n.own = l.own_tag) " +
		"OR (n.lvl = 4 AND n.pl = l.pl_tag) OR (n.lvl = 5 AND n.cust = l.cust_tag)"
	steps := []struct{ name, cand string }{
		{"rg", evidence("rg", "rg")},
		{"biz", biz},
		{"acctenv", evidence("acctenv", "account")},
		{"account", evidence("account", "account")},
		{"region", evidence("region", "region")},
	}
	prev := "s0"
	for i, s := range steps {
		k := i + 1
		b.WriteString(step(k, prev, s.cand, s.name) + ",\n")
		prev = fmt.Sprintf("s%d", k)
	}

	b.WriteString(strings.NewReplacer(
		"{LZ}", evidence("lz", "lz"),
		"{OV}", evidence("override", "resource_id"),
		"{LAST}", prev,
		"{AGG}", aggColumns(),
	).Replace(tail))
	q := linesRef.ReplaceAllString(b.String(), "ln l")
	return cteDef.ReplaceAllString(q, "$1 AS MATERIALIZED (")
}

// linesRef matches the alias-qualified lines table. Everything after the alias step reads the
// aliased copy "ln" instead of the raw "lines" table.
var linesRef = regexp.MustCompile(`\blines l\b`)

// tagCols lists the tag columns of the lines table in alias order: hierarchy, level, column.
var tagCols = []struct {
	hier string
	lvl  int
	col  string
}{
	{"project", 1, "p"}, {"project", 2, "a"}, {"project", 3, "e"}, {"project", 4, "f"}, {"project", 5, "fn"},
	{"biz", 1, "bu_tag"}, {"biz", 2, "cc_tag"}, {"biz", 3, "own_tag"}, {"biz", 4, "pl_tag"}, {"biz", 5, "cust_tag"},
}

// aliasCTEs renders the "lal" (alias lookups) and "ln" (aliased lines) CTEs.
func aliasCTEs() string {
	var lookups, applied, flags []string
	for _, t := range tagCols {
		lookups = append(lookups, fmt.Sprintf(
			"(SELECT MIN(a.code) FROM alias a WHERE a.hier = '%s' AND a.lvl = %d AND a.v = r.%s) AS %s_al", t.hier, t.lvl, t.col, t.col))
		applied = append(applied, fmt.Sprintf("COALESCE(x.%[1]s_al, x.%[1]s) AS %[1]s", t.col))
		flags = append(flags, fmt.Sprintf("x.%s_al IS NOT NULL", t.col))
	}
	return "lal AS (SELECT r.*,\n    " + strings.Join(lookups, ",\n    ") + "\n  FROM lines r),\n" +
		"ln AS (SELECT x.line_id, x.resource_id, x.rg, x.account, x.region, x.lz,\n    " + strings.Join(applied, ",\n    ") + ",\n" +
		"    CASE WHEN " + strings.Join(flags, " OR ") + " THEN 1 ELSE 0 END AS aliased\n  FROM lal x)"
}

// cteDef matches the start of a CTE definition. Every CTE is materialized: the chain refers to
// earlier sets several times, and engines that inline CTEs (DuckDB before 1.3) would otherwise
// re-evaluate them exponentially. PostgreSQL 12+ and DuckDB both accept AS MATERIALIZED.
var cteDef = regexp.MustCompile(`(?m)^(\w+) AS \(`)

func tagScore() string {
	parts := make([]string, len(pathCols))
	for i, c := range pathCols {
		parts[i] = fmt.Sprintf("(CASE WHEN n.%s = l.%s THEN %d ELSE 0 END)", c, c, 1<<i)
	}
	return strings.Join(parts, " + ")
}

// compat is true when two nodes agree on every level both define.
func compat(x, y string) string {
	parts := make([]string, len(pathCols))
	for i, c := range pathCols {
		parts[i] = fmt.Sprintf("(%[1]s.%[3]s IS NULL OR %[2]s.%[3]s IS NULL OR %[1]s.%[3]s = %[2]s.%[3]s)", x, y, c)
	}
	return strings.Join(parts, " AND ")
}

// step refines candidate set `prev` with the nodes of one evidence class.
func step(k int, prev, cand, name string) string {
	return strings.NewReplacer(
		"{K}", fmt.Sprint(k), "{PREV}", prev, "{NAME}", name, "{CAND}", cand, "{COMPAT}", compat("sn", "cn"),
	).Replace(stepTmpl)
}

const stepTmpl = `c_{NAME} AS ({CAND}),
p{K} AS (
  SELECT s.line_id, CASE WHEN sn.lvl >= cn.lvl THEN sn.node_id ELSE cn.node_id END AS node_id
  FROM {PREV} s
  JOIN node sn ON sn.node_id = s.node_id
  JOIN c_{NAME} c ON c.line_id = s.line_id
  JOIN node cn ON cn.node_id = c.node_id
  WHERE {COMPAT}),
s{K} AS (
  SELECT line_id, node_id FROM p{K}
  UNION SELECT c.line_id, c.node_id FROM c_{NAME} c
        WHERE NOT EXISTS (SELECT 1 FROM {PREV} s WHERE s.line_id = c.line_id)
  UNION SELECT s.line_id, s.node_id FROM {PREV} s
        WHERE NOT EXISTS (SELECT 1 FROM p{K} p WHERE p.line_id = s.line_id)),
t_{NAME} AS (
  SELECT l.line_id,
         CASE WHEN EXISTS (SELECT 1 FROM c_{NAME} c WHERE c.line_id = l.line_id)
                   AND (EXISTS (SELECT 1 FROM p{K} p WHERE p.line_id = l.line_id)
                        OR NOT EXISTS (SELECT 1 FROM {PREV} s WHERE s.line_id = l.line_id)) THEN '{NAME} '
              WHEN EXISTS (SELECT 1 FROM c_{NAME} c WHERE c.line_id = l.line_id) THEN '{NAME}! '
              ELSE '' END AS piece
  FROM lines l)`

// aggColumns: one common-ancestor expression per path level and per biz level.
func aggColumns() string {
	var parts []string
	for _, c := range append(append([]string{}, pathCols...), bizCols...) {
		parts = append(parts, fmt.Sprintf(
			"CASE WHEN COUNT(nd.%[1]s) = COUNT(*) AND COUNT(DISTINCT nd.%[1]s) = 1 THEN MIN(nd.%[1]s) END AS %[1]s_u", c))
	}
	return strings.Join(parts, ",\n    ")
}

const tail = `c_lz AS ({LZ}),
-- 3: LZ is a last resort: used only when nothing else is known.
slz AS (SELECT line_id, node_id FROM {LAST}
       UNION SELECT c.line_id, c.node_id FROM c_lz c WHERE NOT EXISTS (SELECT 1 FROM {LAST} s WHERE s.line_id = c.line_id)),
-- 0: explicit resource override always wins.
ov AS ({OV}),
fs AS (SELECT line_id, node_id FROM ov
       UNION SELECT s.line_id, s.node_id FROM slz s WHERE NOT EXISTS (SELECT 1 FROM ov o WHERE o.line_id = s.line_id)),
-- 4: common ancestor of the remaining candidates.
agg AS (
  SELECT f.line_id,
    {AGG}
  FROM fs f JOIN node nd ON nd.node_id = f.node_id GROUP BY f.line_id),
fin AS (
  SELECT l.line_id, l.p AS tp, l.a AS ta, l.e AS te, l.f AS tf, l.fn AS tfn,
    g.p_u,
    CASE WHEN g.p_u IS NOT NULL THEN g.a_u END AS a_f,
    CASE WHEN g.p_u IS NOT NULL AND g.a_u IS NOT NULL THEN g.e_u END AS e_f,
    CASE WHEN g.p_u IS NOT NULL AND g.a_u IS NOT NULL AND g.e_u IS NOT NULL THEN g.f_u END AS f_f,
    CASE WHEN g.p_u IS NOT NULL AND g.a_u IS NOT NULL AND g.e_u IS NOT NULL AND g.f_u IS NOT NULL THEN g.fn_u END AS fn_f,
    g.bu_u,
    CASE WHEN g.p_u IS NOT NULL THEN g.cc_u END AS cc_f,
    CASE WHEN g.p_u IS NOT NULL AND g.a_u IS NOT NULL THEN g.own_u END AS own_f,
    CASE WHEN g.p_u IS NOT NULL AND g.a_u IS NOT NULL AND g.e_u IS NOT NULL THEN g.pl_u END AS pl_f,
    CASE WHEN g.p_u IS NOT NULL AND g.a_u IS NOT NULL AND g.e_u IS NOT NULL AND g.f_u IS NOT NULL THEN g.cust_u END AS cust_f
  FROM lines l LEFT JOIN agg g ON g.line_id = l.line_id)
-- 5: pad missing levels with UNSPEC (or UNALLOC when nothing resolved).
SELECT f.line_id,
  '|' || COALESCE(f.p_u, 'PRJ-UNALLOC') || '|' ||
    CASE WHEN f.p_u IS NULL THEN 'APP-UNALLOC' ELSE COALESCE(f.a_f, 'APP-UNSPEC') END || '|' ||
    CASE WHEN f.p_u IS NULL THEN 'ENV-UNALLOC' ELSE COALESCE(f.e_f, 'ENV-UNSPEC') END || '|' ||
    CASE WHEN f.p_u IS NULL THEN 'FEAT-UNALLOC' ELSE COALESCE(f.f_f, 'FEAT-UNSPEC') END || '|' ||
    CASE WHEN f.p_u IS NULL THEN 'FUNC-UNALLOC' ELSE COALESCE(f.fn_f, 'FUNC-UNSPEC') END || '|' AS project_hash,
  '|' || CASE WHEN f.p_u IS NULL THEN 'BU-UNALLOC' ELSE f.bu_u END || '|' ||
    CASE WHEN f.p_u IS NULL THEN 'CC-UNALLOC' ELSE COALESCE(f.cc_f, 'CC-UNSPEC') END || '|' ||
    CASE WHEN f.p_u IS NULL THEN 'TEAM-UNALLOC' ELSE COALESCE(f.own_f, 'TEAM-UNSPEC') END || '|' ||
    CASE WHEN f.p_u IS NULL THEN 'PL-UNALLOC' ELSE COALESCE(f.pl_f, 'PL-UNSPEC') END || '|' ||
    CASE WHEN f.p_u IS NULL THEN 'CUST-UNALLOC' ELSE COALESCE(f.cust_f, 'CUST-UNSPEC') END || '|' AS biz_hash,
  TRIM(
    CASE WHEN EXISTS (SELECT 1 FROM lines l WHERE l.line_id = f.line_id AND l.aliased = 1) THEN 'alias ' ELSE '' END ||
    CASE WHEN EXISTS (SELECT 1 FROM ov o WHERE o.line_id = f.line_id) THEN 'override ' ELSE '' END ||
    CASE WHEN EXISTS (SELECT 1 FROM s0 x WHERE x.line_id = f.line_id) THEN 'tags ' ELSE '' END ||
    (SELECT piece FROM t_rg t WHERE t.line_id = f.line_id) || (SELECT piece FROM t_biz t WHERE t.line_id = f.line_id) ||
    (SELECT piece FROM t_acctenv t WHERE t.line_id = f.line_id) ||
    (SELECT piece FROM t_account t WHERE t.line_id = f.line_id) || (SELECT piece FROM t_region t WHERE t.line_id = f.line_id) ||
    CASE WHEN EXISTS (SELECT 1 FROM c_lz c WHERE c.line_id = f.line_id) AND NOT EXISTS (SELECT 1 FROM {LAST} s WHERE s.line_id = f.line_id) THEN 'lz ' ELSE '' END ||
    CASE WHEN (f.tp IS NOT NULL AND f.p_u IS NOT NULL AND f.tp <> f.p_u) OR (f.ta IS NOT NULL AND f.a_f IS NOT NULL AND f.ta <> f.a_f)
           OR (f.te IS NOT NULL AND f.e_f IS NOT NULL AND f.te <> f.e_f) OR (f.tf IS NOT NULL AND f.f_f IS NOT NULL AND f.tf <> f.f_f)
           OR (f.tfn IS NOT NULL AND f.fn_f IS NOT NULL AND f.tfn <> f.fn_f) THEN 'tag-corrected ' ELSE '' END
  ) AS trail
FROM fin f ORDER BY f.line_id`
