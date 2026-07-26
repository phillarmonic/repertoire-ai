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
		_, manifest, err := loadScope(*globalScope, *projectScope)
		if err != nil {
			return nil, completionDirective
		}
		completions := availableSkillCompletions(manifest, *catalogName, toComplete, "")
		return completions, completionDirective
	}
}

func availableSkillCompletions(manifest state.Manifest, catalogName, toComplete, cacheRoot string) []string {
	manager, err := catalog.NewManager(cacheRoot)
	if err != nil {
		return nil
	}
	byName := map[string][]string{}
	for _, source := range catalog.Sources(manifest) {
		if catalogName != "" && source.Name != catalogName {
			continue
		}
		resolved, err := manager.InspectCached(source)
		if err != nil {
			continue
		}
		for name := range resolved.Manifest.Catalog.Skills {
			if strings.HasPrefix(name, toComplete) {
				byName[name] = append(byName[name], source.Name)
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
		catalogs := byName[name]
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

func completeCatalogs(globalScope, projectScope *bool, registeredOnly bool) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		_, manifest, err := loadScope(*globalScope, *projectScope)
		if err != nil {
			return nil, completionDirective
		}
		var completions []string
		if registeredOnly {
			names := make([]string, 0, len(manifest.Catalogs))
			for name := range manifest.Catalogs {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				if strings.HasPrefix(name, toComplete) {
					completions = append(completions, name+"\t[registered] "+catalog.RedactSource(manifest.Catalogs[name].Source))
				}
			}
			return completions, completionDirective
		}
		for _, source := range catalog.Sources(manifest) {
			if strings.HasPrefix(source.Name, toComplete) {
				kind := "[catalog] "
				if source.Builtin {
					kind = "[built-in] "
				}
				completions = append(completions, source.Name+"\t"+kind+catalog.RedactSource(source.Registration.Source))
			}
		}
		sort.Strings(completions)
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
