package cost

import "testing"

func TestParseTagGroupKey(t *testing.T) {
	if got := ParseTagGroupKey("Application$APP-CHK"); got != "APP-CHK" {
		t.Fatalf("got %q", got)
	}
	if got := ParseTagGroupKey("Application$"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestTagMatchesApp(t *testing.T) {
	if !TagMatchesApp("app-b", "APP-B") {
		t.Fatal("expected case-insensitive match")
	}
}
