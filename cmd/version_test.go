package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCmd(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error executing version command: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got != "dev" {
		t.Fatalf("expected version %q, got %q", "dev", got)
	}
}
