package install

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

// commandLookPath finds executables during auto-detection. Tests replace it.
var commandLookPath = exec.LookPath

type Target struct {
	Name string
	Root string
	// Commands is the directory the client reads user-invoked entries from, for
	// a client that lists them separately from skills. It is empty for every
	// target whose client has no such layout, which is most of them: a command
	// file written for a client that does not read the directory would simply be
	// dead weight on disk. Verify a client's layout before setting it here.
	Commands string
}

var supportedTargetNames = []string{
	"agents", "aider", "amp", "antigravity", "antigravity-windows", "claude",
	"claw", "cline", "codebuddy", "codex", "copilot", "cursor", "devin",
	"droid", "dsh", "gemini", "hermes", "junie", "kilo", "kimi", "kiro",
	"openclaw", "opencode", "pi", "roo", "trae", "trae-cn", "vscode",
	"windows", "windsurf",
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
		if slices.Contains(requested, "all") {
			expanded = supportedTargetNames
		}
		result := make([]Target, 0, len(expanded))
		for _, name := range expanded {
			root, err := targetRoot(scope, home, name)
			if err != nil {
				return nil, err
			}
			result = append(result, newTarget(scope, home, name, root))
		}
		return deduplicateTargets(result), nil
	}

	var result []Target
	for _, name := range supportedTargetNames {
		if explicitOnlyTarget(name) {
			continue
		}
		root, err := targetRoot(scope, home, name)
		if err != nil {
			continue
		}
		if !targetPresent(scope, home, name, root) {
			continue
		}
		result = append(result, newTarget(scope, home, name, root))
	}
	if len(result) == 0 {
		return nil, errors.New("no supported agent clients detected; use --target")
	}
	return deduplicateTargets(result), nil
}

// newTarget builds one resolved target. Both the explicit and the detected path
// go through here so a target's layout cannot depend on how it was selected.
func newTarget(scope state.Scope, home, name, root string) Target {
	return Target{Name: name, Root: root, Commands: targetCommandRoot(scope, home, name)}
}

// targetCommandRoot returns the directory a client lists user-invoked entries
// from. OpenCode is the only client that keeps them apart from skills: it reads
// {command,commands}/**/*.md beside its configuration, where a skill is offered
// to the model and a command to the user. Every other target returns "", which
// means the caller installs skills only.
func targetCommandRoot(scope state.Scope, home, name string) string {
	if name != "opencode" {
		return ""
	}
	if scope.Global {
		return filepath.Join(configHome(home), "opencode", "command")
	}
	return filepath.Join(scope.Root, ".opencode", "command")
}

func targetPresent(scope state.Scope, home, name, root string) bool {
	if _, err := os.Stat(targetMarker(scope, home, name, root)); err == nil {
		return true
	}
	// Command detection is machine-wide. Project scope still requires a
	// worktree marker so `add --project` does not create agent dirs because
	// a CLI happens to be on PATH.
	return scope.Global && targetCommandPresent(name)
}

func targetCommandPresent(name string) bool {
	for _, command := range targetCommands(name) {
		if _, err := commandLookPath(command); err == nil {
			return true
		}
	}
	return false
}

func targetCommands(name string) []string {
	switch name {
	case "aider", "claude", "cline", "codebuddy", "codex", "copilot", "cursor",
		"devin", "droid", "dsh", "gemini", "hermes", "junie", "kimi", "kiro",
		"opencode", "openclaw", "trae", "windsurf":
		return []string{name}
	default:
		return nil
	}
}

func explicitOnlyTarget(name string) bool {
	switch name {
	case "agents", "amp", "antigravity", "antigravity-windows", "claw", "vscode", "windows":
		return true
	default:
		return false
	}
}

// configHome returns the directory a client that follows the XDG base directory
// specification keeps its configuration in: $XDG_CONFIG_HOME when it is set,
// otherwise ~/.config.
//
// Only targets whose client is known to resolve its own directory this way may
// use it. For a client that hardcodes ~/.config, honouring XDG here would move
// the install to a directory the client never reads — the same failure this
// helper exists to prevent, pointed the other way. Codex, DeepSeek Harness,
// Kimi and OpenClaw each have their own variable and keep their own branches;
// OpenCode has none, so XDG is the mechanism for it. Verify a client's
// behaviour before extending this.
func configHome(home string) string {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return configHome
	}
	return filepath.Join(home, ".config")
}

func targetRoot(scope state.Scope, home, name string) (string, error) {
	if scope.Global {
		switch name {
		case "aider":
			return filepath.Join(home, ".aider"), nil
		case "amp":
			return filepath.Join(home, ".config", "agents", "skills"), nil
		case "antigravity", "antigravity-windows":
			return filepath.Join(home, ".gemini", "config", "skills"), nil
		case "codex":
			base := os.Getenv("CODEX_HOME")
			if base == "" {
				base = filepath.Join(home, ".codex")
			}
			return filepath.Join(base, "skills"), nil
		case "claude":
			return filepath.Join(home, ".claude", "skills"), nil
		case "windows":
			return filepath.Join(home, ".claude", "skills"), nil
		case "claw":
			return filepath.Join(home, ".openclaw", "skills"), nil
		case "cline":
			return filepath.Join(home, ".cline", "skills"), nil
		case "codebuddy":
			return filepath.Join(home, ".codebuddy", "skills"), nil
		case "copilot":
			return filepath.Join(home, ".copilot", "skills"), nil
		case "cursor":
			return filepath.Join(home, ".cursor", "skills"), nil
		case "gemini":
			return filepath.Join(home, ".gemini", "skills"), nil
		case "devin":
			return filepath.Join(home, ".config", "devin", "skills"), nil
		case "droid":
			return filepath.Join(home, ".factory", "skills"), nil
		case "dsh":
			// DeepSeek Harness roots under $DSH_HOME and otherwise ~/.dsh.
			base := os.Getenv("DSH_HOME")
			if base == "" {
				base = filepath.Join(home, ".dsh")
			}
			return filepath.Join(base, "skills"), nil
		case "hermes":
			return filepath.Join(home, ".hermes", "skills"), nil
		case "junie":
			return filepath.Join(home, ".junie", "skills"), nil
		case "kimi":
			base := os.Getenv("KIMI_CODE_HOME")
			if base == "" {
				base = filepath.Join(home, ".kimi-code")
			}
			return filepath.Join(base, "skills"), nil
		case "opencode":
			// OpenCode resolves its configuration directory from
			// XDG_CONFIG_HOME, so a skill written to ~/.config/opencode for a
			// user who sets it is one OpenCode never loads.
			return filepath.Join(configHome(home), "opencode", "skills"), nil
		case "kilo":
			return filepath.Join(home, ".config", "kilo", "skills"), nil
		case "openclaw":
			base := os.Getenv("OPENCLAW_STATE_DIR")
			if base == "" {
				base = filepath.Join(home, ".openclaw")
			}
			return filepath.Join(base, "skills"), nil
		case "kiro":
			return filepath.Join(home, ".kiro", "skills"), nil
		case "pi":
			return filepath.Join(home, ".pi", "agent", "skills"), nil
		case "trae":
			return filepath.Join(home, ".trae", "skills"), nil
		case "trae-cn":
			return filepath.Join(home, ".trae-cn", "skills"), nil
		case "vscode":
			return filepath.Join(home, ".copilot", "skills"), nil
		case "roo":
			return filepath.Join(home, ".roo", "skills"), nil
		case "windsurf":
			return filepath.Join(home, ".codeium", "windsurf", "skills"), nil
		case "agents":
			return filepath.Join(home, ".agents", "skills"), nil
		}
	} else {
		switch name {
		case "aider":
			return filepath.Join(scope.Root, ".aider"), nil
		case "amp", "antigravity", "antigravity-windows":
			return filepath.Join(scope.Root, ".agents", "skills"), nil
		case "codex":
			return filepath.Join(scope.Root, ".codex", "skills"), nil
		case "claude":
			return filepath.Join(scope.Root, ".claude", "skills"), nil
		case "windows":
			return filepath.Join(scope.Root, ".claude", "skills"), nil
		case "claw":
			return filepath.Join(scope.Root, ".openclaw", "skills"), nil
		case "cline":
			return filepath.Join(scope.Root, ".cline", "skills"), nil
		case "codebuddy":
			return filepath.Join(scope.Root, ".codebuddy", "skills"), nil
		case "copilot":
			if _, err := os.Stat(filepath.Join(scope.Root, ".copilot")); err == nil {
				return filepath.Join(scope.Root, ".copilot", "skills"), nil
			}
			return filepath.Join(scope.Root, ".github", "skills"), nil
		case "cursor":
			return filepath.Join(scope.Root, ".cursor", "skills"), nil
		case "gemini":
			return filepath.Join(scope.Root, ".gemini", "skills"), nil
		case "devin":
			return filepath.Join(scope.Root, ".devin", "skills"), nil
		case "droid":
			return filepath.Join(scope.Root, ".factory", "skills"), nil
		case "dsh":
			return filepath.Join(scope.Root, ".dsh", "skills"), nil
		case "hermes":
			return filepath.Join(scope.Root, ".hermes", "skills"), nil
		case "junie":
			return filepath.Join(scope.Root, ".junie", "skills"), nil
		case "kimi":
			return filepath.Join(scope.Root, ".kimi-code", "skills"), nil
		case "kiro":
			return filepath.Join(scope.Root, ".kiro", "skills"), nil
		case "kilo":
			return filepath.Join(scope.Root, ".config", "kilo", "skills"), nil
		case "opencode":
			return filepath.Join(scope.Root, ".opencode", "skills"), nil
		case "openclaw":
			return filepath.Join(scope.Root, "skills"), nil
		case "pi":
			return filepath.Join(scope.Root, ".pi", "agent", "skills"), nil
		case "roo":
			return filepath.Join(scope.Root, ".roo", "skills"), nil
		case "windsurf":
			return filepath.Join(scope.Root, ".windsurf", "skills"), nil
		case "trae":
			return filepath.Join(scope.Root, ".trae", "skills"), nil
		case "trae-cn":
			return filepath.Join(scope.Root, ".trae-cn", "skills"), nil
		case "vscode":
			return filepath.Join(scope.Root, ".github", "skills"), nil
		case "agents":
			return filepath.Join(scope.Root, ".agents", "skills"), nil
		}
	}
	return "", fmt.Errorf("unknown target %q", name)
}

func targetMarker(scope state.Scope, home, name, root string) string {
	if name == "aider" || (!scope.Global && name == "openclaw") {
		// These roots are the client directory (or a generic project `skills/`
		// folder). Their parent is $HOME or the worktree, which always exists.
		return root
	}
	return filepath.Dir(root)
}

func deduplicateTargets(targets []Target) []Target {
	seen := map[string]bool{}
	result := make([]Target, 0, len(targets))
	for _, target := range targets {
		key := target.Name + "\x00" + target.Root
		if !seen[key] {
			seen[key] = true
			result = append(result, target)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
