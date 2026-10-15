// Package taxonomy maps sensed cost rows onto the three cost hierarchies.
//
//	Biz:     BusinessUnit → CostCenter → OwnerTeam → ProductLine → Customer
//	Project: Project → App → Environment → Feature → Function
//	Cloud:   LZ/Connection → Account → Region/Zone → ServiceCat/Service
//	         → ServiceUsage/UsageType → ResourceGroup/Resource
//
// The taxonomy is edited in Directus and stored as one flat JSON snapshot
// (see Schema for the collections and the relations between them). The authoritative
// specification is docs/taxonomy-spec.md; internal/attribution implements its attribution
// rules, this package validates the data and offers the legacy two-way Resolve.
package taxonomy

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Slot is a code and its display name.
type Slot struct {
	ID   string
	Name string
}

// Input is the state of one cost row. Empty, "Untagged", and "Unknown" values are unfilled.
type Input struct {
	AccountID    string
	AccountName  string
	ConnectionID string
	CSP          string
	Region       string
	Zone         string
	Service      string
	UsageType    string
	ResourceID   string

	// Tag values sensed from the cloud provider. A value that is present is kept as-is.
	Project         string
	App             string
	Environment     string
	Feature         string
	Function        string
	ServiceCategory string
	BusinessUnit    string
	CostCenter      string
	OwnerTeam       string
	ProductLine     string
	Customer        string
}

// Output is the resolved hierarchy for one cost row.
type Output struct {
	BusinessUnit Slot
	CostCenter   Slot
	OwnerTeam    Slot
	ProductLine  Slot
	Customer     Slot

	Project     Slot
	App         Slot
	Environment Slot
	Feature     Slot
	Function    Slot

	LZ              Slot
	Connection      Slot
	OU              Slot // the OU the account sits in (the leaf of its chain)
	Account         Slot
	CSP             Slot
	Region          Slot
	Zone            Slot
	ServiceCategory Slot
	Service         Slot
	ServiceUsage    Slot
	Usage           Slot
	ResourceGroup   Slot
	Resource        Slot

	// BillingCurrency is the currency of the row's LZ.
	BillingCurrency string
	// RegionCSPMismatch is set when the Region's CSP differs from its LZ's CSP.
	RegionCSPMismatch bool
	// ZoneRegionMismatch is set when the Zone belongs to another Region than the row's Region.
	ZoneRegionMismatch bool
}

type node struct {
	coll     string
	id, name string
	uid      string // coll|scope|code: unique across the catalog
	scope    string
	path     string // project hierarchy only: |PRJ|APP|ENV|
	attrs    map[string]string
	refs     map[string]*node
	mapped   *node              // project_path attribute: the Project node a Resource/ResourceGroup maps to
	multi    map[string][]*node // many-to-many parents, keyed by parent collection
	children map[string][]*node // keyed by child collection
}

func (n *node) ref(field string) *node {
	if n == nil {
		return nil
	}
	return n.refs[field]
}

func (n *node) slot() Slot {
	if n == nil {
		return Slot{}
	}
	return Slot{ID: n.id, Name: n.name}
}

// Catalog is a validated taxonomy snapshot.
type Catalog struct {
	byID     map[string]map[string][]*node
	byName   map[string]map[string][]*node
	byPath   map[string]*node
	ordered  map[string][]*node // nodes of each collection in snapshot order
	counts   map[string]int
	warnings []string
}

// Warnings lists consistency problems that do not prevent loading.
func (c *Catalog) Warnings() []string { return c.warnings }

// Count returns the number of rows loaded for a collection.
func (c *Catalog) Count(collection string) int { return c.counts[collection] }

// Load reads and validates a taxonomy JSON snapshot.
func Load(path string) (*Catalog, error) {
	snap, err := ReadSnapshot(path)
	if err != nil {
		return nil, err
	}
	cat, err := Build(snap)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cat, nil
}

// Build validates a snapshot and indexes it. All validation errors are reported together.
func Build(snap *Snapshot) (*Catalog, error) {
	c := &Catalog{
		byID:    map[string]map[string][]*node{},
		byName:  map[string]map[string][]*node{},
		byPath:  map[string]*node{},
		ordered: map[string][]*node{},
		counts:  map[string]int{},
	}
	var errs []error
	for _, spec := range Schema {
		c.byID[spec.Name] = map[string][]*node{}
		c.byName[spec.Name] = map[string][]*node{}
		if spec.Junction {
			errs = append(errs, c.link(spec, snap.Rows(spec.Name))...)
			continue
		}
		for i, row := range orderSelfRef(spec, snap.Rows(spec.Name)) {
			where := fmt.Sprintf("%s[%d]", spec.Name, i)
			if row.Code() == "" || row.Name() == "" {
				errs = append(errs, fmt.Errorf("%s: code and name are required (code=%q name=%q)", where, row.Code(), row.Name()))
				continue
			}
			where = fmt.Sprintf("%s %q", spec.Label, row.Code())
			n := &node{coll: spec.Name, id: row.Code(), name: row.Name(), attrs: map[string]string{},
				refs: map[string]*node{}, multi: map[string][]*node{}, children: map[string][]*node{}}

			for _, ref := range spec.Refs {
				raw := strings.TrimSpace(row[ref.Field])
				if raw == "" {
					if ref.Required() {
						errs = append(errs, fmt.Errorf("%s: %s is required", where, ref.Field))
					}
					continue
				}
				target, err := c.lookupRef(ref.Target, raw)
				if err != nil {
					errs = append(errs, fmt.Errorf("%s: %s %q: %w", where, ref.Field, raw, err))
					continue
				}
				n.refs[ref.Field] = target
				target.children[spec.Name] = append(target.children[spec.Name], n)
			}
			for _, a := range spec.Attrs {
				if err := checkAttr(a, strings.TrimSpace(row[a.Field])); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", where, err))
					continue
				}
				if v := strings.TrimSpace(row[a.Field]); v != "" {
					n.attrs[a.Field] = v
				}
			}
			if f := spec.ScopeField; f != "" {
				if p := n.refs[f]; p != nil {
					n.scope = p.uid
				} else {
					n.scope = key(n.attrs[f])
				}
			}
			n.uid = spec.Name + "|" + n.scope + "|" + key(n.id)

			if spec.Hierarchy == HierProject {
				n.path = "|" + n.id + "|"
				for _, ref := range spec.Refs {
					if p := n.refs[ref.Field]; ref.Kind == RefParent && p != nil {
						n.path = p.path + n.id + "|"
					}
				}
				c.byPath[key(n.path)] = n
			}

			for _, other := range c.byID[spec.Name][key(n.id)] {
				if other.scope == n.scope {
					scope := ""
					if spec.ScopeField != "" {
						scope = " within its " + spec.ScopeField
					}
					errs = append(errs, fmt.Errorf("%s: duplicate code%s", where, scope))
				}
			}
			c.byID[spec.Name][key(n.id)] = append(c.byID[spec.Name][key(n.id)], n)
			c.byName[spec.Name][key(n.name)] = append(c.byName[spec.Name][key(n.name)], n)
			c.ordered[spec.Name] = append(c.ordered[spec.Name], n)
			c.counts[spec.Name]++
		}
	}
	// Resource and ResourceGroup mappings to a Project path need the whole project tree.
	for _, coll := range []string{ResourceGroups, Resources} {
		for _, n := range c.ordered[coll] {
			if p := n.attrs["project_path"]; p != "" {
				if t := c.byPath[key(p)]; t != nil {
					n.mapped = t
				} else {
					errs = append(errs, fmt.Errorf("%s %q: project_path %q is not a valid Project path", specLabel(coll), n.id, p))
				}
			}
		}
	}
	errs = append(errs, c.checkOUs()...)
	errs = append(errs, c.checkAliases()...)
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	c.checkBiz()
	c.checkCloud()
	return c, nil
}

// checkOUs requires every OU to hang under a parent OU or directly under a landing zone, and
// rejects a parent chain that never reaches a landing zone.
func (c *Catalog) checkOUs() []error {
	var errs []error
	for _, ou := range c.ordered[OUs] {
		if ou.ref("parent") == nil && ou.ref("landing_zone") == nil {
			errs = append(errs, fmt.Errorf("OU %q: needs a parent OU or a landing_zone", ou.id))
		}
	}
	return errs
}

// ouLZ returns the landing zone an OU belongs to: the landing_zone of the root of its chain.
func ouLZ(ou *node) *node {
	for ou != nil && ou.ref("parent") != nil {
		ou = ou.ref("parent")
	}
	return ou.ref("landing_zone")
}

// accountApps returns the Apps an Account hosts through its OUs: the links of the nearest OU
// above it (its own OU first) that has any, so a more specific OU overrides its parents.
func accountApps(acc *node) []*node {
	for ou := acc.ref("ou"); ou != nil; ou = ou.ref("parent") {
		if apps := ou.multi[Apps]; len(apps) > 0 {
			return apps
		}
	}
	return nil
}

// checkAliases validates alias targets: a Project path of the alias level's depth, or a Biz code.
func (c *Catalog) checkAliases() []error {
	var errs []error
	projectLevels, bizLevels := Levels(HierProject), Levels(HierBiz)
	for _, a := range c.ordered[Aliases] {
		level := indexOf(AliasLevels, a.attrs["level"])
		target := a.attrs["target"]
		if level < 0 || target == "" {
			continue // reported by the attribute checks
		}
		where := fmt.Sprintf("Alias %q (%s)", a.id, a.attrs["level"])
		if level < len(projectLevels) {
			if n := c.byPath[key(target)]; n == nil || n.coll != projectLevels[level] {
				errs = append(errs, fmt.Errorf("%s: target %q is not a valid %s path", where, target, specLabel(projectLevels[level])))
			}
		} else if c.lookup(bizLevels[level-len(projectLevels)], target) == nil {
			errs = append(errs, fmt.Errorf("%s: target %q is not a %s code", where, target, specLabel(bizLevels[level-len(projectLevels)])))
		}
	}
	return errs
}

// orderSelfRef puts the rows of a self-referencing collection (OUs) parents first, so a row
// can be listed before its parent in the file. Rows whose parent never appears (unknown code
// or a cycle) are appended and reported by the reference check.
func orderSelfRef(spec CollectionSpec, rows []Row) []Row {
	var selfField string
	for _, r := range spec.Refs {
		if r.Target == spec.Name {
			selfField = r.Field
		}
	}
	if selfField == "" {
		return rows
	}
	byRef := map[string]bool{} // codes and names of rows already placed
	var ordered, rest []Row
	rest = append(rest, rows...)
	for progress := true; progress && len(rest) > 0; {
		progress = false
		var next []Row
		for _, r := range rest {
			parent := strings.ToLower(strings.TrimSpace(r[selfField]))
			if parent == "" || byRef[parent] {
				ordered = append(ordered, r)
				byRef[strings.ToLower(r.Code())] = true
				byRef[strings.ToLower(r.Name())] = true
				progress = true
			} else {
				next = append(next, r)
			}
		}
		rest = next
	}
	return append(ordered, rest...)
}

func checkAttr(a AttrSpec, v string) error {
	if v == "" {
		if a.Required {
			return fmt.Errorf("%s is required", a.Field)
		}
		return nil
	}
	if len(a.Choices) > 0 {
		for _, ch := range a.Choices {
			if ch == v {
				return nil
			}
		}
		return fmt.Errorf("%s %q is not one of %s", a.Field, v, strings.Join(a.Choices, ", "))
	}
	if a.Pattern != "" && !regexp.MustCompile(a.Pattern).MatchString(v) {
		return fmt.Errorf("%s %q does not match %s", a.Field, v, a.Pattern)
	}
	return nil
}

// link reads a junction collection: each row joins a child (Refs[0]) to a parent (Refs[1]).
func (c *Catalog) link(spec CollectionSpec, rows []Row) []error {
	var errs []error
	childRef, parentRef := spec.Refs[0], spec.Refs[1]
	seen := map[[2]*node]bool{}
	for i, row := range rows {
		where := fmt.Sprintf("%s[%d]", spec.Name, i)
		ends := make([]*node, 2)
		ok := true
		for k, ref := range []RefSpec{childRef, parentRef} {
			raw := strings.TrimSpace(row[ref.Field])
			if raw == "" {
				errs = append(errs, fmt.Errorf("%s: %s is required", where, ref.Field))
				ok = false
				continue
			}
			t, err := c.lookupRef(ref.Target, raw)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %s %q: %w", where, ref.Field, raw, err))
				ok = false
				continue
			}
			ends[k] = t
		}
		if !ok {
			continue
		}
		child, parent := ends[0], ends[1]
		if seen[[2]*node{child, parent}] {
			errs = append(errs, fmt.Errorf("%s: duplicate link %s %q ↔ %s %q", where, childRef.Field, child.id, parentRef.Field, parent.id))
			continue
		}
		seen[[2]*node{child, parent}] = true
		child.multi[parent.coll] = append(child.multi[parent.coll], parent)
		parent.children[child.coll] = append(parent.children[child.coll], child)
		c.counts[spec.Name]++
	}
	return errs
}

func specLabel(coll string) string {
	if s, ok := Spec(coll); ok {
		return s.Label
	}
	return coll
}

// lookupRef resolves a reference value to exactly one node, or explains why it cannot.
func (c *Catalog) lookupRef(coll, raw string) (*node, error) {
	if n := c.lookup(coll, raw); n != nil {
		return n, nil
	}
	if strings.HasPrefix(strings.TrimSpace(raw), "|") {
		return nil, fmt.Errorf("is not a valid %s path", specLabel(coll))
	}
	if len(c.byID[coll][key(raw)]) > 1 || len(c.byName[coll][key(raw)]) > 1 {
		return nil, fmt.Errorf("matches several %s nodes; use the path hash, e.g. %s", specLabel(coll), samplePath(c, coll, raw))
	}
	return nil, fmt.Errorf("does not match a %s code or unique name", specLabel(coll))
}

func samplePath(c *Catalog, coll, raw string) string {
	for _, idx := range []map[string][]*node{c.byID[coll], c.byName[coll]} {
		if ns := idx[key(raw)]; len(ns) > 0 && ns[0].path != "" {
			return ns[0].path
		}
	}
	return "|PRJ|APP|ENV|"
}

// lookup finds a node by path hash, then by code, then by name. Ambiguous matches return nil.
func (c *Catalog) lookup(coll, raw string) *node {
	if isEmpty(raw) {
		return nil
	}
	if strings.HasPrefix(strings.TrimSpace(raw), "|") {
		if n := c.byPath[key(raw)]; n != nil && n.coll == coll {
			return n
		}
		return nil
	}
	if ns := c.byID[coll][key(raw)]; len(ns) == 1 {
		return ns[0]
	}
	if ns := c.byName[coll][key(raw)]; len(ns) == 1 {
		return ns[0]
	}
	return nil
}

// lookupScoped finds a node of a scoped collection (region per CSP, zone per Region) by code,
// then by name, restricted to scope when one is known. Nodes without a scope also match.
func (c *Catalog) lookupScoped(coll, scope, raw string) *node {
	if isEmpty(raw) {
		return nil
	}
	if scope == "" {
		return c.lookup(coll, raw)
	}
	for _, idx := range []map[string][]*node{c.byID[coll], c.byName[coll]} {
		var hit *node
		n := 0
		for _, r := range idx[key(raw)] {
			if r.scope == scope || r.scope == "" {
				hit = r
				n++
			}
		}
		if n == 1 {
			return hit
		}
	}
	return nil
}

// lookupUsage matches a usage type exactly, then without an AWS region prefix
// (e.g. "APS1-BoxUsage:t3.medium" → "BoxUsage:t3.medium").
func (c *Catalog) lookupUsage(raw string) *node {
	if n := c.lookup(Usages, raw); n != nil {
		return n
	}
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "-"); i > 0 && i <= 5 && raw[:i] == strings.ToUpper(raw[:i]) {
		return c.lookup(Usages, raw[i+1:])
	}
	return nil
}

func key(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func isEmpty(s string) bool {
	switch key(s) {
	case "", "untagged", "unknown", "none", "null", "n/a", "-":
		return true
	default:
		return false
	}
}

func clean(s string) string {
	if isEmpty(s) {
		return ""
	}
	return strings.TrimSpace(s)
}

// --- Biz derivation and consistency checks -------------------------------------------------

// bizRef returns the Biz node a Project-hierarchy node references (its "must/can ref").
func bizRef(n *node) *node {
	spec, _ := Spec(n.coll)
	for _, ref := range spec.Refs {
		if t, _ := Spec(ref.Target); t.Hierarchy == HierBiz {
			return n.refs[ref.Field]
		}
	}
	return nil
}

// bizAncestor climbs a Biz node to the given level (0 = BusinessUnit); nil when it is above it.
func bizAncestor(b *node, level int) *node {
	levels := Levels(HierBiz)
	for b != nil {
		idx := indexOf(levels, b.coll)
		if idx == level {
			return b
		}
		if idx < level {
			return nil
		}
		spec, _ := Spec(b.coll)
		var next *node
		for _, ref := range spec.Refs {
			if ref.Kind == RefParent {
				next = b.refs[ref.Field]
			}
		}
		b = next
	}
	return nil
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// chain returns the nodes from the Project root down to n.
func chain(n *node) []*node {
	var out []*node
	for cur := n; cur != nil; {
		out = append([]*node{cur}, out...)
		spec, _ := Spec(cur.coll)
		var next *node
		for _, ref := range spec.Refs {
			if ref.Kind == RefParent {
				next = cur.refs[ref.Field]
			}
		}
		cur = next
	}
	return out
}

// bizVector returns the Biz node at each Biz level for a Project node: the level's own
// reference when set, else the ancestor of the deepest reference below it. Levels below the
// node's depth are nil.
func bizVector(n *node) []*node {
	ch := chain(n)
	explicit := make([]*node, len(ch))
	for k, p := range ch {
		explicit[k] = bizRef(p)
	}
	vec := make([]*node, len(Levels(HierBiz)))
	for j := range ch {
		if explicit[j] != nil {
			vec[j] = explicit[j]
			continue
		}
		for k := len(ch) - 1; k > j; k-- {
			if explicit[k] != nil {
				vec[j] = bizAncestor(explicit[k], j)
				break
			}
		}
	}
	return vec
}

// checkBiz warns when the Biz references along a Project path do not form a valid Biz path.
func (c *Catalog) checkBiz() {
	for _, coll := range Levels(HierProject) {
		for _, n := range c.ordered[coll] {
			ch := chain(n)
			d := len(ch) - 1
			own := bizRef(n)
			if own == nil {
				continue
			}
			for j := 0; j < d; j++ {
				up := bizRef(ch[j])
				if up == nil {
					continue
				}
				if anc := bizAncestor(own, j); anc != up {
					c.warnings = append(c.warnings, fmt.Sprintf(
						"Project path %s: %s %q is not under %s %q",
						n.path, specLabel(own.coll), own.id, specLabel(up.coll), up.id))
					break
				}
			}
		}
	}
}

// checkCloud warns when an Account's Environment belongs to an App its OUs do not host, and
// when an Account's LZ is not one of the LZs of the Projects it serves.
func (c *Catalog) checkCloud() {
	for _, acc := range c.ordered[Accounts] {
		apps := accountApps(acc)
		if len(apps) > 0 {
			hosted := map[*node]bool{}
			for _, app := range apps {
				hosted[app] = true
			}
			for _, env := range acc.multi[Environments] {
				if app := env.ref("app"); !hosted[app] {
					c.warnings = append(c.warnings, fmt.Sprintf(
						"Account %q is linked to Environment %s but its OUs do not host that App %q (%s)", acc.id, env.path, app.id, AppOUs))
				}
			}
		}
		lz := acc.ref("connection").ref("landing_zone")
		if lz == nil {
			lz = ouLZ(acc.ref("ou"))
		}
		if lz == nil {
			continue
		}
		served, hostedLZ := map[*node]bool{}, false
		for _, app := range apps {
			for _, l := range app.ref("project").multi[LandingZones] {
				hostedLZ = true
				served[l] = true
			}
		}
		if hostedLZ && !served[lz] {
			c.warnings = append(c.warnings, fmt.Sprintf(
				"Account %q is in LZ %q, which hosts none of the Projects its OUs serve", acc.id, lz.id))
		}
	}
}

// ProjectNode is one valid Project path prefix, with the Biz codes its levels reference.
// Codes are indexed by Project level (Project, App, Environment, Feature, Function) and
// Biz level (BusinessUnit, CostCenter, OwnerTeam, ProductLine, Customer); "" means none.
type ProjectNode struct {
	ID     int
	Level  int // 1 (Project) .. 5 (Function)
	Path   string
	Codes  [5]string
	Biz    [5]string
	System bool // a generated *-UNSPEC node
}

// ProjectNodes lists every valid Project path prefix plus the system-generated UNSPEC nodes
// (spec §4): under every Environment, a Feature and a Function whose Biz refs are
// PL-UNSPEC and CUST-UNSPEC. UNALLOC paths are produced by the resolver and never stored.
func (c *Catalog) ProjectNodes() []ProjectNode {
	var real []*node
	for _, coll := range Levels(HierProject) {
		real = append(real, c.ordered[coll]...)
	}
	sort.Slice(real, func(i, j int) bool { return key(real[i].path) < key(real[j].path) })
	var out []ProjectNode
	add := func(pn ProjectNode) {
		pn.ID = len(out) + 1
		out = append(out, pn)
	}
	for _, n := range real {
		ch := chain(n)
		pn := ProjectNode{Level: len(ch), Path: n.path}
		for i, p := range ch {
			pn.Codes[i] = p.id
		}
		for j, b := range bizVector(n) {
			if b != nil {
				pn.Biz[j] = b.id
			}
		}
		add(pn)
		if n.coll == Environments {
			feat := pn
			feat.Level, feat.System = 4, true
			feat.Codes[3] = ProjectPrefixes[3] + "-UNSPEC"
			feat.Path = n.path + feat.Codes[3] + "|"
			feat.Biz[3] = BizPrefixes[3] + "-UNSPEC"
			add(feat)
			fn := feat
			fn.Level = 5
			fn.Codes[4] = ProjectPrefixes[4] + "-UNSPEC"
			fn.Path = feat.Path + fn.Codes[4] + "|"
			fn.Biz[4] = BizPrefixes[4] + "-UNSPEC"
			add(fn)
		}
	}
	return out
}

// Resolve maps one cost row onto the hierarchies. This is the legacy two-way matcher; the
// attribution rules of spec §6 live in internal/attribution. Matching runs in two directions
// until nothing changes:
//
//   - Evidence: tag values that match the taxonomy, plus the row's own account, connection,
//     region, zone, service, usage type and resource id.
//   - Bottom-up: every resolved node fills its parents, most specific first (Resource →
//     ResourceGroup; Function → Feature → Environment → App → Project → BusinessUnit; Account →
//     Connection → LZ). A many-to-many link fills the other side only when it has one entry.
//   - Narrowing: an empty organizational level is filled when every resolved neighbor's
//     candidates intersect to exactly one node — a parent's children (top-down) and a child's
//     many-to-many parents. When several candidates remain but all share a parent, that parent
//     is filled. Catalog levels, NoNarrow levels and levels the row already carries a value
//     for are never filled this way.
//
// A slot is never overwritten once filled, so tags that match the taxonomy win over derived
// values. A tag that matches nothing does not block derivation; it is kept only when the slot
// stays empty. A tag that is ambiguous by name (e.g. "Production") is matched again among the
// children of an already-resolved parent.
func (c *Catalog) Resolve(in Input) Output { return c.resolve(in, false) }

// ResolveCloud resolves only the Cloud side of a row (account, connection, LZ, CSP, region,
// zone, service, usage type, resource and its group). Tags are ignored and nothing is derived
// from the Project or Biz hierarchies, so the result is safe to use as attribution evidence.
func (c *Catalog) ResolveCloud(in Input) Output { return c.resolve(in, true) }

func (c *Catalog) resolve(in Input, cloudOnly bool) Output {
	r := &resolver{c: c, slots: map[string]*node{}, pending: map[string]string{}, cloudOnly: cloudOnly}
	if cloudOnly {
		in.Project, in.App, in.Environment, in.Feature, in.Function = "", "", "", "", ""
		in.ServiceCategory, in.BusinessUnit, in.CostCenter, in.OwnerTeam, in.ProductLine, in.Customer = "", "", "", "", "", ""
	}

	tags := []struct{ coll, raw string }{
		{Projects, in.Project}, {Apps, in.App}, {Environments, in.Environment},
		{Features, in.Feature}, {Functions, in.Function},
		{ServiceCats, in.ServiceCategory}, {BusinessUnits, in.BusinessUnit},
		{CostCenters, in.CostCenter}, {OwnerTeams, in.OwnerTeam},
		{ProductLines, in.ProductLine}, {Customers, in.Customer},
	}
	for _, t := range tags {
		if n := c.lookup(t.coll, t.raw); n != nil {
			r.set(n)
		} else if !isEmpty(t.raw) {
			r.pending[t.coll] = clean(t.raw)
		}
	}
	// The primary/secondary tag can name a different level than the slot it was sensed in.
	if _, ok := r.pending[Projects]; ok {
		r.set(c.lookup(BusinessUnits, in.Project))
	}
	if _, ok := r.pending[Apps]; ok {
		r.set(c.lookup(Environments, in.App))
		r.set(c.lookup(CostCenters, in.App))
	}

	account := c.lookup(Accounts, in.AccountID)
	if account == nil {
		account = c.lookup(Accounts, in.AccountName)
	}
	r.set(account)
	r.set(c.lookup(Connections, in.ConnectionID))
	r.up()
	r.set(ouLZ(r.slots[OUs])) // an account without a connection reaches its LZ through its OUs

	// The row's CSP is its own value, else its LZ's, else its Region's.
	csp := clean(in.CSP)
	if csp == "" {
		csp = r.slots[LandingZones].attr("csp")
	}
	region := c.lookupScoped(Regions, key(csp), in.Region)
	r.set(region)
	if csp == "" {
		csp = region.attr("csp")
	}
	zone := c.lookupScoped(Zones, r.regionScope(region), in.Zone)
	if zone == nil && region != nil {
		// A zone of another region is still the row's zone; it is flagged below.
		zone = c.lookup(Zones, in.Zone)
	}
	r.set(zone)
	r.set(c.lookup(Services, in.Service))
	r.set(c.lookupUsage(in.UsageType))
	r.set(c.lookup(Resources, in.ResourceID))

	// Levels the row carries its own value for are never filled top-down.
	given := map[string]bool{
		Accounts:    !isEmpty(in.AccountID) || !isEmpty(in.AccountName),
		Connections: !isEmpty(in.ConnectionID),
		Regions:     !isEmpty(in.Region),
		Zones:       !isEmpty(in.Zone),
		Services:    !isEmpty(in.Service),
		Usages:      !isEmpty(in.UsageType),
		Resources:   !isEmpty(in.ResourceID),
	}
	for {
		changed := r.up()
		if r.matchPendingInScope() {
			changed = true
		}
		if !changed && (cloudOnly || !r.narrow(given)) {
			break
		}
	}
	if csp == "" {
		csp = r.slots[Regions].attr("csp")
	}

	out := Output{
		BusinessUnit:    r.slot(BusinessUnits),
		CostCenter:      r.slot(CostCenters),
		OwnerTeam:       r.slot(OwnerTeams),
		ProductLine:     r.slot(ProductLines),
		Customer:        r.slot(Customers),
		Project:         r.slot(Projects),
		App:             r.slot(Apps),
		Environment:     r.slot(Environments),
		Feature:         r.slot(Features),
		Function:        r.slot(Functions),
		LZ:              r.slot(LandingZones),
		Connection:      slotOf(r.slots[Connections], in.ConnectionID),
		OU:              r.slot(OUs),
		ServiceCategory: r.slot(ServiceCats),
		ServiceUsage:    r.slot(ServiceUsages),
		ResourceGroup:   r.slot(ResourceGroups),
		Region:          slotOf(r.slots[Regions], in.Region),
		Zone:            slotOf(r.slots[Zones], in.Zone),
		Service:         slotOf(r.slots[Services], in.Service),
	}
	if csp != "" {
		out.CSP = Slot{ID: csp, Name: CSPName(csp)}
	}
	if n := r.slots[Accounts]; n != nil {
		out.Account = n.slot()
	} else {
		out.Account = Slot{ID: clean(in.AccountID), Name: clean(in.AccountName)}
	}
	if n := r.slots[Usages]; n != nil {
		out.Usage = n.slot()
	} else {
		out.Usage = Slot{ID: clean(in.UsageType)}
	}
	if n := r.slots[Resources]; n != nil {
		out.Resource = n.slot()
	} else {
		out.Resource = Slot{ID: clean(in.ResourceID)}
	}

	lz := r.slots[LandingZones]
	out.BillingCurrency = lz.attr("currency")
	if rc, lc := r.slots[Regions].attr("csp"), lz.attr("csp"); rc != "" && lc != "" && rc != lc {
		out.RegionCSPMismatch = true
	}
	if z, rg := r.slots[Zones], r.slots[Regions]; z != nil && rg != nil && z.ref("region") != nil && z.ref("region") != rg {
		out.ZoneRegionMismatch = true
	}
	return out
}

func (n *node) attr(field string) string {
	if n == nil {
		return ""
	}
	return n.attrs[field]
}

// regionScope is the uniqueness scope of zones in region ("" when the region is unknown).
func (r *resolver) regionScope(region *node) string {
	if region == nil {
		return ""
	}
	return region.uid
}

// resolver holds the resolved node per collection for one cost row.
type resolver struct {
	c       *Catalog
	slots   map[string]*node
	pending map[string]string // unmatched tag values by collection
	// cloudOnly keeps bottom-up derivation inside the Cloud hierarchy.
	cloudOnly bool
}

// allowed reports whether the resolver may fill a node of coll.
func (r *resolver) allowed(coll string) bool {
	if !r.cloudOnly {
		return true
	}
	spec, _ := Spec(coll)
	return spec.Hierarchy == HierCloud
}

// set fills n's collection slot if it is still empty.
func (r *resolver) set(n *node) bool {
	if n == nil || r.slots[n.coll] != nil || !r.allowed(n.coll) {
		return false
	}
	r.slots[n.coll] = n
	return true
}

// up fills parents from resolved nodes, most specific level first, until nothing changes.
func (r *resolver) up() bool {
	filled := false
	for changed := true; changed; {
		changed = false
		for i := len(Schema) - 1; i >= 0; i-- {
			n := r.slots[Schema[i].Name]
			if n == nil {
				continue
			}
			for _, ref := range Schema[i].Refs {
				if r.set(n.refs[ref.Field]) {
					changed, filled = true, true
				}
			}
			for _, parents := range n.multi {
				if len(parents) == 1 && r.set(parents[0]) {
					changed, filled = true, true
				}
			}
		}
	}
	return filled
}

// narrow fills one empty organizational level from the candidates offered by every resolved
// node, intersected: a resolved parent offers its children there (top-down) and a resolved
// child offers its many-to-many parents there. One candidate fills the level. Several
// candidates that all share the same parent fill that parent instead, so bottom-up continues
// through an ambiguous level. It returns after the first fill so bottom-up runs again.
func (r *resolver) narrow(given map[string]bool) bool {
	for _, target := range Schema {
		if target.Catalog || target.Junction || target.NoNarrow || given[target.Name] || r.slots[target.Name] != nil {
			continue
		}
		cand := r.candidates(target.Name)
		if len(cand) == 1 {
			return r.set(cand[0])
		}
		if len(cand) > 1 {
			for _, ref := range target.Refs {
				if r.set(commonRef(cand, ref.Field)) {
					return true
				}
			}
		}
	}
	return false
}

// candidates intersects what every resolved node offers for coll, in a stable order.
// It returns nil when no resolved node constrains coll.
func (r *resolver) candidates(coll string) []*node {
	var cand []*node
	constrained := false
	for _, spec := range Schema {
		n := r.slots[spec.Name]
		// Catalog nodes describe the row, they do not identify who owns it: a Region is only
		// weak evidence for an Environment (spec §3.2).
		if n == nil || spec.Catalog {
			continue
		}
		offer := n.children[coll]
		if len(offer) == 0 {
			offer = n.multi[coll]
		}
		if len(offer) == 0 {
			continue
		}
		if !constrained {
			cand, constrained = append([]*node(nil), offer...), true
			continue
		}
		keep := cand[:0]
		for _, c := range cand {
			for _, o := range offer {
				if c == o {
					keep = append(keep, c)
					break
				}
			}
		}
		cand = keep
	}
	return cand
}

// commonRef returns the node every candidate references through field, or nil.
func commonRef(cand []*node, field string) *node {
	var common *node
	for _, c := range cand {
		p := c.refs[field]
		if p == nil || (common != nil && p != common) {
			return nil
		}
		common = p
	}
	return common
}

// matchPendingInScope matches unmatched tag values among the children of resolved parents,
// so a name like "Production" resolves once its App is known.
func (r *resolver) matchPendingInScope() bool {
	changed := false
	for coll, raw := range r.pending {
		if r.slots[coll] != nil {
			continue
		}
		for _, spec := range Schema {
			parent := r.slots[spec.Name]
			if parent == nil {
				continue
			}
			var hit *node
			n := 0
			for _, child := range parent.children[coll] {
				if key(child.id) == key(raw) || key(child.name) == key(raw) {
					hit = child
					n++
				}
			}
			if n == 1 && r.set(hit) {
				changed = true
				break
			}
		}
	}
	return changed
}

// slot returns the resolved slot, or the unmatched tag value as a name.
func (r *resolver) slot(coll string) Slot {
	if n := r.slots[coll]; n != nil {
		return n.slot()
	}
	return Slot{Name: r.pending[coll]}
}

// slotOf returns the node's slot, or the raw sensed value as a name when unmatched.
func slotOf(n *node, raw string) Slot {
	if n != nil {
		return n.slot()
	}
	return Slot{Name: clean(raw)}
}
