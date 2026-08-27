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

const completionDirective = cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder

func newCompletionCommand() *cobra.Command {
	command := &cobra.Command{
		Use:                   "completion <shell>",
		Short:                 "Generate shell completion scripts",
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		DisableFlagsInUseLine: true,
		RunE: func(command *cobra.Command, args []string) error {
			root := command.Root()
			output := command.OutOrStdout()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(output)
			case "zsh":
				return root.GenZshCompletion(output)
			case "fish":
				return root.GenFishCompletion(output, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(output)
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
	}
	return command
}

func completeAvailableSkills(globalScope, projectScope *bool, catalogName *string) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		manifest := knownManifest(*globalScope, *projectScope, "")
		completions := availableSkillCompletions(manifest, *catalogName, toComplete, "")
		return completions, completionDirective
	}
}

func availableSkillCompletions(manifest state.Manifest, catalogName, toComplete, cacheRoot string) []string {
	manager, err := catalog.NewManager(cacheRoot)
	if err != nil {
		return nil
	}
	type skillMatch struct {
		source   string
		catalogs []string
	}
	byName := map[string]skillMatch{}
	wantNamespaced := strings.Contains(toComplete, "/") || strings.Contains(toComplete, ".")
	for _, source := range catalog.Sources(manifest) {
		if catalogName != "" && source.Name != catalogName {
			continue
		}
		resolved, err := manager.InspectCached(source)
		if err != nil {
			continue
		}
		for name := range resolved.Manifest.Catalog.Skills {
			id := catalog.SkillID(source.Registration.Source, name)
			if wantNamespaced {
				if strings.HasPrefix(name, toComplete) {
					match := byName[name]
					match.catalogs = append(match.catalogs, source.Name)
					match.source = source.Registration.Source
					byName[name] = match
				}
				if strings.HasPrefix(id, toComplete) {
					match := byName[id]
					match.catalogs = append(match.catalogs, source.Name)
					match.source = source.Registration.Source
					byName[id] = match
				}
				continue
			}
			if strings.HasPrefix(name, toComplete) {
				match := byName[name]
				match.catalogs = append(match.catalogs, source.Name)
				match.source = source.Registration.Source
				byName[name] = match
			}
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	completions := make([]string, 0, len(names))
	for _, name := range names {
		match := byName[name]
		catalogs := append([]string(nil), match.catalogs...)
		sort.Strings(catalogs)
		completions = append(completions, name+"\t[available] "+strings.Join(catalogs, ", "))
	}
	return completions
}

func completeInstalledSkills(globalScope, projectScope *bool) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		_, _, lock, err := loadInstallationState(*globalScope, *projectScope)
		if err != nil {
			return nil, completionDirective
		}
		names := make([]string, 0, len(lock.Skills))
		for name := range lock.Skills {
			if strings.HasPrefix(name, toComplete) {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		completions := make([]string, 0, len(names))
		for _, name := range names {
			entry := lock.Skills[name]
			detail := "[" + entry.EffectiveOrigin() + "] " + entry.Catalog
			if len(entry.Targets) != 0 {
				detail += " → " + strings.Join(entry.Targets, ",")
			}
			completions = append(completions, name+"\t"+detail)
		}
		return completions, completionDirective
	}
}

func completeInstalledSkillsAndCatalogs(globalScope, projectScope *bool) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		_, manifest, lock, err := loadInstallationState(*globalScope, *projectScope)
		if err != nil {
			return nil, completionDirective
		}
		type completion struct {
			value  string
			detail string
		}
		byValue := map[string]completion{}
		for name, entry := range lock.Skills {
			if !strings.HasPrefix(name, toComplete) {
				continue
			}
			detail := "[" + entry.EffectiveOrigin() + "] " + entry.Catalog
			if len(entry.Targets) != 0 {
				detail += " → " + strings.Join(entry.Targets, ",")
			}
			byValue[name] = completion{value: name, detail: detail}
		}
		for _, source := range catalog.Sources(manifest) {
			if !strings.HasPrefix(source.Name, toComplete) {
				continue
			}
			if _, exists := byValue[source.Name]; exists {
				continue
			}
			kind := "catalog"
			if source.Builtin {
				kind = "built-in catalog"
			}
			byValue[source.Name] = completion{
				value:  source.Name,
				detail: "[" + kind + "] " + catalog.RedactSource(source.Registration.Source),
			}
		}
		values := make([]string, 0, len(byValue))
		for value := range byValue {
			values = append(values, value)
		}
		sort.Strings(values)
		completions := make([]string, 0, len(values))
		for _, value := range values {
			match := byValue[value]
			completions = append(completions, match.value+"\t"+match.detail)
		}
		return completions, completionDirective
	}
}

func completeCatalogs(globalScope, projectScope *bool, registeredOnly bool) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if registeredOnly {
			_, manifest, err := loadScope(*globalScope, *projectScope)
			if err != nil {
				return nil, completionDirective
			}
			names := make([]string, 0, len(manifest.Catalogs))
			for name := range manifest.Catalogs {
				names = append(names, name)
			}
			sort.Strings(names)
			completions := make([]string, 0, len(names))
			for _, name := range names {
				if strings.HasPrefix(name, toComplete) {
					completions = append(completions, name+"\t[registered] "+catalog.RedactSource(manifest.Catalogs[name].Source))
				}
			}
			return completions, completionDirective
		}

		completions := make([]string, 0)
		for _, known := range knownCatalogs(*globalScope, *projectScope, "") {
			if !strings.HasPrefix(known.Name, toComplete) {
				continue
			}
			completions = append(completions, known.Name+"\t["+known.Kind+"] "+known.Source)
		}
		return completions, completionDirective
	}
}

func completeCatalogSources(globalScope, projectScope *bool) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		seen := map[string]string{}
		add := func(source, detail string) {
			source = catalog.RedactSource(catalog.NormalizeSource(source))
			compact := strings.TrimSuffix(strings.TrimPrefix(source, "https://"), ".git")
			for _, candidate := range []string{compact, source} {
				if candidate == "" || !strings.HasPrefix(candidate, toComplete) {
					continue
				}
				if _, exists := seen[candidate]; exists {
					continue
				}
				seen[candidate] = detail
			}
		}

		add(catalog.BuiltinSource, "[built-in]")
		for _, known := range knownCatalogs(*globalScope, *projectScope, "") {
			add(known.Source, "["+known.Kind+"] "+known.Name)
		}

		candidates := make([]string, 0, len(seen))
		for candidate := range seen {
			candidates = append(candidates, candidate)
		}
		sort.Strings(candidates)
		completions := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			completions = append(completions, candidate+"\t"+seen[candidate])
		}
		return completions, completionDirective
	}
}

func completeTargets(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	targets := append([]string{"all"}, installer.SupportedTargetNames()...)
	completions := make([]string, 0, len(targets))
	for _, target := range targets {
		if strings.HasPrefix(target, toComplete) {
			detail := "[agent target]"
			if target == "all" {
				detail = "[all agent targets]"
			}
			completions = append(completions, target+"\t"+detail)
		}
	}
	return completions, completionDirective
}

func completeListFormats(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	formats := []struct {
		name, detail string
	}{
		{string(skillListFormatAuto), "[table in terminals, TSV when redirected]"},
		{string(skillListFormatTable), "[compact table]"},
		{string(skillListFormatTSV), "[headerless tab-separated values]"},
		{string(skillListFormatJSON), "[JSON array]"},
	}
	var completions []string
	for _, format := range formats {
		if strings.HasPrefix(format.name, toComplete) {
			completions = append(completions, format.name+"\t"+format.detail)
		}
	}
	return completions, completionDirective
}
