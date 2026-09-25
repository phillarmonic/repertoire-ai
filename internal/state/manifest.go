package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	SchemaVersion = 1
	ManifestTool  = "https://github.com/phillarmonic/repertoire-ai"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Manifest struct {
	Catalog      *CatalogDefinition             `yaml:"catalog,omitempty"`
	Catalogs     map[string]CatalogRegistration `yaml:"catalogs,omitempty"`
	Skills       map[string]BootstrapSkill      `yaml:"skills,omitempty"`
	Requirements map[string]Requirement         `yaml:"requirements,omitempty"`
	// Directory is the folder that contains this manifest. Trust key paths
	// are resolved from here. It is runtime state and is not written back.
	Directory string `yaml:"-"`
	Tool      string `yaml:"tool,omitempty"`
	Schema    int    `yaml:"schema"`
}

type CatalogDefinition struct {
	Skills      map[string]SkillEntry `yaml:"skills"`
	Name        string                `yaml:"name"`
	Description string                `yaml:"description,omitempty"`
}

type SkillEntry struct {
	Variants     map[string]string          `yaml:"variants,omitempty"`
	Instructions map[string][]ArtifactEntry `yaml:"instructions,omitempty"`
	Artifacts    map[string][]ArtifactEntry `yaml:"artifacts,omitempty"`
	Path         string                     `yaml:"path"`
}

const (
	ArtifactModeCopy            = "copy"
	ArtifactModeMarkdownSection = "markdown-section"
	ArtifactModeJSONMerge       = "json-merge"
)

type ArtifactEntry struct {
	ID          string `yaml:"id"`
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
	Mode        string `yaml:"mode"`
	Executable  bool   `yaml:"executable,omitempty"`
}

type CatalogRegistration struct {
	Trust  *CatalogTrust `yaml:"trust,omitempty"`
	Source string        `yaml:"source"`
	Ref    string        `yaml:"ref,omitempty"`
}

// CatalogTrust names the public keys allowed to speak for one catalog.
// It is omitted when a registration does not certify its catalog.
type CatalogTrust struct {
	Keys []CatalogTrustKey `yaml:"keys"`
}

// CatalogTrustKey is one armored public key file and the fingerprint it must match.
// Path is relative to the manifest directory.
type CatalogTrustKey struct {
	Path        string `yaml:"path"`
	Fingerprint string `yaml:"fingerprint"`
}

type Requirement struct {
	Catalog string   `yaml:"catalog"`
	Targets []string `yaml:"targets,omitempty"`
	Hooks   bool     `yaml:"hooks,omitempty"`
}

func NewManifest() Manifest {
	return Manifest{
		Schema:       SchemaVersion,
		Tool:         ManifestTool,
		Catalogs:     map[string]CatalogRegistration{},
		Skills:       map[string]BootstrapSkill{},
		Requirements: map[string]Requirement{},
	}
}

func LoadManifest(path string) (Manifest, error) {
	// #nosec G304 -- path is the resolved manifest path
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		manifest := NewManifest()
		manifest.Directory = filepath.Dir(path)
		return manifest, nil
	}
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}

	manifest := NewManifest()
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	defaultSkillScopes(manifest.Skills)
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	manifest.Directory = filepath.Dir(path)
	return manifest, nil
}

func (m Manifest) Marshal() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	// Encode with a fixed 2-space indent and sequences nested under their
	// parent key so generated manifests keep one consistent indentation
	// style instead of the mixed alignment yaml.v3 produced.
	content, err := yaml.MarshalWithOptions(m, yaml.Indent(2), yaml.IndentSequence(true))
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	return content, nil
}

func (m Manifest) MarshalYAML() (any, error) {
	items := yaml.MapSlice{{Key: "schema", Value: m.Schema}}
	items = appendYAML(items, "tool", m.Tool, true)
	items = appendYAML(items, "catalog", m.Catalog, true)
	items = appendYAML(items, "catalogs", m.Catalogs, true)
	items = appendYAML(items, "skills", m.Skills, true)
	items = appendYAML(items, "requirements", m.Requirements, true)
	return items, nil
}

func (c CatalogRegistration) MarshalYAML() (any, error) {
	items := yaml.MapSlice{{Key: "source", Value: c.Source}}
	items = appendYAML(items, "ref", c.Ref, true)
	items = appendYAML(items, "trust", c.Trust, true)
	return items, nil
}

func (c CatalogDefinition) MarshalYAML() (any, error) {
	items := yaml.MapSlice{{Key: "name", Value: c.Name}}
	items = appendYAML(items, "description", c.Description, true)
	items = append(items, yaml.MapItem{Key: "skills", Value: c.Skills})
	return items, nil
}

func (e SkillEntry) MarshalYAML() (any, error) {
	items := yaml.MapSlice{{Key: "path", Value: e.Path}}
	items = appendYAML(items, "variants", e.Variants, true)
	items = appendYAML(items, "instructions", e.Instructions, true)
	items = appendYAML(items, "artifacts", e.Artifacts, true)
	return items, nil
}

func appendYAML(items yaml.MapSlice, key string, value any, omitempty bool) yaml.MapSlice {
	if omitempty && yamlEmpty(value) {
		return items
	}
	return append(items, yaml.MapItem{Key: key, Value: value})
}

func yamlEmpty(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed == ""
	case *CatalogDefinition:
		return typed == nil
	case *CatalogTrust:
		return typed == nil
	case map[string]CatalogRegistration:
		return len(typed) == 0
	case map[string]BootstrapSkill:
		return len(typed) == 0
	case map[string]Requirement:
		return len(typed) == 0
	case map[string]string:
		return len(typed) == 0
	case map[string][]ArtifactEntry:
		return len(typed) == 0
	default:
		return value == nil
	}
}

func (m Manifest) Validate() error {
	if m.Schema != SchemaVersion {
		return fmt.Errorf("unsupported repertoire schema %d", m.Schema)
	}
	if m.Catalog != nil {
		if err := ValidateName(m.Catalog.Name); err != nil {
			return fmt.Errorf("catalog name: %w", err)
		}
		if len(m.Catalog.Skills) == 0 {
			return errors.New("catalog must declare at least one skill")
		}
		for name, entry := range m.Catalog.Skills {
			if err := ValidateCatalogSkillName(name); err != nil {
				return fmt.Errorf("skill %q: %w", name, err)
			}
			if err := ValidateRelativePath(entry.Path); err != nil {
				return fmt.Errorf("skill %q path: %w", name, err)
			}
			for target, path := range entry.Variants {
				if err := ValidateName(target); err != nil {
					return fmt.Errorf("skill %q variant %q: %w", name, target, err)
				}
				if err := ValidateRelativePath(path); err != nil {
					return fmt.Errorf("skill %q variant %q path: %w", name, target, err)
				}
			}
			usedArtifactIDs := map[string]map[string]string{}
			if err := validateArtifactEntries(name, "instruction", entry.Instructions, usedArtifactIDs); err != nil {
				return err
			}
			if err := validateArtifactEntries(name, "artifact", entry.Artifacts, usedArtifactIDs); err != nil {
				return err
			}
		}
	}
	for name, catalog := range m.Catalogs {
		if err := validateCatalogRegistration(name, catalog); err != nil {
			return err
		}
	}
	if err := validateSkillDeclarations(m.Skills); err != nil {
		return err
	}
	for name, requirement := range m.Requirements {
		if err := ValidateSkillReference(name); err != nil {
			return fmt.Errorf("requirement %q: %w", name, err)
		}
		if err := ValidateName(requirement.Catalog); err != nil {
			return fmt.Errorf("requirement %q catalog: %w", name, err)
		}
	}
	return nil
}

func validateArtifactEntries(
	skillName, kind string,
	entries map[string][]ArtifactEntry,
	usedIDs map[string]map[string]string,
) error {
	for target, artifacts := range entries {
		if err := ValidateName(target); err != nil {
			return fmt.Errorf("skill %q %s target %q: %w", skillName, kind, target, err)
		}
		if usedIDs[target] == nil {
			usedIDs[target] = map[string]string{}
		}
		for _, artifact := range artifacts {
			if err := ValidateName(artifact.ID); err != nil {
				return fmt.Errorf("skill %q %s %q id: %w", skillName, kind, artifact.ID, err)
			}
			if previousKind, exists := usedIDs[target][artifact.ID]; exists {
				return fmt.Errorf(
					"skill %q repeats managed artifact id %q for target %q across %s and %s entries",
					skillName,
					artifact.ID,
					target,
					previousKind,
					kind,
				)
			}
			usedIDs[target][artifact.ID] = kind
			if err := ValidateRelativePath(artifact.Source); err != nil {
				return fmt.Errorf("skill %q %s %q source: %w", skillName, kind, artifact.ID, err)
			}
			if err := ValidateRelativePath(artifact.Destination); err != nil {
				return fmt.Errorf("skill %q %s %q destination: %w", skillName, kind, artifact.ID, err)
			}
			switch artifact.Mode {
			case ArtifactModeCopy, ArtifactModeMarkdownSection, ArtifactModeJSONMerge:
			default:
				return fmt.Errorf("skill %q %s %q has unknown mode %q", skillName, kind, artifact.ID, artifact.Mode)
			}
			if artifact.Executable && artifact.Mode != ArtifactModeCopy {
				return fmt.Errorf("skill %q %s %q can only be executable in copy mode", skillName, kind, artifact.ID)
			}
		}
	}
	return nil
}

func ValidateName(name string) error {
	if len(name) == 0 || len(name) > 64 || !skillNamePattern.MatchString(name) {
		return errors.New("must contain 1-64 lowercase letters, digits, or single hyphens")
	}
	return nil
}

func ValidateCatalogSkillName(name string) error {
	if strings.TrimSpace(name) != name || name == "" {
		return errors.New("skill name is required")
	}
	parts := strings.SplitSeq(name, "/")
	for part := range parts {
		if err := ValidateName(part); err != nil {
			return fmt.Errorf("segment %q: %w", part, err)
		}
	}
	return nil
}

// ValidateSkillReference accepts a short skill name or a namespaced skill ID
// such as github.com/phillarmonic/ai-skills/zensical.
func ValidateSkillReference(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("skill id is required")
	}
	if !strings.Contains(id, "/") {
		return ValidateName(id)
	}
	index := strings.LastIndex(id, "/")
	namespace := id[:index]
	skillName := id[index+1:]
	if namespace == "" {
		return errors.New("skill namespace is required")
	}
	if strings.Contains(namespace, " ") {
		return errors.New("skill namespace must not contain spaces")
	}
	if err := ValidateName(skillName); err != nil {
		return fmt.Errorf("skill name: %w", err)
	}
	return nil
}

func ValidateRelativePath(path string) error {
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(path) || cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return errors.New("must be a contained relative path")
	}
	return nil
}
