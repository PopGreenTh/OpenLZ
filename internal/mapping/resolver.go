package mapping

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

type bindKey struct {
	orgID string
	ouID  string
}

type ouFields struct {
	app         string
	appRule     string
	environment string
	envRule     string
	appSet      bool
	envSet      bool
}

// Resolve maps every inventory account using tax and inv.
func Resolve(tax *taxonomy.Taxonomy, src Source) ResolveResult {
	var findings []Finding
	findings = append(findings, src.PathFindings()...)

	bindings, linkFindings, conflict := buildOULinkBindings(tax, src)
	findings = append(findings, linkFindings...)
	if conflict != nil {
		return ResolveResult{Findings: findings, Conflict: conflict}
	}

	discovered := map[string]struct{}{}
	for _, org := range src.Organizations() {
		idx := src.OrgIndex(org)
		for id := range idx.AccountIDs() {
			discovered[id] = struct{}{}
		}
	}
	for _, entry := range tax.Accounts {
		if entry == nil {
			continue
		}
		if _, ok := discovered[entry.Account]; !ok {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Code:     "account-not-discovered",
				Subject:  entry.Account,
				Message:  fmt.Sprintf("account entry %s is not in the inventory", entry.Account),
			})
		}
	}

	var allocations []Allocation
	for _, org := range src.Organizations() {
		idx := src.OrgIndex(org)
		lz, lzOK := tax.LZByManagementAccount(org.ManagementAccount)
		if !lzOK {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Code:     "lz-unknown",
				Subject:  org.ID,
				Message:  fmt.Sprintf("no LZ for management account %s", org.ManagementAccount),
			})
			for _, acct := range org.Accounts {
				allocations = append(allocations, unallocAllocation(lzCodeUnknown, acct, idx.AccountPath(acct.ID)))
			}
			continue
		}

		for _, acct := range org.Accounts {
			allocs, acctFindings := resolveAccount(tax, org.ID, lz, acct, idx, bindings)
			allocations = append(allocations, allocs...)
			findings = append(findings, acctFindings...)
		}
	}

	sort.Slice(allocations, func(i, j int) bool {
		if allocations[i].AccountID != allocations[j].AccountID {
			return allocations[i].AccountID < allocations[j].AccountID
		}
		return allocations[i].App < allocations[j].App
	})
	sortFindings(findings)
	return ResolveResult{Allocations: allocations, Findings: findings}
}

const lzCodeUnknown = taxonomy.LZUnalloc

func buildOULinkBindings(tax *taxonomy.Taxonomy, src Source) (map[bindKey]ouFields, []Finding, *Finding) {
	bindings := make(map[bindKey]ouFields)
	var findings []Finding

	for _, link := range tax.OULinks {
		if link == nil {
			continue
		}
		matched := 0
		for _, org := range src.Organizations() {
			idx := src.OrgIndex(org)
			ou, ouPath, ok := resolveOULink(idx, link.OU)
			if !ok {
				continue
			}
			matched++
			key := bindKey{orgID: org.ID, ouID: ou.ID}
			fields := bindings[key]
			rulePath := ouPath
			if rulePath == "" {
				rulePath = link.OU
			}
			rule := "ou:" + rulePath

			if link.App != "" {
				if !idx.HasChildOUs(ou.ID) {
					findings = append(findings, Finding{
						Severity: SeverityWarning,
						Code:     "app-on-leaf-ou",
						Subject:  linkSubject(link.OU, rulePath),
						Message:  fmt.Sprintf("app link on leaf OU %s", rulePath),
					})
				} else {
					if fields.appSet {
						conflict := Finding{
							Severity: SeverityError,
							Code:     "ou-link-conflict",
							Subject:  ou.ID,
							Message:  fmt.Sprintf("conflicting app links for OU %s", ou.ID),
						}
						return bindings, findings, &conflict
					}
					fields.app = link.App
					fields.appRule = rule
					fields.appSet = true
				}
			}
			if link.Environment != "" {
				if fields.envSet {
					conflict := Finding{
						Severity: SeverityError,
						Code:     "ou-link-conflict",
						Subject:  ou.ID,
						Message:  fmt.Sprintf("conflicting environment links for OU %s", ou.ID),
					}
					return bindings, findings, &conflict
				}
				fields.environment = link.Environment
				fields.envRule = rule
				fields.envSet = true
			}
			bindings[key] = fields
		}
		if matched == 0 {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Code:     "ou-not-found",
				Subject:  link.OU,
				Message:  fmt.Sprintf("OU link %q matches no OU", link.OU),
			})
		}
	}
	return bindings, findings, nil
}

func linkSubject(written, resolvedPath string) string {
	if strings.HasPrefix(written, "/") {
		return written
	}
	if resolvedPath != "" {
		return resolvedPath
	}
	return written
}

func resolveOULink(idx *OrgIndex, ref string) (OU, string, bool) {
	if strings.HasPrefix(ref, "/") {
		ou, ok := idx.OUByPath(ref)
		return ou, ref, ok
	}
	ou, ok := idx.OUByID(ref)
	if !ok {
		return OU{}, "", false
	}
	path := idx.OUPath(ou.ID)
	return ou, path, true
}

func accountEntry(tax *taxonomy.Taxonomy, accountID string) *taxonomy.AccountEntry {
	for _, e := range tax.Accounts {
		if e != nil && e.Account == accountID {
			return e
		}
	}
	return nil
}

func resolveAccount(
	tax *taxonomy.Taxonomy,
	orgID string,
	lz *taxonomy.LZ,
	acct Account,
	idx *OrgIndex,
	bindings map[bindKey]ouFields,
) ([]Allocation, []Finding) {
	var findings []Finding
	entry := accountEntry(tax, acct.ID)
	ouPath := idx.AccountPath(acct.ID)
	chain, _ := idx.AccountOUChain(acct.ID)

	type appChoice struct {
		code string
		rule string
	}
	var apps []appChoice

	if entry != nil && len(entry.Apps) > 0 {
		for _, a := range entry.Apps {
			if a == nil {
				continue
			}
			apps = append(apps, appChoice{a.App, "account"})
		}
	} else if code, rule, ok := deepestBoundField(chain, orgID, bindings, true); ok {
		apps = []appChoice{{code, rule}}
	} else if lz.Default != nil && lz.Default.App != "" {
		apps = []appChoice{{lz.Default.App, "lz-default"}}
	} else if tax.Defaults != nil && tax.Defaults.App != "" {
		apps = []appChoice{{tax.Defaults.App, "default"}}
	} else {
		apps = []appChoice{{taxonomy.APPUnalloc, "unalloc"}}
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Code:     "account-unmapped",
			Subject:  acct.ID,
			Message:  fmt.Sprintf("account %s has no App mapping", acct.ID),
		})
	}

	envCode, envRule, envFromAccount := resolveEnvironment(tax, lz, entry, chain, orgID, bindings, apps[0].code)
	if envRule == "unspec" && apps[0].code != taxonomy.APPUnalloc {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Code:     "environment-unresolved",
			Subject:  acct.ID,
			Message:  fmt.Sprintf("account %s has no Environment mapping", acct.ID),
		})
	}

	var allocations []Allocation
	multiApp := len(apps) > 1
	for _, app := range apps {
		finalEnv := envCode
		finalEnvRule := envRule
		if !envFromAccount && app.code != taxonomy.APPUnalloc {
			if _, ok := tax.EnvironmentByAppAndCode(app.code, envCode); envCode != taxonomy.ENVUnspec && !ok {
				finalEnv = taxonomy.ENVUnspec
				finalEnvRule = "unspec"
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Code:     "environment-not-in-app",
					Subject:  acct.ID + "/" + app.code,
					Message:  fmt.Sprintf("environment %s is not defined under app %s", envCode, app.code),
				})
			}
		}

		alloc := buildAllocation(tax, lz.Code, acct, ouPath, app.code, app.rule, finalEnv, finalEnvRule, multiApp)
		if app.code != taxonomy.APPUnalloc {
			if prj, ok := tax.ProjectByCode(alloc.Project); ok && len(prj.LZs) > 0 {
				if !contains(prj.LZs, lz.Code) {
					findings = append(findings, Finding{
						Severity: SeverityWarning,
						Code:     "project-lz-unlinked",
						Subject:  acct.ID + "/" + prj.Code,
						Message:  fmt.Sprintf("project %s is not linked to LZ %s", prj.Code, lz.Code),
					})
				}
			}
		}
		allocations = append(allocations, alloc)
	}
	return allocations, findings
}

func resolveEnvironment(
	tax *taxonomy.Taxonomy,
	lz *taxonomy.LZ,
	entry *taxonomy.AccountEntry,
	chain []OU,
	orgID string,
	bindings map[bindKey]ouFields,
	firstApp string,
) (code, rule string, fromAccount bool) {
	if entry != nil && entry.Environment != "" {
		return entry.Environment, "account", true
	}
	if env, envRule, ok := deepestBoundField(chain, orgID, bindings, false); ok {
		return env, envRule, false
	}
	if lz.Default != nil && lz.Default.Environment != "" {
		return lz.Default.Environment, "lz-default", false
	}
	if tax.Defaults != nil && tax.Defaults.Environment != "" {
		return tax.Defaults.Environment, "default", false
	}
	if firstApp != taxonomy.APPUnalloc {
		return taxonomy.ENVUnspec, "unspec", false
	}
	return taxonomy.ENVUnalloc, "unalloc", false
}

func deepestBoundField(chain []OU, orgID string, bindings map[bindKey]ouFields, appField bool) (string, string, bool) {
	for i := len(chain) - 1; i >= 0; i-- {
		fields := bindings[bindKey{orgID: orgID, ouID: chain[i].ID}]
		if appField {
			if fields.app != "" {
				return fields.app, fields.appRule, true
			}
		} else if fields.environment != "" {
			return fields.environment, fields.envRule, true
		}
	}
	return "", "", false
}

func buildAllocation(
	tax *taxonomy.Taxonomy,
	lzCode string,
	acct Account,
	ouPath string,
	appCode string,
	appRule string,
	envCode string,
	envRule string,
	multiApp bool,
) Allocation {
	share := shareOne()
	shareRule := "single"
	if multiApp {
		share = nil
		shareRule = "tag"
	}

	project := taxonomy.PRJUnalloc
	bu := taxonomy.BUUnalloc
	cc := taxonomy.CCUnalloc
	if app, ok := tax.AppByCode(appCode); ok && app != nil {
		cc = app.CostCenter
		if prj, ok := tax.ProjectByCode(app.Project); ok && prj != nil {
			project = prj.Code
			bu = prj.BusinessUnit
		}
	}

	return Allocation{
		LZ:              lzCode,
		AccountID:       acct.ID,
		AccountName:     acct.Name,
		OUPath:          ouPath,
		Project:         project,
		App:             appCode,
		Environment:     envCode,
		BusinessUnit:    bu,
		CostCenter:      cc,
		Share:           share,
		ShareRule:       shareRule,
		AppRule:         appRule,
		EnvironmentRule: envRule,
	}
}

func unallocAllocation(lzCode string, acct Account, ouPath string) Allocation {
	return Allocation{
		LZ:              lzCode,
		AccountID:       acct.ID,
		AccountName:     acct.Name,
		OUPath:          ouPath,
		Project:         taxonomy.PRJUnalloc,
		App:             taxonomy.APPUnalloc,
		Environment:     taxonomy.ENVUnalloc,
		BusinessUnit:    taxonomy.BUUnalloc,
		CostCenter:      taxonomy.CCUnalloc,
		Share:           shareOne(),
		ShareRule:       "single",
		AppRule:         "unalloc",
		EnvironmentRule: "unalloc",
	}
}

func shareOne() *float64 {
	v := 1.0
	return &v
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		return findings[i].Subject < findings[j].Subject
	})
}
