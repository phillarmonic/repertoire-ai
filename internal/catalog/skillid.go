package catalog

import (
	"errors"
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
	if after, ok := strings.CutPrefix(normalized, "git@"); ok {
		rest := after
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
		return "", "", errors.New("skill id is required")
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
		return "", "", errors.New("skill namespace is required")
	}
	if strings.Contains(namespace, " ") {
		return "", "", errors.New("skill namespace must not contain spaces")
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

// DefaultBootstrapSkills builds starter bootstrap declarations that install the
// given short skill names from source under global scope using namespaced IDs.
func DefaultBootstrapSkills(source string, skillNames []string) map[string]state.BootstrapSkill {
	skills := map[string]state.BootstrapSkill{}
	for _, name := range skillNames {
		skills[SkillID(source, name)] = state.BootstrapSkill{
			Scope: state.BootstrapScopeGlobal,
		}
	}
	return skills
}
