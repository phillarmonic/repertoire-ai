package catalog

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

const (
	BuiltinName   = "phillarmonic"
	BuiltinSource = "https://github.com/phillarmonic/ai-skills.git"

	// OverridesEnv carries local catalog overrides for testing: a
	// comma-separated list of name=path or source=path pairs that redirect
	// a catalog to a local checkout so skills can be tested before pushing.
	OverridesEnv = "REPERTOIRE_OVERRIDES"
)

var commitPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

type Source struct {
	Name         string
	Registration state.CatalogRegistration
	Builtin      bool
}

type Materialized struct {
	Manifest state.Manifest
	Root     string
	Commit   string
	Source
	Tracking bool
}

type Manager struct {
	// Overrides redirects a catalog (matched by catalog name or by the
	// normalized source URL) to a local path. Env-driven overrides set by
	// OverridesFromEnv plus CLI --override pairs are merged here.
	Overrides map[string]string
	CacheRoot string
}

func NewManager(cacheRoot string) (*Manager, error) {
	if cacheRoot == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user cache: %w", err)
		}
		cacheRoot = filepath.Join(userCache, "repertoire", "catalogs")
	}
	return &Manager{CacheRoot: cacheRoot}, nil
}

func Sources(manifest state.Manifest) []Source {
	sources := make([]Source, 0, len(manifest.Catalogs)+1)
	if _, overridden := manifest.Catalogs[BuiltinName]; !overridden {
		sources = append(sources, Source{
			Name: BuiltinName, Builtin: true,
			Registration: state.CatalogRegistration{Source: BuiltinSource},
		})
	}
	for name, registration := range manifest.Catalogs {
		sources = append(sources, Source{Name: name, Registration: registration})
	}
	return sources
}

// ParseOverrides parses a comma-separated list of name=path or source=path
// pairs into an override map. Empty entries are ignored.
func ParseOverrides(raw string) (map[string]string, error) {
	overrides := map[string]string{}
	for pair := range strings.SplitSeq(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name, path, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("invalid catalog override %q: expected name=path or source=path", pair)
		}
		overrides[strings.TrimSpace(name)] = strings.TrimSpace(path)
	}
	return overrides, nil
}

// OverridesFromEnv reads catalog overrides from the REPERTOIRE_OVERRIDES
// environment variable. A missing or empty variable yields an empty map.
func OverridesFromEnv() (map[string]string, error) {
	return ParseOverrides(os.Getenv(OverridesEnv))
}

// OverrideFor returns the local override path for a source, if any. A source
// matches by catalog name or by its normalized source URL.
func (m *Manager) OverrideFor(source Source) (string, bool) {
	if len(m.Overrides) == 0 {
		return "", false
	}
	if path, ok := m.Overrides[source.Name]; ok {
		return path, true
	}
	if path, ok := m.Overrides[NormalizeSource(source.Registration.Source)]; ok {
		return path, true
	}
	return "", false
}

// applyOverride redirects source to a local checkout when an override matches,
// and reports whether it did. The override path must be an existing directory.
func (m *Manager) applyOverride(source *Source) (bool, error) {
	path, ok := m.OverrideFor(*source)
	if !ok {
		return false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("catalog override for %q: %s: %w", source.Name, path, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("catalog override for %q: %s is not a directory", source.Name, path)
	}
	source.Registration.Source = path
	source.Registration.Ref = ""
	return true, nil
}

func NormalizeSource(source string) string {
	source = strings.TrimSpace(source)
	if strings.HasPrefix(source, "github.com/") {
		return "https://" + strings.TrimSuffix(source, ".git") + ".git"
	}
	return source
}

func RedactSource(source string) string {
	parsed, err := url.Parse(source)
	if err == nil && parsed.User != nil {
		parsed.User = nil
		return parsed.String()
	}
	return source
}

func IsLocal(source string) bool {
	if strings.Contains(source, "://") || strings.HasPrefix(source, "git@") {
		return false
	}
	_, err := os.Stat(source)
	return err == nil
}

func (m *Manager) Materialize(source Source, refresh bool) (Materialized, error) {
	source.Registration.Source = NormalizeSource(source.Registration.Source)
	if overridden, err := m.applyOverride(&source); err != nil {
		return Materialized{}, err
	} else if overridden {
		refresh = false
	}
	if IsLocal(source.Registration.Source) {
		root := strings.TrimPrefix(source.Registration.Source, "file://")
		absolute, err := filepath.Abs(root)
		if err != nil {
			return Materialized{}, fmt.Errorf("resolve local catalog: %w", err)
		}
		manifest, err := loadCatalog(absolute)
		if err != nil {
			return Materialized{}, err
		}
		commit, _ := gitOutput(absolute, "rev-parse", "HEAD")
		return Materialized{Source: source, Root: absolute, Commit: commit, Tracking: false, Manifest: manifest}, nil
	}

	root := filepath.Join(m.CacheRoot, source.Name)
	if _, err := os.Stat(filepath.Join(root, ".git")); errors.Is(err, os.ErrNotExist) {
		// #nosec G301 -- cache root keeps the shared 0755 directory convention
		if mkdirErr := os.MkdirAll(m.CacheRoot, 0o755); mkdirErr != nil {
			return Materialized{}, fmt.Errorf("create catalog cache: %w", mkdirErr)
		}
		if runErr := runGit("", "clone", "--quiet", source.Registration.Source, root); runErr != nil {
			return Materialized{}, fmt.Errorf("clone catalog %q from %s: %w", source.Name, RedactSource(source.Registration.Source), runErr)
		}
	} else if err != nil {
		return Materialized{}, fmt.Errorf("inspect catalog cache: %w", err)
	}
	if refresh {
		if err := runGit(root, "fetch", "--quiet", "--all", "--tags", "--prune"); err != nil {
			return Materialized{}, fmt.Errorf("refresh catalog %q: %w", source.Name, err)
		}
	}

	ref, tracking, err := resolveRef(root, source.Registration.Ref)
	if err != nil {
		return Materialized{}, fmt.Errorf("resolve catalog %q ref: %w", source.Name, err)
	}
	commit, err := gitOutput(root, "rev-parse", ref+"^{commit}")
	if err != nil {
		return Materialized{}, fmt.Errorf("resolve catalog %q commit: %w", source.Name, err)
	}
	if runErr := runGit(root, "checkout", "--quiet", "--detach", commit); runErr != nil {
		return Materialized{}, fmt.Errorf("checkout catalog %q: %w", source.Name, runErr)
	}
	manifest, err := loadCatalog(root)
	if err != nil {
		return Materialized{}, err
	}
	return Materialized{Source: source, Root: root, Commit: commit, Tracking: tracking, Manifest: manifest}, nil
}

// InspectCached reads a local catalog or an existing remote catalog cache
// without cloning, fetching, or changing the cached checkout.
func (m *Manager) InspectCached(source Source) (Materialized, error) {
	source.Registration.Source = NormalizeSource(source.Registration.Source)
	// An override redirects the source to the local checkout; the local branch
	// below reads it directly regardless of cache state.
	if _, err := m.applyOverride(&source); err != nil {
		return Materialized{}, err
	}
	if IsLocal(source.Registration.Source) {
		root, err := filepath.Abs(source.Registration.Source)
		if err != nil {
			return Materialized{}, fmt.Errorf("resolve local catalog: %w", err)
		}
		manifest, err := loadCatalog(root)
		if err != nil {
			return Materialized{}, err
		}
		commit, _ := gitOutput(root, "rev-parse", "HEAD")
		return Materialized{Source: source, Root: root, Commit: commit, Tracking: false, Manifest: manifest}, nil
	}

	root := filepath.Join(m.CacheRoot, source.Name)
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Materialized{}, fmt.Errorf("catalog %q is not cached", source.Name)
		}
		return Materialized{}, fmt.Errorf("inspect catalog cache: %w", err)
	}
	manifest, err := loadCatalog(root)
	if err != nil {
		return Materialized{}, err
	}
	commit, _ := gitOutput(root, "rev-parse", "HEAD")
	return Materialized{Source: source, Root: root, Commit: commit, Tracking: true, Manifest: manifest}, nil
}

func loadCatalog(root string) (state.Manifest, error) {
	manifest, err := state.LoadManifest(filepath.Join(root, "repertoire.yaml"))
	if err != nil {
		return state.Manifest{}, fmt.Errorf("load catalog at %s: %w", root, err)
	}
	if manifest.Catalog == nil {
		return state.Manifest{}, fmt.Errorf("catalog at %s has no catalog section", root)
	}
	return manifest, nil
}

func resolveRef(root, requested string) (string, bool, error) {
	if requested == "" {
		ref, err := gitOutput(root, "symbolic-ref", "refs/remotes/origin/HEAD")
		return ref, true, err
	}
	if commitPattern.MatchString(requested) {
		return requested, false, nil
	}
	if err := runGit(root, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+requested); err == nil {
		return "refs/remotes/origin/" + requested, true, nil
	}
	if err := runGit(root, "show-ref", "--verify", "--quiet", "refs/tags/"+requested); err == nil {
		return "refs/tags/" + requested, false, nil
	}
	return requested, false, nil
}

func runGit(directory string, arguments ...string) error {
	commandArguments := arguments
	if directory != "" {
		commandArguments = append([]string{"-C", directory}, arguments...)
	}
	// #nosec G204 -- argv is built from the catalog registration, not user input
	command := exec.Command("git", commandArguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return nil
}

func gitOutput(directory string, arguments ...string) (string, error) {
	commandArguments := append([]string{"-C", directory}, arguments...)
	// #nosec G204 -- argv is built from the catalog registration, not user input
	output, err := exec.Command("git", commandArguments...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}
