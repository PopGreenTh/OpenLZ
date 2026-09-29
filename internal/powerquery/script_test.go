package powerquery

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRawFinOpsCsvMScript_AbsolutePath(t *testing.T) {
	relPath := filepath.Join("data", "finops_raw.csv")
	mCode := GenerateRawFinOpsCsvMScript(relPath)

	if !strings.Contains(mCode, "Csv.Document(File.Contents(") {
		t.Errorf("expected Csv.Document in M script, got:\n%s", mCode)
	}

	absPath, err := filepath.Abs(relPath)
	if err != nil {
		t.Fatalf("failed getting abs path: %v", err)
	}
	expectedEscaped := strings.ReplaceAll(filepath.Clean(absPath), `\`, `\\`)

	if !strings.Contains(mCode, expectedEscaped) {
		t.Errorf("expected M script to contain absolute escaped path %q, got:\n%s", expectedEscaped, mCode)
	}

	if !strings.Contains(mCode, `Columns=17`) {
		t.Errorf("expected Columns=17 in M script, got:\n%s", mCode)
	}
}

func TestGenerateEnrichedCsvMScript_AbsolutePath(t *testing.T) {
	relPath := filepath.Join("data", "finops_report.csv")
	mCode := GenerateEnrichedCsvMScript(relPath)

	if !strings.Contains(mCode, "Csv.Document(File.Contents(") {
		t.Errorf("expected Csv.Document in M script, got:\n%s", mCode)
	}

	absPath, err := filepath.Abs(relPath)
	if err != nil {
		t.Fatalf("failed getting abs path: %v", err)
	}
	expectedEscaped := strings.ReplaceAll(filepath.Clean(absPath), `\`, `\\`)

	if !strings.Contains(mCode, expectedEscaped) {
		t.Errorf("expected M script to contain absolute escaped path %q, got:\n%s", expectedEscaped, mCode)
	}

	if !strings.Contains(mCode, `Columns=16`) {
		t.Errorf("expected Columns=16 in M script, got:\n%s", mCode)
	}
}

func TestGenerateParquetMScript_AbsolutePath(t *testing.T) {
	relPath := filepath.Join("data", "finops_report.parquet")
	mCode := GenerateParquetMScript(relPath)

	if !strings.Contains(mCode, "Parquet.Document(File.Contents(") {
		t.Errorf("expected Parquet.Document in M script, got:\n%s", mCode)
	}

	absPath, err := filepath.Abs(relPath)
	if err != nil {
		t.Fatalf("failed getting abs path: %v", err)
	}
	expectedEscaped := strings.ReplaceAll(filepath.Clean(absPath), `\`, `\\`)

	if !strings.Contains(mCode, expectedEscaped) {
		t.Errorf("expected M script to contain absolute escaped path %q, got:\n%s", expectedEscaped, mCode)
	}
}
