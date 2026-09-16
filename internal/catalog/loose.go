package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

// maxSkillDepth is the number of path components from the catalog root to a
// skill directory. Depth 4 covers skills/<category>/<category>/<skill> and
// marketplace trees such as plugins/<plugin>/skills/<skill>.
const maxSkillDepth = 4

var agentSkillRoots = map[string]struct{}{
	".agents": {},
	".claude": {},
	".cursor": {},
	".codex":  {},
	".github": {},
}

// Synthesize builds an in-memory catalog manifest from SKILL.md directories
// under root. The catalog name is the directory basename when that is a valid
// catalog name; Materialize overwrites it with the registration name.
func Synthesize(root string) (state.Manifest, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return state.Manifest{}, fmt.Errorf("resolve catalog root: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return state.Manifest{}, fmt.Errorf("inspect catalog root: %w", err)
	}
	if !info.IsDir() {
		return state.Manifest{}, fmt.Errorf("catalog root %s is not a directory", absolute)
	}

	skills := map[string]state.SkillEntry{}
	err = walkSkillDirs(absolute, absolute, 0, func(directory string) error {
		header, rel, ok := acceptSkillDir(absolute, directory)
		if !ok {
			return nil
		}
		if previous, exists := skills[header.Name]; exists {
			return fmt.Errorf("duplicate skill name %q at %s and %s", header.Name, previous.Path, rel)
		}
		skills[header.Name] = state.SkillEntry{Path: rel}
		return nil
	})
	if err != nil {
		return state.Manifest{}, err
	}
	if len(skills) == 0 {
		return state.Manifest{}, fmt.Errorf("no repertoire.yaml and no SKILL.md directories found under %s", absolute)
	}

	manifest := state.NewManifest()
	manifest.Catalog = &state.CatalogDefinition{
		Name:   looseCatalogName(absolute),
		Skills: skills,
	}
	return manifest, nil
}

func looseCatalogName(root string) string {
	base := filepath.Base(root)
	if err := state.ValidateName(base); err != nil {
		return ""
	}
	return base
}

func assignLooseCatalogName(manifest *state.Manifest, registrationName, root string) error {
	if manifest.Catalog == nil {
		return fmt.Errorf("synthesized catalog at %s has no catalog section", root)
	}
	if registrationName != "" {
		if err := state.ValidateName(registrationName); err != nil {
			return fmt.Errorf("catalog name: %w", err)
		}
		manifest.Catalog.Name = registrationName
		return nil
	}
	if manifest.Catalog.Name != "" {
		return nil
	}
	return errors.New("catalog name: must contain 1-64 lowercase letters, digits, or single hyphens; pass --name")
}

func acceptSkillDir(root, directory string) (SkillFrontmatter, string, bool) {
	if !pathInsideRoot(root, directory) {
		return SkillFrontmatter{}, "", false
	}
	skillFile := filepath.Join(directory, "SKILL.md")
	info, err := os.Lstat(skillFile)
	if err != nil {
		return SkillFrontmatter{}, "", false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, resolveErr := filepath.EvalSymlinks(skillFile)
		if resolveErr != nil || !pathInsideRoot(root, resolved) {
			return SkillFrontmatter{}, "", false
		}
	}
	// #nosec G304 -- directory is a walk result contained in root
	content, err := os.ReadFile(skillFile)
	if err != nil {
		return SkillFrontmatter{}, "", false
	}
	header, err := ParseSkillFrontmatter(content)
	if err != nil {
		return SkillFrontmatter{}, "", false
	}
	if state.ValidateName(header.Name) != nil {
		return SkillFrontmatter{}, "", false
	}
	if strings.TrimSpace(header.Description) == "" {
		return SkillFrontmatter{}, "", false
	}
	rel, err := filepath.Rel(root, directory)
	if err != nil {
		return SkillFrontmatter{}, "", false
	}
	return header, filepath.ToSlash(rel), true
}

func walkSkillDirs(root, current string, depth int, visit func(string) error) error {
	if depth > maxSkillDepth {
		return nil
	}
	if !pathInsideRoot(root, current) {
		return nil
	}
	if err := visit(current); err != nil {
		return err
	}
	if depth == maxSkillDepth {
		return nil
	}
	entries, err := os.ReadDir(current)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if skipWalkName(entry.Name()) {
			continue
		}
		child := filepath.Join(current, entry.Name())
		info, err := os.Lstat(child)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !info.IsDir() {
			continue
		}
		if err := walkSkillDirs(root, child, depth+1, visit); err != nil {
			return err
		}
	}
	return nil
}

func skipWalkName(name string) bool {
	switch name {
	case ".git", "node_modules":
		return true
	}
	if strings.HasPrefix(name, ".") {
		_, allowed := agentSkillRoots[name]
		return !allowed
	}
	return false
}

func pathInsideRoot(root, path string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
