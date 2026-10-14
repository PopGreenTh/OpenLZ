package taxonomy

import (
	"fmt"
	"os"
	"strings"
)

// DefaultPath is the single taxonomy snapshot.
const DefaultPath = "taxonomy/taxonomy.json"

// ResolvePath returns explicitPath when set, otherwise the default snapshot path.
func ResolvePath(explicitPath string) string {
	if strings.TrimSpace(explicitPath) != "" {
		return explicitPath
	}
	return DefaultPath
}

// LoadOptional loads a taxonomy snapshot.
// When path is empty, the default snapshot is used and a missing file means
// "no taxonomy" (nil, nil). An explicit path that does not exist is an error.
func LoadOptional(path string) (*Catalog, error) {
	explicit := strings.TrimSpace(path) != ""
	path = ResolvePath(path)
	if _, err := os.Stat(path); err != nil {
		if !explicit && os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cost taxonomy %s: %w", path, err)
	}
	return Load(path)
}
