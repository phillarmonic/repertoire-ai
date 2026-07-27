package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/spf13/cobra"
)

func newBootstrapCommands(globalScope, projectScope, force *bool) []*cobra.Command {
	bootstrap := &cobra.Command{
		Use:   "bootstrap",
		Short: "Install skills declared by the project bootstrap manifest",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return runBootstrap(command, *globalScope, *projectScope, *force, false)
		},
	}
	syncCommand := &cobra.Command{
		Use:   "sync",
		Short: "Refresh and synchronize project bootstrap skills",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return runBootstrap(command, *globalScope, *projectScope, *force, true)
		},
	}
	return []*cobra.Command{bootstrap, syncCommand}
}

func runBootstrap(command *cobra.Command, globalFlag, projectFlag, force, refresh bool) error {
	if globalFlag || projectFlag {
		return errors.New("bootstrap and sync use per-skill scope; --global and --project are not supported")
	}
	projectScope, err := state.ResolveScope(state.ScopeOptions{Project: true})
	if err != nil {
		return err
	}
	bootstrapPath := filepath.Join(projectScope.Root, state.BootstrapFileName)
	bootstrap, created, err := loadOrCreateBootstrapManifest(bootstrapPath, refresh)
	if err != nil {
		return err
	}
	if created {
		_, _ = fmt.Fprintf(command.OutOrStdout(), "created %s\n", state.BootstrapFileName)
	}
	resolutionManifest := bootstrap.ResolutionManifest()
	if refresh {
		if err := refreshBootstrapCatalogs(bootstrap, resolutionManifest); err != nil {
			return err
		}
	}

	projectLock, err := state.LoadLock(projectScope.LockPath)
	if err != nil {
		return err
	}
	var globalScope state.Scope
	var globalLock state.Lock
	globalLoaded := false

	names := make([]string, 0, len(bootstrap.Skills))
	for name := range bootstrap.Skills {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		declaration := bootstrap.Skills[name]
		scope := projectScope
		lock := &projectLock
		protectGlobal := false
		if declaration.Scope == state.BootstrapScopeGlobal {
			if !globalLoaded {
				globalScope, err = state.ResolveScope(state.ScopeOptions{Global: true, Directory: projectScope.Root})
				if err != nil {
					return err
				}
				globalLock, err = state.LoadLock(globalScope.LockPath)
				if err != nil {
					return err
				}
				globalLoaded = true
			}
			scope = globalScope
			lock = &globalLock
			protectGlobal = true
		}
		hooks := hookChoiceNo
		if declaration.Hooks {
			hooks = hookChoiceYes
		}
		if _, err := installManaged(
			command,
			scope,
			resolutionManifest,
			nil,
			lock,
			name,
			declaration.Catalog,
			declaration.Targets,
			state.LockOriginBootstrap,
			force,
			false,
			protectGlobal,
			hooks,
		); err != nil {
			return err
		}
		action := "bootstrapped"
		if refresh {
			action = "synced"
		}
		_, _ = fmt.Fprintf(command.OutOrStdout(), "%s %s (%s)\n", action, name, declaration.Scope)
	}
	return nil
}

func loadOrCreateBootstrapManifest(path string, refresh bool) (state.BootstrapManifest, bool, error) {
	bootstrap, err := state.LoadBootstrapManifest(path)
	if err == nil {
		return bootstrap, false, nil
	}
	if refresh || !errors.Is(err, os.ErrNotExist) {
		return state.BootstrapManifest{}, false, err
	}
	manager, err := catalog.NewManager("")
	if err != nil {
		return state.BootstrapManifest{}, false, err
	}
	materialized, err := manager.Materialize(catalog.Source{
		Name:    catalog.BuiltinName,
		Builtin: true,
		Registration: state.CatalogRegistration{
			Source: catalog.BuiltinSource,
		},
	}, false)
	if err != nil {
		return state.BootstrapManifest{}, false, err
	}
	if materialized.Manifest.Catalog == nil || len(materialized.Manifest.Catalog.Skills) == 0 {
		return state.BootstrapManifest{}, false, errors.New("built-in catalog declares no skills")
	}
	names := make([]string, 0, len(materialized.Manifest.Catalog.Skills))
	for name := range materialized.Manifest.Catalog.Skills {
		names = append(names, name)
	}
	sort.Strings(names)
	bootstrap = catalog.DefaultBootstrapManifest(catalog.BuiltinSource, names)
	if err := state.SaveBootstrapManifest(path, bootstrap); err != nil {
		return state.BootstrapManifest{}, false, err
	}
	return bootstrap, true, nil
}

func refreshBootstrapCatalogs(bootstrap state.BootstrapManifest, manifest state.Manifest) error {
	refreshAll := false
	used := map[string]struct{}{}
	for _, skill := range bootstrap.Skills {
		if skill.Catalog == "" {
			refreshAll = true
		} else {
			used[skill.Catalog] = struct{}{}
		}
	}
	manager, err := catalog.NewManager("")
	if err != nil {
		return err
	}
	for _, source := range catalog.Sources(manifest) {
		if !refreshAll {
			if _, required := used[source.Name]; !required {
				continue
			}
		}
		if _, err := manager.Materialize(source, true); err != nil {
			return err
		}
	}
	return nil
}
