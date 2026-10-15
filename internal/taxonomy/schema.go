package taxonomy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Collection names. They are shared by the JSON snapshot and the Directus collections.
const (
	// Biz hierarchy.
	BusinessUnits = "business_units"
	CostCenters   = "cost_centers"
	OwnerTeams    = "owner_teams"
	ProductLines  = "product_lines"
	Customers     = "customers"

	// Project hierarchy.
	Projects     = "projects"
	Apps         = "apps"
	Environments = "environments"
	Features     = "features"
	Functions    = "functions"

	// Cloud hierarchy.
	LandingZones   = "landing_zones"
	Connections    = "connections"
	OUs            = "ous"
	Accounts       = "accounts"
	Regions        = "regions"
	Zones          = "zones"
	ServiceCats    = "service_cats"
	Services       = "services"
	ServiceUsages  = "service_usages"
	Usages         = "usages"
	ResourceGroups = "resource_groups"
	Resources      = "resources"

	// Cloud <-> Project links (many-to-many junctions).
	ProjectLZs          = "project_lzs"
	AppOUs              = "app_ous"
	AccountEnvironments = "account_environments"

	// Aliases map invalid tag values to the correct code (spec §3.3).
	Aliases = "aliases"
)

// Hierarchy names for CollectionSpec.Hierarchy.
const (
	HierBiz     = "biz"
	HierProject = "project"
	HierCloud   = "cloud"
	HierMeta    = "meta" // not a cost level: data about the other levels
)

// AliasLevels names the levels a tag alias can repair: the five Project levels, then the five
// Biz levels, both top level first.
var AliasLevels = []string{
	"project", "app", "environment", "feature", "function",
	"business_unit", "cost_center", "owner_team", "product_line", "customer",
}

// Ref kinds describe what a reference field means in the cost hierarchy.
const (
	// RefParent places a node under its parent level in the same hierarchy (required).
	RefParent = "parent"
	// RefMust is a required cross-hierarchy reference ("must ref to").
	RefMust = "must"
	// RefCan is an optional reference ("can ref to" / "can be mapped to").
	RefCan = "can"
)

// CSPChoices is the fixed list of cloud service providers. CSP is an attribute of an LZ and
// of a Region, never a level of its own.
var CSPChoices = []string{"AWS", "Azure", "GCP", "OCI"}

// cspNames gives the display name of each CSP code.
var cspNames = map[string]string{
	"AWS":   "Amazon Web Services",
	"Azure": "Microsoft Azure",
	"GCP":   "Google Cloud Platform",
	"OCI":   "Oracle Cloud Infrastructure",
}

// CSPName returns the display name for a CSP code (the code itself when unknown).
func CSPName(code string) string {
	if n, ok := cspNames[code]; ok {
		return n
	}
	return code
}

// System-node code prefixes per level. A path level padded because the evidence could not
// reach it is "<PREFIX>-UNSPEC"; a level with nothing resolved at all is "<PREFIX>-UNALLOC".
var (
	ProjectPrefixes = []string{"PRJ", "APP", "ENV", "FEAT", "FUNC"}
	BizPrefixes     = []string{"BU", "CC", "TEAM", "PL", "CUST"}
)

// RefSpec is a reference field on a collection that holds the code of a row in Target.
type RefSpec struct {
	Field  string
	Target string
	Kind   string
}

// Required reports whether every row must set this reference.
func (r RefSpec) Required() bool { return r.Kind == RefParent || r.Kind == RefMust }

// AttrSpec is a plain string attribute of a collection (not a reference).
type AttrSpec struct {
	Field    string
	Required bool
	Choices  []string // when set, the value must be one of these
	Pattern  string   // when set, the value must match this regular expression
}

// CollectionSpec describes one taxonomy level.
type CollectionSpec struct {
	Name      string
	Label     string
	Hierarchy string // HierBiz, HierProject or HierCloud
	Refs      []RefSpec
	Attrs     []AttrSpec
	// ScopeField names a ref or attribute whose value scopes code uniqueness: environments are
	// unique per App, regions per CSP, zones per Region. Empty means unique within the level.
	ScopeField string
	// Catalog levels describe the cost row itself (where, what, which resource). They are
	// only filled from the row's own data, never top-down from an organizational parent.
	Catalog bool
	// NoNarrow levels are never guessed from a single remaining candidate: the evidence must
	// name them (Feature and Function have no Cloud counterpart).
	NoNarrow bool
	// Junction collections link two levels many-to-many. Their rows have no code or name;
	// Refs[0] is the child side and Refs[1] the parent side (e.g. account ↔ app).
	Junction bool
}

// Schema lists every collection in dependency order: a collection only references
// collections that appear before it. It is the single definition of the taxonomy.
//
//	Biz:     BusinessUnit → CostCenter → OwnerTeam → ProductLine → Customer
//	Project: Project → App → Environment → Feature → Function
//	Cloud:   LZ/Connection → OU (nested) → Account → Region/Zone → ServiceCat/Service
//	         → ServiceUsage/UsageType → ResourceGroup/Resource
//
// Project → Biz (one level for one level):
//
//	Project must ref BusinessUnit, App must ref CostCenter, Environment can ref OwnerTeam,
//	Feature can ref ProductLine, Function can ref Customer
//
// Cloud ↔ Project (many-to-many, aligned levels):
//
//	Project ↔ LZ (project_lzs), App ↔ OU (app_ous: an OU, nested, hosts Apps; an Account's
//	Apps come from the nearest OU above it that has links), Account ↔ Environment
//	(account_environments: a dedicated Account lives in one Environment); a Resource or
//	ResourceGroup can be mapped to a Project path
//	at any depth (attribute project_path, written as a path hash such as |PRJ|APP|ENV|)
var Schema = []CollectionSpec{
	{Name: BusinessUnits, Label: "Business Unit", Hierarchy: HierBiz},
	{Name: CostCenters, Label: "Cost Center", Hierarchy: HierBiz,
		Refs: []RefSpec{{Field: "business_unit", Target: BusinessUnits, Kind: RefParent}}},
	{Name: OwnerTeams, Label: "Owner Team", Hierarchy: HierBiz,
		Refs: []RefSpec{{Field: "cost_center", Target: CostCenters, Kind: RefParent}}},
	{Name: ProductLines, Label: "Product Line", Hierarchy: HierBiz,
		Refs: []RefSpec{{Field: "owner_team", Target: OwnerTeams, Kind: RefParent}}},
	{Name: Customers, Label: "Customer", Hierarchy: HierBiz,
		Refs: []RefSpec{{Field: "product_line", Target: ProductLines, Kind: RefParent}}},

	{Name: Projects, Label: "Project", Hierarchy: HierProject,
		Refs: []RefSpec{{Field: "business_unit", Target: BusinessUnits, Kind: RefMust}}},
	{Name: Apps, Label: "App", Hierarchy: HierProject,
		Refs: []RefSpec{
			{Field: "project", Target: Projects, Kind: RefParent},
			{Field: "cost_center", Target: CostCenters, Kind: RefMust},
		}},
	{Name: Environments, Label: "Environment", Hierarchy: HierProject, ScopeField: "app",
		Refs: []RefSpec{
			{Field: "app", Target: Apps, Kind: RefParent},
			{Field: "owner_team", Target: OwnerTeams, Kind: RefCan},
		}},
	{Name: Features, Label: "Feature", Hierarchy: HierProject, ScopeField: "environment", NoNarrow: true,
		Refs: []RefSpec{
			{Field: "environment", Target: Environments, Kind: RefParent},
			{Field: "product_line", Target: ProductLines, Kind: RefCan},
		}},
	{Name: Functions, Label: "Function", Hierarchy: HierProject, ScopeField: "feature", NoNarrow: true,
		Refs: []RefSpec{
			{Field: "feature", Target: Features, Kind: RefParent},
			{Field: "customer", Target: Customers, Kind: RefCan},
		}},

	{Name: LandingZones, Label: "Landing Zone", Hierarchy: HierCloud,
		Attrs: []AttrSpec{
			{Field: "csp", Required: true, Choices: CSPChoices},
			{Field: "currency", Required: true, Pattern: "^[A-Z]{3}$"},
		}},
	{Name: Connections, Label: "Connection", Hierarchy: HierCloud,
		Refs: []RefSpec{{Field: "landing_zone", Target: LandingZones, Kind: RefParent}}},
	// OUs nest: a root OU names its landing_zone, every other OU names its parent. A Target equal
	// to the collection itself is the one allowed self-reference.
	{Name: OUs, Label: "OU", Hierarchy: HierCloud,
		Refs: []RefSpec{
			{Field: "parent", Target: OUs, Kind: RefCan},
			{Field: "landing_zone", Target: LandingZones, Kind: RefCan},
		}},
	{Name: Accounts, Label: "Account", Hierarchy: HierCloud,
		Refs: []RefSpec{
			{Field: "connection", Target: Connections, Kind: RefCan},
			{Field: "ou", Target: OUs, Kind: RefCan},
		}},
	{Name: Regions, Label: "Region", Hierarchy: HierCloud, Catalog: true, ScopeField: "csp",
		Attrs: []AttrSpec{{Field: "csp", Choices: CSPChoices}}},
	{Name: Zones, Label: "Zone", Hierarchy: HierCloud, Catalog: true, ScopeField: "region",
		Refs: []RefSpec{{Field: "region", Target: Regions, Kind: RefCan}}},
	{Name: ServiceCats, Label: "Service Category", Hierarchy: HierCloud, Catalog: true},
	{Name: Services, Label: "Service", Hierarchy: HierCloud, Catalog: true,
		Refs: []RefSpec{{Field: "service_cat", Target: ServiceCats, Kind: RefCan}}},
	{Name: ServiceUsages, Label: "Service Usage", Hierarchy: HierCloud, Catalog: true,
		Refs: []RefSpec{{Field: "service", Target: Services, Kind: RefCan}}},
	{Name: Usages, Label: "Usage Type", Hierarchy: HierCloud, Catalog: true,
		Refs: []RefSpec{{Field: "service_usage", Target: ServiceUsages, Kind: RefCan}}},
	{Name: ResourceGroups, Label: "Resource Group", Hierarchy: HierCloud, Catalog: true,
		Attrs: []AttrSpec{{Field: "project_path", Pattern: `^\|.+\|$`}}},
	{Name: Resources, Label: "Resource", Hierarchy: HierCloud, Catalog: true,
		Refs:  []RefSpec{{Field: "resource_group", Target: ResourceGroups, Kind: RefCan}},
		Attrs: []AttrSpec{{Field: "project_path", Pattern: `^\|.+\|$`}}},

	{Name: ProjectLZs, Label: "Project ↔ LZ", Hierarchy: HierCloud, Junction: true,
		Refs: []RefSpec{
			{Field: "project", Target: Projects, Kind: RefMust},
			{Field: "landing_zone", Target: LandingZones, Kind: RefMust},
		}},
	{Name: AppOUs, Label: "App ↔ OU", Hierarchy: HierCloud, Junction: true,
		Refs: []RefSpec{
			{Field: "ou", Target: OUs, Kind: RefMust},
			{Field: "app", Target: Apps, Kind: RefMust},
		}},
	{Name: AccountEnvironments, Label: "Account ↔ Environment", Hierarchy: HierCloud, Junction: true,
		Refs: []RefSpec{
			{Field: "account", Target: Accounts, Kind: RefMust},
			{Field: "environment", Target: Environments, Kind: RefMust},
		}},

	// Alias: code is the invalid tag value, level says which tag it was found in, and target
	// is the correct node (a Project path hash for Project levels, a Biz code for Biz levels).
	{Name: Aliases, Label: "Alias", Hierarchy: HierMeta, ScopeField: "level",
		Attrs: []AttrSpec{
			{Field: "level", Required: true, Choices: AliasLevels},
			{Field: "target", Required: true},
		}},
}

// Spec returns the collection spec by name.
func Spec(name string) (CollectionSpec, bool) {
	for _, s := range Schema {
		if s.Name == name {
			return s, true
		}
	}
	return CollectionSpec{}, false
}

// Levels returns the collection names of one hierarchy, top level first.
func Levels(hierarchy string) []string {
	var out []string
	for _, s := range Schema {
		if s.Hierarchy == hierarchy && !s.Junction {
			out = append(out, s.Name)
		}
	}
	return out
}

// Row is one taxonomy entry: "code", "name", and one key per reference field and attribute.
type Row map[string]string

// Code returns the row's business key.
func (r Row) Code() string { return strings.TrimSpace(r["code"]) }

// Name returns the row's display name.
func (r Row) Name() string { return strings.TrimSpace(r["name"]) }

// Snapshot is the on-disk taxonomy. It mirrors the Directus collections.
type Snapshot struct {
	Collections map[string][]Row
}

// NewSnapshot returns an empty snapshot.
func NewSnapshot() *Snapshot {
	return &Snapshot{Collections: map[string][]Row{}}
}

// Rows returns the rows of a collection.
func (s *Snapshot) Rows(collection string) []Row { return s.Collections[collection] }

// Add appends a row to a collection.
func (s *Snapshot) Add(collection string, row Row) {
	s.Collections[collection] = append(s.Collections[collection], row)
}

// UnmarshalJSON reads {"<collection>": [{...}], ...}. The key "id" is read as an alias of
// "code" and a legacy top-level "tenant" key is ignored (both for one release).
func (s *Snapshot) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Collections = map[string][]Row{}
	for k, v := range raw {
		if k == "tenant" {
			continue
		}
		spec, ok := Spec(k)
		if !ok {
			return fmt.Errorf("unknown taxonomy collection %q", k)
		}
		var items []map[string]any
		dec := json.NewDecoder(bytes.NewReader(v))
		dec.UseNumber()
		if err := dec.Decode(&items); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
		for _, item := range items {
			row := Row{}
			for field, val := range item {
				switch tv := val.(type) {
				case nil:
				case string:
					row[field] = tv
				case json.Number:
					// Numeric codes (e.g. account ids typed as numbers) are kept verbatim.
					row[field] = tv.String()
				default:
					return fmt.Errorf("%s.%s: expected a string, got %T", k, field, val)
				}
			}
			if id, ok := row["id"]; ok && !spec.Junction {
				if _, has := row["code"]; !has {
					row["code"] = id
				}
				delete(row, "id")
			}
			s.Collections[k] = append(s.Collections[k], row)
		}
	}
	return nil
}

// MarshalJSON writes collections in schema order with "code" and "name" first.
func (s *Snapshot) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{")
	for si, spec := range Schema {
		rows := s.Collections[spec.Name]
		if si > 0 {
			buf.WriteString(",")
		}
		fmt.Fprintf(&buf, "\n  %q: [", spec.Name)
		for i, row := range rows {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString("\n    {")
			keys := []string{"code", "name"}
			if spec.Junction {
				keys = nil
			}
			for _, ref := range spec.Refs {
				keys = append(keys, ref.Field)
			}
			for _, a := range spec.Attrs {
				keys = append(keys, a.Field)
			}
			var extra []string
			for k := range row {
				if !contains(keys, k) {
					extra = append(extra, k)
				}
			}
			sort.Strings(extra)
			first := true
			for _, k := range append(keys, extra...) {
				v, ok := row[k]
				if !ok || (v == "" && (spec.Junction || (k != "code" && k != "name"))) {
					continue
				}
				if !first {
					buf.WriteString(", ")
				}
				first = false
				kb, _ := json.Marshal(k)
				vb, _ := json.Marshal(v)
				buf.Write(kb)
				buf.WriteString(": ")
				buf.Write(vb)
			}
			buf.WriteString("}")
		}
		if len(rows) > 0 {
			buf.WriteString("\n  ")
		}
		buf.WriteString("]")
	}
	buf.WriteString("\n}\n")
	return buf.Bytes(), nil
}

// ReadSnapshot reads a taxonomy JSON file.
func ReadSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read taxonomy: %w", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parse taxonomy %s: %w", path, err)
	}
	return &snap, nil
}

// WriteSnapshot writes a taxonomy JSON file, creating its directory.
func WriteSnapshot(path string, snap *Snapshot) error {
	data, err := snap.MarshalJSON()
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
