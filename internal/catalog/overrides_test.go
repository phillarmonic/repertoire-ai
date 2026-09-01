package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestParseOverrides(t *testing.T) {
	t.Parallel()
	parsed, err := ParseOverrides("phillarmonic=/tmp/ai-skills,company=../company-skills")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed["phillarmonic"] != "/tmp/ai-skills" || parsed["company"] != "../company-skills" {
		t.Fatalf("unexpected overrides: %#v", parsed)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected two overrides, got %#v", parsed)
	}

	if empty, err := ParseOverrides(""); err != nil || len(empty) != 0 {
		t.Fatalf("empty parse = %#v, %v", empty, err)
	}
	if trailing, err := ParseOverrides("one=/a,,"); err != nil || len(trailing) != 1 {
		t.Fatalf("trailing comma parse = %#v, %v", trailing, err)
	}
	for _, bad := range []string{"noequals", "=path", "name="} {
		if _, err := ParseOverrides(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
	// "a=b=c" is valid: the first = splits key from a path that may contain =.
	if parsed, err := ParseOverrides("a=b=c"); err != nil || parsed["a"] != "b=c" {
		t.Fatalf("path with '=' = %#v, %v", parsed, err)
	}
}

func TestOverridesFromEnv(t *testing.T) {
	t.Setenv(OverridesEnv, "phillarmonic=/tmp/ai-skills")
	overrides, err := OverridesFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if overrides["phillarmonic"] != "/tmp/ai-skills" {
		t.Fatalf("unexpected overrides: %#v", overrides)
	}
	t.Setenv(OverridesEnv, "")
	overrides, err = OverridesFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 0 {
		t.Fatalf("expected no overrides, got %#v", overrides)
	}
}

func TestOverrideForMatchesNameAndNormalizedSource(t *testing.T) {
	t.Parallel()
	manager := &Manager{Overrides: map[string]string{
		"phillarmonic": "/tmp/ai-skills",
	}}
	source := Source{Name: "phillarmonic", Registration: state.CatalogRegistration{Source: BuiltinSource}}
	if path, ok := manager.OverrideFor(source); !ok || path != "/tmp/ai-skills" {
		t.Fatalf("name match = %q, %v", path, ok)
	}
	// A registration that spells the source differently still matches via
	// the normalized URL.
	compact := Source{Name: "phillarmonic", Registration: state.CatalogRegistration{Source: "github.com/phillarmonic/ai-skills"}}
	if path, ok := manager.OverrideFor(compact); !ok || path != "/tmp/ai-skills" {
		t.Fatalf("normalized source match = %q, %v", path, ok)
	}
	unrelated := Source{Name: "company", Registration: state.CatalogRegistration{Source: "https://example.invalid/skills.git"}}
	if _, ok := manager.OverrideFor(unrelated); ok {
		t.Fatal("unrelated source matched an override")
	}
}

func TestMaterializeUsesLocalOverride(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	writeCatalog(t, repository)

	manager, err := NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	manager.Overrides = map[string]string{"remote": repository}

	// The registered source is a remote URL; the override redirects it to the
	// local checkout so no clone or fetch happens.
	materialized, err := manager.Materialize(Source{
		Name: "remote", Registration: state.CatalogRegistration{Source: "https://example.invalid/skills.git"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if materialized.Root != repository {
		t.Fatalf("root = %q, want %q", materialized.Root, repository)
	}
	if materialized.Tracking {
		t.Fatal("override must not track a remote ref")
	}
	if materialized.Manifest.Catalog.Name != "example" {
		t.Fatalf("unexpected manifest: %+v", materialized.Manifest)
	}
}

func TestInspectCachedUsesLocalOverride(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	writeCatalog(t, repository)

	manager, err := NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	manager.Overrides = map[string]string{"remote": repository}

	// InspectCached normally requires a cached clone; an override reads the
	// local checkout instead.
	materialized, err := manager.InspectCached(Source{
		Name: "remote", Registration: state.CatalogRegistration{Source: "https://example.invalid/skills.git"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if materialized.Root != repository {
		t.Fatalf("root = %q, want %q", materialized.Root, repository)
	}
}

func TestOverridePathMustExist(t *testing.T) {
	t.Parallel()
	manager, err := NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	manager.Overrides = map[string]string{"remote": filepath.Join(t.TempDir(), "missing")}
	_, err = manager.Materialize(Source{
		Name: "remote", Registration: state.CatalogRegistration{Source: "https://example.invalid/skills.git"},
	}, false)
	if err == nil || !strings.Contains(err.Error(), "override") {
		t.Fatalf("expected override error, got %v", err)
	}

	file := filepath.Join(t.TempDir(), "not-a-directory")
	if writeErr := os.WriteFile(file, []byte("x"), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	manager.Overrides = map[string]string{"remote": file}
	if _, err := manager.Materialize(Source{
		Name: "remote", Registration: state.CatalogRegistration{Source: "https://example.invalid/skills.git"},
	}, false); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not-a-directory error, got %v", err)
	}
}
