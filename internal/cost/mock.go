package cost

import (
	"encoding/json"
	"fmt"
	"os"
)

const defaultMockCostPath = "testdata/cost/demo.json"

// LoadMockRaw reads deterministic demo cost rows.
func LoadMockRaw(path string) ([]RawRow, error) {
	if path == "" {
		path = defaultMockCostPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mock cost: %w", err)
	}
	var rows []RawRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("parse mock cost: %w", err)
	}
	return rows, nil
}
