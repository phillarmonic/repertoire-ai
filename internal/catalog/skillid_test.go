package catalog

import (
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestSourceNamespaceAndSkillID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		source string
		skill  string
		want   string
	}{
		{BuiltinSource, "zensical", "github.com/phillarmonic/ai-skills/zensical"},
		{"github.com/phillarmonic/ai-skills", "repertoire", "github.com/phillarmonic/ai-skills/repertoire"},
		{"git@github.com:phillarmonic/ai-skills.git", "zensical", "github.com/phillarmonic/ai-skills/zensical"},
		{"https://github.com/example/skills.git", "demo", "github.com/example/skills/demo"},
	}
	for _, test := range tests {
		if got := SkillID(test.source, test.skill); got != test.want {
			t.Fatalf("SkillID(%q, %q) = %q, want %q", test.source, test.skill, got, test.want)
		}
		if got := SourceNamespace(test.source); got+"/"+test.skill != test.want {
			t.Fatalf("SourceNamespace(%q) = %q", test.source, got)
		}
	}
}

func TestParseSkillID(t *testing.T) {
	t.Parallel()
	namespace, name, err := ParseSkillID("github.com/phillarmonic/ai-skills/zensical")
	if err != nil || namespace != "github.com/phillarmonic/ai-skills" || name != "zensical" {
		t.Fatalf("namespaced parse = %q %q %v", namespace, name, err)
	}
	namespace, name, err = ParseSkillID("zensical")
	if err != nil || namespace != "" || name != "zensical" {
		t.Fatalf("short parse = %q %q %v", namespace, name, err)
	}
	if err := state.ValidateSkillReference("github.com/phillarmonic/ai-skills/zensical"); err != nil {
		t.Fatal(err)
	}
	if err := state.ValidateSkillReference("Bad_Name"); err == nil {
		t.Fatal("expected invalid short name to fail")
	}
	if SourceMatchesNamespace(BuiltinSource, "github.com/phillarmonic/ai-skills") != true {
		t.Fatal("expected builtin source to match namespace")
	}
	if SourceMatchesNamespace(BuiltinSource, "github.com/other/skills") {
		t.Fatal("expected namespace mismatch")
	}
}

func TestDefaultBootstrapManifest(t *testing.T) {
	t.Parallel()
	manifest := DefaultBootstrapManifest(BuiltinSource, []string{"zensical", "repertoire"})
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	zensical := manifest.Skills["github.com/phillarmonic/ai-skills/zensical"]
	if zensical.Scope != state.BootstrapScopeGlobal {
		t.Fatalf("scope = %q", zensical.Scope)
	}
	if _, ok := manifest.Skills["github.com/phillarmonic/ai-skills/repertoire"]; !ok {
		t.Fatal("expected repertoire skill id")
	}
}
