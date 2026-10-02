package catalog

import (
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

// SkillFrontmatter is the YAML header required of every SKILL.md.
type SkillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	// DisableModelInvocation marks a skill the model must not load on its own,
	// so a host that can list user-invoked entries separately should offer it
	// there instead. It is read from the frontmatter only: a SKILL.md may well
	// mention the key in its prose.
	DisableModelInvocation bool `yaml:"disable-model-invocation"`
	// ArgumentHint describes the arguments the skill takes, in the same
	// convention. Its presence is what says the skill expects to be invoked
	// with input.
	ArgumentHint string `yaml:"argument-hint"`
}

// rawFrontmatter mirrors SkillFrontmatter with the convention fields left
// untyped.
//
// A SKILL.md is written by other tools and by hand, and the convention admits
// shapes a strict field rejects: `argument-hint: [target]` is a YAML sequence
// unless the author quotes it, and the flag may be written as a quoted string.
// A decode failure here is reported as invalid frontmatter, which fails the
// skill outright — so a skill that installed before this field existed must not
// stop installing because of how it spells a key this tool merely reads.
type rawFrontmatter struct {
	Name                   string `yaml:"name"`
	Description            string `yaml:"description"`
	DisableModelInvocation any    `yaml:"disable-model-invocation"`
	ArgumentHint           any    `yaml:"argument-hint"`
}

// ParseSkillFrontmatter reads the YAML document between the opening ---
// fences of a SKILL.md. It does not validate name or description.
func ParseSkillFrontmatter(content []byte) (SkillFrontmatter, error) {
	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[0]) != "" {
		return SkillFrontmatter{}, errors.New("invalid YAML frontmatter")
	}
	var raw rawFrontmatter
	if err := yaml.Unmarshal([]byte(parts[1]), &raw); err != nil {
		return SkillFrontmatter{}, fmt.Errorf("decode YAML frontmatter: %w", err)
	}
	return SkillFrontmatter{
		Name:                   raw.Name,
		Description:            raw.Description,
		DisableModelInvocation: truthy(raw.DisableModelInvocation),
		ArgumentHint:           hintText(raw.ArgumentHint),
	}, nil
}

// truthy reads a flag that may be written as a boolean or as text. Anything
// unrecognised is false, which is the safe reading: it leaves the skill
// model-invocable rather than hiding it from the model.
func truthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

// hintText renders an argument hint written either as a string or as the bare
// bracket list the convention allows.
func hintText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, fmt.Sprint(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return ""
	}
}
