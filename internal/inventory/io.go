package inventory

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// ReadFile loads an inventory document from JSON.
func ReadFile(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read inventory: %w", err)
	}
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse inventory: %w", err)
	}
	return &doc, nil
}

// WriteFile writes doc to path, creating parent directories if needed.
func WriteFile(path string, doc *Document) error {
	if doc == nil {
		return fmt.Errorf("inventory document is nil")
	}
	normalized := NormalizeDocument(*doc)
	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal inventory: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write inventory: %w", err)
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

// NormalizeDocument returns a copy with OUs and accounts sorted by ID.
func NormalizeDocument(doc Document) Document {
	out := doc
	for i := range out.Organizations {
		sort.Slice(out.Organizations[i].OUs, func(a, b int) bool {
			return out.Organizations[i].OUs[a].ID < out.Organizations[i].OUs[b].ID
		})
		sort.Slice(out.Organizations[i].Accounts, func(a, b int) bool {
			return out.Organizations[i].Accounts[a].ID < out.Organizations[i].Accounts[b].ID
		})
	}
	return out
}
