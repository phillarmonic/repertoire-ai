package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"gopkg.in/yaml.v3"
)

type ResolvedSkill struct {
	Name    string
	Catalog catalog.Materialized
	Root    string
	Digest  string
}

func Resolve(manager *catalog.Manager, manifest state.Manifest, name, catalogName string, refresh bool) (ResolvedSkill, error) {
	namespace, skillName, err := catalog.ParseSkillID(name)
	if err != nil {
		return ResolvedSkill{}, err
	}
	var matches []ResolvedSkill
	for _, source := range catalog.Sources(manifest) {
		if catalogName != "" && source.Name != catalogName {
			continue
		}
		if namespace != "" && !catalog.SourceMatchesNamespace(source.Registration.Source, namespace) {
			continue
		}
		materialized, err := manager.Materialize(source, refresh)
		if err != nil {
			return ResolvedSkill{}, err
		}
		entry, exists := materialized.Manifest.Catalog.Skills[skillName]
		if !exists {
			continue
		}
		root := filepath.Join(materialized.Root, filepath.FromSlash(entry.Path))
		if err := ValidateSkill(root, skillName); err != nil {
			return ResolvedSkill{}, fmt.Errorf("catalog %q: %w", source.Name, err)
		}
		digest, err := Digest(root)
		if err != nil {
			return ResolvedSkill{}, err
		}
		matches = append(matches, ResolvedSkill{Name: skillName, Catalog: materialized, Root: root, Digest: digest})
	}
	if len(matches) == 0 {
		return ResolvedSkill{}, fmt.Errorf("skill %q was not found", name)
	}
	if len(matches) > 1 {
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].Catalog.Name < matches[j].Catalog.Name
		})
		definitions := make([]string, 0, len(matches))
		for _, match := range matches {
			source := catalog.RedactSource(catalog.NormalizeSource(match.Catalog.Registration.Source))
			id := catalog.SkillID(match.Catalog.Registration.Source, match.Name)
			definitions = append(definitions, fmt.Sprintf("  - %s (%s) [%s]", match.Catalog.Name, source, id))
		}
		return ResolvedSkill{}, fmt.Errorf(
			"skill %q is defined in multiple catalogs:\n%s\nspecify a catalog with --catalog <name> or a namespaced skill id",
			name,
			strings.Join(definitions, "\n"),
		)
	}
	return matches[0], nil
}

type skillHeader struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func ValidateSkill(root, expectedName string) error {
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("inspect skill %q: %w", expectedName, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("skill %q path is not a directory", expectedName)
	}
	if filepath.Base(root) != expectedName {
		return fmt.Errorf("skill %q directory name does not match", expectedName)
	}
	content, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("read skill %q SKILL.md: %w", expectedName, err)
	}
	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[0]) != "" {
		return fmt.Errorf("skill %q has invalid YAML frontmatter", expectedName)
	}
	var header skillHeader
	if err := yaml.Unmarshal([]byte(parts[1]), &header); err != nil {
		return fmt.Errorf("skill %q frontmatter: %w", expectedName, err)
	}
	if header.Name != expectedName {
		return fmt.Errorf("skill %q frontmatter name is %q", expectedName, header.Name)
	}
	if strings.TrimSpace(header.Description) == "" {
		return errors.New("skill description is required")
	}
	return nil
}
