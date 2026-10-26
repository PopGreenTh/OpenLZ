package taxonomy

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	codeRegex     = regexp.MustCompile(`^[A-Z][A-Z0-9-]{1,31}$`)
	currencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)
	accountRegex  = regexp.MustCompile(`^[0-9]{12}$`)
)

func isCodeValid(code string) bool {
	if !codeRegex.MatchString(code) {
		return false
	}
	if strings.HasSuffix(code, "-UNALLOC") || strings.HasSuffix(code, "-UNSPEC") || strings.HasSuffix(code, "-INVALID") {
		return false
	}
	return true
}

// Validate executes all validation rules against the taxonomy and returns findings.
// It never stops at the first finding, returning all errors and warnings discovered.
func Validate(t *Taxonomy) []Finding {
	var findings []Finding

	buMap := make(map[string]*BusinessUnit)
	seenBUCodes := make(map[string]bool)
	referencedBUs := make(map[string]bool)

	ccMap := make(map[string]*CostCenter)
	seenCCCodes := make(map[string]bool)
	referencedCCs := make(map[string]bool)

	prjMap := make(map[string]*Project)
	seenPRJCodes := make(map[string]bool)

	appMap := make(map[string]*App)
	seenAppCodes := make(map[string]bool)

	lzMap := make(map[string]*LZ)
	seenLZCodes := make(map[string]bool)

	// 1. Business Units
	for _, bu := range t.BusinessUnits {
		if bu == nil {
			continue
		}
		path := fmt.Sprintf("business_units[%s]", bu.Code)
		if !isCodeValid(bu.Code) {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "code-invalid",
				Path:     path + ".code",
				Message:  fmt.Sprintf("business unit code %q is invalid or uses a reserved suffix", bu.Code),
				File:     bu.File,
			})
		}
		if seenBUCodes[bu.Code] {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "code-duplicate",
				Path:     path,
				Message:  fmt.Sprintf("duplicate business unit code %q", bu.Code),
				File:     bu.File,
			})
		} else {
			seenBUCodes[bu.Code] = true
			buMap[bu.Code] = bu
		}
	}

	// 2. Landing Zones
	for _, lz := range t.LZs {
		if lz == nil {
			continue
		}
		path := fmt.Sprintf("lzs[%s]", lz.Code)
		if !isCodeValid(lz.Code) {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "code-invalid",
				Path:     path + ".code",
				Message:  fmt.Sprintf("landing zone code %q is invalid or uses a reserved suffix", lz.Code),
				File:     lz.File,
			})
		}
		if seenLZCodes[lz.Code] {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "code-duplicate",
				Path:     path,
				Message:  fmt.Sprintf("duplicate landing zone code %q", lz.Code),
				File:     lz.File,
			})
		} else {
			seenLZCodes[lz.Code] = true
			lzMap[lz.Code] = lz
		}

		// lz-invalid checks
		if lz.CSP != "AWS" {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "lz-invalid",
				Path:     path + ".csp",
				Message:  fmt.Sprintf("csp must be AWS, got %q", lz.CSP),
				File:     lz.File,
			})
		}
		if !currencyRegex.MatchString(lz.Currency) {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "lz-invalid",
				Path:     path + ".currency",
				Message:  fmt.Sprintf("currency must be three capital letters (ISO 4217), got %q", lz.Currency),
				File:     lz.File,
			})
		}
		if !accountRegex.MatchString(lz.ManagementAccount) {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "lz-invalid",
				Path:     path + ".management_account",
				Message:  fmt.Sprintf("management_account must be 12 digits, got %q", lz.ManagementAccount),
				File:     lz.File,
			})
		}
	}

	// 3. Cost Centers
	for _, cc := range t.CostCenters {
		if cc == nil {
			continue
		}
		path := fmt.Sprintf("cost_centers[%s]", cc.Code)
		if !isCodeValid(cc.Code) {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "code-invalid",
				Path:     path + ".code",
				Message:  fmt.Sprintf("cost center code %q is invalid or uses a reserved suffix", cc.Code),
				File:     cc.File,
			})
		}
		if seenCCCodes[cc.Code] {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "code-duplicate",
				Path:     path,
				Message:  fmt.Sprintf("duplicate cost center code %q", cc.Code),
				File:     cc.File,
			})
		} else {
			seenCCCodes[cc.Code] = true
			ccMap[cc.Code] = cc
		}

		if cc.BusinessUnit == "" {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "ref-missing",
				Path:     path + ".business_unit",
				Message:  "cost center must reference a business unit",
				File:     cc.File,
			})
		} else {
			if !seenBUCodes[cc.BusinessUnit] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "ref-unknown",
					Path:     path + ".business_unit",
					Message:  fmt.Sprintf("referenced business unit %q not found", cc.BusinessUnit),
					File:     cc.File,
				})
			} else {
				referencedBUs[cc.BusinessUnit] = true
			}
		}
	}

	// 4. Projects
	for _, prj := range t.Projects {
		if prj == nil {
			continue
		}
		path := fmt.Sprintf("projects[%s]", prj.Code)
		if !isCodeValid(prj.Code) {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "code-invalid",
				Path:     path + ".code",
				Message:  fmt.Sprintf("project code %q is invalid or uses a reserved suffix", prj.Code),
				File:     prj.File,
			})
		}
		if seenPRJCodes[prj.Code] {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "code-duplicate",
				Path:     path,
				Message:  fmt.Sprintf("duplicate project code %q", prj.Code),
				File:     prj.File,
			})
		} else {
			seenPRJCodes[prj.Code] = true
			prjMap[prj.Code] = prj
		}

		if prj.BusinessUnit == "" {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "ref-missing",
				Path:     path + ".business_unit",
				Message:  "project must reference a business unit",
				File:     prj.File,
			})
		} else {
			if !seenBUCodes[prj.BusinessUnit] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "ref-unknown",
					Path:     path + ".business_unit",
					Message:  fmt.Sprintf("referenced business unit %q not found", prj.BusinessUnit),
					File:     prj.File,
				})
			} else {
				referencedBUs[prj.BusinessUnit] = true
			}
		}

		for _, lzCode := range prj.LZs {
			if !seenLZCodes[lzCode] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "ref-unknown",
					Path:     fmt.Sprintf("%s.lzs[%s]", path, lzCode),
					Message:  fmt.Sprintf("referenced landing zone %q not found", lzCode),
					File:     prj.File,
				})
			}
		}
	}

	// 5. Apps & Environments
	for _, app := range t.Apps {
		if app == nil {
			continue
		}
		path := fmt.Sprintf("apps[%s]", app.Code)
		if !isCodeValid(app.Code) {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "code-invalid",
				Path:     path + ".code",
				Message:  fmt.Sprintf("app code %q is invalid or uses a reserved suffix", app.Code),
				File:     app.File,
			})
		}
		if seenAppCodes[app.Code] {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "code-duplicate",
				Path:     path,
				Message:  fmt.Sprintf("duplicate app code %q", app.Code),
				File:     app.File,
			})
		} else {
			seenAppCodes[app.Code] = true
			appMap[app.Code] = app
		}

		if app.Project == "" {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "ref-missing",
				Path:     path + ".project",
				Message:  "app must reference a project",
				File:     app.File,
			})
		} else if !seenPRJCodes[app.Project] {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "ref-unknown",
				Path:     path + ".project",
				Message:  fmt.Sprintf("referenced project %q not found", app.Project),
				File:     app.File,
			})
		}

		if app.CostCenter == "" {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "ref-missing",
				Path:     path + ".cost_center",
				Message:  "app must reference a cost center",
				File:     app.File,
			})
		} else if !seenCCCodes[app.CostCenter] {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "ref-unknown",
				Path:     path + ".cost_center",
				Message:  fmt.Sprintf("referenced cost center %q not found", app.CostCenter),
				File:     app.File,
			})
		} else {
			referencedCCs[app.CostCenter] = true
		}

		// Environments under app
		seenEnvCodes := make(map[string]bool)
		for _, env := range app.Environments {
			if env == nil {
				continue
			}
			envPath := fmt.Sprintf("%s.environments[%s]", path, env.Code)
			if !isCodeValid(env.Code) {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "code-invalid",
					Path:     envPath + ".code",
					Message:  fmt.Sprintf("environment code %q is invalid or uses a reserved suffix", env.Code),
					File:     app.File,
				})
			}
			if seenEnvCodes[env.Code] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "code-duplicate",
					Path:     envPath,
					Message:  fmt.Sprintf("duplicate environment code %q under app %s", env.Code, app.Code),
					File:     app.File,
				})
			} else {
				seenEnvCodes[env.Code] = true
			}
		}

		// Warning: biz-path-mismatch
		if app.Project != "" && app.CostCenter != "" {
			prj := prjMap[app.Project]
			cc := ccMap[app.CostCenter]
			if prj != nil && cc != nil && prj.BusinessUnit != "" && cc.BusinessUnit != "" {
				if prj.BusinessUnit != cc.BusinessUnit {
					findings = append(findings, Finding{
						Severity: SeverityWarning,
						Code:     "biz-path-mismatch",
						Path:     path + ".cost_center",
						Message:  fmt.Sprintf("app cost center %s is under business unit %s, but project %s is under %s", cc.Code, cc.BusinessUnit, prj.Code, prj.BusinessUnit),
						File:     app.File,
					})
				}
			}
		}
	}

	// 6. LZ defaults & checks
	for _, lz := range t.LZs {
		if lz == nil || lz.Default == nil {
			continue
		}
		path := fmt.Sprintf("lzs[%s].default", lz.Code)
		if lz.Default.App != "" {
			if !seenAppCodes[lz.Default.App] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "ref-unknown",
					Path:     path + ".app",
					Message:  fmt.Sprintf("referenced default app %q not found", lz.Default.App),
					File:     lz.File,
				})
			}
		}
		if lz.Default.App != "" && lz.Default.Environment != "" {
			app := appMap[lz.Default.App]
			if app != nil && !app.HasEnvironment(lz.Default.Environment) {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "default-env-not-in-app",
					Path:     path + ".environment",
					Message:  fmt.Sprintf("default environment %q is not defined under app %q", lz.Default.Environment, app.Code),
					File:     lz.File,
				})
			}
		}
	}

	// 7. Global Defaults
	if t.Defaults != nil {
		path := "defaults"
		if t.Defaults.App != "" {
			if !seenAppCodes[t.Defaults.App] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "ref-unknown",
					Path:     path + ".app",
					Message:  fmt.Sprintf("referenced default app %q not found", t.Defaults.App),
					File:     t.Defaults.File,
				})
			}
		}
		if t.Defaults.App != "" && t.Defaults.Environment != "" {
			app := appMap[t.Defaults.App]
			if app != nil && !app.HasEnvironment(t.Defaults.Environment) {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "default-env-not-in-app",
					Path:     path + ".environment",
					Message:  fmt.Sprintf("default environment %q is not defined under app %q", t.Defaults.Environment, app.Code),
					File:     t.Defaults.File,
				})
			}
		}
	}

	// 8. OU Links
	seenOUApp := make(map[string]bool)
	seenOUEnv := make(map[string]bool)
	for _, link := range t.OULinks {
		if link == nil {
			continue
		}
		path := fmt.Sprintf("ou_links[%s]", link.OU)
		if link.App == "" && link.Environment == "" {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "ou-link-empty",
				Path:     path,
				Message:  "OU link must set at least one of app or environment",
				File:     link.File,
			})
			continue
		}

		if link.App != "" {
			if seenOUApp[link.OU] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "ou-link-duplicate",
					Path:     path + ".app",
					Message:  fmt.Sprintf("duplicate app link for OU %q", link.OU),
					File:     link.File,
				})
			} else {
				seenOUApp[link.OU] = true
			}
			if !seenAppCodes[link.App] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "ref-unknown",
					Path:     path + ".app",
					Message:  fmt.Sprintf("referenced app %q not found", link.App),
					File:     link.File,
				})
			}
		}

		if link.Environment != "" {
			if seenOUEnv[link.OU] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "ou-link-duplicate",
					Path:     path + ".environment",
					Message:  fmt.Sprintf("duplicate environment link for OU %q", link.OU),
					File:     link.File,
				})
			} else {
				seenOUEnv[link.OU] = true
			}
		}
	}

	// 9. Accounts
	seenAccounts := make(map[string]bool)
	for _, acc := range t.Accounts {
		if acc == nil {
			continue
		}
		path := fmt.Sprintf("accounts[%s]", acc.Account)

		isInvalid := false
		if !accountRegex.MatchString(acc.Account) {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "account-invalid",
				Path:     path + ".account",
				Message:  fmt.Sprintf("account %q is not 12 digits", acc.Account),
				File:     acc.File,
			})
			isInvalid = true
		}
		if seenAccounts[acc.Account] {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "account-invalid",
				Path:     path,
				Message:  fmt.Sprintf("duplicate account %q listed in accounts", acc.Account),
				File:     acc.File,
			})
			isInvalid = true
		} else {
			seenAccounts[acc.Account] = true
		}

		if acc.Environment == "" && len(acc.Apps) == 0 {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "account-invalid",
				Path:     path,
				Message:  "account entry must set at least one of environment or apps",
				File:     acc.File,
			})
			isInvalid = true
		}

		if isInvalid && !seenAccounts[acc.Account] {
			continue
		}

		seenAccApps := make(map[string]bool)
		for _, accApp := range acc.Apps {
			if accApp == nil {
				continue
			}
			if seenAccApps[accApp.App] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "account-app-duplicate",
					Path:     fmt.Sprintf("%s.apps[%s]", path, accApp.App),
					Message:  fmt.Sprintf("app %q listed multiple times for account %s", accApp.App, acc.Account),
					File:     acc.File,
				})
			} else {
				seenAccApps[accApp.App] = true
			}

			if !seenAppCodes[accApp.App] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "ref-unknown",
					Path:     fmt.Sprintf("%s.apps[%s]", path, accApp.App),
					Message:  fmt.Sprintf("referenced app %q not found", accApp.App),
					File:     acc.File,
				})
			}
		}

		// account-env-not-in-app
		if acc.Environment != "" && len(acc.Apps) > 0 {
			for _, accApp := range acc.Apps {
				if accApp == nil {
					continue
				}
				app := appMap[accApp.App]
				if app != nil && !app.HasEnvironment(acc.Environment) {
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "account-env-not-in-app",
						Path:     path + ".environment",
						Message:  fmt.Sprintf("environment %q is not defined under app %q", acc.Environment, app.Code),
						File:     acc.File,
					})
					break // report once per account
				}
			}
		}
	}

	// 10. Cost Allocation
	if t.CostAllocation != nil {
		tag := t.CostAllocation.AppTag
		if tag == "" || len(tag) > 128 || strings.HasPrefix(strings.ToLower(tag), "aws:") {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "tag-key-invalid",
				Path:     "cost_allocation.app_tag",
				Message:  fmt.Sprintf("tag key %q is invalid (empty, longer than 128 chars, or starts with aws:)", tag),
				File:     t.CostAllocation.File,
			})
		}
	}

	// 11. Warning: unused-node
	// 11a. Unused BusinessUnits
	for _, bu := range t.BusinessUnits {
		if bu == nil {
			continue
		}
		if !referencedBUs[bu.Code] {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Code:     "unused-node",
				Path:     fmt.Sprintf("business_units[%s]", bu.Code),
				Message:  fmt.Sprintf("business unit %q is not referenced by any cost center or project", bu.Code),
				File:     bu.File,
			})
		}
	}

	// 11b. Unused CostCenters
	for _, cc := range t.CostCenters {
		if cc == nil {
			continue
		}
		if !referencedCCs[cc.Code] {
			findings = append(findings, Finding{
				Severity: SeverityWarning,
				Code:     "unused-node",
				Path:     fmt.Sprintf("cost_centers[%s]", cc.Code),
				Message:  fmt.Sprintf("cost center %q is not referenced by any app", cc.Code),
				File:     cc.File,
			})
		}
	}

	// 11c. Unused Environments
	for _, app := range t.Apps {
		if app == nil {
			continue
		}
		for _, env := range app.Environments {
			if env == nil {
				continue
			}
			referenced := false

			// Check LZ defaults
			for _, lz := range t.LZs {
				if lz != nil && lz.Default != nil && lz.Default.Environment == env.Code {
					if lz.Default.App == "" || lz.Default.App == app.Code {
						referenced = true
						break
					}
				}
			}

			// Check Global defaults
			if !referenced && t.Defaults != nil && t.Defaults.Environment == env.Code {
				if t.Defaults.App == "" || t.Defaults.App == app.Code {
					referenced = true
				}
			}

			// Check OU links
			if !referenced {
				for _, link := range t.OULinks {
					if link != nil && link.Environment == env.Code {
						if link.App == "" || link.App == app.Code {
							referenced = true
							break
						}
					}
				}
			}

			// Check Accounts
			if !referenced {
				for _, acc := range t.Accounts {
					if acc != nil && acc.Environment == env.Code {
						if len(acc.Apps) == 0 {
							referenced = true
							break
						}
						for _, accApp := range acc.Apps {
							if accApp != nil && accApp.App == app.Code {
								referenced = true
								break
							}
						}
						if referenced {
							break
						}
					}
				}
			}

			if !referenced {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Code:     "unused-node",
					Path:     fmt.Sprintf("apps[%s].environments[%s]", app.Code, env.Code),
					Message:  fmt.Sprintf("environment %q under app %s is not referenced anywhere", env.Code, app.Code),
					File:     app.File,
				})
			}
		}
	}

	return findings
}
