package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	installer "github.com/phillarmonic/repertoire-ai/internal/install"
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
	manifest, err := state.LoadManifest(projectScope.ManifestPath)
	if err != nil {
		return err
	}
	legacyPath := filepath.Join(projectScope.Root, state.BootstrapFileName)
	if err := ensureBootstrapSkills(command, &manifest, projectScope.ManifestPath, legacyPath, refresh); err != nil {
		return err
	}
	if refresh {
		if err := refreshBootstrapCatalogs(manifest); err != nil {
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

	names := make([]string, 0, len(manifest.Skills))
	for name := range manifest.Skills {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		declaration := manifest.Skills[name]
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
			manifest,
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
		if declaration.Scope == state.BootstrapScopeGlobal {
			entry := globalLock.Skills[name]
			if err := installBootstrapProjectArtifacts(
				projectScope,
				globalScope.LockPath,
				manifest,
				&globalLock,
				name,
				declaration.Catalog,
				entry.Targets,
				declaration.Hooks,
				force,
			); err != nil {
				return err
			}
		}
		action := "bootstrapped"
		if refresh {
			action = "synced"
		}
		_, _ = fmt.Fprintf(command.OutOrStdout(), "%s %s (%s)\n", action, name, declaration.Scope)
	}
	return nil
}

func installBootstrapProjectArtifacts(
	projectScope state.Scope,
	globalLockPath string,
	resolutionManifest state.Manifest,
	globalLock *state.Lock,
	name, catalogName string,
	targetNames []string,
	includeOptional, force bool,
) error {
	manager, err := catalog.NewManager("")
	if err != nil {
		return err
	}
	resolved, err := installer.Resolve(manager, resolutionManifest, name, catalogName, false)
	if err != nil {
		return err
	}
	targets, err := installer.ResolveTargets(projectScope, targetNames, "")
	if err != nil {
		return err
	}
	projectArtifacts := globalLock.Projects[projectScope.Root]
	if projectArtifacts == nil {
		projectArtifacts = map[string]state.LockProjectArtifacts{}
	}
	previous := projectArtifacts[name]
	selected := installer.ProjectArtifacts(resolved, includeOptional)
	artifacts, err := installer.InstallArtifacts(
		selected,
		targets,
		projectScope.Root,
		previous.Artifacts,
		force,
	)
	if err != nil {
		return err
	}
	if len(artifacts) == 0 {
		delete(projectArtifacts, name)
		if len(projectArtifacts) == 0 {
			delete(globalLock.Projects, projectScope.Root)
		} else {
			globalLock.Projects[projectScope.Root] = projectArtifacts
		}
		return state.SaveLock(globalLockPath, *globalLock)
	}
	projectArtifacts[name] = state.LockProjectArtifacts{
		Catalog:      resolved.Catalog.Name,
		Source:       catalog.RedactSource(resolved.Catalog.Registration.Source),
		Ref:          resolved.Catalog.Registration.Ref,
		Commit:       resolved.Catalog.Commit,
		Targets:      targetNames,
		Artifacts:    artifacts,
		Instructions: hasProjectInstructions(resolved, targets),
		Hooks:        includeOptional && hasManagedArtifacts(resolved, targets),
	}
	globalLock.Projects[projectScope.Root] = projectArtifacts
	return state.SaveLock(globalLockPath, *globalLock)
}

// ensureBootstrapSkills guarantees the project repertoire.yaml carries
// bootstrap declarations before bootstrap/sync runs. A legacy .repertoire.yaml
// is migrated into repertoire.yaml (and removed); a project without any
// declarations gets a generated starter on bootstrap, while sync errors.
func ensureBootstrapSkills(command *cobra.Command, manifest *state.Manifest, manifestPath, legacyPath string, refresh bool) error {
	out := command.OutOrStdout()
	_, statErr := os.Stat(legacyPath)
	legacyExists := statErr == nil

	if len(manifest.Skills) > 0 {
		if legacyExists {
			_, _ = fmt.Fprintf(
				out,
				"warning: %s is deprecated and ignored; merge its declarations into %s and delete it\n",
				state.BootstrapFileName,
				filepath.Base(manifestPath),
			)
		}
		return nil
	}

	if legacyExists {
		legacy, err := state.LoadBootstrapManifest(legacyPath)
		if err != nil {
			return err
		}
		for name, registration := range legacy.Catalogs {
			if existing, ok := manifest.Catalogs[name]; ok && existing.Source != registration.Source {
				_, _ = fmt.Fprintf(
					out,
					"warning: keeping catalog %q from %s; %s registers a different source\n",
					name,
					filepath.Base(manifestPath),
					state.BootstrapFileName,
				)
				continue
			}
			manifest.Catalogs[name] = registration
		}
		for name, skill := range legacy.Skills {
			manifest.Skills[name] = skill
		}
		if err := state.SaveManifest(manifestPath, *manifest); err != nil {
			return err
		}
		if err := os.Remove(legacyPath); err != nil {
			return fmt.Errorf("remove legacy bootstrap manifest: %w", err)
		}
		_, _ = fmt.Fprintf(out, "migrated %s into %s\n", state.BootstrapFileName, filepath.Base(manifestPath))
		return nil
	}

	if refresh {
		return fmt.Errorf("%s declares no bootstrap skills", filepath.Base(manifestPath))
	}

	manager, err := catalog.NewManager("")
	if err != nil {
		return err
	}
	materialized, err := manager.Materialize(catalog.Source{
		Name:    catalog.BuiltinName,
		Builtin: true,
		Registration: state.CatalogRegistration{
			Source: catalog.BuiltinSource,
		},
	}, false)
	if err != nil {
		return err
	}
	if materialized.Manifest.Catalog == nil || len(materialized.Manifest.Catalog.Skills) == 0 {
		return errors.New("built-in catalog declares no skills")
	}
	names := make([]string, 0, len(materialized.Manifest.Catalog.Skills))
	for name := range materialized.Manifest.Catalog.Skills {
		names = append(names, name)
	}
	sort.Strings(names)
	for name, skill := range catalog.DefaultBootstrapSkills(catalog.BuiltinSource, names) {
		manifest.Skills[name] = skill
	}
	_, statErr = os.Stat(manifestPath)
	manifestExists := statErr == nil
	if err := state.SaveManifest(manifestPath, *manifest); err != nil {
		return err
	}
	if manifestExists {
		_, _ = fmt.Fprintf(out, "added bootstrap skills to %s\n", filepath.Base(manifestPath))
	} else {
		_, _ = fmt.Fprintf(out, "created %s\n", filepath.Base(manifestPath))
	}
	return nil
}

func refreshBootstrapCatalogs(manifest state.Manifest) error {
	refreshAll := false
	used := map[string]struct{}{}
	for _, skill := range manifest.Skills {
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
