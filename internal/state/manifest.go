package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const SchemaVersion = 1

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Manifest struct {
	Schema       int                            `yaml:"schema"`
	Catalog      *CatalogDefinition             `yaml:"catalog,omitempty"`
	Catalogs     map[string]CatalogRegistration `yaml:"catalogs,omitempty"`
	Requirements map[string]Requirement         `yaml:"requirements,omitempty"`
}

type CatalogDefinition struct {
	Name        string                `yaml:"name"`
	Description string                `yaml:"description,omitempty"`
	Skills      map[string]SkillEntry `yaml:"skills"`
}

type SkillEntry struct {
	Path      string                     `yaml:"path"`
	Variants  map[string]string          `yaml:"variants,omitempty"`
	Artifacts map[string][]ArtifactEntry `yaml:"artifacts,omitempty"`
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
	Source string `yaml:"source"`
	Ref    string `yaml:"ref,omitempty"`
}

type Requirement struct {
	Catalog string   `yaml:"catalog"`
	Targets []string `yaml:"targets,omitempty"`
	Hooks   bool     `yaml:"hooks,omitempty"`
}

func NewManifest() Manifest {
	return Manifest{
		Schema:       SchemaVersion,
		Catalogs:     map[string]CatalogRegistration{},
		Requirements: map[string]Requirement{},
	}
}

func LoadManifest(path string) (Manifest, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NewManifest(), nil
	}
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}

	manifest := NewManifest()
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (m Manifest) Marshal() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	content, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	return content, nil
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
			for target, artifacts := range entry.Artifacts {
				if err := ValidateName(target); err != nil {
					return fmt.Errorf("skill %q artifact target %q: %w", name, target, err)
				}
				seen := map[string]struct{}{}
				for _, artifact := range artifacts {
					if err := ValidateName(artifact.ID); err != nil {
						return fmt.Errorf("skill %q artifact %q id: %w", name, artifact.ID, err)
					}
					if _, exists := seen[artifact.ID]; exists {
						return fmt.Errorf("skill %q repeats artifact id %q for target %q", name, artifact.ID, target)
					}
					seen[artifact.ID] = struct{}{}
					if err := ValidateRelativePath(artifact.Source); err != nil {
						return fmt.Errorf("skill %q artifact %q source: %w", name, artifact.ID, err)
					}
					if err := ValidateRelativePath(artifact.Destination); err != nil {
						return fmt.Errorf("skill %q artifact %q destination: %w", name, artifact.ID, err)
					}
					switch artifact.Mode {
					case ArtifactModeCopy, ArtifactModeMarkdownSection, ArtifactModeJSONMerge:
					default:
						return fmt.Errorf("skill %q artifact %q has unknown mode %q", name, artifact.ID, artifact.Mode)
					}
					if artifact.Executable && artifact.Mode != ArtifactModeCopy {
						return fmt.Errorf("skill %q artifact %q can only be executable in copy mode", name, artifact.ID)
					}
				}
			}
		}
	}
	for name, catalog := range m.Catalogs {
		if err := ValidateName(name); err != nil {
			return fmt.Errorf("catalog registration %q: %w", name, err)
		}
		if strings.TrimSpace(catalog.Source) == "" {
			return fmt.Errorf("catalog registration %q has an empty source", name)
		}
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
	parts := strings.Split(name, "/")
	for _, part := range parts {
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
