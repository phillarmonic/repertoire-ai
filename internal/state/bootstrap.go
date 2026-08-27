package state

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// BootstrapFileName is the legacy bootstrap manifest filename. Bootstrap
// declarations now live in the `skills` section of repertoire.yaml; the
// legacy file is only read for automatic migration and completion fallback.
const BootstrapFileName = ".repertoire.yaml"

const (
	BootstrapScopeProject = "project"
	BootstrapScopeGlobal  = "global"
)

var bootstrapTargets = map[string]struct{}{
	"agents": {}, "aider": {}, "amp": {}, "antigravity": {}, "antigravity-windows": {},
	"claude": {}, "claw": {}, "cline": {}, "codebuddy": {}, "codex": {}, "copilot": {},
	"cursor": {}, "devin": {}, "droid": {}, "dsh": {}, "gemini": {}, "hermes": {}, "junie": {},
	"kilo": {}, "kimi": {}, "kiro": {}, "opencode": {}, "openclaw": {}, "pi": {},
	"roo": {}, "trae": {}, "trae-cn": {}, "vscode": {}, "windows": {}, "windsurf": {},
}

// BootstrapSkill declares one skill to install for a project bootstrap. It is
// the value type of the `skills` section in repertoire.yaml.
type BootstrapSkill struct {
	Catalog string   `yaml:"catalog,omitempty"`
	Scope   string   `yaml:"scope,omitempty"`
	Targets []string `yaml:"targets,omitempty"`
	Hooks   bool     `yaml:"hooks,omitempty"`
}

// BootstrapManifest is the legacy .repertoire.yaml format, kept only so
// existing projects can be migrated into repertoire.yaml.
type BootstrapManifest struct {
	Catalogs map[string]CatalogRegistration `yaml:"catalogs,omitempty"`
	Skills   map[string]BootstrapSkill      `yaml:"skills"`
	Tool     string                         `yaml:"tool,omitempty"`
	Schema   int                            `yaml:"schema"`
}

// LoadBootstrapManifest reads a legacy .repertoire.yaml. Unlike LoadManifest
// it reports an error when the file is missing, so callers can distinguish
// "nothing to migrate" from "migrate this".
func LoadBootstrapManifest(path string) (BootstrapManifest, error) {
	// #nosec G304 -- path is the bootstrap manifest path
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return BootstrapManifest{}, fmt.Errorf("bootstrap manifest %s does not exist: %w", path, err)
	}
	if err != nil {
		return BootstrapManifest{}, fmt.Errorf("read bootstrap manifest: %w", err)
	}
	manifest := BootstrapManifest{
		Schema:   SchemaVersion,
		Tool:     ManifestTool,
		Catalogs: map[string]CatalogRegistration{},
		Skills:   map[string]BootstrapSkill{},
	}
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return BootstrapManifest{}, fmt.Errorf("decode bootstrap manifest: %w", err)
	}
	defaultSkillScopes(manifest.Skills)
	if err := manifest.Validate(); err != nil {
		return BootstrapManifest{}, err
	}
	return manifest, nil
}

func (m BootstrapManifest) Validate() error {
	if m.Schema != SchemaVersion {
		return fmt.Errorf("unsupported bootstrap schema %d", m.Schema)
	}
	if len(m.Skills) == 0 {
		return errors.New("bootstrap manifest must declare at least one skill")
	}
	for name, registration := range m.Catalogs {
		if err := validateCatalogRegistration(name, registration); err != nil {
			return err
		}
	}
	if err := validateSkillDeclarations(m.Skills); err != nil {
		return err
	}
	// The legacy format is strict about dangling catalog references; the
	// unified Manifest tolerates them so catalog remove --force can save.
	for name, skill := range m.Skills {
		if skill.Catalog != "" && skill.Catalog != "phillarmonic" {
			if _, exists := m.Catalogs[skill.Catalog]; !exists {
				return fmt.Errorf("skill %q references unknown catalog %q", name, skill.Catalog)
			}
		}
	}
	return nil
}

func defaultSkillScopes(skills map[string]BootstrapSkill) {
	for name, skill := range skills {
		if skill.Scope == "" {
			skill.Scope = BootstrapScopeGlobal
			skills[name] = skill
		}
	}
}

func validateCatalogRegistration(name string, registration CatalogRegistration) error {
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
	return nil
}

func validateSkillDeclarations(skills map[string]BootstrapSkill) error {
	for name, skill := range skills {
		if err := ValidateSkillReference(name); err != nil {
			return fmt.Errorf("skill %q: %w", name, err)
		}
		if skill.Catalog != "" {
			if err := ValidateName(skill.Catalog); err != nil {
				return fmt.Errorf("skill %q catalog: %w", name, err)
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
