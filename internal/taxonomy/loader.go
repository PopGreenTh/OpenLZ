package taxonomy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var knownTopLevelKeys = map[string]bool{
	"business_units":  true,
	"cost_centers":    true,
	"projects":        true,
	"apps":            true,
	"lzs":             true,
	"defaults":        true,
	"ou_links":        true,
	"accounts":        true,
	"cost_allocation": true,
}

// LoadDir reads all *.yaml files in dir and merges their top-level keys into a Taxonomy.
// Any file parsing errors, unknown keys, or duplicate top-level keys across files are returned as findings.
func LoadDir(dir string) (*Taxonomy, []Finding, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("read taxonomy directory %s: %w", dir, err)
	}

	var yamlFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			yamlFiles = append(yamlFiles, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(yamlFiles)

	return LoadFiles(yamlFiles)
}

// LoadFiles reads the specified YAML files and merges their top-level keys into a Taxonomy.
func LoadFiles(files []string) (*Taxonomy, []Finding, error) {
	tax := &Taxonomy{}
	var findings []Finding
	seenKeys := make(map[string]string)

	for _, file := range files {
		relPath := filepath.ToSlash(file)
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, nil, fmt.Errorf("read file %s: %w", file, err)
		}

		var rootNode yaml.Node
		if err := yaml.Unmarshal(data, &rootNode); err != nil {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "yaml-invalid",
				Path:     "",
				Message:  fmt.Sprintf("failed to parse YAML in %s: %v", relPath, err),
				File:     relPath,
			})
			continue
		}

		// Empty file or comments only
		if len(rootNode.Content) == 0 {
			continue
		}

		doc := rootNode.Content[0]
		if doc.Kind != yaml.MappingNode {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Code:     "yaml-invalid",
				Path:     "",
				Message:  fmt.Sprintf("expected top-level mapping in %s", relPath),
				File:     relPath,
			})
			continue
		}

		for i := 0; i < len(doc.Content); i += 2 {
			keyNode := doc.Content[i]
			valNode := doc.Content[i+1]
			key := keyNode.Value

			if !knownTopLevelKeys[key] {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "yaml-invalid",
					Path:     key,
					Message:  fmt.Sprintf("unknown top-level key %q", key),
					File:     relPath,
				})
				continue
			}

			if prevFile, exists := seenKeys[key]; exists {
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "yaml-invalid",
					Path:     key,
					Message:  fmt.Sprintf("key %q appears in two files (%s and %s)", key, prevFile, relPath),
					File:     relPath,
				})
				continue
			}
			seenKeys[key] = relPath

			switch key {
			case "business_units":
				var bus []*BusinessUnit
				if err := valNode.Decode(&bus); err != nil {
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     key,
						Message:  fmt.Sprintf("invalid business_units: %v", err),
						File:     relPath,
					})
				} else {
					for _, bu := range bus {
						if bu != nil {
							bu.File = relPath
						}
					}
					tax.BusinessUnits = bus
				}

			case "cost_centers":
				var ccs []*CostCenter
				if err := valNode.Decode(&ccs); err != nil {
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     key,
						Message:  fmt.Sprintf("invalid cost_centers: %v", err),
						File:     relPath,
					})
				} else {
					for _, cc := range ccs {
						if cc != nil {
							cc.File = relPath
						}
					}
					tax.CostCenters = ccs
				}

			case "projects":
				var prjs []*Project
				if err := valNode.Decode(&prjs); err != nil {
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     key,
						Message:  fmt.Sprintf("invalid projects: %v", err),
						File:     relPath,
					})
				} else {
					for _, prj := range prjs {
						if prj != nil {
							prj.File = relPath
						}
					}
					tax.Projects = prjs
				}

			case "apps":
				var apps []*App
				if err := valNode.Decode(&apps); err != nil {
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     key,
						Message:  fmt.Sprintf("invalid apps: %v", err),
						File:     relPath,
					})
				} else {
					for _, app := range apps {
						if app != nil {
							app.File = relPath
						}
					}
					tax.Apps = apps
				}

			case "lzs":
				var lzs []*LZ
				if err := valNode.Decode(&lzs); err != nil {
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     key,
						Message:  fmt.Sprintf("invalid lzs: %v", err),
						File:     relPath,
					})
				} else {
					for _, lz := range lzs {
						if lz != nil {
							lz.File = relPath
						}
					}
					tax.LZs = lzs
				}

			case "defaults":
				var defaults Defaults
				if err := valNode.Decode(&defaults); err != nil {
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     key,
						Message:  fmt.Sprintf("invalid defaults: %v", err),
						File:     relPath,
					})
				} else {
					defaults.File = relPath
					tax.Defaults = &defaults
				}

			case "ou_links":
				var links []*OULink
				if err := valNode.Decode(&links); err != nil {
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     key,
						Message:  fmt.Sprintf("invalid ou_links: %v", err),
						File:     relPath,
					})
				} else {
					for _, link := range links {
						if link != nil {
							link.File = relPath
						}
					}
					tax.OULinks = links
				}

			case "accounts":
				var accs []*AccountEntry
				if err := valNode.Decode(&accs); err != nil {
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     key,
						Message:  fmt.Sprintf("invalid accounts: %v", err),
						File:     relPath,
					})
				} else {
					for _, acc := range accs {
						if acc != nil {
							acc.File = relPath
						}
					}
					tax.Accounts = accs
				}

			case "cost_allocation":
				var costAlloc CostAllocation
				if err := valNode.Decode(&costAlloc); err != nil {
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     key,
						Message:  fmt.Sprintf("invalid cost_allocation: %v", err),
						File:     relPath,
					})
				} else {
					costAlloc.File = relPath
					tax.CostAllocation = &costAlloc
				}
			}
		}
	}

	return tax, findings, nil
}
