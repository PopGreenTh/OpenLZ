package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type stage struct {
	name string
	run  func(repoRoot string) error
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found in current directory or any parent")
}

func runGoBuild(repoRoot string) error {
	cmd := exec.Command("go", "build", "-tags=no_duckdb_arrow", "./...")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			fmt.Print(string(out))
		}
		return fmt.Errorf("go build: %w", err)
	}
	return nil
}

func runGoVet(repoRoot string) error {
	cmd := exec.Command("go", "vet", "-tags=no_duckdb_arrow", "./...")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			fmt.Print(string(out))
		}
		return fmt.Errorf("go vet: %w", err)
	}
	return nil
}

func runGofmt(repoRoot string) error {
	cmd := exec.Command("gofmt", "-l", ".")
	cmd.Dir = repoRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			fmt.Print(stderr.String())
		}
		return fmt.Errorf("gofmt execution failed: %w", err)
	}
	output := strings.TrimSpace(stdout.String())
	if output != "" {
		fmt.Printf("Unformatted files:\n%s\n", output)
		return fmt.Errorf("unformatted files found")
	}
	return nil
}

func runGoTest(repoRoot string) error {
	cmd := exec.Command("go", "test", "-tags=no_duckdb_arrow", "./...")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			fmt.Print(string(out))
		}
		return fmt.Errorf("go test: %w", err)
	}
	return nil
}

func main() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error locating repository root: %v\n", err)
		os.Exit(1)
	}

	stages := []stage{
		{name: "go build", run: runGoBuild},
		{name: "go vet", run: runGoVet},
		{name: "gofmt", run: runGofmt},
		{name: "go test", run: runGoTest},
	}

	anyFailed := false
	for _, s := range stages {
		if err := s.run(repoRoot); err != nil {
			fmt.Printf("FAIL %s\n", s.name)
			anyFailed = true
		} else {
			fmt.Printf("PASS %s\n", s.name)
		}
	}

	if anyFailed {
		os.Exit(1)
	}
}
