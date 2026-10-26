package mapping

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

// TaxonomyHash returns SHA-256 of the merged taxonomy in canonical JSON.
func TaxonomyHash(tax *taxonomy.Taxonomy) (string, error) {
	if tax == nil {
		return "", nil
	}
	doc := toCanonicalTaxonomy(tax)
	data, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

type canonicalTaxonomy struct {
	BusinessUnits  []canonicalBU            `json:"business_units,omitempty"`
	CostCenters    []canonicalCC            `json:"cost_centers,omitempty"`
	Projects       []canonicalProject       `json:"projects,omitempty"`
	Apps           []canonicalApp           `json:"apps,omitempty"`
	LZs            []canonicalLZ            `json:"lzs,omitempty"`
	Defaults       *taxonomy.Defaults       `json:"defaults,omitempty"`
	OULinks        []canonicalOULink        `json:"ou_links,omitempty"`
	Accounts       []canonicalAccount       `json:"accounts,omitempty"`
	CostAllocation *taxonomy.CostAllocation `json:"cost_allocation,omitempty"`
}

type canonicalBU struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type canonicalCC struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	BusinessUnit string `json:"business_unit"`
}

type canonicalProject struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	BusinessUnit string   `json:"business_unit"`
	LZs          []string `json:"lzs,omitempty"`
}

type canonicalApp struct {
	Code         string                 `json:"code"`
	Name         string                 `json:"name"`
	Project      string                 `json:"project"`
	CostCenter   string                 `json:"cost_center"`
	Environments []taxonomy.Environment `json:"environments,omitempty"`
}

type canonicalLZ struct {
	Code              string              `json:"code"`
	Name              string              `json:"name"`
	CSP               string              `json:"csp"`
	Currency          string              `json:"currency"`
	ManagementAccount string              `json:"management_account"`
	Default           *taxonomy.LZDefault `json:"default,omitempty"`
}

type canonicalOULink struct {
	OU          string `json:"ou"`
	App         string `json:"app,omitempty"`
	Environment string `json:"environment,omitempty"`
}

type canonicalAccount struct {
	Account     string                `json:"account"`
	Environment string                `json:"environment,omitempty"`
	Apps        []taxonomy.AccountApp `json:"apps,omitempty"`
}

func toCanonicalTaxonomy(tax *taxonomy.Taxonomy) canonicalTaxonomy {
	out := canonicalTaxonomy{
		Defaults:       tax.Defaults,
		CostAllocation: tax.CostAllocation,
	}
	for _, bu := range tax.BusinessUnits {
		if bu == nil {
			continue
		}
		out.BusinessUnits = append(out.BusinessUnits, canonicalBU{Code: bu.Code, Name: bu.Name})
	}
	sort.Slice(out.BusinessUnits, func(i, j int) bool { return out.BusinessUnits[i].Code < out.BusinessUnits[j].Code })

	for _, cc := range tax.CostCenters {
		if cc == nil {
			continue
		}
		out.CostCenters = append(out.CostCenters, canonicalCC{Code: cc.Code, Name: cc.Name, BusinessUnit: cc.BusinessUnit})
	}
	sort.Slice(out.CostCenters, func(i, j int) bool { return out.CostCenters[i].Code < out.CostCenters[j].Code })

	for _, prj := range tax.Projects {
		if prj == nil {
			continue
		}
		lzs := append([]string(nil), prj.LZs...)
		sort.Strings(lzs)
		out.Projects = append(out.Projects, canonicalProject{Code: prj.Code, Name: prj.Name, BusinessUnit: prj.BusinessUnit, LZs: lzs})
	}
	sort.Slice(out.Projects, func(i, j int) bool { return out.Projects[i].Code < out.Projects[j].Code })

	for _, app := range tax.Apps {
		if app == nil {
			continue
		}
		envs := make([]taxonomy.Environment, 0, len(app.Environments))
		for _, e := range app.Environments {
			if e != nil {
				envs = append(envs, *e)
			}
		}
		sort.Slice(envs, func(i, j int) bool { return envs[i].Code < envs[j].Code })
		out.Apps = append(out.Apps, canonicalApp{Code: app.Code, Name: app.Name, Project: app.Project, CostCenter: app.CostCenter, Environments: envs})
	}
	sort.Slice(out.Apps, func(i, j int) bool { return out.Apps[i].Code < out.Apps[j].Code })

	for _, lz := range tax.LZs {
		if lz == nil {
			continue
		}
		out.LZs = append(out.LZs, canonicalLZ{Code: lz.Code, Name: lz.Name, CSP: lz.CSP, Currency: lz.Currency, ManagementAccount: lz.ManagementAccount, Default: lz.Default})
	}
	sort.Slice(out.LZs, func(i, j int) bool { return out.LZs[i].Code < out.LZs[j].Code })

	for _, link := range tax.OULinks {
		if link == nil {
			continue
		}
		out.OULinks = append(out.OULinks, canonicalOULink{OU: link.OU, App: link.App, Environment: link.Environment})
	}
	sort.Slice(out.OULinks, func(i, j int) bool { return out.OULinks[i].OU < out.OULinks[j].OU })

	for _, acct := range tax.Accounts {
		if acct == nil {
			continue
		}
		apps := make([]taxonomy.AccountApp, 0, len(acct.Apps))
		for _, a := range acct.Apps {
			if a != nil {
				apps = append(apps, *a)
			}
		}
		sort.Slice(apps, func(i, j int) bool { return apps[i].App < apps[j].App })
		out.Accounts = append(out.Accounts, canonicalAccount{Account: acct.Account, Environment: acct.Environment, Apps: apps})
	}
	sort.Slice(out.Accounts, func(i, j int) bool { return out.Accounts[i].Account < out.Accounts[j].Account })

	return out
}
