package cli

import (
	"errors"
	"fmt"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func registerCatalog(
	scope state.Scope,
	manifest *state.Manifest,
	manager *catalog.Manager,
	source catalog.Source,
	force bool,
) error {
	prepared, err := prepareCatalogRegistration(manifest, source, force)
	if err != nil {
		return err
	}
	if _, err := manager.Materialize(prepared, true); err != nil {
		return err
	}
	manifest.Catalogs[prepared.Name] = prepared.Registration
	return state.SaveManifest(scope.ManifestPath, *manifest)
}

func prepareCatalogRegistration(manifest *state.Manifest, source catalog.Source, force bool) (catalog.Source, error) {
	normalized := catalog.NormalizeSource(source.Registration.Source)
	if catalog.RedactSource(normalized) != normalized {
		return catalog.Source{}, errors.New("catalog URLs must not contain embedded credentials; use system Git credentials")
	}
	source.Registration.Source = normalized
	if source.Name == "" {
		return catalog.Source{}, errors.New("catalog name is required")
	}
	if err := state.ValidateName(source.Name); err != nil {
		return catalog.Source{}, fmt.Errorf("catalog name: %w", err)
	}
	if _, exists := manifest.Catalogs[source.Name]; exists && !force {
		return catalog.Source{}, fmt.Errorf("catalog %q already exists; use --force to replace it", source.Name)
	}
	return source, nil
}

func catalogNameForAdd(manager *catalog.Manager, rawSource, name string) (string, error) {
	if name != "" {
		if err := state.ValidateName(name); err != nil {
			return "", fmt.Errorf("catalog name: %w", err)
		}
		return name, nil
	}
	if catalog.IsLocal(rawSource) {
		resolved, err := manager.Materialize(catalog.Source{
			Registration: state.CatalogRegistration{Source: rawSource},
		}, false)
		if err != nil {
			return "", err
		}
		if resolved.Manifest.Catalog == nil || resolved.Manifest.Catalog.Name == "" {
			return "", errors.New("--name is required when the catalog has no name")
		}
		return resolved.Manifest.Catalog.Name, nil
	}
	return "", errors.New("--name is required for remote catalogs")
}
