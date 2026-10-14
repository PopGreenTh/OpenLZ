package attribution

import (
	"context"
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

// Row is the part of a cost row that attribution reads: where it ran and the two sensed tags.
type Row struct {
	AccountID, AccountName, ConnectionID string
	CSP, Region, Zone                    string
	Service, UsageType, ResourceID       string
	PrimaryTag, SecondaryTag             string // Project and App tags
}

// CloudInput converts the row to the Cloud-side input of taxonomy.ResolveCloud.
func (r Row) CloudInput() taxonomy.Input {
	return taxonomy.Input{
		AccountID: r.AccountID, AccountName: r.AccountName, ConnectionID: r.ConnectionID,
		CSP: r.CSP, Region: r.Region, Zone: r.Zone,
		Service: r.Service, UsageType: r.UsageType, ResourceID: r.ResourceID,
	}
}

// Runner loads the tables and runs the attribution query (see duckdb.Engine.Attribute).
type Runner func(ctx context.Context, nodes []Node, evidence []Evidence, aliases []Alias, lines []Line) ([]Result, error)

// Attributed is the outcome for one row: its resolved Cloud side and its attribution.
type Attributed struct {
	Cloud  taxonomy.Output
	Result Result
}

// ResolveRows attributes rows to Project and Biz paths. The Cloud side of each row comes from
// taxonomy.ResolveCloud, which ignores tags and never climbs into the Project or Biz
// hierarchies, so a tag cannot leak into the evidence. The tag-scoring step compares every
// line with every node, so distinct line combinations are resolved once and joined back.
func ResolveRows(ctx context.Context, run Runner, cat *taxonomy.Catalog, rows []Row) ([]Attributed, error) {
	type lineKey struct{ resource, rg, account, region, lz, p, a string }
	cloudOut := map[Row]taxonomy.Output{}
	lineIDs := map[lineKey]int{}
	var lines []Line
	clouds := make([]taxonomy.Output, len(rows))
	rowLine := make([]int, len(rows))

	for i, row := range rows {
		cloudRow := Row{AccountID: row.AccountID, AccountName: row.AccountName, ConnectionID: row.ConnectionID, CSP: row.CSP,
			Region: row.Region, Zone: row.Zone, Service: row.Service, UsageType: row.UsageType, ResourceID: row.ResourceID}
		co, ok := cloudOut[cloudRow]
		if !ok {
			co = cat.ResolveCloud(cloudRow.CloudInput())
			cloudOut[cloudRow] = co
		}
		clouds[i] = co

		region := co.Region.ID
		if region == "" {
			region = row.Region
		}
		lk := lineKey{
			resource: row.ResourceID,
			rg:       co.ResourceGroup.ID,
			account:  co.Account.ID,
			region:   region,
			lz:       co.LZ.ID,
			p:        cat.CodeOf(taxonomy.Projects, row.PrimaryTag),
			a:        cat.CodeOf(taxonomy.Apps, row.SecondaryTag),
		}
		id, ok := lineIDs[lk]
		if !ok {
			id = len(lines) + 1
			lineIDs[lk] = id
			lines = append(lines, Line{ID: id, ResourceID: lk.resource, RG: lk.rg, Account: lk.account,
				Region: lk.region, LZ: lk.lz, P: lk.p, A: lk.a})
		}
		rowLine[i] = id
	}

	tn, tev := cat.AttributionTables()
	nodes := make([]Node, len(tn))
	for i, n := range tn {
		nodes[i] = Node{
			ID: n.ID, Level: n.Level,
			P: n.Codes[0], A: n.Codes[1], E: n.Codes[2], F: n.Codes[3], Fn: n.Codes[4],
			BU: n.Biz[0], CC: n.Biz[1], Own: n.Biz[2], PL: n.Biz[3], Cust: n.Biz[4],
		}
	}
	evidence := make([]Evidence, len(tev))
	for i, e := range tev {
		evidence[i] = Evidence{Kind: e.Kind, Key: e.Key, NodeID: e.NodeID}
	}
	var aliases []Alias
	for _, a := range cat.AliasRows() {
		aliases = append(aliases, Alias{Hier: a.Hier, Level: a.Level, Value: a.Value, Code: a.Code})
	}

	results, err := run(ctx, nodes, evidence, aliases, lines)
	if err != nil {
		return nil, fmt.Errorf("attribute cost rows: %w", err)
	}
	byLine := make(map[int]Result, len(results))
	for _, r := range results {
		byLine[r.LineID] = r
	}
	out := make([]Attributed, len(rows))
	for i := range rows {
		r, ok := byLine[rowLine[i]]
		if !ok {
			return nil, fmt.Errorf("attribution returned no result for line %d", rowLine[i])
		}
		out[i] = Attributed{Cloud: clouds[i], Result: r}
	}
	return out, nil
}
