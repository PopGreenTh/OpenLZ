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

// Resolve maps one cost row onto the hierarchies. Tag values already present are kept;
// empty levels are filled from the taxonomy relations.
func (c *Catalog) Resolve(in Input) Output {
	// Explicit tags first.
	project := c.lookup(Projects, in.Project)
	app := c.lookup(Apps, in.App)
	env := c.lookup(Environments, in.Environment)
	cat := c.lookup(ServiceCats, in.ServiceCategory)
	bu := c.lookup(BusinessUnits, in.BusinessUnit)
	cc := c.lookup(CostCenters, in.CostCenter)
	owner := c.lookup(Owners, in.Owner)
	cf := c.lookup(CustomerFuncs, in.CustomerFunction)

	// A primary/secondary tag can name a different level than the slot it was sensed in
	// (e.g. the Project tag holds a business unit). Use it when it is unambiguous.
	if project == nil && !isEmpty(in.Project) && bu == nil {
		bu = c.lookup(BusinessUnits, in.Project)
	}
	if app == nil && !isEmpty(in.App) {
		if env == nil {
			env = c.lookup(Environments, in.App)
		}
		if cc == nil {
			cc = c.lookup(CostCenters, in.App)
		}
	}

	// Project hierarchy: Account → Environment → App → Project.
	account := c.lookup(Accounts, in.AccountID)
	if account == nil {
		account = c.lookup(Accounts, in.AccountName)
	}
	if env == nil && isEmpty(in.Environment) {
		env = account.ref("environment")
	}
	if app == nil && isEmpty(in.App) {
		app = env.ref("app")
	}
	if project == nil && isEmpty(in.Project) {
		project = app.ref("project")
	}

	// CSP / Region.
	csp := c.lookup(CSPs, in.CSP)
	region := c.lookupRegion(csp, in.Region)
	if csp == nil {
		csp = region.ref("csp")
	}

	// ServiceCat / Service / ServiceUsage / Usage.
	usage := c.lookupUsage(in.UsageType)
	serviceUsage := usage.ref("service_usage")
	service := c.lookup(Services, in.Service)
	if service == nil && isEmpty(in.Service) {
		service = serviceUsage.ref("service")
	}
	if cat == nil && isEmpty(in.ServiceCategory) {
		cat = service.ref("service_cat")
	}

	// ResourceGroup / Resource → CustomerFunc → Owner.
	resource := c.lookup(Resources, in.ResourceID)
	group := resource.ref("resource_group")
	if cf == nil && isEmpty(in.CustomerFunction) {
		cf = group.ref("customer_func")
	}
	if owner == nil && isEmpty(in.Owner) {
		owner = cf.ref("owner")
	}

	// Business hierarchy: App must ref CostCenter, Project must ref BusinessUnit.
	if cc == nil && isEmpty(in.CostCenter) {
		cc = app.ref("cost_center")
		if cc == nil {
			cc = owner.ref("cost_center")
		}
	}
	if bu == nil && isEmpty(in.BusinessUnit) {
		bu = project.ref("business_unit")
		if bu == nil {
			bu = cc.ref("business_unit")
		}
	}
	if owner == nil && isEmpty(in.Owner) {
		owner = cc.onlyChild(Owners)
	}
	if cf == nil && isEmpty(in.CustomerFunction) {
		cf = owner.onlyChild(CustomerFuncs)
	}

	out := Output{
		BusinessUnit:     slotOf(bu, in.BusinessUnit),
		CostCenter:       slotOf(cc, in.CostCenter),
		Owner:            slotOf(owner, in.Owner),
		CustomerFunction: slotOf(cf, in.CustomerFunction),
		Project:          slotOf(project, in.Project),
		App:              slotOf(app, in.App),
		Environment:      slotOf(env, in.Environment),
		CSP:              slotOf(csp, in.CSP),
		Region:           slotOf(region, in.Region),
		ServiceCategory:  slotOf(cat, in.ServiceCategory),
		ServiceUsage:     serviceUsage.slot(),
		ResourceGroup:    group.slot(),
	}
	if account != nil {
		out.Account = account.slot()
	} else {
		out.Account = Slot{ID: clean(in.AccountID), Name: clean(in.AccountName)}
	}
	out.Service = slotOf(service, in.Service)
	if usage != nil {
		out.Usage = usage.slot()
	} else {
		out.Usage = Slot{ID: clean(in.UsageType)}
	}
	if resource != nil {
		out.Resource = resource.slot()
	} else {
		out.Resource = Slot{ID: clean(in.ResourceID)}
	}
	return out
}

// slotOf returns the node's slot, or the raw sensed value as a name when unmatched.
func slotOf(n *node, raw string) Slot {
	if n != nil {
		return n.slot()
	}
	return Slot{Name: clean(raw)}
}
