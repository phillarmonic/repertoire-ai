package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/spf13/cobra"
)

const (
	catalogInitDescription = "Replace this placeholder with a short catalog description."
	skillInitDescription   = "Replace this placeholder with a one-line skill description."
)

func newCatalogInitCommand(force, dryRun *bool) *cobra.Command {
	var skills []string
	command := &cobra.Command{
		Use:               "init [name]",
		Short:             "Scaffold a catalog repository in the current directory",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, args []string) error {
			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve current directory: %w", err)
			}
			return runCatalogInit(command, root, optionalArg(args), skills, *force, *dryRun)
		},
	}
	command.Flags().StringArrayVar(&skills, "skill", nil, "skill name to scaffold (repeatable)")
	_ = command.RegisterFlagCompletionFunc("skill", cobra.NoFileCompletions)
	return command
}

func runCatalogInit(command *cobra.Command, root, nameArg string, skills []string, force, dryRun bool) error {
	name, err := catalogInitName(root, nameArg)
	if err != nil {
		return err
	}
	skillNames, err := catalogInitSkills(name, skills)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(root, "repertoire.yaml")
	if _, err := os.Stat(manifestPath); err == nil && !force {
		return errors.New("repertoire.yaml already exists; use --force to replace it")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect repertoire.yaml: %w", err)
	}

	entries := make(map[string]state.SkillEntry, len(skillNames))
	for _, skill := range skillNames {
		entries[skill] = state.SkillEntry{Path: filepath.ToSlash(filepath.Join("skills", skill))}
	}
	manifest := state.Manifest{
		Schema: state.SchemaVersion,
		Tool:   state.ManifestTool,
		Catalog: &state.CatalogDefinition{
			Name:        name,
			Description: catalogInitDescription,
			Skills:      entries,
		},
	}

	out := command.OutOrStdout()
	if dryRun {
		_, _ = fmt.Fprintf(out, "would create %s\n", filepath.Base(manifestPath))
		for _, skill := range skillNames {
			_, _ = fmt.Fprintf(out, "would create %s\n", filepath.ToSlash(filepath.Join("skills", skill, "SKILL.md")))
		}
		return nil
	}

	if err := state.SaveManifest(manifestPath, manifest); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "created %s\n", filepath.Base(manifestPath))
	for _, skill := range skillNames {
		skillFile := filepath.Join(root, "skills", skill, "SKILL.md")
		if err := state.WriteFileAtomic(skillFile, []byte(catalogInitSkillMarkdown(skill)), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", filepath.ToSlash(filepath.Join("skills", skill, "SKILL.md")), err)
		}
		_, _ = fmt.Fprintf(out, "created %s\n", filepath.ToSlash(filepath.Join("skills", skill, "SKILL.md")))
	}

	if err := validateCatalogInit(root, name, skillNames, entries); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "next: edit skills/*/SKILL.md, then `repertoire --override %s=. add %s --catalog %s`\n", name, skillNames[0], name)
	_, _ = fmt.Fprintf(out, "next: git init, commit, and push, then `repertoire catalog add <remote> --name %s`\n", name)
	return nil
}

func catalogInitName(root, nameArg string) (string, error) {
	if nameArg != "" {
		if err := state.ValidateName(nameArg); err != nil {
			return "", fmt.Errorf("catalog name: %w", err)
		}
		return nameArg, nil
	}
	name, err := catalog.DefaultCatalogName(root)
	if err != nil {
		return "", fmt.Errorf("catalog name derived from %q is invalid; pass a name", filepath.Base(root))
	}
	return name, nil
}

func catalogInitSkills(catalogName string, skills []string) ([]string, error) {
	if len(skills) == 0 {
		example := catalogName + "-example"
		if err := state.ValidateName(example); err != nil {
			return nil, fmt.Errorf("example skill %q: %w", example, err)
		}
		return []string{example}, nil
	}
	seen := make(map[string]struct{}, len(skills))
	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		if err := state.ValidateName(skill); err != nil {
			return nil, fmt.Errorf("skill %q: %w", skill, err)
		}
		if _, exists := seen[skill]; exists {
			return nil, fmt.Errorf("duplicate skill %q", skill)
		}
		seen[skill] = struct{}{}
		names = append(names, skill)
	}
	return names, nil
}

func catalogInitSkillMarkdown(name string) string {
	var body strings.Builder
	body.WriteString("---\n")
	body.WriteString("name: " + name + "\n")
	body.WriteString("description: " + skillInitDescription + "\n")
	body.WriteString("---\n\n")
	body.WriteString("# " + skillHeading(name) + "\n\n")
	body.WriteString("Instructions for the agent go here.\n")
	return body.String()
}

func skillHeading(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		runes := []rune(part)
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

func validateCatalogInit(root, name string, skillNames []string, entries map[string]state.SkillEntry) error {
	manager, err := catalog.NewManager("")
	if err != nil {
		return err
	}
	materialized, err := manager.Materialize(catalog.Source{
		Name:         name,
		Registration: state.CatalogRegistration{Source: root},
	}, false)
	if err != nil {
		return fmt.Errorf("load scaffolded catalog: %w", err)
	}
	if materialized.Loose {
		return errors.New("scaffolded catalog loaded as a loose catalog")
	}
	if materialized.Manifest.Catalog == nil || materialized.Manifest.Catalog.Name != name {
		return fmt.Errorf("scaffolded catalog name is %q", materialized.Manifest.Catalog.Name)
	}
	for _, skill := range skillNames {
		path := filepath.Join(root, filepath.FromSlash(entries[skill].Path))
		if err := installer.ValidateSkill(path, skill); err != nil {
			return fmt.Errorf("validate skill %q: %w", skill, err)
		}
	}
	return nil
}
