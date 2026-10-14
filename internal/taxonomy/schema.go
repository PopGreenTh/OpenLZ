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
	BusinessUnits  = "business_units"
	CostCenters    = "cost_centers"
	Owners         = "owners"
	CustomerFuncs  = "customer_funcs"
	Projects       = "projects"
	Apps           = "apps"
	Environments   = "environments"
	Accounts       = "accounts"
	CSPs           = "csps"
	Regions        = "regions"
	ServiceCats    = "service_cats"
	Services       = "services"
	ServiceUsages  = "service_usages"
	Usages         = "usages"
	ResourceGroups = "resource_groups"
	Resources      = "resources"
)

// Ref kinds describe what a reference field means in the cost hierarchy.
const (
	// RefParent places a node under its parent level in the same hierarchy (required).
	RefParent = "parent"
	// RefMust is a required cross-hierarchy reference ("must ref to").
	RefMust = "must"
	// RefCan is an optional reference ("can be mapped to" / "can ref to").
	RefCan = "can"
)

// RefSpec is a reference field on a collection that holds the id of a row in Target.
type RefSpec struct {
	Field  string
	Target string
	Kind   string
}

// Required reports whether every row must set this reference.
func (r RefSpec) Required() bool { return r.Kind == RefParent || r.Kind == RefMust }

// CollectionSpec describes one taxonomy level.
type CollectionSpec struct {
	Name      string
	Label     string
	Hierarchy string // "business" or "project"
	Refs      []RefSpec
	// ScopeRef names a ref whose value scopes id uniqueness (regions are unique per CSP).
	ScopeRef string
}

// Schema lists every collection in dependency order: a collection only references
// collections that appear before it.
//
//	Biz:     BusinessUnit → CostCenter → Owner → CustomerFunc
//	Project: Project → App → Environment/Account → CSP/Region → ServiceCat/Service
//	         → ServiceUsage/Usage → ResourceGroup/Resource
//
// Relations:
//
//	Project       must ref BusinessUnit
//	App           must ref CostCenter
//	ResourceGroup can ref CustomerFunc
//	Region        can be mapped to CSP (region id is unique within its CSP)
//	Account       can be mapped to Environment
//	Service       can be mapped to ServiceCat
//	Usage         can be mapped to ServiceUsage
//	Resource      can be mapped to ResourceGroup
var Schema = []CollectionSpec{
	{Name: BusinessUnits, Label: "Business Unit", Hierarchy: "business"},
	{Name: CostCenters, Label: "Cost Center", Hierarchy: "business",
		Refs: []RefSpec{{Field: "business_unit", Target: BusinessUnits, Kind: RefParent}}},
	{Name: Owners, Label: "Owner", Hierarchy: "business",
		Refs: []RefSpec{{Field: "cost_center", Target: CostCenters, Kind: RefParent}}},
	{Name: CustomerFuncs, Label: "Customer Function", Hierarchy: "business",
		Refs: []RefSpec{{Field: "owner", Target: Owners, Kind: RefParent}}},

	{Name: Projects, Label: "Project", Hierarchy: "project",
		Refs: []RefSpec{{Field: "business_unit", Target: BusinessUnits, Kind: RefMust}}},
	{Name: Apps, Label: "App", Hierarchy: "project",
		Refs: []RefSpec{
			{Field: "project", Target: Projects, Kind: RefParent},
			{Field: "cost_center", Target: CostCenters, Kind: RefMust},
		}},
	{Name: Environments, Label: "Environment", Hierarchy: "project",
		Refs: []RefSpec{{Field: "app", Target: Apps, Kind: RefParent}}},
	{Name: Accounts, Label: "Account", Hierarchy: "project",
		Refs: []RefSpec{{Field: "environment", Target: Environments, Kind: RefCan}}},
	{Name: CSPs, Label: "CSP", Hierarchy: "project"},
	{Name: Regions, Label: "Region", Hierarchy: "project", ScopeRef: "csp",
		Refs: []RefSpec{{Field: "csp", Target: CSPs, Kind: RefCan}}},
	{Name: ServiceCats, Label: "Service Category", Hierarchy: "project"},
	{Name: Services, Label: "Service", Hierarchy: "project",
		Refs: []RefSpec{{Field: "service_cat", Target: ServiceCats, Kind: RefCan}}},
	{Name: ServiceUsages, Label: "Service Usage", Hierarchy: "project",
		Refs: []RefSpec{{Field: "service", Target: Services, Kind: RefCan}}},
	{Name: Usages, Label: "Usage", Hierarchy: "project",
		Refs: []RefSpec{{Field: "service_usage", Target: ServiceUsages, Kind: RefCan}}},
	{Name: ResourceGroups, Label: "Resource Group", Hierarchy: "project",
		Refs: []RefSpec{{Field: "customer_func", Target: CustomerFuncs, Kind: RefCan}}},
	{Name: Resources, Label: "Resource", Hierarchy: "project",
		Refs: []RefSpec{{Field: "resource_group", Target: ResourceGroups, Kind: RefCan}}},
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

// Row is one taxonomy entry: "id", "name", and one key per reference field.
type Row map[string]string

// ID returns the row's id.
func (r Row) ID() string { return strings.TrimSpace(r["id"]) }

// Name returns the row's display name.
func (r Row) Name() string { return strings.TrimSpace(r["name"]) }

// Snapshot is the on-disk taxonomy for one tenant. It mirrors the Directus collections.
type Snapshot struct {
	Tenant      string
	Collections map[string][]Row
}

// NewSnapshot returns an empty snapshot for tenant.
func NewSnapshot(tenant string) *Snapshot {
	return &Snapshot{Tenant: tenant, Collections: map[string][]Row{}}
}

// Rows returns the rows of a collection.
func (s *Snapshot) Rows(collection string) []Row { return s.Collections[collection] }

// Add appends a row to a collection.
func (s *Snapshot) Add(collection string, row Row) {
	s.Collections[collection] = append(s.Collections[collection], row)
}

// UnmarshalJSON reads {"tenant": "...", "<collection>": [{...}], ...}.
func (s *Snapshot) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Collections = map[string][]Row{}
	for k, v := range raw {
		if k == "tenant" {
			if err := json.Unmarshal(v, &s.Tenant); err != nil {
				return fmt.Errorf("tenant: %w", err)
			}
			continue
		}
		if _, ok := Spec(k); !ok {
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
					// Numeric ids (e.g. account ids typed as numbers) are kept verbatim.
					row[field] = tv.String()
				default:
					return fmt.Errorf("%s.%s: expected a string, got %T", k, field, val)
				}
			}
			s.Collections[k] = append(s.Collections[k], row)
		}
	}
	return nil
}

// MarshalJSON writes collections in schema order with "id" and "name" first.
func (s *Snapshot) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	tenant, _ := json.Marshal(s.Tenant)
	fmt.Fprintf(&buf, "  \"tenant\": %s", tenant)
	for _, spec := range Schema {
		rows := s.Collections[spec.Name]
		fmt.Fprintf(&buf, ",\n  %q: [", spec.Name)
		for i, row := range rows {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString("\n    {")
			keys := []string{"id", "name"}
			for _, ref := range spec.Refs {
				keys = append(keys, ref.Field)
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
				if !ok || (v == "" && k != "id" && k != "name") {
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
