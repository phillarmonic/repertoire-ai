package cli

import (
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestClassifyAddArgument(t *testing.T) {
	t.Parallel()
	manifest := state.NewManifest()
	known := map[string]struct{}{"phillarmonkey/code": {}}

	kind, skills, _ := classifyAddArgument("phillarmonkey/code", known, manifest)
	if kind != addArgSkill || len(skills) != 1 || skills[0] != "phillarmonkey/code" {
		t.Fatalf("slash skill key classified as %+v %v", kind, skills)
	}

	kind, _, _ = classifyAddArgument("github.com/phillarmonic/ai-skills/zensical", nil, manifest)
	if kind != addArgSkill {
		t.Fatal("builtin qualified ID should resolve as a skill")
	}

	kind, _, source := classifyAddArgument("acme/skills", known, manifest)
	if kind != addArgSource || source.Source != "github.com/acme/skills" {
		t.Fatalf("owner/repo classified as kind=%v source=%+v", kind, source)
	}

	kind, _, _ = classifyAddArgument("zensical", known, manifest)
	if kind != addArgSkill {
		t.Fatal("short name should stay a skill")
	}
}

func TestSkillsToInstall(t *testing.T) {
	t.Parallel()
	materialized := catalog.Materialized{
		Name: "company",
		Manifest: state.Manifest{
			Catalog: &state.CatalogDefinition{
				Name: "company",
				Skills: map[string]state.SkillEntry{
					"alpha": {Path: "skills/alpha"},
					"beta":  {Path: "skills/beta"},
				},
			},
		},
	}
	all, err := skillsToInstall(materialized, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0] != "alpha" || all[1] != "beta" {
		t.Fatalf("all skills = %v", all)
	}
	subset, err := skillsToInstall(materialized, "beta", []string{"alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if len(subset) != 2 || subset[0] != "beta" || subset[1] != "alpha" {
		t.Fatalf("subset = %v", subset)
	}
	if _, err := skillsToInstall(materialized, "missing", nil); err == nil {
		t.Fatal("expected missing skill error")
	}
}
