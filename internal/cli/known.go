package cli

import (
	"path/filepath"
	"sort"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

type knownCatalog struct {
	Name   string
	Source string
	Kind   string
}

// knownCatalogs collects registries Repertoire can complete without network I/O:
// built-in, global and project registrations, bootstrap catalogs, lock sources,
// and already-cached remotes.
func knownCatalogs(activeGlobal, activeProject bool, cacheRoot string) []knownCatalog {
	byName := map[string]knownCatalog{}
	remember := func(name, source, kind string) {
		if name == "" || source == "" {
			return
		}
		source = catalog.RedactSource(catalog.NormalizeSource(source))
		if existing, ok := byName[name]; ok && kindRank(kind) >= kindRank(existing.Kind) {
			return
		}
		byName[name] = knownCatalog{Name: name, Source: source, Kind: kind}
	}

	remember(catalog.BuiltinName, catalog.BuiltinSource, "built-in")

	if global, err := state.ResolveScope(state.ScopeOptions{Global: true}); err == nil {
		kind := "global"
		if activeGlobal || (!activeGlobal && !activeProject) {
			kind = "registered"
		}
		addManifestCatalogs(remember, global, kind)
		addLockCatalogs(remember, global)
	}

	if project, err := state.ResolveScope(state.ScopeOptions{Project: true}); err == nil {
		kind := "project"
		if activeProject {
			kind = "registered"
		}
		addManifestCatalogs(remember, project, kind)
		addLockCatalogs(remember, project)
		addBootstrapCatalogs(remember, project.Root)
	}

	if manager, err := catalog.NewManager(cacheRoot); err == nil {
		if cached, err := manager.ListCached(); err == nil {
			for _, source := range cached {
				remember(source.Name, source.Registration.Source, "cached")
			}
		}
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]knownCatalog, 0, len(names))
	for _, name := range names {
		result = append(result, byName[name])
	}
	return result
}

func kindRank(kind string) int {
	switch kind {
	case "built-in":
		return 0
	case "registered":
		return 1
	case "project", "global":
		return 2
	case "bootstrap":
		return 3
	case "cached":
		return 4
	default:
		return 5
	}
}

func addManifestCatalogs(remember func(string, string, string), scope state.Scope, kind string) {
	manifest, err := state.LoadManifest(scope.ManifestPath)
	if err != nil {
		return
	}
	for name, registration := range manifest.Catalogs {
		remember(name, registration.Source, kind)
	}
}

func addLockCatalogs(remember func(string, string, string), scope state.Scope) {
	lock, err := state.LoadLock(scope.LockPath)
	if err != nil {
		return
	}
	for _, skill := range lock.Skills {
		if skill.Catalog == "" || skill.Source == "" {
			continue
		}
		remember(skill.Catalog, skill.Source, "cached")
	}
}

func addBootstrapCatalogs(remember func(string, string, string), projectRoot string) {
	bootstrap, err := state.LoadBootstrapManifest(filepath.Join(projectRoot, state.BootstrapFileName))
	if err != nil {
		return
	}
	for name, registration := range bootstrap.Catalogs {
		remember(name, registration.Source, "bootstrap")
	}
}

func knownManifest(activeGlobal, activeProject bool, cacheRoot string) state.Manifest {
	manifest := state.NewManifest()
	builtinSource := catalog.RedactSource(catalog.NormalizeSource(catalog.BuiltinSource))
	for _, known := range knownCatalogs(activeGlobal, activeProject, cacheRoot) {
		if known.Name == catalog.BuiltinName && known.Source == builtinSource {
			// Keep the built-in visible through catalog.Sources unless overridden.
			continue
		}
		manifest.Catalogs[known.Name] = state.CatalogRegistration{Source: known.Source}
	}
	return manifest
}
