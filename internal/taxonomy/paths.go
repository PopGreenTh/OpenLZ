package taxonomy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// DefaultDir holds the taxonomy snapshots.
	DefaultDir = "taxonomy"
	// DefaultTenant names the tenant stored in the un-suffixed snapshot.
	DefaultTenant = "default"
	// DefaultPath is the snapshot used when no tenant is given.
	DefaultPath = "taxonomy/taxonomy.json"
)

// PathForTenant returns the snapshot path for a tenant inside dir.
// The empty tenant and "default" map to taxonomy.json.
func PathForTenant(dir, tenant string) string {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultDir
	}
	tenant = strings.TrimSpace(tenant)
	if tenant == "" || tenant == DefaultTenant {
		return filepath.Join(dir, "taxonomy.json")
	}
	return filepath.Join(dir, fmt.Sprintf("taxonomy_%s.json", tenant))
}

// TenantCode returns the Directus tenant code for a CLI tenant value.
func TenantCode(tenant string) string {
	if t := strings.TrimSpace(tenant); t != "" {
		return t
	}
	return DefaultTenant
}

// ResolvePath returns explicitPath when set, otherwise the tenant's default snapshot path.
func ResolvePath(explicitPath, tenant string) string {
	if strings.TrimSpace(explicitPath) != "" {
		return explicitPath
	}
	return PathForTenant(DefaultDir, tenant)
}

// LoadOptional loads a taxonomy snapshot.
// When path is empty, the tenant's default snapshot is used and a missing file means
// "no taxonomy" (nil, nil). An explicit path that does not exist is an error.
func LoadOptional(path, tenant string) (*Catalog, error) {
	explicit := strings.TrimSpace(path) != ""
	path = ResolvePath(path, tenant)
	if _, err := os.Stat(path); err != nil {
		if !explicit && os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cost taxonomy %s: %w", path, err)
	}
	return Load(path)
}

// DiscoverTenants returns the tenants that have a taxonomy_<tenant>.json snapshot in dir.
func DiscoverTenants(dir string) ([]string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultDir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read taxonomy dir %s: %w", dir, err)
	}
	var tenants []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "taxonomy_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		if t := strings.TrimSuffix(strings.TrimPrefix(name, "taxonomy_"), ".json"); t != "" {
			tenants = append(tenants, t)
		}
	}
	sort.Strings(tenants)
	return tenants, nil
}
