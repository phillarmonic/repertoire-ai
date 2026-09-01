package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/phillarmonic/repertoire-ai/internal/stub"
)

type ResolvedSkill struct {
	Variants     map[string]ResolvedVariant
	Instructions map[string][]ResolvedArtifact
	Artifacts    map[string][]ResolvedArtifact
	Name         string
	Root         string
	Digest       string
	Catalog      catalog.Materialized
}

type ResolvedVariant struct {
	Root   string
	Digest string
}

type ResolvedArtifact struct {
	SourcePath string
	Digest     string
	state.ArtifactEntry
}

func Resolve(manager *catalog.Manager, manifest state.Manifest, name, catalogName string, refresh bool) (ResolvedSkill, error) {
	name = strings.TrimSpace(name)
	namespace, skillName, err := catalog.ParseSkillID(name)
	if err != nil {
		return ResolvedSkill{}, err
	}
	sources := catalog.Sources(manifest)
	if namespace != "" {
		qualified := make([]catalog.Source, 0, 1)
		for _, source := range sources {
			if catalog.SourceMatchesNamespace(source.Registration.Source, namespace) {
				qualified = append(qualified, source)
			}
		}
		// A slash is also valid inside a logical catalog skill key, such as
		// "phillarmonkey/code". Only narrow the source set when the parsed
		// namespace actually identifies a visible catalog.
		if len(qualified) != 0 {
			sources = qualified
		}
	}
	preferMainline := namespace == "" && catalogName == ""
	if preferMainline {
		sort.SliceStable(sources, func(i, j int) bool {
			return sources[i].Name == catalog.BuiltinName && sources[j].Name != catalog.BuiltinName
		})
	}
	var matches []ResolvedSkill
	for _, source := range sources {
		if catalogName != "" && source.Name != catalogName {
			continue
		}
		materialized, err := manager.Materialize(source, refresh)
		if err != nil {
			return ResolvedSkill{}, err
		}
		for _, candidate := range skillCandidates(name, namespace, skillName, source) {
			entry, exists := materialized.Manifest.Catalog.Skills[candidate]
			if !exists {
				continue
			}
			root := filepath.Join(materialized.Root, filepath.FromSlash(entry.Path))
			if err := ValidateSkill(root, candidate); err != nil {
				return ResolvedSkill{}, fmt.Errorf("catalog %q: %w", source.Name, err)
			}
			digest, err := Digest(root)
			if err != nil {
				return ResolvedSkill{}, err
			}
			resolved := ResolvedSkill{
				Name: candidate, Catalog: materialized, Root: root, Digest: digest,
				Variants:     map[string]ResolvedVariant{},
				Instructions: map[string][]ResolvedArtifact{},
				Artifacts:    map[string][]ResolvedArtifact{},
			}
			for target, path := range entry.Variants {
				variantRoot := filepath.Join(materialized.Root, filepath.FromSlash(path))
				if err := validateSkill(variantRoot, candidate, false); err != nil {
					return ResolvedSkill{}, fmt.Errorf("catalog %q variant %q: %w", source.Name, target, err)
				}
				variantDigest, err := Digest(variantRoot)
				if err != nil {
					return ResolvedSkill{}, err
				}
				resolved.Variants[target] = ResolvedVariant{Root: variantRoot, Digest: variantDigest}
			}
			resolveArtifacts := func(
				kind string,
				declarationsByTarget map[string][]state.ArtifactEntry,
				destination map[string][]ResolvedArtifact,
			) error {
				for target, declarations := range declarationsByTarget {
					for _, declaration := range declarations {
						sourcePath := filepath.Join(materialized.Root, filepath.FromSlash(declaration.Source))
						info, err := os.Lstat(sourcePath)
						if err != nil {
							return fmt.Errorf("catalog %q %s %q: %w", source.Name, kind, declaration.ID, err)
						}
						if !info.Mode().IsRegular() {
							return fmt.Errorf("catalog %q %s %q source is not a regular file", source.Name, kind, declaration.ID)
						}
						artifactDigest, err := DigestFile(sourcePath)
						if err != nil {
							return err
						}
						destination[target] = append(destination[target], ResolvedArtifact{
							ArtifactEntry: declaration, SourcePath: sourcePath, Digest: artifactDigest,
						})
					}
				}
				return nil
			}
			if err := resolveArtifacts("instruction", entry.Instructions, resolved.Instructions); err != nil {
				return ResolvedSkill{}, err
			}
			if err := resolveArtifacts("artifact", entry.Artifacts, resolved.Artifacts); err != nil {
				return ResolvedSkill{}, err
			}
			if preferMainline && source.Name == catalog.BuiltinName {
				return resolved, nil
			}
			matches = append(matches, resolved)
		}
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

func skillCandidates(requested, namespace, skillName string, source catalog.Source) []string {
	candidates := make([]string, 0, 2)
	seen := map[string]struct{}{}
	add := func(candidate string) {
		if candidate == "" {
			return
		}
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}
	if namespace == "" || catalog.SourceMatchesNamespace(source.Registration.Source, namespace) {
		add(skillName)
	}
	if requested != skillName {
		add(requested)
	}
	return candidates
}

type skillHeader struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func ValidateSkill(root, expectedName string) error {
	return validateSkill(root, expectedName, true)
}

func validateSkill(root, expectedName string, requireMatchingDirectory bool) error {
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect skill %q: %w", expectedName, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("skill %q path is not a directory", expectedName)
	}
	if requireMatchingDirectory && filepath.Base(root) != catalogSkillLeaf(expectedName) {
		return fmt.Errorf("skill %q directory name does not match", expectedName)
	}
	// #nosec G304 -- root is the resolved skill directory
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
	if _, err := stub.Load(root); err != nil {
		return err
	}
	return nil
}

func catalogSkillLeaf(name string) string {
	index := strings.LastIndex(name, "/")
	if index < 0 {
		return name
	}
	return name[index+1:]
}
