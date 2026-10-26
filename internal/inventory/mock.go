package inventory

import (
	"fmt"
	"os"
	"time"
)

const defaultDemoPath = "testdata/inventory/demo.json"

// LoadDemo reads the demo landing zone fixture from path.
func LoadDemo(path string) (*Document, error) {
	if path == "" {
		path = defaultDemoPath
	}
	doc, err := ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(doc.Organizations) == 0 {
		return nil, fmt.Errorf("demo inventory has no organizations")
	}
	return doc, nil
}

// MockDocument returns a fresh mock inventory from the demo fixture.
func MockDocument(demoPath string) (*Document, error) {
	doc, err := LoadDemo(demoPath)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	out := Document{
		CollectedAt:   now,
		Source:        SourceMock,
		Organizations: doc.Organizations,
	}
	for i := range out.Organizations {
		_, findings := BuildIndex(out.Organizations[i])
		if len(findings) > 0 {
			out.PathFindings = append(out.PathFindings, findings...)
		}
	}
	normalized := NormalizeDocument(out)
	return &normalized, nil
}

// DemoPath returns the default demo fixture path if the file exists from the working directory.
func DemoPath() string {
	if _, err := os.Stat(defaultDemoPath); err == nil {
		return defaultDemoPath
	}
	return defaultDemoPath
}
