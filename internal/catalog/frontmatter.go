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
}

// ParseSkillFrontmatter reads the YAML document between the opening ---
// fences of a SKILL.md. It does not validate name or description.
func ParseSkillFrontmatter(content []byte) (SkillFrontmatter, error) {
	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[0]) != "" {
		return SkillFrontmatter{}, errors.New("invalid YAML frontmatter")
	}
	var header SkillFrontmatter
	if err := yaml.Unmarshal([]byte(parts[1]), &header); err != nil {
		return SkillFrontmatter{}, fmt.Errorf("decode YAML frontmatter: %w", err)
	}
	return header, nil
}
