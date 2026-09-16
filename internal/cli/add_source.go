package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/spf13/cobra"
)

type addOptions struct {
	overrideFlags    *[]string
	catalogName      string
	name             string
	skills           []string
	requestedTargets []string
	hooks            hookChoice
	force            bool
	dryRun           bool
}

func runAdd(
	command *cobra.Command,
	scope state.Scope,
	manifest *state.Manifest,
	lock *state.Lock,
	args []string,
	options addOptions,
) error {
	manager, err := newCatalogManager("", *options.overrideFlags)
	if err != nil {
		return err
	}
	known := knownSkillSelectors(manager, *manifest, *lock)
	var skillArgs []string
	var sources []catalog.InstallSource
	for _, arg := range args {
		kind, skills, source := classifyAddArgument(arg, known, *manifest)
		switch kind {
		case addArgSource:
			sources = append(sources, source)
		default:
			skillArgs = append(skillArgs, skills...)
		}
	}
	if len(sources) > 0 && len(skillArgs) > 0 {
		return errors.New("cannot mix catalog sources with skill names")
	}
	if len(sources) > 1 {
		return errors.New("add accepts one catalog source at a time")
	}
	if len(sources) == 1 {
		return addFromSource(command, scope, manifest, lock, manager, sources[0], options)
	}
	if options.name != "" {
		return errors.New("--name is only valid when adding a catalog source")
	}
	if len(options.skills) > 0 {
		return errors.New("--skill is only valid when adding a catalog source")
	}
	names, err := expandSkillSelectors(*manifest, options.catalogName, skillArgs, options.overrideFlags)
	if err != nil {
		return err
	}
	for _, name := range names {
		if _, err := installNamed(command, scope, manifest, lock, name, options.catalogName, options.requestedTargets, true, options.force, false, options.hooks, options.dryRun, options.overrideFlags); err != nil {
			return err
		}
		if options.dryRun {
			continue
		}
		_, _ = fmt.Fprintf(command.OutOrStdout(), "added %s from %s (%s)\n",
			name, lock.Skills[name].Catalog, summarizeTargets(lock.Skills[name].Targets))
	}
	return nil
}

type addArgKind int

const (
	addArgSkill addArgKind = iota
	addArgSource
)

func classifyAddArgument(arg string, known map[string]struct{}, manifest state.Manifest) (addArgKind, []string, catalog.InstallSource) {
	trimmed := strings.TrimSpace(arg)
	if strings.Contains(trimmed, ",") || strings.ContainsAny(trimmed, "*?") {
		return addArgSkill, []string{trimmed}, catalog.InstallSource{}
	}
	if _, exists := known[trimmed]; exists {
		return addArgSkill, []string{trimmed}, catalog.InstallSource{}
	}
	namespace, _, err := catalog.ParseSkillID(trimmed)
	if err == nil && namespace != "" {
		for _, source := range catalog.Sources(manifest) {
			if catalog.SourceMatchesNamespace(source.Registration.Source, namespace) {
				return addArgSkill, []string{trimmed}, catalog.InstallSource{}
			}
		}
	}
	parsed, ok := catalog.ParseInstallSource(trimmed)
	if ok {
		return addArgSource, nil, parsed
	}
	return addArgSkill, []string{trimmed}, catalog.InstallSource{}
}

func knownSkillSelectors(manager *catalog.Manager, manifest state.Manifest, lock state.Lock) map[string]struct{} {
	known := make(map[string]struct{})
	for name := range lock.Skills {
		known[name] = struct{}{}
	}
	for _, source := range catalog.Sources(manifest) {
		resolved, err := manager.InspectCached(source)
		if err != nil {
			continue
		}
		if resolved.Manifest.Catalog == nil {
			continue
		}
		for name := range resolved.Manifest.Catalog.Skills {
			known[name] = struct{}{}
			known[catalog.SkillID(source.Registration.Source, name)] = struct{}{}
		}
	}
	return known
}

func addFromSource(
	command *cobra.Command,
	scope state.Scope,
	manifest *state.Manifest,
	lock *state.Lock,
	manager *catalog.Manager,
	parsed catalog.InstallSource,
	options addOptions,
) error {
	if options.catalogName != "" {
		return errors.New("--catalog cannot be combined with a catalog source")
	}
	name := options.name
	if name == "" {
		derived, err := catalog.DefaultCatalogName(parsed.Source)
		if err != nil {
			return err
		}
		name = derived
	} else if err := state.ValidateName(name); err != nil {
		return fmt.Errorf("catalog name: %w", err)
	}
	if err := rejectNameCollision(*manifest, name, parsed.Source); err != nil {
		return err
	}
	source := catalog.Source{
		Name: name,
		Registration: state.CatalogRegistration{
			Source: parsed.Source,
			Ref:    parsed.Ref,
		},
	}
	_, registered := manifest.Catalogs[name]
	if !registered {
		if options.dryRun {
			if err := previewCatalogAdd(command, manifest, manager, source, options.force); err != nil {
				return err
			}
			if !manager.HasCachedClone(source) {
				return nil
			}
			manifest.Catalogs[name] = source.Registration
		} else {
			if err := registerCatalog(scope, manifest, manager, source, options.force); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(command.OutOrStdout(), "registered %s\n", name)
		}
	}
	if options.dryRun {
		materialized, err := manager.InspectCached(catalog.Source{
			Name:         name,
			Registration: manifest.Catalogs[name],
		})
		if err != nil {
			if !manager.HasCachedClone(source) {
				return nil
			}
			return err
		}
		skillNames, err := skillsToInstall(materialized, parsed.Skill, options.skills)
		if err != nil {
			return err
		}
		for _, skillName := range skillNames {
			if _, err := installNamed(command, scope, manifest, lock, skillName, name, options.requestedTargets, true, options.force, false, options.hooks, true, options.overrideFlags); err != nil {
				return err
			}
		}
		return nil
	}
	materialized, err := manager.Materialize(catalog.Source{
		Name:         name,
		Registration: manifest.Catalogs[name],
	}, true)
	if err != nil {
		return err
	}
	skillNames, err := skillsToInstall(materialized, parsed.Skill, options.skills)
	if err != nil {
		return err
	}
	for _, skillName := range skillNames {
		if _, err := installNamed(command, scope, manifest, lock, skillName, name, options.requestedTargets, true, options.force, false, options.hooks, options.dryRun, options.overrideFlags); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(command.OutOrStdout(), "added %s from %s (%s)\n",
			skillName, lock.Skills[skillName].Catalog, summarizeTargets(lock.Skills[skillName].Targets))
	}
	return nil
}

func rejectNameCollision(manifest state.Manifest, name, source string) error {
	if name == catalog.BuiltinName && !catalog.SameSource(catalog.BuiltinSource, source) {
		if _, overridden := manifest.Catalogs[catalog.BuiltinName]; !overridden {
			return catalogNameCollisionError(name, catalog.BuiltinSource)
		}
	}
	existing, ok := manifest.Catalogs[name]
	if !ok {
		return nil
	}
	if catalog.SameSource(existing.Source, source) {
		return nil
	}
	return catalogNameCollisionError(name, existing.Source)
}

func catalogNameCollisionError(name, existingSource string) error {
	return fmt.Errorf("catalog %q is already registered from %s; pass --name to register this source under a different name",
		name, catalog.RedactSource(catalog.NormalizeSource(existingSource)))
}

func skillsToInstall(materialized catalog.Materialized, tail string, flagged []string) ([]string, error) {
	if materialized.Manifest.Catalog == nil {
		return nil, fmt.Errorf("catalog %q has no skills", materialized.Name)
	}
	offered := materialized.Manifest.Catalog.Skills
	var requested []string
	if tail != "" {
		requested = append(requested, tail)
	}
	requested = append(requested, flagged...)
	requested = dedupeNames(requested)
	if len(requested) == 0 {
		names := make([]string, 0, len(offered))
		for name := range offered {
			names = append(names, name)
		}
		sort.Strings(names)
		return names, nil
	}
	for _, name := range requested {
		if _, ok := offered[name]; !ok {
			return nil, fmt.Errorf("skill %q was not found in catalog %q", name, materialized.Name)
		}
	}
	return requested, nil
}
