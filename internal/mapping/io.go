package mapping

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

// WriteAccountMap writes the account map document to path.
func WriteAccountMap(path string, doc *AccountMapDocument) error {
	if doc == nil {
		return fmt.Errorf("account map document is nil")
	}
	normalized := *doc
	sortAllocations(normalized.Allocations)
	sortFindings(normalized.Findings)
	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal account map: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write account map: %w", err)
	}
	return nil
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

func sortAllocations(allocs []Allocation) {
	sort.Slice(allocs, func(i, j int) bool {
		if allocs[i].AccountID != allocs[j].AccountID {
			return allocs[i].AccountID < allocs[j].AccountID
		}
		return allocs[i].App < allocs[j].App
	})
}

// BuildAccountMapDocument constructs the on-disk document from resolver output.
func BuildAccountMapDocument(tax *taxonomy.Taxonomy, inv Inventory, result ResolveResult, generatedAt time.Time) (*AccountMapDocument, error) {
	hash, err := TaxonomyHash(tax)
	if err != nil {
		return nil, err
	}
	findings := result.Findings
	if result.Conflict != nil {
		findings = append(findings, *result.Conflict)
		sortFindings(findings)
	}
	return &AccountMapDocument{
		GeneratedAt:          generatedAt.UTC().Truncate(time.Second),
		TaxonomyHash:         hash,
		InventoryCollectedAt: inv.CollectedAt.UTC().Truncate(time.Second),
		Allocations:          result.Allocations,
		Findings:             findings,
	}, nil
}
