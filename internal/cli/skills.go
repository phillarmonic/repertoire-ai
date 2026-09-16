package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/spf13/cobra"
)

func newSkillCommands(globalScope, projectScope, force, dryRun *bool, overrideFlags *[]string) []*cobra.Command {
	var catalogName string
	var requestedTargets []string
	var addWithHooks, addNoHooks bool
	var addSourceName string
	var addSkills []string
	add := &cobra.Command{
		Use:   "add <skill|source>[,<skill>...]",
		Short: "Declare and install skills, or register a catalog source and install from it",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, manifest, lock, err := loadInstallationState(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			hooks, err := hookChoiceFromFlags(addWithHooks, addNoHooks, hookChoicePrompt)
			if err != nil {
				return err
			}
			return runAdd(command, scope, &manifest, &lock, args, addOptions{
				catalogName:      catalogName,
				name:             addSourceName,
				skills:           addSkills,
				requestedTargets: requestedTargets,
				overrideFlags:    overrideFlags,
				force:            *force,
				dryRun:           *dryRun,
				hooks:            hooks,
			})
		},
	}
	add.Flags().StringVar(&catalogName, "catalog", "", "resolve from this catalog")
	add.Flags().StringVar(&addSourceName, "name", "", "catalog name when adding a source")
	add.Flags().StringSliceVar(&addSkills, "skill", nil, "install these skills from a source (repeatable)")
	add.Flags().StringSliceVar(&requestedTargets, "target", nil, "agent target (repeatable; omit to detect installed clients)")
	add.Flags().BoolVar(&addWithHooks, "with-hooks", false, "install optional managed hooks and project integrations")
	add.Flags().BoolVar(&addNoHooks, "no-hooks", false, "skip optional managed hooks and project integrations")
	add.ValidArgsFunction = completeAvailableSkills(globalScope, projectScope, &catalogName, overrideFlags)
	_ = add.RegisterFlagCompletionFunc("catalog", completeCatalogs(globalScope, projectScope, false))
	_ = add.RegisterFlagCompletionFunc("name", cobra.NoFileCompletions)
	_ = add.RegisterFlagCompletionFunc("skill", completeAddSourceSkills(&addSourceName, overrideFlags))
	_ = add.RegisterFlagCompletionFunc("target", completeTargets)

	var installCatalog string
	var installTargets []string
	var installWithHooks, installNoHooks bool
	installCommand := &cobra.Command{
		Use:   "install [skill]",
		Short: "Install one skill or synchronize requirements",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, manifest, lock, err := loadInstallationState(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			hooks, err := hookChoiceFromFlags(installWithHooks, installNoHooks, hookChoicePrompt)
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
					if !installWithHooks && !installNoHooks {
						hooks = hookChoiceNo
						if requirement.Hooks {
							hooks = hookChoiceYes
						}
					}
				}
				if _, err := installNamed(command, scope, &manifest, &lock, args[0], selectedCatalog, targets, declared, *force, false, hooks, *dryRun, overrideFlags); err != nil {
					return err
				}
				if !*dryRun {
					_, _ = fmt.Fprintf(command.OutOrStdout(), "installed %s\n", args[0])
				}
				return nil
			}
			names := sortedRequirements(manifest.Requirements)
			for _, name := range names {
				requirement := manifest.Requirements[name]
				targets := requirement.Targets
				if len(installTargets) > 0 {
					targets = installTargets
				}
				requirementHooks := hooks
				if !installWithHooks && !installNoHooks {
					requirementHooks = hookChoiceNo
					if requirement.Hooks {
						requirementHooks = hookChoiceYes
					}
				}
				if _, err := installNamed(command, scope, &manifest, &lock, name, requirement.Catalog, targets, true, *force, false, requirementHooks, *dryRun, overrideFlags); err != nil {
					return err
				}
				if !*dryRun {
					_, _ = fmt.Fprintf(command.OutOrStdout(), "installed %s\n", name)
				}
			}
			return nil
		},
	}
	installCommand.Flags().StringVar(&installCatalog, "catalog", "", "resolve from this catalog")
	installCommand.Flags().StringSliceVar(&installTargets, "target", nil, "agent target (repeatable)")
	installCommand.Flags().BoolVar(&installWithHooks, "with-hooks", false, "install optional managed hooks and project integrations")
	installCommand.Flags().BoolVar(&installNoHooks, "no-hooks", false, "skip optional managed hooks and project integrations")
	installCommand.ValidArgsFunction = completeAvailableSkills(globalScope, projectScope, &installCatalog, overrideFlags)
	_ = installCommand.RegisterFlagCompletionFunc("catalog", completeCatalogs(globalScope, projectScope, false))
	_ = installCommand.RegisterFlagCompletionFunc("target", completeTargets)

	var available bool
	var listCatalog, listFormat string
	var listWide bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List installed or available skills",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			noteDryRunNoOp(command, *dryRun)
			format, err := resolveSkillListFormat(listFormat, command.OutOrStdout(), listWide)
			if err != nil {
				return err
			}
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
				entries := make([]skillListEntry, 0, len(names))
				for _, name := range names {
					entry := lock.Skills[name]
					entries = append(entries, skillListEntry{
						Name: name, Catalog: entry.Catalog,
						Origin: entry.EffectiveOrigin(), Targets: entry.Targets,
					})
				}
				return writeSkillList(command.OutOrStdout(), entries, format, listWide, false)
			}
			manager, err := newCatalogManager("", *overrideFlags)
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
					if listCatalog != "" {
						return err
					}
					_, _ = fmt.Fprintf(command.ErrOrStderr(), "warning: skipped catalog %s: %v\n", source.Name, err)
					continue
				}
				catalogName := source.Name
				if resolved.Loose {
					catalogName += " (loose)"
				}
				for name := range resolved.Manifest.Catalog.Skills {
					skills = append(skills, availableSkill{name: name, catalog: catalogName})
				}
			}
			sort.Slice(skills, func(i, j int) bool {
				if skills[i].name == skills[j].name {
					return skills[i].catalog < skills[j].catalog
				}
				return skills[i].name < skills[j].name
			})
			entries := make([]skillListEntry, 0, len(skills))
			for _, skill := range skills {
				entries = append(entries, skillListEntry{
					Name: skill.name, Catalog: skill.catalog, Origin: "available",
				})
			}
			return writeSkillList(command.OutOrStdout(), entries, format, listWide, true)
		},
	}
	list.Flags().BoolVar(&available, "available", false, "list skills offered by catalogs")
	list.Flags().StringVar(&listCatalog, "catalog", "", "limit available skills to a catalog")
	list.Flags().StringVar(&listFormat, "format", "auto", "output format: auto, table, tsv, or json")
	list.Flags().BoolVar(&listWide, "wide", false, "show individual targets in table output")
	_ = list.RegisterFlagCompletionFunc("catalog", completeCatalogs(globalScope, projectScope, false))
	_ = list.RegisterFlagCompletionFunc("format", completeListFormats)

	var updateTargets []string
	var updateWithHooks, updateNoHooks bool
	update := &cobra.Command{
		Use:   "update [skill]",
		Short: "Refresh and update installed skills",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, manifest, lock, err := loadInstallationState(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			manager, err := newCatalogManager("", *overrideFlags)
			if err != nil {
				return err
			}
			var names []string
			if len(args) == 1 {
				if _, exists := lock.Skills[args[0]]; !exists {
					if !catalogVisible(manifest, args[0]) {
						return fmt.Errorf("skill %q is not installed", args[0])
					}
					if *dryRun {
						return previewCatalogUpdate(command, manager, manifest, args[0])
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
				if len(lock.Skills) == 0 && len(manifest.Skills) == 0 && len(manifest.Requirements) == 0 {
					_, _ = fmt.Fprintln(command.OutOrStdout(), "update: nothing installed; nothing to update")
					return nil
				}
				if *dryRun {
					if err := previewCatalogUpdate(command, manager, manifest, ""); err != nil {
						return err
					}
				} else {
					refreshed, err := refreshCatalogsWarn(manager, manifest, "", command.ErrOrStderr())
					if err != nil {
						return err
					}
					for _, source := range refreshed {
						_, _ = fmt.Fprintf(command.OutOrStdout(), "updated catalog %s\t%s\n", source.Name, source.Commit)
					}
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
				hooks, err := hookChoiceFromFlags(updateWithHooks, updateNoHooks, hookChoiceNo)
				if err != nil {
					return err
				}
				if !updateWithHooks && !updateNoHooks &&
					(entry.Hooks || (!entry.Instructions && len(entry.Artifacts) > 0)) {
					hooks = hookChoiceYes
				}
				if _, err := installNamed(command, scope, &manifest, &lock, name, entry.Catalog, targets, entry.Declared, *force, true, hooks, *dryRun, overrideFlags); err != nil {
					return err
				}
				if !*dryRun {
					_, _ = fmt.Fprintf(command.OutOrStdout(), "updated %s\n", name)
				}
			}
			return nil
		},
	}
	update.Flags().StringSliceVar(&updateTargets, "target", nil, "agent target (repeatable; use \"all\" for every target)")
	update.Flags().BoolVar(&updateWithHooks, "with-hooks", false, "install optional managed hooks and project integrations")
	update.Flags().BoolVar(&updateNoHooks, "no-hooks", false, "remove optional managed hooks and project integrations")
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
			if *dryRun {
				plans, planErr := installer.PlanRemove(args[0], targets, entry, *force)
				if planErr != nil {
					return planErr
				}
				if err := writeRemoveCopyPlans(command.OutOrStdout(), plans); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(command.OutOrStdout(), "would remove lock entry %s\n", args[0])
				if entry.EffectiveOrigin() == state.LockOriginDeclared {
					_, _ = fmt.Fprintf(command.OutOrStdout(), "would remove requirement %s\n", args[0])
				}
				return nil
			}
			if err := installer.RemoveArtifacts(entry.Artifacts, scope.Root, *force); err != nil {
				return err
			}
			if scope.Global {
				if err := removeGlobalProjectArtifacts(&lock, args[0], *force); err != nil {
					return err
				}
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

	var showFormat string
	show := newShowCommand(globalScope, projectScope, dryRun, overrideFlags, &showFormat)
	return []*cobra.Command{add, installCommand, list, show, update, remove}
}

func removeGlobalProjectArtifacts(lock *state.Lock, skillName string, force bool) error {
	for projectRoot, skills := range lock.Projects {
		entry, exists := skills[skillName]
		if !exists {
			continue
		}
		if err := installer.RemoveArtifacts(entry.Artifacts, projectRoot, force); err != nil {
			return fmt.Errorf("remove project artifacts from %s: %w", projectRoot, err)
		}
		delete(skills, skillName)
		if len(skills) == 0 {
			delete(lock.Projects, projectRoot)
		} else {
			lock.Projects[projectRoot] = skills
		}
	}
	return nil
}

func loadInstallationState(globalScope, projectScope bool) (state.Scope, state.Manifest, state.Lock, error) {
	scope, manifest, err := loadScope(globalScope, projectScope)
	if err != nil {
		return state.Scope{}, state.Manifest{}, state.Lock{}, err
	}
	lock, err := state.LoadLock(scope.LockPath)
	return scope, manifest, lock, err
}

func installNamed(
	command *cobra.Command,
	scope state.Scope,
	manifest *state.Manifest,
	lock *state.Lock,
	name, catalogName string,
	requestedTargets []string,
	declared, force, refresh bool,
	hooks hookChoice,
	dryRun bool,
	overrideFlags *[]string,
) (bool, error) {
	origin := state.LockOriginAdHoc
	if declared {
		origin = state.LockOriginDeclared
	}
	return installManaged(command, scope, *manifest, manifest, lock, name, catalogName, requestedTargets, origin, force, refresh, false, hooks, dryRun, overrideFlags)
}

func installManaged(
	command *cobra.Command,
	scope state.Scope,
	resolutionManifest state.Manifest,
	requirementsManifest *state.Manifest,
	lock *state.Lock,
	name, catalogName string,
	requestedTargets []string,
	origin string,
	force, refresh, protectGlobal bool,
	hooks hookChoice,
	dryRun bool,
	overrideFlags *[]string,
) (bool, error) {
	manager, err := newCatalogManager("", *overrideFlags)
	if err != nil {
		return false, err
	}
	var resolved installer.ResolvedSkill
	if dryRun {
		resolved, err = installer.ResolveCached(manager, resolutionManifest, name, catalogName)
	} else {
		resolved, err = installer.Resolve(manager, resolutionManifest, name, catalogName, refresh)
	}
	if err != nil {
		if dryRun {
			if cloneLine, ok := dryRunCloneLine(manager, resolutionManifest, catalogName); ok {
				_, _ = fmt.Fprintln(command.OutOrStdout(), cloneLine)
				return false, nil
			}
		}
		return false, err
	}
	targets, err := installer.ResolveTargets(scope, requestedTargets, "")
	if err != nil {
		return false, err
	}
	var previous *state.LockSkill
	if entry, exists := lock.Skills[name]; exists {
		if protectGlobal && scope.Global && !force {
			source := catalog.RedactSource(catalog.NormalizeSource(resolved.Catalog.Registration.Source))
			if entry.Catalog != resolved.Catalog.Name || entry.Source != source || entry.Ref != resolved.Catalog.Registration.Ref {
				if dryRun {
					_, _ = fmt.Fprintf(command.OutOrStdout(), "would refuse: global skill %q is managed from a different catalog source or ref\n", name)
				}
				return false, fmt.Errorf("global skill %q is managed from a different catalog source or ref; use --force to replace it", name)
			}
		}
		previous = &entry
	}
	if dryRun {
		plans, planErr := installer.PlanSkill(resolved, targets, previous, force)
		if planErr != nil {
			return false, planErr
		}
		if writeErr := writeSkillCopyPlans(command.OutOrStdout(), plans); writeErr != nil {
			return false, writeErr
		}
		if origin == state.LockOriginDeclared && requirementsManifest != nil {
			_, _ = fmt.Fprintf(command.OutOrStdout(), "would write requirement %s\n", name)
		}
		_, _ = fmt.Fprintf(command.OutOrStdout(), "would write lock entry %s\n", name)
		return false, nil
	}
	locations, targetDigests, err := installer.SkillWithDigests(resolved, targets, previous, force)
	if err != nil {
		return false, err
	}
	hooksEnabled, err := resolveHookChoice(command, scope, resolved, targets, hooks)
	if err != nil {
		return false, err
	}
	var artifacts []state.LockArtifact
	instructionsEnabled := !scope.Global && hasProjectInstructions(resolved, targets)
	if !scope.Global {
		var previousArtifacts []state.LockArtifact
		if previous != nil {
			previousArtifacts = previous.Artifacts
		}
		selected := installer.ProjectArtifacts(resolved, hooksEnabled)
		artifacts, err = installer.InstallArtifacts(selected, targets, scope.Root, previousArtifacts, force)
		if err != nil {
			return false, err
		}
	}
	targetNames := make([]string, 0, len(targets))
	for _, target := range targets {
		targetNames = append(targetNames, target.Name)
	}
	source := catalog.RedactSource(resolved.Catalog.Registration.Source)
	lock.Skills[name] = state.LockSkill{
		Catalog: resolved.Catalog.Name, Source: source, Ref: resolved.Catalog.Registration.Ref,
		Commit: resolved.Catalog.Commit, Digest: resolved.Digest, TargetDigests: targetDigests,
		Targets: targetNames, Locations: locations, Artifacts: artifacts,
		Instructions: instructionsEnabled, Hooks: hooksEnabled,
		Declared: origin == state.LockOriginDeclared, Origin: origin,
	}
	if origin == state.LockOriginDeclared && requirementsManifest != nil {
		requirementsManifest.Requirements[name] = state.Requirement{
			Catalog: resolved.Catalog.Name, Targets: targetNames, Hooks: hooksEnabled,
		}
		if err := state.SaveManifest(scope.ManifestPath, *requirementsManifest); err != nil {
			return false, err
		}
	}
	return hooksEnabled, state.SaveLock(scope.LockPath, *lock)
}

type hookChoice int

const (
	hookChoicePrompt hookChoice = iota
	hookChoiceYes
	hookChoiceNo
)

func hookChoiceFromFlags(withHooks, noHooks bool, fallback hookChoice) (hookChoice, error) {
	if withHooks && noHooks {
		return fallback, errors.New("--with-hooks and --no-hooks are mutually exclusive")
	}
	if withHooks {
		return hookChoiceYes, nil
	}
	if noHooks {
		return hookChoiceNo, nil
	}
	return fallback, nil
}

func resolveHookChoice(
	command *cobra.Command,
	scope state.Scope,
	resolved installer.ResolvedSkill,
	targets []installer.Target,
	choice hookChoice,
) (bool, error) {
	if scope.Global || !hasManagedArtifacts(resolved, targets) {
		return false, nil
	}
	if choice == hookChoiceYes {
		return true, nil
	}
	if choice == hookChoiceNo || !interactiveInput(command.InOrStdin()) {
		return false, nil
	}
	_, _ = fmt.Fprint(command.OutOrStdout(), "Install optional managed hooks and project integrations? [y/N] ")
	answer, err := bufio.NewReader(command.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func hasManagedArtifacts(resolved installer.ResolvedSkill, targets []installer.Target) bool {
	if len(resolved.Artifacts["all"]) > 0 {
		return true
	}
	for _, target := range targets {
		if len(resolved.Artifacts[target.Name]) > 0 {
			return true
		}
	}
	return false
}

func hasProjectInstructions(resolved installer.ResolvedSkill, targets []installer.Target) bool {
	if len(resolved.Instructions["all"]) > 0 {
		return true
	}
	for _, target := range targets {
		if len(resolved.Instructions[target.Name]) > 0 {
			return true
		}
	}
	return false
}

func interactiveInput(input io.Reader) bool {
	file, ok := input.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// expandSkillSelectors turns add-command arguments into a concrete, ordered,
// deduplicated list of skill names. Each argument may be a comma-separated
// list, and any element containing glob metacharacters (* or ?) is expanded
// against the skills offered by the visible catalogs. Plain names pass
// through unvalidated; resolution reports unknown names later.
func expandSkillSelectors(manifest state.Manifest, catalogName string, args []string, overrideFlags *[]string) ([]string, error) {
	var selectors []string
	for _, arg := range args {
		for part := range strings.SplitSeq(arg, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				selectors = append(selectors, part)
			}
		}
	}
	if len(selectors) == 0 {
		return nil, errors.New("at least one skill name is required")
	}
	hasGlob := false
	for _, selector := range selectors {
		if strings.ContainsAny(selector, "*?") {
			hasGlob = true
			break
		}
	}
	if !hasGlob {
		return dedupeNames(selectors), nil
	}
	manager, err := newCatalogManager("", *overrideFlags)
	if err != nil {
		return nil, err
	}
	var available []string
	var materializeErrs []error
	for _, source := range catalog.Sources(manifest) {
		if catalogName != "" && source.Name != catalogName {
			continue
		}
		resolved, err := manager.Materialize(source, true)
		if err != nil {
			materializeErrs = append(materializeErrs, err)
			continue
		}
		for name := range resolved.Manifest.Catalog.Skills {
			available = append(available, name)
		}
	}
	sort.Strings(available)
	expanded := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		if !strings.ContainsAny(selector, "*?") {
			expanded = append(expanded, selector)
			continue
		}
		matched := false
		for _, name := range available {
			if ok, _ := path.Match(selector, name); ok {
				expanded = append(expanded, name)
				matched = true
			}
		}
		if !matched {
			if len(available) == 0 && len(materializeErrs) > 0 {
				return nil, materializeErrs[0]
			}
			return nil, fmt.Errorf("pattern %q matched no available skills", selector)
		}
	}
	return dedupeNames(expanded), nil
}

func dedupeNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, name := range names {
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
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
