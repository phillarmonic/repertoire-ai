package catalog

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

// ListCached returns catalogs already present in the local cache. Completion and
// other read-only flows use this to surface registries Repertoire has seen
// before without cloning or refreshing.
func (m *Manager) ListCached() ([]Source, error) {
	entries, err := os.ReadDir(m.CacheRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	sources := make([]Source, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(m.CacheRoot, entry.Name())
		if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
			continue
		}
		if _, err := loadCatalog(root); err != nil {
			continue
		}
		sourceURL, err := gitOutput(root, "remote", "get-url", "origin")
		if err != nil || sourceURL == "" {
			continue
		}
		sources = append(sources, Source{
			Name: entry.Name(),
			Registration: state.CatalogRegistration{
				Source: NormalizeSource(sourceURL),
			},
		})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
	return sources, nil
}
