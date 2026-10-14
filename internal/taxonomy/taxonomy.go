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
		for i, row := range snap.Rows(spec.Name) {
			where := fmt.Sprintf("%s[%d]", spec.Name, i)
			if row.ID() == "" || row.Name() == "" {
				errs = append(errs, fmt.Errorf("%s: id and name are required (id=%q name=%q)", where, row.ID(), row.Name()))
				continue
			}
			where = fmt.Sprintf("%s %q", spec.Label, row.ID())
			n := &node{coll: spec.Name, id: row.ID(), name: row.Name(), refs: map[string]*node{}, children: map[string][]*node{}}

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
//   - Bottom-up: every resolved node fills its parents (Resource → ResourceGroup → CustomerFunc
//     → Owner → CostCenter → BusinessUnit; Account → Environment → App → Project → BusinessUnit).
//   - Top-down: a resolved node fills an empty child level when it has exactly one child there
//     (BusinessUnit → Project → App → Environment → Account). Catalog levels and levels the row
//     already carries a value for are never filled top-down.
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
		if !changed && !r.down(given) {
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
		}
	}
	return filled
}

// down fills one empty organizational level from a resolved parent with exactly one child
// there. It returns after the first fill so bottom-up can run again.
func (r *resolver) down(given map[string]bool) bool {
	for _, parentSpec := range Schema {
		parent := r.slots[parentSpec.Name]
		if parent == nil {
			continue
		}
		for _, childSpec := range Schema {
			if childSpec.Catalog || given[childSpec.Name] || r.slots[childSpec.Name] != nil {
				continue
			}
			if r.set(parent.onlyChild(childSpec.Name)) {
				return true
			}
		}
	}
	return false
}

// matchPendingInScope matches unmatched tag values among the children of resolved parents,
// so a name like "Production" resolves once its App is known.
func (r *resolver) matchPendingInScope() bool {
	changed := false
	for coll, raw := range r.pending {
		if r.slots[coll] != nil {
			continue
		}
		spec, _ := Spec(coll)
		for _, ref := range spec.Refs {
			parent := r.slots[ref.Target]
			if parent == nil {
				continue
			}
			var hit *node
			n := 0
			for _, child := range parent.children[coll] {
				if child.refs[ref.Field] == parent && (key(child.id) == key(raw) || key(child.name) == key(raw)) {
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
