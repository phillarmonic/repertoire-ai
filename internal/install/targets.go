package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

type Target struct {
	Name string
	Root string
}

var supportedTargetNames = []string{
	"agents", "claude", "cline", "codex", "copilot", "cursor", "gemini",
	"junie", "kimi", "kiro", "openclaw", "opencode", "roo", "windsurf",
}

func SupportedTargetNames() []string {
	return append([]string(nil), supportedTargetNames...)
}

func ResolveTargets(scope state.Scope, requested []string, home string) ([]Target, error) {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}
	if len(requested) > 0 {
		expanded := requested
		for _, name := range requested {
			if name == "all" {
				expanded = supportedTargetNames
				break
			}
		}
		result := make([]Target, 0, len(expanded))
		for _, name := range expanded {
			root, err := targetRoot(scope, home, name)
			if err != nil {
				return nil, err
			}
			result = append(result, Target{Name: name, Root: root})
		}
		return deduplicateTargets(result), nil
	}

	var result []Target
	for _, name := range supportedTargetNames {
		if name == "agents" {
			continue
		}
		root, err := targetRoot(scope, home, name)
		if err != nil {
			continue
		}
		if _, err := os.Stat(targetMarker(scope, home, name, root)); err == nil {
			result = append(result, Target{Name: name, Root: root})
		}
	}
	if len(result) == 0 {
		return nil, errors.New("no supported agent clients detected; use --target")
	}
	return deduplicateTargets(result), nil
}

func targetRoot(scope state.Scope, home, name string) (string, error) {
	if scope.Global {
		switch name {
		case "codex":
			base := os.Getenv("CODEX_HOME")
			if base == "" {
				base = filepath.Join(home, ".codex")
			}
			return filepath.Join(base, "skills"), nil
		case "claude":
			return filepath.Join(home, ".claude", "skills"), nil
		case "cline":
			return filepath.Join(home, ".cline", "skills"), nil
		case "copilot":
			return filepath.Join(home, ".copilot", "skills"), nil
		case "cursor":
			return filepath.Join(home, ".cursor", "skills"), nil
		case "gemini":
			return filepath.Join(home, ".gemini", "skills"), nil
		case "junie":
			return filepath.Join(home, ".junie", "skills"), nil
		case "kimi":
			base := os.Getenv("KIMI_CODE_HOME")
			if base == "" {
				base = filepath.Join(home, ".kimi-code")
			}
			return filepath.Join(base, "skills"), nil
		case "opencode":
			return filepath.Join(home, ".config", "opencode", "skills"), nil
		case "openclaw":
			base := os.Getenv("OPENCLAW_STATE_DIR")
			if base == "" {
				base = filepath.Join(home, ".openclaw")
			}
			return filepath.Join(base, "skills"), nil
		case "kiro":
			return filepath.Join(home, ".kiro", "skills"), nil
		case "roo":
			return filepath.Join(home, ".roo", "skills"), nil
		case "windsurf":
			return filepath.Join(home, ".codeium", "windsurf", "skills"), nil
		case "agents":
			return filepath.Join(home, ".agents", "skills"), nil
		}
	} else {
		switch name {
		case "codex":
			return filepath.Join(scope.Root, ".codex", "skills"), nil
		case "claude":
			return filepath.Join(scope.Root, ".claude", "skills"), nil
		case "cline":
			return filepath.Join(scope.Root, ".cline", "skills"), nil
		case "copilot":
			if _, err := os.Stat(filepath.Join(scope.Root, ".copilot")); err == nil {
				return filepath.Join(scope.Root, ".copilot", "skills"), nil
			}
			return filepath.Join(scope.Root, ".github", "skills"), nil
		case "cursor":
			return filepath.Join(scope.Root, ".cursor", "skills"), nil
		case "gemini":
			return filepath.Join(scope.Root, ".gemini", "skills"), nil
		case "junie":
			return filepath.Join(scope.Root, ".junie", "skills"), nil
		case "kimi":
			return filepath.Join(scope.Root, ".kimi-code", "skills"), nil
		case "kiro":
			return filepath.Join(scope.Root, ".kiro", "skills"), nil
		case "opencode":
			return filepath.Join(scope.Root, ".opencode", "skills"), nil
		case "openclaw":
			return filepath.Join(scope.Root, "skills"), nil
		case "roo":
			return filepath.Join(scope.Root, ".roo", "skills"), nil
		case "windsurf":
			return filepath.Join(scope.Root, ".windsurf", "skills"), nil
		case "agents":
			return filepath.Join(scope.Root, ".agents", "skills"), nil
		}
	}
	return "", fmt.Errorf("unknown target %q", name)
}

func targetMarker(scope state.Scope, home, name, root string) string {
	if name == "openclaw" && !scope.Global {
		// A workspace has no required .openclaw directory. Only auto-detect its
		// native skills root when it already exists; an explicit target creates it.
		return root
	}
	return filepath.Dir(root)
}

func deduplicateTargets(targets []Target) []Target {
	seen := map[string]bool{}
	result := make([]Target, 0, len(targets))
	for _, target := range targets {
		if !seen[target.Root] {
			seen[target.Root] = true
			result = append(result, target)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
