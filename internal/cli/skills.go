package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/spf13/cobra"
)

func newSkillCommands(globalScope, projectScope, force *bool) []*cobra.Command {
	var catalogName string
	var requestedTargets []string
	add := &cobra.Command{
		Use:   "add <skill>",
		Short: "Declare and install a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, manifest, lock, err := loadInstallationState(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			if err := installNamed(scope, &manifest, &lock, args[0], catalogName, requestedTargets, true, *force, false); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(command.OutOrStdout(), "added %s from %s\n", args[0], lock.Skills[args[0]].Catalog)
			return nil
		},
	}
	add.Flags().StringVar(&catalogName, "catalog", "", "resolve from this catalog")
	add.Flags().StringSliceVar(&requestedTargets, "target", nil, "agent target (repeatable)")
	add.ValidArgsFunction = completeAvailableSkills(globalScope, projectScope, &catalogName)
	_ = add.RegisterFlagCompletionFunc("catalog", completeCatalogs(globalScope, projectScope, false))
	_ = add.RegisterFlagCompletionFunc("target", completeTargets)

	var installCatalog string
	var installTargets []string
	installCommand := &cobra.Command{
		Use:   "install [skill]",
		Short: "Install one skill or synchronize requirements",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, manifest, lock, err := loadInstallationState(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				requirement, declared := manifest.Requirements[args[0]]
				selectedCatalog := installCatalog
				targets := installTargets
				if declared {
					if selectedCatalog == "" {
						selectedCatalog = requirement.Catalog
					}
					if len(targets) == 0 {
						targets = requirement.Targets
					}
				}
				if err := installNamed(scope, &manifest, &lock, args[0], selectedCatalog, targets, declared, *force, false); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(command.OutOrStdout(), "installed %s\n", args[0])
				return nil
			}
			names := sortedRequirements(manifest.Requirements)
			for _, name := range names {
				requirement := manifest.Requirements[name]
				targets := requirement.Targets
				if len(installTargets) > 0 {
					targets = installTargets
				}
				if err := installNamed(scope, &manifest, &lock, name, requirement.Catalog, targets, true, *force, false); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(command.OutOrStdout(), "installed %s\n", name)
			}
			return nil
		},
	}
	installCommand.Flags().StringVar(&installCatalog, "catalog", "", "resolve from this catalog")
	installCommand.Flags().StringSliceVar(&installTargets, "target", nil, "agent target (repeatable)")
	installCommand.ValidArgsFunction = completeAvailableSkills(globalScope, projectScope, &installCatalog)
	_ = installCommand.RegisterFlagCompletionFunc("catalog", completeCatalogs(globalScope, projectScope, false))
	_ = installCommand.RegisterFlagCompletionFunc("target", completeTargets)

	var available bool
	var listCatalog string
	list := &cobra.Command{
		Use:   "list",
		Short: "List installed or available skills",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_, manifest, lock, err := loadInstallationState(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			if !available {
				names := make([]string, 0, len(lock.Skills))
				for name := range lock.Skills {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					entry := lock.Skills[name]
					kind := entry.EffectiveOrigin()
					_, _ = fmt.Fprintf(command.OutOrStdout(), "%s\t%s\t%s\t%s\n", name, entry.Catalog, kind, strings.Join(entry.Targets, ","))
				}
				return nil
			}
			manager, err := catalog.NewManager("")
			if err != nil {
				return err
			}
			type availableSkill struct{ name, catalog string }
			var skills []availableSkill
			for _, source := range catalog.Sources(manifest) {
				if listCatalog != "" && source.Name != listCatalog {
					continue
				}
				resolved, err := manager.Materialize(source, true)
				if err != nil {
					return err
				}
				for name := range resolved.Manifest.Catalog.Skills {
					skills = append(skills, availableSkill{name: name, catalog: source.Name})
				}
			}
			sort.Slice(skills, func(i, j int) bool {
				if skills[i].name == skills[j].name {
					return skills[i].catalog < skills[j].catalog
				}
				return skills[i].name < skills[j].name
			})
			for _, skill := range skills {
				_, _ = fmt.Fprintf(command.OutOrStdout(), "%s\t%s\tavailable\n", skill.name, skill.catalog)
			}
			return nil
		},
	}
	list.Flags().BoolVar(&available, "available", false, "list skills offered by catalogs")
	list.Flags().StringVar(&listCatalog, "catalog", "", "limit available skills to a catalog")
	_ = list.RegisterFlagCompletionFunc("catalog", completeCatalogs(globalScope, projectScope, false))

	var updateTargets []string
	update := &cobra.Command{
		Use:   "update [skill]",
		Short: "Refresh and update installed skills",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, manifest, lock, err := loadInstallationState(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			manager, err := catalog.NewManager("")
			if err != nil {
				return err
			}
			var names []string
			if len(args) == 1 {
				if _, exists := lock.Skills[args[0]]; !exists {
					if !catalogVisible(manifest, args[0]) {
						return fmt.Errorf("skill %q is not installed", args[0])
					}
					refreshed, err := refreshCatalogs(manager, manifest, args[0])
					if err != nil {
						return err
					}
					for _, source := range refreshed {
						_, _ = fmt.Fprintf(command.OutOrStdout(), "updated catalog %s\t%s\n", source.Name, source.Commit)
					}
					return nil
				}
				names = args
			} else {
				refreshed, err := refreshCatalogs(manager, manifest, "")
				if err != nil {
					return err
				}
				for _, source := range refreshed {
					_, _ = fmt.Fprintf(command.OutOrStdout(), "updated catalog %s\t%s\n", source.Name, source.Commit)
				}
				for name := range lock.Skills {
					names = append(names, name)
				}
				sort.Strings(names)
			}
			for _, name := range names {
				entry := lock.Skills[name]
				targets := entry.Targets
				if len(updateTargets) > 0 {
					targets = updateTargets
				}
				if err := installNamed(scope, &manifest, &lock, name, entry.Catalog, targets, entry.Declared, *force, true); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(command.OutOrStdout(), "updated %s\n", name)
			}
			return nil
		},
	}
	update.Flags().StringSliceVar(&updateTargets, "target", nil, "agent target (repeatable; use \"all\" for every target)")
	update.ValidArgsFunction = completeInstalledSkillsAndCatalogs(globalScope, projectScope)
	_ = update.RegisterFlagCompletionFunc("target", completeTargets)
	remove := &cobra.Command{
		Use:   "remove <skill>",
		Short: "Remove a managed skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, manifest, lock, err := loadInstallationState(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			entry, exists := lock.Skills[args[0]]
			if !exists {
				return fmt.Errorf("skill %q is not installed", args[0])
			}
			targets, err := installer.ResolveTargets(scope, entry.Targets, "")
			if err != nil {
				return err
			}
			if err := installer.Remove(args[0], targets, entry, *force); err != nil {
				return err
			}
			delete(lock.Skills, args[0])
			if entry.EffectiveOrigin() == state.LockOriginDeclared {
				delete(manifest.Requirements, args[0])
				if err := state.SaveManifest(scope.ManifestPath, manifest); err != nil {
					return err
				}
			}
			if err := state.SaveLock(scope.LockPath, lock); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(command.OutOrStdout(), "removed %s\n", args[0])
			return nil
		},
	}
	remove.ValidArgsFunction = completeInstalledSkills(globalScope, projectScope)
	return []*cobra.Command{add, installCommand, list, update, remove}
}

func loadInstallationState(globalScope, projectScope bool) (state.Scope, state.Manifest, state.Lock, error) {
	scope, manifest, err := loadScope(globalScope, projectScope)
	if err != nil {
		return state.Scope{}, state.Manifest{}, state.Lock{}, err
	}
	lock, err := state.LoadLock(scope.LockPath)
	return scope, manifest, lock, err
}

func installNamed(scope state.Scope, manifest *state.Manifest, lock *state.Lock, name, catalogName string, requestedTargets []string, declared, force, refresh bool) error {
	origin := state.LockOriginAdHoc
	if declared {
		origin = state.LockOriginDeclared
	}
	return installManaged(scope, *manifest, manifest, lock, name, catalogName, requestedTargets, origin, force, refresh, false)
}

func installManaged(
	scope state.Scope,
	resolutionManifest state.Manifest,
	requirementsManifest *state.Manifest,
	lock *state.Lock,
	name, catalogName string,
	requestedTargets []string,
	origin string,
	force, refresh, protectGlobal bool,
) error {
	manager, err := catalog.NewManager("")
	if err != nil {
		return err
	}
	resolved, err := installer.Resolve(manager, resolutionManifest, name, catalogName, refresh)
	if err != nil {
		return err
	}
	targets, err := installer.ResolveTargets(scope, requestedTargets, "")
	if err != nil {
		return err
	}
	var previous *state.LockSkill
	if entry, exists := lock.Skills[name]; exists {
		if protectGlobal && scope.Global && !force {
			source := catalog.RedactSource(catalog.NormalizeSource(resolved.Catalog.Registration.Source))
			if entry.Catalog != resolved.Catalog.Name || entry.Source != source || entry.Ref != resolved.Catalog.Registration.Ref {
				return fmt.Errorf("global skill %q is managed from a different catalog source or ref; use --force to replace it", name)
			}
		}
		previous = &entry
	}
	locations, err := installer.Skill(resolved, targets, previous, force)
	if err != nil {
		return err
	}
	targetNames := make([]string, 0, len(targets))
	for _, target := range targets {
		targetNames = append(targetNames, target.Name)
	}
	source := catalog.RedactSource(resolved.Catalog.Registration.Source)
	lock.Skills[name] = state.LockSkill{
		Catalog: resolved.Catalog.Name, Source: source, Ref: resolved.Catalog.Registration.Ref,
		Commit: resolved.Catalog.Commit, Digest: resolved.Digest, Targets: targetNames,
		Locations: locations, Declared: origin == state.LockOriginDeclared, Origin: origin,
	}
	if origin == state.LockOriginDeclared && requirementsManifest != nil {
		requirementsManifest.Requirements[name] = state.Requirement{Catalog: resolved.Catalog.Name, Targets: targetNames}
		if err := state.SaveManifest(scope.ManifestPath, *requirementsManifest); err != nil {
			return err
		}
	}
	return state.SaveLock(scope.LockPath, *lock)
}

func catalogVisible(manifest state.Manifest, name string) bool {
	for _, source := range catalog.Sources(manifest) {
		if source.Name == name {
			return true
		}
	}
	return false
}

func sortedRequirements(requirements map[string]state.Requirement) []string {
	names := make([]string, 0, len(requirements))
	for name := range requirements {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
