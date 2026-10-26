package inventory

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMockDocumentStableExceptCollectedAt(t *testing.T) {
	demo := "../../testdata/inventory/demo.json"
	a, err := MockDocument(demo)
	if err != nil {
		t.Fatalf("MockDocument: %v", err)
	}
	b, err := MockDocument(demo)
	if err != nil {
		t.Fatalf("MockDocument: %v", err)
	}
	if a.Source != SourceMock || b.Source != SourceMock {
		t.Fatalf("source = %q / %q", a.Source, b.Source)
	}
	a.CollectedAt = time.Time{}
	b.CollectedAt = time.Time{}
	ab, _ := json.Marshal(NormalizeDocument(*a))
	bb, _ := json.Marshal(NormalizeDocument(*b))
	if string(ab) != string(bb) {
		t.Fatalf("documents differ apart from collected_at:\n%s\n---\n%s", ab, bb)
	}
}
