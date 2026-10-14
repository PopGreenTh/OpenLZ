// Package taxonomy maps sensed cost rows onto the business and project cost hierarchies.
//
// Business hierarchy: BusinessUnit → CostCenter → Owner → CustomerFunc.
// Project hierarchy:  Project → App → Environment/Account → CSP/Region → ServiceCat/Service
// → ServiceUsage/Usage → ResourceGroup/Resource.
//
// The taxonomy is edited in Directus and stored as one flat JSON snapshot per tenant
// (see Schema for the collections and the relations between them).
package taxonomy

import (
	"errors"
	"fmt"
	"strings"
)

// Slot is an identifier and its display name.
type Slot struct {
	ID   string
	Name string
}

// Input is the state of one cost row. Empty, "Untagged", and "Unknown" values are unfilled.
type Input struct {
	AccountID   string
	AccountName string
	CSP         string
	Region      string
	Service     string
	UsageType   string
	ResourceID  string

	// Tag values sensed from the cloud provider. A value that is present is kept as-is.
	Project          string
	App              string
	Environment      string
	ServiceCategory  string
	BusinessUnit     string
	CostCenter       string
	Owner            string
	CustomerFunction string
}

// Output is the resolved hierarchy for one cost row.
type Output struct {
	BusinessUnit     Slot
	CostCenter       Slot
	Owner            Slot
	CustomerFunction Slot

	Project         Slot
	App             Slot
	Environment     Slot
	Account         Slot
	CSP             Slot
	Region          Slot
	ServiceCategory Slot
	Service         Slot
	ServiceUsage    Slot
	Usage           Slot
	ResourceGroup   Slot
	Resource        Slot
}

type node struct {
	coll     string
	id, name string
	scope    string
	refs     map[string]*node
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

// onlyChild returns the single child in coll, or nil when there are zero or several.
func (n *node) onlyChild(coll string) *node {
	if n == nil || len(n.children[coll]) != 1 {
		return nil
	}
	return n.children[coll][0]
}

// Catalog is a validated taxonomy snapshot.
type Catalog struct {
	Tenant   string
	byID     map[string]map[string][]*node
	byName   map[string]map[string][]*node
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
		Tenant: snap.Tenant,
		byID:   map[string]map[string][]*node{},
		byName: map[string]map[string][]*node{},
		counts: map[string]int{},
	}
	var errs []error
	for _, spec := range Schema {
		c.byID[spec.Name] = map[string][]*node{}
		c.byName[spec.Name] = map[string][]*node{}
		if spec.Junction {
			errs = append(errs, c.link(spec, snap.Rows(spec.Name))...)
			continue
		}
		for i, row := range snap.Rows(spec.Name) {
			where := fmt.Sprintf("%s[%d]", spec.Name, i)
			if row.ID() == "" || row.Name() == "" {
				errs = append(errs, fmt.Errorf("%s: id and name are required (id=%q name=%q)", where, row.ID(), row.Name()))
				continue
			}
			where = fmt.Sprintf("%s %q", spec.Label, row.ID())
			n := &node{coll: spec.Name, id: row.ID(), name: row.Name(), refs: map[string]*node{}, multi: map[string][]*node{}, children: map[string][]*node{}}

			for _, ref := range spec.Refs {
				raw := strings.TrimSpace(row[ref.Field])
				if raw == "" {
					if ref.Required() {
						errs = append(errs, fmt.Errorf("%s: %s is required", where, ref.Field))
					}
					continue
				}
				target := c.lookup(ref.Target, raw)
				if target == nil {
					errs = append(errs, fmt.Errorf("%s: %s %q does not match a %s id or unique name", where, ref.Field, raw, specLabel(ref.Target)))
					continue
				}
				n.refs[ref.Field] = target
				target.children[spec.Name] = append(target.children[spec.Name], n)
			}
			if spec.ScopeRef != "" {
				if s := n.refs[spec.ScopeRef]; s != nil {
					n.scope = key(s.id)
				}
			}

			for _, other := range c.byID[spec.Name][key(n.id)] {
				if other.scope == n.scope {
					scope := ""
					if spec.ScopeRef != "" {
						scope = fmt.Sprintf(" within %s %q", spec.ScopeRef, n.scope)
					}
					errs = append(errs, fmt.Errorf("%s: duplicate id%s", where, scope))
				}
			}
			c.byID[spec.Name][key(n.id)] = append(c.byID[spec.Name][key(n.id)], n)
			c.byName[spec.Name][key(n.name)] = append(c.byName[spec.Name][key(n.name)], n)
			c.counts[spec.Name]++
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	for _, envs := range c.byID[Environments] {
		for _, env := range envs {
			if len(env.multi[Apps]) == 0 {
				c.warnings = append(c.warnings, fmt.Sprintf("Environment %q is not linked to any App (%s)", env.id, AppEnvironments))
			}
		}
	}

	// An App's cost center should roll up to the same business unit as its project.
	for _, apps := range c.byID[Apps] {
		for _, app := range apps {
			projBU := app.ref("project").ref("business_unit")
			ccBU := app.ref("cost_center").ref("business_unit")
			if projBU != nil && ccBU != nil && projBU != ccBU {
				c.warnings = append(c.warnings, fmt.Sprintf(
					"App %q: project %q is in business unit %q but cost center %q is in business unit %q",
					app.id, app.ref("project").id, projBU.id, app.ref("cost_center").id, ccBU.id))
			}
		}
	}
	return c, nil
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
			if ends[k] = c.lookup(ref.Target, raw); ends[k] == nil {
				errs = append(errs, fmt.Errorf("%s: %s %q does not match a %s id or unique name", where, ref.Field, raw, specLabel(ref.Target)))
				ok = false
			}
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

// lookup finds a node by id, then by name. Ambiguous matches return nil.
func (c *Catalog) lookup(coll, raw string) *node {
	if isEmpty(raw) {
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

// lookupRegion finds a region by id or name, scoped to a CSP when one is known.
func (c *Catalog) lookupRegion(csp *node, raw string) *node {
	if isEmpty(raw) {
		return nil
	}
	if csp == nil {
		return c.lookup(Regions, raw)
	}
	for _, idx := range []map[string][]*node{c.byID[Regions], c.byName[Regions]} {
		var hit *node
		n := 0
		for _, r := range idx[key(raw)] {
			if r.scope == key(csp.id) {
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

// Resolve maps one cost row onto the hierarchies. Matching runs in two directions until
// nothing changes:
//
//   - Evidence: tag values that match the taxonomy, plus the row's own account, CSP, region,
//     service, usage type and resource id.
//   - Bottom-up: every resolved node fills its parents, most specific first (Resource →
//     ResourceGroup → CustomerFunc → Owner → CostCenter → BusinessUnit; Resource → Environment;
//     Account → Environment → App → Project → BusinessUnit). An Environment fills its App only
//     when it is linked to exactly one.
//   - Narrowing: an empty organizational level is filled when every resolved neighbor's
//     candidates intersect to exactly one node — a parent's children (top-down, BusinessUnit →
//     Project → App → Environment → Account) and a child's many-to-many parents (an Environment
//     shared by several Apps). When several candidates remain but all share a parent (both
//     Apps of a shared Environment are in one Project), that parent is filled. Catalog levels
//     and levels the row already carries a value for are never filled this way.
//
// A slot is never overwritten once filled, so tags that match the taxonomy win over derived
// values. A tag that matches nothing does not block derivation; it is kept only when the slot
// stays empty. A tag that is ambiguous by name (e.g. "Production") is matched again among the
// children of an already-resolved parent.
func (c *Catalog) Resolve(in Input) Output {
	r := &resolver{c: c, slots: map[string]*node{}, pending: map[string]string{}}

	tags := []struct{ coll, raw string }{
		{Projects, in.Project}, {Apps, in.App}, {Environments, in.Environment},
		{ServiceCats, in.ServiceCategory}, {BusinessUnits, in.BusinessUnit},
		{CostCenters, in.CostCenter}, {Owners, in.Owner}, {CustomerFuncs, in.CustomerFunction},
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
	csp := c.lookup(CSPs, in.CSP)
	r.set(account)
	r.set(csp)
	r.set(c.lookupRegion(csp, in.Region))
	r.set(c.lookup(Services, in.Service))
	r.set(c.lookupUsage(in.UsageType))
	r.set(c.lookup(Resources, in.ResourceID))

	// Levels the row carries its own value for are never filled top-down.
	given := map[string]bool{
		Accounts:  !isEmpty(in.AccountID) || !isEmpty(in.AccountName),
		CSPs:      !isEmpty(in.CSP),
		Regions:   !isEmpty(in.Region),
		Services:  !isEmpty(in.Service),
		Usages:    !isEmpty(in.UsageType),
		Resources: !isEmpty(in.ResourceID),
	}
	for {
		changed := r.up()
		if r.matchPendingInScope() {
			changed = true
		}
		if !changed && !r.narrow(given) {
			break
		}
	}

	out := Output{
		BusinessUnit:     r.slot(BusinessUnits),
		CostCenter:       r.slot(CostCenters),
		Owner:            r.slot(Owners),
		CustomerFunction: r.slot(CustomerFuncs),
		Project:          r.slot(Projects),
		App:              r.slot(Apps),
		Environment:      r.slot(Environments),
		ServiceCategory:  r.slot(ServiceCats),
		ServiceUsage:     r.slot(ServiceUsages),
		ResourceGroup:    r.slot(ResourceGroups),
		CSP:              slotOf(r.slots[CSPs], in.CSP),
		Region:           slotOf(r.slots[Regions], in.Region),
		Service:          slotOf(r.slots[Services], in.Service),
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
	return out
}

// resolver holds the resolved node per collection for one cost row.
type resolver struct {
	c       *Catalog
	slots   map[string]*node
	pending map[string]string // unmatched tag values by collection
}

// set fills n's collection slot if it is still empty.
func (r *resolver) set(n *node) bool {
	if n == nil || r.slots[n.coll] != nil {
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
// child offers its many-to-many parents there (bottom-up, e.g. an Environment shared by
// several Apps). One candidate fills the level. Several candidates that all share the same
// parent fill that parent instead, so bottom-up continues through an ambiguous level (a shared
// Environment whose Apps all belong to one Project still resolves the Project and BU).
// It returns after the first fill so bottom-up runs again.
func (r *resolver) narrow(given map[string]bool) bool {
	for _, target := range Schema {
		if target.Catalog || target.Junction || given[target.Name] || r.slots[target.Name] != nil {
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
		if n == nil {
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
