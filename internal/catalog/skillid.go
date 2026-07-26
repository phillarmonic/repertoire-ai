package catalog

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

// SourceNamespace converts a catalog source into the stable namespace prefix used
// in skill IDs (for example github.com/phillarmonic/ai-skills).
func SourceNamespace(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	normalized := NormalizeSource(source)
	if strings.HasPrefix(normalized, "git@") {
		rest := strings.TrimPrefix(normalized, "git@")
		host, path, ok := strings.Cut(rest, ":")
		if ok {
			return host + "/" + strings.TrimSuffix(strings.TrimPrefix(path, "/"), ".git")
		}
	}
	if parsed, err := url.Parse(normalized); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		path := strings.Trim(strings.TrimSuffix(parsed.Path, ".git"), "/")
		if path == "" {
			return parsed.Host
		}
		return parsed.Host + "/" + path
	}
	local := strings.TrimPrefix(normalized, "file://")
	local = strings.TrimSuffix(local, ".git")
	if absolute, err := filepath.Abs(local); err == nil {
		return filepath.ToSlash(absolute)
	}
	return filepath.ToSlash(local)
}

// SkillID builds a namespaced skill reference from a catalog source and short skill name.
func SkillID(source, skillName string) string {
	namespace := SourceNamespace(source)
	if namespace == "" {
		return skillName
	}
	return namespace + "/" + skillName
}

// ParseSkillID splits a skill reference into catalog namespace and short skill name.
// Short names return an empty namespace.
func ParseSkillID(id string) (namespace, skillName string, err error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "", fmt.Errorf("skill id is required")
	}
	if !strings.Contains(id, "/") {
		if err := state.ValidateName(id); err != nil {
			return "", "", err
		}
		return "", id, nil
	}
	index := strings.LastIndex(id, "/")
	namespace = id[:index]
	skillName = id[index+1:]
	if namespace == "" {
		return "", "", fmt.Errorf("skill namespace is required")
	}
	if strings.Contains(namespace, " ") {
		return "", "", fmt.Errorf("skill namespace must not contain spaces")
	}
	if err := state.ValidateName(skillName); err != nil {
		return "", "", fmt.Errorf("skill name: %w", err)
	}
	return namespace, skillName, nil
}

// SourceMatchesNamespace reports whether a catalog source maps to the namespace.
func SourceMatchesNamespace(source, namespace string) bool {
	if namespace == "" {
		return true
	}
	return SourceNamespace(source) == namespace
}

// DefaultBootstrapManifest builds a starter bootstrap manifest that installs the
// given short skill names from source under global scope using namespaced IDs.
func DefaultBootstrapManifest(source string, skillNames []string) state.BootstrapManifest {
	manifest := state.BootstrapManifest{
		Schema:   state.SchemaVersion,
		Catalogs: map[string]state.CatalogRegistration{},
		Skills:   map[string]state.BootstrapSkill{},
	}
	for _, name := range skillNames {
		manifest.Skills[SkillID(source, name)] = state.BootstrapSkill{
			Scope: state.BootstrapScopeGlobal,
		}
	}
	return manifest
}
