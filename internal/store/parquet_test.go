package store

import (
	"path/filepath"
	"testing"
)

func TestRawParquetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raw.parquet")
	in := []RawRecord{{
		LZ: "LZ-MAIN", AccountID: "1", Month: "2026-09", Metric: "UnblendedCost",
		Amount: 10.5, AppTag: "APP-A", Currency: "USD",
	}}
	if err := WriteRawParquet(path, in); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := ReadRawParquet(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(out) != 1 || out[0].Amount != in[0].Amount {
		t.Fatalf("got %+v", out)
	}
}
