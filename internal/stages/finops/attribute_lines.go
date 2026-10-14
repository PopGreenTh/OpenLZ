package finops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/delta"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

// DefaultAttributionStore holds the per-line attribution results and their history.
const DefaultAttributionStore = "data/openlz_attribution.duckdb"

// AttributeLinesInput models inputs for finops-attribute-lines.
type AttributeLinesInput struct {
	InputPath    string `json:"inputPath"`    // raw FinOps parquet; default data/finops_raw.parquet
	TaxonomyPath string `json:"taxonomyPath"` // default taxonomy/taxonomy.json
	StorePath    string `json:"storePath"`    // default data/openlz_attribution.duckdb
}

// ExecuteAttributeLines keeps the attribution of every raw cost line current (CloudValueDelta,
// docs/taxonomy-spec.md §8): it classifies what changed in the data and in the taxonomy since
// the last run, re-attributes only the lines that need it and reports the cost that moved.
func ExecuteAttributeLines(ctx context.Context, in AttributeLinesInput) (*types.StageResult, error) {
	if in.InputPath == "" {
		in.InputPath = "data/finops_raw.parquet"
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.StorePath == "" {
		in.StorePath = DefaultAttributionStore
	}
	in.StorePath = types.ResolveAbsolutePath(in.StorePath)

	taxPath := taxonomy.ResolvePath(in.TaxonomyPath)
	snap, err := taxonomy.ReadSnapshot(taxPath)
	if err != nil {
		return nil, err
	}

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()
	raw, err := engine.ReadRawFinOpsLines(ctx, in.InputPath)
	if err != nil {
		return nil, err
	}
	lines := make([]delta.Line, len(raw))
	for i, r := range raw {
		lines[i] = delta.FromRaw(r)
	}

	if err := os.MkdirAll(filepath.Dir(in.StorePath), 0o755); err != nil {
		return nil, err
	}
	store, err := delta.Open(ctx, in.StorePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	rep, err := store.Sync(ctx, snap, lines, delta.Options{})
	if err != nil {
		return nil, err
	}
	var report strings.Builder
	rep.Print(&report)
	return &types.StageResult{
		Stage:       types.StageFinOpsAttributeLines,
		Success:     true,
		RecordCount: rep.Reattributed,
		OutputPath:  in.StorePath,
		Data:        rep,
		Message: fmt.Sprintf("Attributed %d of %d cost lines into %s\n%s",
			rep.Reattributed, len(lines), in.StorePath, strings.TrimRight(report.String(), "\n")),
	}, nil
}
