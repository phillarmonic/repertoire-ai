package state

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const BootstrapFileName = ".repertoire.yaml"

const (
	BootstrapScopeProject = "project"
	BootstrapScopeGlobal  = "global"
)

var bootstrapTargets = map[string]struct{}{
	"agents": {}, "aider": {}, "amp": {}, "antigravity": {}, "antigravity-windows": {},
	"claude": {}, "claw": {}, "cline": {}, "codebuddy": {}, "codex": {}, "copilot": {},
	"cursor": {}, "devin": {}, "droid": {}, "gemini": {}, "hermes": {}, "junie": {},
	"kilo": {}, "kimi": {}, "kiro": {}, "opencode": {}, "openclaw": {}, "pi": {},
	"roo": {}, "trae": {}, "trae-cn": {}, "vscode": {}, "windows": {}, "windsurf": {},
}

type BootstrapManifest struct {
	Schema   int                            `yaml:"schema"`
	Catalogs map[string]CatalogRegistration `yaml:"catalogs,omitempty"`
	Skills   map[string]BootstrapSkill      `yaml:"skills"`
}

type BootstrapSkill struct {
	Catalog string   `yaml:"catalog,omitempty"`
	Scope   string   `yaml:"scope,omitempty"`
	Targets []string `yaml:"targets,omitempty"`
	Hooks   bool     `yaml:"hooks,omitempty"`
}

func LoadBootstrapManifest(path string) (BootstrapManifest, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return BootstrapManifest{}, fmt.Errorf("bootstrap manifest %s does not exist: %w", path, err)
	}
	if err != nil {
		return BootstrapManifest{}, fmt.Errorf("read bootstrap manifest: %w", err)
	}
	manifest := BootstrapManifest{
		Schema:   SchemaVersion,
		Catalogs: map[string]CatalogRegistration{},
		Skills:   map[string]BootstrapSkill{},
	}
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return BootstrapManifest{}, fmt.Errorf("decode bootstrap manifest: %w", err)
	}
	for name, skill := range manifest.Skills {
		if skill.Scope == "" {
			skill.Scope = BootstrapScopeGlobal
			manifest.Skills[name] = skill
		}
	}
	if err := manifest.Validate(); err != nil {
		return BootstrapManifest{}, err
	}
	return manifest, nil
}

func (m BootstrapManifest) Marshal() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	content, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encode bootstrap manifest: %w", err)
	}
	return content, nil
}

func (m BootstrapManifest) Validate() error {
	if m.Schema != SchemaVersion {
		return fmt.Errorf("unsupported bootstrap schema %d", m.Schema)
	}
	if len(m.Skills) == 0 {
		return errors.New("bootstrap manifest must declare at least one skill")
	}
	for name, registration := range m.Catalogs {
		if err := ValidateName(name); err != nil {
			return fmt.Errorf("catalog registration %q: %w", name, err)
		}
		if strings.TrimSpace(registration.Source) == "" {
			return fmt.Errorf("catalog registration %q has an empty source", name)
		}
		parsed, err := url.Parse(registration.Source)
		if err == nil && parsed.User != nil {
			return fmt.Errorf("catalog registration %q must not contain embedded credentials", name)
		}
	}
	for name, skill := range m.Skills {
		if err := ValidateSkillReference(name); err != nil {
			return fmt.Errorf("skill %q: %w", name, err)
		}
		if skill.Catalog != "" {
			if err := ValidateName(skill.Catalog); err != nil {
				return fmt.Errorf("skill %q catalog: %w", name, err)
			}
			if skill.Catalog != "phillarmonic" {
				if _, exists := m.Catalogs[skill.Catalog]; !exists {
					return fmt.Errorf("skill %q references unknown catalog %q", name, skill.Catalog)
				}
			}
		}
		if skill.Scope != BootstrapScopeProject && skill.Scope != BootstrapScopeGlobal {
			return fmt.Errorf("skill %q scope must be project or global", name)
		}
		seenTargets := map[string]struct{}{}
		for _, target := range skill.Targets {
			if _, supported := bootstrapTargets[target]; !supported {
				return fmt.Errorf("skill %q has unknown target %q", name, target)
			}
			if _, duplicate := seenTargets[target]; duplicate {
				return fmt.Errorf("skill %q repeats target %q", name, target)
			}
			seenTargets[target] = struct{}{}
		}
	}
	return nil
}

func (m BootstrapManifest) ResolutionManifest() Manifest {
	manifest := NewManifest()
	for name, registration := range m.Catalogs {
		manifest.Catalogs[name] = registration
	}
	return manifest
}
