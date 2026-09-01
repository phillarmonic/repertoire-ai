package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/phillarmonic/repertoire-ai/internal/stub"
	"github.com/spf13/cobra"
)

type resolvedStub struct {
	ID         string
	Definition stub.Definition
	AssetPath  string
}

func newStubCommand(globalScope, projectScope *bool) *cobra.Command {
	command := &cobra.Command{
		Use:   "stub",
		Short: "Discover file stubs from installed skills",
	}

	var raw bool
	get := &cobra.Command{
		Use:   "get <skill>/<stub>",
		Short: "Show an agent how to use an installed stub",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			_, _, lock, err := loadInstallationState(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			resolved, err := resolveStub(lock, args[0])
			if err != nil {
				return err
			}
			if raw {
				content, err := os.ReadFile(resolved.AssetPath)
				if err != nil {
					return fmt.Errorf("read stub %q asset: %w", resolved.ID, err)
				}
				_, err = command.OutOrStdout().Write(content)
				return err
			}
			_, _ = fmt.Fprintf(
				command.OutOrStdout(),
				"Stub: %s\nDescription: %s\nAsset: %s\nInstructions:\n%s\n",
				resolved.ID,
				strings.TrimSpace(resolved.Definition.Description),
				resolved.AssetPath,
				strings.TrimSpace(resolved.Definition.Instructions),
			)
			return nil
		},
	}
	get.Flags().BoolVar(&raw, "raw", false, "Write only the stub asset content to stdout, suitable for redirection")
	get.ValidArgsFunction = completeInstalledStubs(globalScope, projectScope)

	list := &cobra.Command{
		Use:   "list [skill]",
		Short: "List stubs exposed by installed skills",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			_, _, lock, err := loadInstallationState(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			names := sortedInstalledSkillNames(lock)
			if len(args) == 1 {
				if _, exists := lock.Skills[args[0]]; !exists {
					return fmt.Errorf("skill %q is not installed", args[0])
				}
				names = args
			}
			for _, name := range names {
				root, err := intactSkillRoot(name, lock.Skills[name])
				if err != nil {
					return err
				}
				manifest, err := stub.Load(root)
				if err != nil {
					return fmt.Errorf("load stubs for installed skill %q: %w", name, err)
				}
				stubNames := make([]string, 0, len(manifest.Stubs))
				for stubName := range manifest.Stubs {
					stubNames = append(stubNames, stubName)
				}
				sort.Strings(stubNames)
				for _, stubName := range stubNames {
					definition := manifest.Stubs[stubName]
					_, _ = fmt.Fprintf(command.OutOrStdout(), "%s/%s\t%s\n", name, stubName, strings.TrimSpace(definition.Description))
				}
			}
			return nil
		},
	}
	list.ValidArgsFunction = completeInstalledSkills(globalScope, projectScope)

	command.AddCommand(get, list)
	return command
}

func resolveStub(lock state.Lock, id string) (resolvedStub, error) {
	index := strings.LastIndex(id, "/")
	if index <= 0 || index == len(id)-1 {
		return resolvedStub{}, fmt.Errorf("stub id %q must be <skill>/<stub>", id)
	}
	skillName, stubName := id[:index], id[index+1:]
	if err := state.ValidateSkillReference(skillName); err != nil {
		return resolvedStub{}, fmt.Errorf("stub skill %q: %w", skillName, err)
	}
	if err := state.ValidateName(stubName); err != nil {
		return resolvedStub{}, fmt.Errorf("stub name %q: %w", stubName, err)
	}
	entry, exists := lock.Skills[skillName]
	if !exists {
		return resolvedStub{}, fmt.Errorf("skill %q is not installed", skillName)
	}
	root, err := intactSkillRoot(skillName, entry)
	if err != nil {
		return resolvedStub{}, err
	}
	manifest, err := stub.Load(root)
	if err != nil {
		return resolvedStub{}, fmt.Errorf("load stubs for installed skill %q: %w", skillName, err)
	}
	definition, exists := manifest.Stubs[stubName]
	if !exists {
		return resolvedStub{}, fmt.Errorf("stub %q is not exposed by installed skill %q", stubName, skillName)
	}
	assetPath, err := stub.AssetPath(root, definition)
	if err != nil {
		return resolvedStub{}, fmt.Errorf("resolve stub %q asset: %w", id, err)
	}
	return resolvedStub{ID: id, Definition: definition, AssetPath: assetPath}, nil
}

func intactSkillRoot(name string, entry state.LockSkill) (string, error) {
	locations := append([]string(nil), entry.Locations...)
	sort.Strings(locations)
	for _, location := range locations {
		digest, err := installer.Digest(location)
		if err == nil && digest == entry.Digest {
			return location, nil
		}
	}
	return "", fmt.Errorf("skill %q has no intact managed installation; run \"repertoire install %s\" to repair it", name, name)
}

func sortedInstalledSkillNames(lock state.Lock) []string {
	names := make([]string, 0, len(lock.Skills))
	for name := range lock.Skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func completeInstalledStubs(globalScope, projectScope *bool) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		_, _, lock, err := loadInstallationState(*globalScope, *projectScope)
		if err != nil {
			return nil, completionDirective
		}
		return installedStubCompletions(lock, toComplete), completionDirective
	}
}

func installedStubCompletions(lock state.Lock, toComplete string) []string {
	var completions []string
	for _, name := range sortedInstalledSkillNames(lock) {
		root, err := intactSkillRoot(name, lock.Skills[name])
		if err != nil {
			continue
		}
		manifest, err := stub.Load(root)
		if err != nil {
			continue
		}
		stubNames := make([]string, 0, len(manifest.Stubs))
		for stubName := range manifest.Stubs {
			stubNames = append(stubNames, stubName)
		}
		sort.Strings(stubNames)
		for _, stubName := range stubNames {
			id := name + "/" + stubName
			if strings.HasPrefix(id, toComplete) {
				completions = append(completions, id+"\t[stub] "+strings.TrimSpace(manifest.Stubs[stubName].Description))
			}
		}
	}
	return completions
}
