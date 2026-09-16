package catalog

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

var (
	ownerRepoPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)
	notNameChar      = regexp.MustCompile(`[^a-z0-9]+`)
)

// InstallSource is a catalog location parsed from a one-shot add argument.
type InstallSource struct {
	Source string
	Ref    string
	Skill  string
}

// ParseInstallSource reports whether arg looks like a catalog source (local
// path, git URL, github.com/owner/repo, or owner/repo) and splits an optional
// skill tail or GitHub /tree/<ref>/<path> suffix.
func ParseInstallSource(arg string) (InstallSource, bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return InstallSource{}, false
	}
	if IsLocal(arg) {
		return InstallSource{Source: arg}, true
	}
	if parent, skill, ok := splitLocalSkill(arg); ok {
		return InstallSource{Source: parent, Skill: skill}, true
	}
	if strings.HasPrefix(arg, "git@") {
		return parseGitSSH(arg)
	}
	if strings.Contains(arg, "://") || strings.HasPrefix(arg, "github.com/") {
		return parseGitHTTP(arg)
	}
	if ownerRepoPattern.MatchString(arg) {
		return InstallSource{Source: "github.com/" + arg}, true
	}
	return InstallSource{}, false
}

// DefaultCatalogName derives a catalog registration name from a source URL or
// local path: basename, strip .git, lower-case kebab, then ValidateName.
func DefaultCatalogName(source string) (string, error) {
	base := sourceBasename(source)
	name := kebabCatalogName(base)
	if err := state.ValidateName(name); err != nil {
		return "", fmt.Errorf("catalog name derived from %q is invalid; pass --name", base)
	}
	return name, nil
}

// SameSource reports whether two catalog sources refer to the same location
// after local absolute-path resolution and NormalizeSource.
func SameSource(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if IsLocal(a) && IsLocal(b) {
		left, err := filepath.Abs(strings.TrimPrefix(a, "file://"))
		if err != nil {
			return false
		}
		right, err := filepath.Abs(strings.TrimPrefix(b, "file://"))
		if err != nil {
			return false
		}
		return left == right
	}
	return NormalizeSource(a) == NormalizeSource(b)
}

func sourceBasename(source string) string {
	source = strings.TrimSpace(source)
	if IsLocal(source) {
		absolute, err := filepath.Abs(strings.TrimPrefix(source, "file://"))
		if err != nil {
			return filepath.Base(source)
		}
		return filepath.Base(absolute)
	}
	normalized := NormalizeSource(source)
	normalized = strings.TrimSuffix(normalized, ".git")
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	if after, ok := strings.CutPrefix(normalized, "git@"); ok {
		_, path, found := strings.Cut(after, ":")
		if found {
			normalized = path
		}
	} else if parsed, err := url.Parse(normalized); err == nil && parsed.Path != "" {
		normalized = parsed.Path
	}
	normalized = strings.Trim(normalized, "/")
	if index := strings.LastIndex(normalized, "/"); index >= 0 {
		normalized = normalized[index+1:]
	}
	return strings.TrimSuffix(normalized, ".git")
}

func kebabCatalogName(raw string) string {
	name := notNameChar.ReplaceAllString(strings.ToLower(strings.TrimSpace(raw)), "-")
	return strings.Trim(name, "-")
}

func splitLocalSkill(arg string) (string, string, bool) {
	parent := filepath.Dir(arg)
	skill := filepath.Base(arg)
	if parent == "." || parent == string(filepath.Separator) {
		return "", "", false
	}
	if state.ValidateName(skill) != nil {
		return "", "", false
	}
	if !IsLocal(parent) {
		return "", "", false
	}
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return "", "", false
	}
	return parent, skill, true
}

func parseGitHTTP(arg string) (InstallSource, bool) {
	raw := arg
	if strings.HasPrefix(raw, "github.com/") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return InstallSource{}, false
	}
	path := strings.Trim(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return InstallSource{}, false
	}
	owner, repo := parts[0], parts[1]
	source := parsed.Scheme + "://" + parsed.Host + "/" + owner + "/" + repo + ".git"
	if parsed.User != nil {
		source = parsed.Scheme + "://" + parsed.User.String() + "@" + parsed.Host + "/" + owner + "/" + repo + ".git"
	}
	rest := parts[2:]
	if len(rest) == 0 {
		return InstallSource{Source: source}, true
	}
	host := strings.ToLower(parsed.Host)
	if host == "github.com" || host == "www.github.com" {
		return githubTail(source, rest)
	}
	if len(rest) == 1 && state.ValidateName(rest[0]) == nil {
		return InstallSource{Source: source, Skill: rest[0]}, true
	}
	return InstallSource{}, false
}

func githubTail(source string, rest []string) (InstallSource, bool) {
	switch rest[0] {
	case "tree", "blob":
		if len(rest) < 2 {
			return InstallSource{}, false
		}
		ref := rest[1]
		skill := ""
		if len(rest) > 2 {
			skill = rest[len(rest)-1]
			if state.ValidateName(skill) != nil {
				return InstallSource{}, false
			}
		}
		return InstallSource{Source: source, Ref: ref, Skill: skill}, true
	default:
		if len(rest) == 1 && state.ValidateName(rest[0]) == nil {
			return InstallSource{Source: source, Skill: rest[0]}, true
		}
		return InstallSource{}, false
	}
}

func parseGitSSH(arg string) (InstallSource, bool) {
	rest, ok := strings.CutPrefix(arg, "git@")
	if !ok {
		return InstallSource{}, false
	}
	host, path, ok := strings.Cut(rest, ":")
	if !ok || host == "" || path == "" {
		return InstallSource{}, false
	}
	path = strings.TrimSuffix(strings.TrimPrefix(path, "/"), ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return InstallSource{}, false
	}
	source := "git@" + host + ":" + parts[0] + "/" + parts[1] + ".git"
	if len(parts) == 2 {
		return InstallSource{Source: source}, true
	}
	if len(parts) == 3 && state.ValidateName(parts[2]) == nil {
		return InstallSource{Source: source, Skill: parts[2]}, true
	}
	return InstallSource{}, false
}
