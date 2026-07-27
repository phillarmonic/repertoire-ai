package stub

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/state"
	"gopkg.in/yaml.v3"
)

const (
	FileName      = "stubs.yaml"
	SchemaVersion = 1
)

type Manifest struct {
	Schema int                   `yaml:"schema"`
	Stubs  map[string]Definition `yaml:"stubs"`
}

type Definition struct {
	Description  string `yaml:"description"`
	Path         string `yaml:"path"`
	Instructions string `yaml:"instructions"`
}

// Load reads and validates a skill's optional stub manifest.
func Load(root string) (Manifest, error) {
	skillRoot, err := os.OpenRoot(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("open skill root: %w", err)
	}
	defer func() { _ = skillRoot.Close() }()
	file, err := skillRoot.Open(FileName)
	if errors.Is(err, os.ErrNotExist) {
		return Manifest{Schema: SchemaVersion, Stubs: map[string]Definition{}}, nil
	}
	if err != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", FileName, err)
	}
	content, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", FileName, readErr)
	}
	if closeErr != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", FileName, closeErr)
	}

	manifest := Manifest{}
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode %s: %w", FileName, err)
	}
	if manifest.Schema != SchemaVersion {
		return Manifest{}, fmt.Errorf("unsupported stub schema %d", manifest.Schema)
	}
	if manifest.Stubs == nil {
		manifest.Stubs = map[string]Definition{}
	}
	for name, definition := range manifest.Stubs {
		if err := state.ValidateName(name); err != nil {
			return Manifest{}, fmt.Errorf("stub %q: %w", name, err)
		}
		if strings.TrimSpace(definition.Description) == "" {
			return Manifest{}, fmt.Errorf("stub %q description is required", name)
		}
		if err := state.ValidateRelativePath(definition.Path); err != nil {
			return Manifest{}, fmt.Errorf("stub %q path: %w", name, err)
		}
		if strings.TrimSpace(definition.Instructions) == "" {
			return Manifest{}, fmt.Errorf("stub %q instructions are required", name)
		}
		if _, err := AssetPath(root, definition); err != nil {
			return Manifest{}, fmt.Errorf("stub %q asset: %w", name, err)
		}
	}
	return manifest, nil
}

// AssetPath resolves a declared asset and verifies that it is one contained
// regular file. A symlink is accepted only when its final target stays inside
// the skill root.
func AssetPath(root string, definition Definition) (string, error) {
	if err := state.ValidateRelativePath(definition.Path); err != nil {
		return "", err
	}
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	skillRoot, err := os.OpenRoot(rootPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = skillRoot.Close() }()
	relative := filepath.FromSlash(definition.Path)
	info, err := skillRoot.Stat(relative)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("path must resolve to a regular file")
	}
	return filepath.Join(rootPath, relative), nil
}
