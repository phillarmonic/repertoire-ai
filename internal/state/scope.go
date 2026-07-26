package state

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ScopeOptions struct {
	Global    bool
	Project   bool
	Directory string
	ConfigDir string
}

type Scope struct {
	Global       bool
	Root         string
	ManifestPath string
	LockPath     string
}

func ResolveScope(options ScopeOptions) (Scope, error) {
	if options.Global && options.Project {
		return Scope{}, errors.New("--global and --project are mutually exclusive")
	}
	directory := options.Directory
	if directory == "" {
		var err error
		directory, err = os.Getwd()
		if err != nil {
			return Scope{}, fmt.Errorf("get current directory: %w", err)
		}
	}

	if options.Project {
		root, err := gitRoot(directory)
		if err != nil {
			return Scope{}, errors.New("--project requires a Git worktree")
		}
		return Scope{
			Root:         root,
			ManifestPath: filepath.Join(root, "repertoire.yaml"),
			LockPath:     filepath.Join(root, "repertoire.lock.json"),
		}, nil
	}

	configDir := options.ConfigDir
	if configDir == "" {
		var err error
		configDir, err = os.UserConfigDir()
		if err != nil {
			return Scope{}, fmt.Errorf("resolve user config directory: %w", err)
		}
		configDir = filepath.Join(configDir, "repertoire")
	}
	return Scope{
		Global:       true,
		Root:         configDir,
		ManifestPath: filepath.Join(configDir, "repertoire.yaml"),
		LockPath:     filepath.Join(configDir, "repertoire.lock.json"),
	}, nil
}

func gitRoot(directory string) (string, error) {
	command := exec.Command("git", "-C", directory, "rev-parse", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return filepath.Clean(strings.TrimSpace(string(output))), nil
}
