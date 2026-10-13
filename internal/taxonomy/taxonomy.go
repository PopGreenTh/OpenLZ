// Package taxonomy maps empty Cost Explorer tags onto the project and business cost hierarchies.
//
// Business hierarchy: BusinessUnit → Owner → CostCenter → CustomerFunc.
// Project hierarchy: Project → App → Environment/Account → ServiceCat/Service.
//
// Project references business. These levels share identity ("must behave"):
//
//	ProjectID/Name     = BusinessUnitID/Name
//	AppID/Name         = OwnerID/Name
//	EnvironmentID/Name = CostCenterID/Name
//
// These links are lookups ("can be mapped") and may point at a different node:
//
//	App     → Project
//	Account → Environment
//	Service → ServiceCat
package taxonomy

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	// DefaultProjectPath is the project hierarchy used when enrich is not given an explicit path.
	DefaultProjectPath = "taxonomy/project_taxonomy.json"
	// DefaultBusinessPath is the business hierarchy used when enrich is not given an explicit path.
	DefaultBusinessPath = "taxonomy/business_taxonomy.json"
)

// Slot is an identifier and its display name.
type Slot struct {
	ID   string
	Name string
}

// Input is the tag state sensed from Cost Explorer. Empty, "Untagged", and "Unknown" are unfilled.
type Input struct {
	AccountID        string
	AccountName      string
	Service          string
	Project          string
	App              string
	Environment      string
	ServiceCategory  string
	BusinessUnit     string
	Owner            string
	CostCenter       string
	CustomerFunction string
}

// Output is the hierarchy after empty tags are filled.
type Output struct {
	Project          Slot
	App              Slot
	Environment      Slot
	ServiceCategory  Slot
	BusinessUnit     Slot
	Owner            Slot
	CostCenter       Slot
	CustomerFunction Slot
	Account          Slot
	Service          Slot
}

type idName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type businessFile struct {
	BusinessUnits []businessUnitJSON `json:"businessUnits"`
}

type businessUnitJSON struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Owners []ownerJSON `json:"owners"`
}

type ownerJSON struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	CostCenters []costCenterJSON `json:"costCenters"`
}

type costCenterJSON struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	CustomerFuncs []idName `json:"customerFuncs"`
}

type projectFile struct {
	Projects []projectJSON `json:"projects"`
}

type projectJSON struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	BusinessUnitID   string    `json:"businessUnitId"`
	BusinessUnitName string    `json:"businessUnitName,omitempty"`
	Apps             []appJSON `json:"apps"`
}

type appJSON struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OwnerID      string    `json:"ownerId"`
	OwnerName    string    `json:"ownerName,omitempty"`
	ProjectID    string    `json:"projectId"`
	ProjectName  string    `json:"projectName,omitempty"`
	Environments []envJSON `json:"environments"`
}

type envJSON struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	CostCenterID      string     `json:"costCenterId"`
	CostCenterName    string     `json:"costCenterName,omitempty"`
	Accounts          []acctJSON `json:"accounts"`
	ServiceCategories []catJSON  `json:"serviceCategories"`
}

type acctJSON struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	EnvironmentID   string `json:"environmentId"`
	EnvironmentName string `json:"environmentName,omitempty"`
}

type catJSON struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	CustomerFuncID   string        `json:"customerFuncId"`
	CustomerFuncName string        `json:"customerFuncName,omitempty"`
	Services         []serviceJSON `json:"services"`
}

type serviceJSON struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ServiceCatID   string `json:"serviceCatId"`
	ServiceCatName string `json:"serviceCatName,omitempty"`
}

type buNode struct {
	id, name string
	owners   []*ownerNode
}

type ownerNode struct {
	id, name    string
	bu          *buNode
	costCenters []*ccNode
}

type ccNode struct {
	id, name string
	owner    *ownerNode
	funcs    []*cfNode
}

type cfNode struct {
	id, name string
	cc       *ccNode
}

type projectNode struct {
	id, name string
	bu       *buNode
	apps     []*appNode
}

type appNode struct {
	id, name      string
	project       *projectNode
	mappedProject *projectNode
	owner         *ownerNode
	envs          []*envNode
}

type envNode struct {
	id, name string
	app      *appNode
	cc       *ccNode
	accounts []*accountNode
	cats     []*catNode
}

type accountNode struct {
	id, name string
	env      *envNode
}

type catNode struct {
	id, name string
	env      *envNode
	cf       *cfNode
	services []*serviceNode
}

type serviceNode struct {
	id, name string
	cat      *catNode
}

// Catalog is the loaded project and business hierarchies.
type Catalog struct {
	business []*buNode
	projects []*projectNode

	buByID, ownerByID, ccByID, cfByID         map[string]*indexed[any]
	projectByID, appByID, envByID, catByID    map[string]*indexed[any]
	accountByID, serviceByID                  map[string]*indexed[any]
	buByName, ownerByName, ccByName, cfByName map[string]*indexed[any]
	projectByName, appByName, envByName       map[string]*indexed[any]
	catByName, accountByName, serviceByName   map[string]*indexed[any]
}

// indexed holds one node or records that the key was ambiguous.
type indexed[T any] struct {
	node *T
	n    int
}

// Load reads both taxonomy files. Project nodes must reference business nodes.
func Load(projectPath, businessPath string) (*Catalog, error) {
	bizRaw, err := os.ReadFile(businessPath)
	if err != nil {
		return nil, fmt.Errorf("read business taxonomy: %w", err)
	}
	projRaw, err := os.ReadFile(projectPath)
	if err != nil {
		return nil, fmt.Errorf("read project taxonomy: %w", err)
	}

	var biz businessFile
	if err := json.Unmarshal(bizRaw, &biz); err != nil {
		return nil, fmt.Errorf("parse business taxonomy: %w", err)
	}
	var proj projectFile
	if err := json.Unmarshal(projRaw, &proj); err != nil {
		return nil, fmt.Errorf("parse project taxonomy: %w", err)
	}
	return build(biz, proj)
}

// LoadOptional loads taxonomy when both files exist.
// Empty paths use the default locations. A missing default pair skips enrichment.
// An explicit path that is missing, or only one of the two files, is an error.
func LoadOptional(projectPath, businessPath string) (*Catalog, error) {
	projectExplicit := strings.TrimSpace(projectPath) != ""
	businessExplicit := strings.TrimSpace(businessPath) != ""
	if !projectExplicit {
		projectPath = DefaultProjectPath
	}
	if !businessExplicit {
		businessPath = DefaultBusinessPath
	}

	_, pErr := os.Stat(projectPath)
	_, bErr := os.Stat(businessPath)
	pOK := pErr == nil
	bOK := bErr == nil
	if !projectExplicit && !businessExplicit && !pOK && !bOK {
		return nil, nil
	}
	if !pOK || !bOK {
		return nil, fmt.Errorf("cost taxonomy requires both %s and %s", projectPath, businessPath)
	}
	return Load(projectPath, businessPath)
}

// DefaultProjectPathForTenant returns the canonical project taxonomy path for a tenant.
func DefaultProjectPathForTenant(tenant string) string {
	tenant = strings.TrimSpace(tenant)
	if tenant == "" {
		return DefaultProjectPath
	}
	return fmt.Sprintf("taxonomy/project_taxonomy_%s.json", tenant)
}

// DefaultBusinessPathForTenant returns the canonical business taxonomy path for a tenant.
func DefaultBusinessPathForTenant(tenant string) string {
	tenant = strings.TrimSpace(tenant)
	if tenant == "" {
		return DefaultBusinessPath
	}
	return fmt.Sprintf("taxonomy/business_taxonomy_%s.json", tenant)
}

// ResolveProjectTaxonomyPath resolves the project taxonomy path for a tenant.
// If explicitPath is non-empty, it is returned directly.
// If tenant is non-empty, candidate paths are checked:
// 1. taxonomy/project_taxonomy_<tenant>.json
// 2. taxonomy/<tenant>_project_taxonomy.json
// 3. taxonomy/<tenant>_project.json
// 4. Default fallback: taxonomy/project_taxonomy_<tenant>.json
// If neither is provided, DefaultProjectPath is returned.
func ResolveProjectTaxonomyPath(explicitPath, tenant string) string {
	if strings.TrimSpace(explicitPath) != "" {
		return explicitPath
	}
	tenant = strings.TrimSpace(tenant)
	if tenant == "" {
		return DefaultProjectPath
	}
	candidates := []string{
		fmt.Sprintf("taxonomy/project_taxonomy_%s.json", tenant),
		fmt.Sprintf("taxonomy/%s_project_taxonomy.json", tenant),
		fmt.Sprintf("taxonomy/%s_project.json", tenant),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

// ResolveBusinessTaxonomyPath resolves the business taxonomy path for a tenant.
// If explicitPath is non-empty, it is returned directly.
// If tenant is non-empty, candidate paths are checked:
// 1. taxonomy/business_taxonomy_<tenant>.json
// 2. taxonomy/<tenant>_business_taxonomy.json
// 3. taxonomy/<tenant>_business.json
// 4. Default fallback: taxonomy/business_taxonomy_<tenant>.json
// If neither is provided, DefaultBusinessPath is returned.
func ResolveBusinessTaxonomyPath(explicitPath, tenant string) string {
	if strings.TrimSpace(explicitPath) != "" {
		return explicitPath
	}
	tenant = strings.TrimSpace(tenant)
	if tenant == "" {
		return DefaultBusinessPath
	}
	candidates := []string{
		fmt.Sprintf("taxonomy/business_taxonomy_%s.json", tenant),
		fmt.Sprintf("taxonomy/%s_business_taxonomy.json", tenant),
		fmt.Sprintf("taxonomy/%s_business.json", tenant),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

// LoadOptionalForTenant loads taxonomy files resolving tenant-specific files if tenant is provided.
func LoadOptionalForTenant(projectPath, businessPath, tenant string) (*Catalog, error) {
	proj := ResolveProjectTaxonomyPath(projectPath, tenant)
	biz := ResolveBusinessTaxonomyPath(businessPath, tenant)
	return LoadOptional(proj, biz)
}

// DiscoverTenants scans the taxonomy directory and returns a sorted list of unique tenant identifiers
// detected from filenames such as project_taxonomy_<tenant>.json, <tenant>_project_taxonomy.json, etc.
func DiscoverTenants(taxonomyDir string) ([]string, error) {
	if strings.TrimSpace(taxonomyDir) == "" {
		taxonomyDir = "taxonomy"
	}
	entries, err := os.ReadDir(taxonomyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read taxonomy dir %s: %w", taxonomyDir, err)
	}

	tenantsMap := make(map[string]struct{})
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}

		if strings.HasPrefix(name, "project_taxonomy_") {
			t := strings.TrimPrefix(name, "project_taxonomy_")
			t = strings.TrimSuffix(t, ".json")
			if t != "" {
				tenantsMap[t] = struct{}{}
			}
		}
		if strings.HasPrefix(name, "business_taxonomy_") {
			t := strings.TrimPrefix(name, "business_taxonomy_")
			t = strings.TrimSuffix(t, ".json")
			if t != "" {
				tenantsMap[t] = struct{}{}
			}
		}
		if strings.HasSuffix(name, "_project_taxonomy.json") {
			t := strings.TrimSuffix(name, "_project_taxonomy.json")
			if t != "" {
				tenantsMap[t] = struct{}{}
			}
		}
		if strings.HasSuffix(name, "_business_taxonomy.json") {
			t := strings.TrimSuffix(name, "_business_taxonomy.json")
			if t != "" {
				tenantsMap[t] = struct{}{}
			}
		}
		if strings.HasSuffix(name, "_project.json") {
			t := strings.TrimSuffix(name, "_project.json")
			if t != "" && t != "sample" {
				tenantsMap[t] = struct{}{}
			}
		}
		if strings.HasSuffix(name, "_business.json") {
			t := strings.TrimSuffix(name, "_business.json")
			if t != "" && t != "sample" {
				tenantsMap[t] = struct{}{}
			}
		}
	}

	var tenants []string
	for t := range tenantsMap {
		tenants = append(tenants, t)
	}
	sort.Strings(tenants)
	return tenants, nil
}

func build(biz businessFile, proj projectFile) (*Catalog, error) {
	c := &Catalog{
		buByID: map[string]*indexed[any]{}, ownerByID: map[string]*indexed[any]{},
		ccByID: map[string]*indexed[any]{}, cfByID: map[string]*indexed[any]{},
		projectByID: map[string]*indexed[any]{}, appByID: map[string]*indexed[any]{},
		envByID: map[string]*indexed[any]{}, catByID: map[string]*indexed[any]{},
		accountByID: map[string]*indexed[any]{}, serviceByID: map[string]*indexed[any]{},
		buByName: map[string]*indexed[any]{}, ownerByName: map[string]*indexed[any]{},
		ccByName: map[string]*indexed[any]{}, cfByName: map[string]*indexed[any]{},
		projectByName: map[string]*indexed[any]{}, appByName: map[string]*indexed[any]{},
		envByName: map[string]*indexed[any]{}, catByName: map[string]*indexed[any]{},
		accountByName: map[string]*indexed[any]{}, serviceByName: map[string]*indexed[any]{},
	}

	for _, bu := range biz.BusinessUnits {
		node, err := c.addBusinessUnit(bu)
		if err != nil {
			return nil, err
		}
		c.business = append(c.business, node)
	}

	var pendingApps []pendingApp
	for _, p := range proj.Projects {
		node, apps, err := c.addProject(p)
		if err != nil {
			return nil, err
		}
		c.projects = append(c.projects, node)
		pendingApps = append(pendingApps, apps...)
	}
	for _, pend := range pendingApps {
		if pend.mapID == "" {
			pend.app.mappedProject = pend.app.project
			continue
		}
		target := one(c.projectByID, pend.mapID)
		if target == nil {
			target = one(c.projectByName, pend.mapID)
		}
		if target == nil {
			return nil, fmt.Errorf("app %q projectId/projectName %q does not match a project", pend.app.id, pend.mapID)
		}
		pend.app.mappedProject = target.(*projectNode)
	}
	return c, nil
}

func (c *Catalog) addBusinessUnit(in businessUnitJSON) (*buNode, error) {
	if err := requireIDName("businessUnit", in.ID, in.Name); err != nil {
		return nil, err
	}
	node := &buNode{id: in.ID, name: in.Name}
	if err := c.index("businessUnit", node, c.buByID, c.buByName); err != nil {
		return nil, err
	}
	for _, ow := range in.Owners {
		owner, err := c.addOwner(ow, node)
		if err != nil {
			return nil, err
		}
		node.owners = append(node.owners, owner)
	}
	return node, nil
}

func (c *Catalog) addOwner(in ownerJSON, bu *buNode) (*ownerNode, error) {
	if err := requireIDName("owner", in.ID, in.Name); err != nil {
		return nil, err
	}
	node := &ownerNode{id: in.ID, name: in.Name, bu: bu}
	if err := c.index("owner", node, c.ownerByID, c.ownerByName); err != nil {
		return nil, err
	}
	for _, cc := range in.CostCenters {
		center, err := c.addCostCenter(cc, node)
		if err != nil {
			return nil, err
		}
		node.costCenters = append(node.costCenters, center)
	}
	return node, nil
}

func (c *Catalog) addCostCenter(in costCenterJSON, owner *ownerNode) (*ccNode, error) {
	if err := requireIDName("costCenter", in.ID, in.Name); err != nil {
		return nil, err
	}
	node := &ccNode{id: in.ID, name: in.Name, owner: owner}
	if err := c.index("costCenter", node, c.ccByID, c.ccByName); err != nil {
		return nil, err
	}
	for _, fn := range in.CustomerFuncs {
		cf, err := c.addCustomerFunc(fn, node)
		if err != nil {
			return nil, err
		}
		node.funcs = append(node.funcs, cf)
	}
	return node, nil
}

func (c *Catalog) addCustomerFunc(in idName, cc *ccNode) (*cfNode, error) {
	if err := requireIDName("customerFunc", in.ID, in.Name); err != nil {
		return nil, err
	}
	node := &cfNode{id: in.ID, name: in.Name, cc: cc}
	if err := c.index("customerFunc", node, c.cfByID, c.cfByName); err != nil {
		return nil, err
	}
	return node, nil
}

type pendingApp struct {
	app   *appNode
	mapID string
}

func (c *Catalog) addProject(in projectJSON) (*projectNode, []pendingApp, error) {
	if err := requireIDName("project", in.ID, in.Name); err != nil {
		return nil, nil, err
	}
	buID := strings.TrimSpace(in.BusinessUnitID)
	if buID == "" {
		buID = strings.TrimSpace(in.BusinessUnitName)
	}
	if buID == "" {
		buID = in.ID
	}
	bu := one(c.buByID, buID)
	if bu == nil {
		bu = one(c.buByName, buID)
	}
	if bu == nil {
		return nil, nil, fmt.Errorf("project %q references unknown businessUnit %q", in.ID, buID)
	}
	buNode := bu.(*buNode)
	if buNode.id != in.ID && buID != in.ID && buID != in.Name {
		return nil, nil, fmt.Errorf("project id %q must behave as businessUnit id %q", in.ID, buNode.id)
	}
	if !strings.EqualFold(buNode.name, in.Name) {
		return nil, nil, fmt.Errorf("project name %q must behave as businessUnit name %q", in.Name, buNode.name)
	}
	node := &projectNode{id: in.ID, name: in.Name, bu: buNode}
	if err := c.index("project", node, c.projectByID, c.projectByName); err != nil {
		return nil, nil, err
	}
	var pending []pendingApp
	for _, app := range in.Apps {
		child, mapID, err := c.addApp(app, node)
		if err != nil {
			return nil, nil, err
		}
		node.apps = append(node.apps, child)
		pending = append(pending, pendingApp{app: child, mapID: mapID})
	}
	return node, pending, nil
}

func (c *Catalog) addApp(in appJSON, project *projectNode) (*appNode, string, error) {
	if err := requireIDName("app", in.ID, in.Name); err != nil {
		return nil, "", err
	}
	ownerID := strings.TrimSpace(in.OwnerID)
	if ownerID == "" {
		ownerID = strings.TrimSpace(in.OwnerName)
	}
	if ownerID == "" {
		ownerID = in.ID
	}
	owner := one(c.ownerByID, ownerID)
	if owner == nil {
		owner = one(c.ownerByName, ownerID)
	}
	if owner == nil {
		return nil, "", fmt.Errorf("app %q references unknown owner %q", in.ID, ownerID)
	}
	ownerNode := owner.(*ownerNode)
	if ownerNode.id != in.ID && ownerID != in.ID && ownerID != in.Name {
		return nil, "", fmt.Errorf("app id %q must behave as owner id %q", in.ID, ownerNode.id)
	}
	if !strings.EqualFold(ownerNode.name, in.Name) {
		return nil, "", fmt.Errorf("app name %q must behave as owner name %q", in.Name, ownerNode.name)
	}
	node := &appNode{id: in.ID, name: in.Name, project: project, owner: ownerNode}
	if err := c.index("app", node, c.appByID, c.appByName); err != nil {
		return nil, "", err
	}
	for _, env := range in.Environments {
		child, err := c.addEnvironment(env, node)
		if err != nil {
			return nil, "", err
		}
		node.envs = append(node.envs, child)
	}
	mapID := strings.TrimSpace(in.ProjectID)
	if mapID == "" {
		mapID = strings.TrimSpace(in.ProjectName)
	}
	return node, mapID, nil
}

func (c *Catalog) addEnvironment(in envJSON, app *appNode) (*envNode, error) {
	if err := requireIDName("environment", in.ID, in.Name); err != nil {
		return nil, err
	}
	ccID := strings.TrimSpace(in.CostCenterID)
	if ccID == "" {
		ccID = strings.TrimSpace(in.CostCenterName)
	}
	if ccID == "" {
		ccID = in.ID
	}
	cc := one(c.ccByID, ccID)
	if cc == nil {
		cc = one(c.ccByName, ccID)
	}
	if cc == nil {
		return nil, fmt.Errorf("environment %q references unknown costCenter %q", in.ID, ccID)
	}
	ccNode := cc.(*ccNode)
	if ccNode.id != in.ID && ccID != in.ID && ccID != in.Name {
		return nil, fmt.Errorf("environment id %q must behave as costCenter id %q", in.ID, ccNode.id)
	}
	if !strings.EqualFold(ccNode.name, in.Name) {
		return nil, fmt.Errorf("environment name %q must behave as costCenter name %q", in.Name, ccNode.name)
	}
	node := &envNode{id: in.ID, name: in.Name, app: app, cc: ccNode}
	if err := c.index("environment", node, c.envByID, c.envByName); err != nil {
		return nil, err
	}
	for _, acct := range in.Accounts {
		child, err := c.addAccount(acct, node)
		if err != nil {
			return nil, err
		}
		node.accounts = append(node.accounts, child)
	}
	for _, cat := range in.ServiceCategories {
		child, err := c.addServiceCat(cat, node)
		if err != nil {
			return nil, err
		}
		node.cats = append(node.cats, child)
	}
	return node, nil
}

func (c *Catalog) addAccount(in acctJSON, parent *envNode) (*accountNode, error) {
	if err := requireIDName("account", in.ID, in.Name); err != nil {
		return nil, err
	}
	env := parent
	mapID := strings.TrimSpace(in.EnvironmentID)
	if mapID == "" {
		mapID = strings.TrimSpace(in.EnvironmentName)
	}
	if mapID != "" {
		found := one(c.envByID, mapID)
		if found == nil {
			found = one(c.envByName, mapID)
		}
		if found == nil {
			return nil, fmt.Errorf("account %q environmentId/environmentName %q does not match an environment", in.ID, mapID)
		}
		env = found.(*envNode)
	}
	node := &accountNode{id: in.ID, name: in.Name, env: env}
	if err := c.index("account", node, c.accountByID, c.accountByName); err != nil {
		return nil, err
	}
	return node, nil
}

func (c *Catalog) addServiceCat(in catJSON, env *envNode) (*catNode, error) {
	if err := requireIDName("serviceCategory", in.ID, in.Name); err != nil {
		return nil, err
	}
	node := &catNode{id: in.ID, name: in.Name, env: env}
	mapID := strings.TrimSpace(in.CustomerFuncID)
	if mapID == "" {
		mapID = strings.TrimSpace(in.CustomerFuncName)
	}
	if mapID != "" {
		found := one(c.cfByID, mapID)
		if found == nil {
			found = one(c.cfByName, mapID)
		}
		if found == nil {
			return nil, fmt.Errorf("serviceCategory %q customerFuncId/customerFuncName %q does not match a customerFunc", in.ID, mapID)
		}
		node.cf = found.(*cfNode)
	}
	if err := c.index("serviceCategory", node, c.catByID, c.catByName); err != nil {
		return nil, err
	}
	for _, svc := range in.Services {
		child, err := c.addService(svc, node)
		if err != nil {
			return nil, err
		}
		node.services = append(node.services, child)
	}
	return node, nil
}

func (c *Catalog) addService(in serviceJSON, parent *catNode) (*serviceNode, error) {
	if err := requireIDName("service", in.ID, in.Name); err != nil {
		return nil, err
	}
	cat := parent
	mapID := strings.TrimSpace(in.ServiceCatID)
	if mapID == "" {
		mapID = strings.TrimSpace(in.ServiceCatName)
	}
	if mapID != "" {
		found := one(c.catByID, mapID)
		if found == nil {
			found = one(c.catByName, mapID)
		}
		if found == nil {
			return nil, fmt.Errorf("service %q serviceCatId/serviceCatName %q does not match a serviceCategory", in.ID, mapID)
		}
		cat = found.(*catNode)
	}
	node := &serviceNode{id: in.ID, name: in.Name, cat: cat}
	if err := c.index("service", node, c.serviceByID, c.serviceByName); err != nil {
		return nil, err
	}
	return node, nil
}

func (c *Catalog) index(level string, node any, byID, byName map[string]*indexed[any]) error {
	id, name := nodeIDName(node)
	if err := putUnique(byID, key(id), node, level+" id"); err != nil {
		return err
	}
	if err := putUnique(byName, key(name), node, level+" name"); err != nil {
		return err
	}
	return nil
}

func nodeIDName(node any) (string, string) {
	switch n := node.(type) {
	case *buNode:
		return n.id, n.name
	case *ownerNode:
		return n.id, n.name
	case *ccNode:
		return n.id, n.name
	case *cfNode:
		return n.id, n.name
	case *projectNode:
		return n.id, n.name
	case *appNode:
		return n.id, n.name
	case *envNode:
		return n.id, n.name
	case *catNode:
		return n.id, n.name
	case *accountNode:
		return n.id, n.name
	case *serviceNode:
		return n.id, n.name
	default:
		return "", ""
	}
}

func putUnique(m map[string]*indexed[any], k string, node any, what string) error {
	if k == "" {
		return fmt.Errorf("empty %s", what)
	}
	if prev, ok := m[k]; ok && prev.n > 0 {
		prev.n++
		prev.node = nil
		return fmt.Errorf("duplicate %s %q", what, k)
	}
	m[k] = &indexed[any]{node: &node, n: 1}
	return nil
}

func one(m map[string]*indexed[any], raw string) any {
	slot, ok := m[key(raw)]
	if !ok || slot.n != 1 || slot.node == nil {
		return nil
	}
	return *slot.node
}

func requireIDName(level, id, name string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s requires id and name", level)
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

// Resolve fills empty tags from the hierarchies. Values already present are kept.
func (c *Catalog) Resolve(in Input) Output {
	var project *projectNode
	var app *appNode
	var env *envNode
	var cat *catNode
	var bu *buNode
	var owner *ownerNode
	var cc *ccNode
	var cf *cfNode
	var account *accountNode
	var service *serviceNode

	if !isEmpty(in.AccountID) {
		if n := one(c.accountByID, in.AccountID); n != nil {
			account = n.(*accountNode)
		}
	}
	if account == nil && !isEmpty(in.AccountName) {
		if n := one(c.accountByName, in.AccountName); n != nil {
			account = n.(*accountNode)
		}
	}
	if !isEmpty(in.Service) {
		if n := one(c.serviceByID, in.Service); n != nil {
			service = n.(*serviceNode)
		} else if n := one(c.serviceByName, in.Service); n != nil {
			service = n.(*serviceNode)
		}
	}

	project = c.matchProject(in.Project)
	app = c.matchApp(in.App)
	env = c.matchEnv(in.Environment)
	cat = c.matchCat(in.ServiceCategory)
	bu = c.matchBU(in.BusinessUnit)
	owner = c.matchOwner(in.Owner)
	cc = c.matchCC(in.CostCenter)
	cf = c.matchCF(in.CustomerFunction)

	// Also classify a raw project/app token when it uniquely names another level.
	if project == nil && !isEmpty(in.Project) {
		c.classify(in.Project, &app, &env, &cat, &bu, &owner, &cc, &cf)
	}
	if app == nil && !isEmpty(in.App) {
		c.classify(in.App, &app, &env, &cat, &bu, &owner, &cc, &cf)
	}

	if env == nil && account != nil && isEmpty(in.Environment) {
		env = account.env
	}
	if cat == nil && service != nil && isEmpty(in.ServiceCategory) {
		if env == nil || service.cat == nil || service.cat.env == env {
			cat = service.cat
		}
	}
	// A service category supplies its environment only when this row has no account,
	// or the account already maps to that same environment.
	if env == nil && cat != nil && isEmpty(in.Environment) && account == nil && isEmpty(in.AccountID) && isEmpty(in.AccountName) {
		env = cat.env
	}
	if app == nil && env != nil && isEmpty(in.App) {
		app = env.app
	}
	if project == nil && app != nil && isEmpty(in.Project) {
		project = app.mappedProject
		if project == nil {
			project = app.project
		}
	}
	if bu == nil && project != nil && isEmpty(in.BusinessUnit) {
		bu = project.bu
	}
	if owner == nil && app != nil && isEmpty(in.Owner) {
		owner = app.owner
	}
	if cc == nil && env != nil && isEmpty(in.CostCenter) {
		cc = env.cc
	}
	if cf == nil && isEmpty(in.CustomerFunction) {
		categoryOwnsRow := cat != nil && (env != nil && cat.env == env || (account == nil && isEmpty(in.AccountID) && isEmpty(in.AccountName)))
		if categoryOwnsRow && cat.cf != nil {
			cf = cat.cf
		} else if cc != nil && len(cc.funcs) == 1 {
			cf = cc.funcs[0]
		}
	}

	if project == nil && bu != nil && isEmpty(in.Project) {
		if n := one(c.projectByID, bu.id); n != nil {
			project = n.(*projectNode)
		}
	}
	if app == nil && owner != nil && isEmpty(in.App) {
		if n := one(c.appByID, owner.id); n != nil {
			app = n.(*appNode)
		}
	}
	if env == nil && cc != nil && isEmpty(in.Environment) {
		if n := one(c.envByID, cc.id); n != nil {
			env = n.(*envNode)
		}
	}
	if app == nil && env != nil && isEmpty(in.App) {
		app = env.app
	}
	if project == nil && app != nil && isEmpty(in.Project) {
		project = app.mappedProject
		if project == nil {
			project = app.project
		}
	}
	if bu == nil && project != nil && isEmpty(in.BusinessUnit) {
		bu = project.bu
	}
	if owner == nil && app != nil && isEmpty(in.Owner) {
		owner = app.owner
	}
	if cc == nil && env != nil && isEmpty(in.CostCenter) {
		cc = env.cc
	}

	out := Output{}
	if account != nil {
		out.Account = Slot{ID: account.id, Name: account.name}
	} else {
		out.Account = Slot{ID: clean(in.AccountID), Name: clean(in.AccountName)}
	}
	if service != nil {
		out.Service = Slot{ID: service.id, Name: service.name}
	} else if !isEmpty(in.Service) {
		out.Service = Slot{Name: strings.TrimSpace(in.Service)}
	}

	out.Project = slotOf(project, in.Project)
	out.App = slotOf(app, in.App)
	out.Environment = slotOf(env, in.Environment)
	out.ServiceCategory = slotOf(cat, in.ServiceCategory)
	out.BusinessUnit = slotOf(bu, in.BusinessUnit)
	out.Owner = slotOf(owner, in.Owner)
	out.CostCenter = slotOf(cc, in.CostCenter)
	out.CustomerFunction = slotOf(cf, in.CustomerFunction)

	// Identity behave: copy whichever side was resolved into the empty side.
	out.Project, out.BusinessUnit = behave(out.Project, out.BusinessUnit)
	out.App, out.Owner = behave(out.App, out.Owner)
	out.Environment, out.CostCenter = behave(out.Environment, out.CostCenter)
	return out
}

func (c *Catalog) classify(token string, app **appNode, env **envNode, cat **catNode, bu **buNode, owner **ownerNode, cc **ccNode, cf **cfNode) {
	if *app == nil {
		*app = c.matchApp(token)
	}
	if *env == nil {
		*env = c.matchEnv(token)
	}
	if *cat == nil {
		*cat = c.matchCat(token)
	}
	if *bu == nil {
		*bu = c.matchBU(token)
	}
	if *owner == nil {
		*owner = c.matchOwner(token)
	}
	if *cc == nil {
		*cc = c.matchCC(token)
	}
	if *cf == nil {
		*cf = c.matchCF(token)
	}
}

func (c *Catalog) matchProject(raw string) *projectNode {
	if isEmpty(raw) {
		return nil
	}
	if n := one(c.projectByID, raw); n != nil {
		return n.(*projectNode)
	}
	if n := one(c.projectByName, raw); n != nil {
		return n.(*projectNode)
	}
	if n := one(c.buByID, raw); n != nil {
		if p := one(c.projectByID, n.(*buNode).id); p != nil {
			return p.(*projectNode)
		}
	}
	if n := one(c.buByName, raw); n != nil {
		if p := one(c.projectByID, n.(*buNode).id); p != nil {
			return p.(*projectNode)
		}
	}
	return nil
}

func (c *Catalog) matchApp(raw string) *appNode {
	if isEmpty(raw) {
		return nil
	}
	if n := one(c.appByID, raw); n != nil {
		return n.(*appNode)
	}
	if n := one(c.appByName, raw); n != nil {
		return n.(*appNode)
	}
	if n := one(c.ownerByID, raw); n != nil {
		if a := one(c.appByID, n.(*ownerNode).id); a != nil {
			return a.(*appNode)
		}
	}
	if n := one(c.ownerByName, raw); n != nil {
		if a := one(c.appByID, n.(*ownerNode).id); a != nil {
			return a.(*appNode)
		}
	}
	return nil
}

func (c *Catalog) matchEnv(raw string) *envNode {
	if isEmpty(raw) {
		return nil
	}
	if n := one(c.envByID, raw); n != nil {
		return n.(*envNode)
	}
	if n := one(c.envByName, raw); n != nil {
		return n.(*envNode)
	}
	if n := one(c.ccByID, raw); n != nil {
		if e := one(c.envByID, n.(*ccNode).id); e != nil {
			return e.(*envNode)
		}
	}
	if n := one(c.ccByName, raw); n != nil {
		if e := one(c.envByID, n.(*ccNode).id); e != nil {
			return e.(*envNode)
		}
	}
	return nil
}

func (c *Catalog) matchCat(raw string) *catNode {
	if isEmpty(raw) {
		return nil
	}
	if n := one(c.catByID, raw); n != nil {
		return n.(*catNode)
	}
	if n := one(c.catByName, raw); n != nil {
		return n.(*catNode)
	}
	return nil
}

func (c *Catalog) matchBU(raw string) *buNode {
	if isEmpty(raw) {
		return nil
	}
	if n := one(c.buByID, raw); n != nil {
		return n.(*buNode)
	}
	if n := one(c.buByName, raw); n != nil {
		return n.(*buNode)
	}
	return nil
}

func (c *Catalog) matchOwner(raw string) *ownerNode {
	if isEmpty(raw) {
		return nil
	}
	if n := one(c.ownerByID, raw); n != nil {
		return n.(*ownerNode)
	}
	if n := one(c.ownerByName, raw); n != nil {
		return n.(*ownerNode)
	}
	return nil
}

func (c *Catalog) matchCC(raw string) *ccNode {
	if isEmpty(raw) {
		return nil
	}
	if n := one(c.ccByID, raw); n != nil {
		return n.(*ccNode)
	}
	if n := one(c.ccByName, raw); n != nil {
		return n.(*ccNode)
	}
	return nil
}

func (c *Catalog) matchCF(raw string) *cfNode {
	if isEmpty(raw) {
		return nil
	}
	if n := one(c.cfByID, raw); n != nil {
		return n.(*cfNode)
	}
	if n := one(c.cfByName, raw); n != nil {
		return n.(*cfNode)
	}
	return nil
}

func slotOf(node any, raw string) Slot {
	switch n := node.(type) {
	case *projectNode:
		if n != nil {
			return Slot{ID: n.id, Name: n.name}
		}
	case *appNode:
		if n != nil {
			return Slot{ID: n.id, Name: n.name}
		}
	case *envNode:
		if n != nil {
			return Slot{ID: n.id, Name: n.name}
		}
	case *catNode:
		if n != nil {
			return Slot{ID: n.id, Name: n.name}
		}
	case *buNode:
		if n != nil {
			return Slot{ID: n.id, Name: n.name}
		}
	case *ownerNode:
		if n != nil {
			return Slot{ID: n.id, Name: n.name}
		}
	case *ccNode:
		if n != nil {
			return Slot{ID: n.id, Name: n.name}
		}
	case *cfNode:
		if n != nil {
			return Slot{ID: n.id, Name: n.name}
		}
	}
	if isEmpty(raw) {
		return Slot{}
	}
	return Slot{Name: strings.TrimSpace(raw)}
}

func behave(left, right Slot) (Slot, Slot) {
	if left.ID == "" {
		left.ID = right.ID
	}
	if right.ID == "" {
		right.ID = left.ID
	}
	if left.Name == "" {
		left.Name = right.Name
	}
	if right.Name == "" {
		right.Name = left.Name
	}
	return left, right
}

func clean(s string) string {
	if isEmpty(s) {
		return ""
	}
	return strings.TrimSpace(s)
}
