package taxonomy

import (
	"bytes"
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

func decodeStrict(node *yaml.Node, out interface{}) error {
	data, err := yaml.Marshal(node)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(out)
}

func extractUnknownField(err error) string {
	msg := err.Error()
	if idx := strings.Index(msg, "field "); idx != -1 {
		rest := msg[idx+len("field "):]
		if endIdx := strings.Index(rest, " not found"); endIdx != -1 {
			return rest[:endIdx]
		}
	}
	return ""
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
				var msg string
				if prevFile == relPath {
					msg = fmt.Sprintf("top-level key %q appears twice in %s", key, relPath)
				} else {
					msg = fmt.Sprintf("key %q appears in two files (%s and %s)", key, prevFile, relPath)
				}
				findings = append(findings, Finding{
					Severity: SeverityError,
					Code:     "yaml-invalid",
					Path:     key,
					Message:  msg,
					File:     relPath,
				})
				continue
			}
			seenKeys[key] = relPath

			switch key {
			case "business_units":
				var bus []*BusinessUnit
				if err := decodeStrict(valNode, &bus); err != nil {
					p := key
					if f := extractUnknownField(err); f != "" {
						p = key + "." + f
					}
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     p,
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
				if err := decodeStrict(valNode, &ccs); err != nil {
					p := key
					if f := extractUnknownField(err); f != "" {
						p = key + "." + f
					}
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     p,
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
				if err := decodeStrict(valNode, &prjs); err != nil {
					p := key
					if f := extractUnknownField(err); f != "" {
						p = key + "." + f
					}
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     p,
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
				if err := decodeStrict(valNode, &apps); err != nil {
					p := key
					if f := extractUnknownField(err); f != "" {
						p = key + "." + f
					}
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     p,
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
				if err := decodeStrict(valNode, &lzs); err != nil {
					p := key
					if f := extractUnknownField(err); f != "" {
						p = key + "." + f
					}
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     p,
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
				if err := decodeStrict(valNode, &defaults); err != nil {
					p := key
					if f := extractUnknownField(err); f != "" {
						p = key + "." + f
					}
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     p,
						Message:  fmt.Sprintf("invalid defaults: %v", err),
						File:     relPath,
					})
				} else {
					defaults.File = relPath
					tax.Defaults = &defaults
				}

			case "ou_links":
				var links []*OULink
				if err := decodeStrict(valNode, &links); err != nil {
					p := key
					if f := extractUnknownField(err); f != "" {
						p = key + "." + f
					}
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     p,
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
				if err := decodeStrict(valNode, &accs); err != nil {
					p := key
					if f := extractUnknownField(err); f != "" {
						p = key + "." + f
					}
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     p,
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
				if err := decodeStrict(valNode, &costAlloc); err != nil {
					p := key
					if f := extractUnknownField(err); f != "" {
						p = key + "." + f
					}
					findings = append(findings, Finding{
						Severity: SeverityError,
						Code:     "yaml-invalid",
						Path:     p,
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
