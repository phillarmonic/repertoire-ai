package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestCompletionCommandGeneratesSupportedShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			var output bytes.Buffer
			command := NewRootCommand("test", &output, &output)
			command.SetArgs([]string{"completion", shell})
			if err := command.Execute(); err != nil {
				t.Fatalf("completion %s: %v", shell, err)
			}
			if !strings.Contains(output.String(), "repertoire") {
				t.Fatalf("completion %s did not name repertoire", shell)
			}
		})
	}
}

func TestAvailableSkillCompletionsReadVisibleCatalogs(t *testing.T) {
	local := writeCompletionCatalog(t, t.TempDir(), "local", map[string]string{
		"alpha":              "skills/alpha",
		"phillarmonkey/code": "skills/code",
		"shared":             "skills/shared",
	})
	cacheRoot := t.TempDir()
	cached := filepath.Join(cacheRoot, "remote")
	if err := os.MkdirAll(filepath.Join(cached, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeCompletionCatalog(t, cached, "remote", map[string]string{
		"shared": "skills/shared",
		"zulu":   "skills/zulu",
	})

	manifest := state.NewManifest()
	manifest.Catalogs["phillarmonic"] = state.CatalogRegistration{Source: local}
	manifest.Catalogs["remote"] = state.CatalogRegistration{Source: "https://example.invalid/skills.git"}
	manifest.Catalogs["uncached"] = state.CatalogRegistration{Source: "https://example.invalid/missing.git"}

	got := availableSkillCompletions(manifest, "", "", cacheRoot, nil)
	want := []string{
		"alpha\t[available] phillarmonic",
		"phillarmonkey/code\t[available] phillarmonic",
		"shared\t[available] phillarmonic, remote",
		"zulu\t[available] remote",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completions = %#v, want %#v", got, want)
	}
	filtered := availableSkillCompletions(manifest, "remote", "z", cacheRoot, nil)
	if !reflect.DeepEqual(filtered, []string{"zulu\t[available] remote"}) {
		t.Fatalf("filtered completions = %#v", filtered)
	}
	qualified := availableSkillCompletions(manifest, "", "phillarmonkey/", cacheRoot, nil)
	if !reflect.DeepEqual(qualified, []string{"phillarmonkey/code\t[available] phillarmonic"}) {
		t.Fatalf("qualified completions = %#v", qualified)
	}
	if entries, err := os.ReadDir(filepath.Join(cacheRoot, "uncached")); err == nil || len(entries) != 0 {
		t.Fatalf("completion created an uncached catalog directory")
	}
}

func TestContextCompletionsUseProjectState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))

	project := t.TempDir()
	if err := exec.Command("git", "-C", project, "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	manifest := state.NewManifest()
	manifest.Catalogs["company"] = state.CatalogRegistration{Source: "/catalogs/company"}
	if err := state.SaveManifest(filepath.Join(project, "repertoire.yaml"), manifest); err != nil {
		t.Fatal(err)
	}
	lock := state.NewLock()
	lock.Skills["reviewer"] = state.LockSkill{
		Catalog: "company", Targets: []string{"codex"}, Digest: "digest",
	}
	if err := state.SaveLock(filepath.Join(project, "repertoire.lock.json"), lock); err != nil {
		t.Fatal(err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	global, projectFlag := false, true
	installed, directive := completeInstalledSkills(&global, &projectFlag)(nil, nil, "rev")
	if directive != completionDirective {
		t.Fatalf("installed directive = %v", directive)
	}
	if !reflect.DeepEqual(installed, []string{"reviewer\t[ad-hoc] company → codex"}) {
		t.Fatalf("installed completions = %#v", installed)
	}
	updateArgs, directive := completeInstalledSkillsAndCatalogs(&global, &projectFlag)(nil, nil, "c")
	if directive != completionDirective {
		t.Fatalf("update directive = %v", directive)
	}
	if !reflect.DeepEqual(updateArgs, []string{"company\t[catalog] /catalogs/company"}) {
		t.Fatalf("update completions = %#v", updateArgs)
	}
	registered, _ := completeCatalogs(&global, &projectFlag, true)(nil, nil, "")
	if !reflect.DeepEqual(registered, []string{"company\t[registered] /catalogs/company"}) {
		t.Fatalf("registered completions = %#v", registered)
	}
	visible, _ := completeCatalogs(&global, &projectFlag, false)(nil, nil, "")
	if len(visible) != 2 || !strings.HasPrefix(visible[0], "company\t[registered]") ||
		!strings.HasPrefix(visible[1], "phillarmonic\t[built-in]") {
		t.Fatalf("visible completions = %#v", visible)
	}
}

func TestTargetCompletionsAreSortedAndDoNotCompleteFiles(t *testing.T) {
	got, directive := completeTargets(nil, nil, "co")
	if !reflect.DeepEqual(got, []string{
		"codebuddy\t[agent target]",
		"codex\t[agent target]",
		"copilot\t[agent target]",
	}) {
		t.Fatalf("targets = %#v", got)
	}
	if directive != completionDirective {
		t.Fatalf("directive = %v", directive)
	}
}

func TestListFormatCompletions(t *testing.T) {
	got, directive := completeListFormats(nil, nil, "t")
	want := []string{
		"table\t[compact table]",
		"tsv\t[headerless tab-separated values]",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("formats = %#v, want %#v", got, want)
	}
	if directive != completionDirective {
		t.Fatalf("directive = %v", directive)
	}
}

func TestNewAgentTargetCompletions(t *testing.T) {
	got, directive := completeTargets(nil, nil, "")
	for _, target := range []string{
		"aider", "amp", "antigravity", "claude", "cline", "codebuddy",
		"cursor", "devin", "droid", "dsh", "gemini", "hermes", "junie", "kilo",
		"kimi", "kiro", "opencode", "openclaw", "pi", "roo", "trae",
		"trae-cn", "vscode", "windows", "windsurf",
	} {
		if !containsCompletion(got, target+"\t[agent target]") {
			t.Fatalf("target %q missing from completions: %#v", target, got)
		}
	}
	if !containsCompletion(got, "all\t[all agent targets]") {
		t.Fatalf("all targets missing from completions: %#v", got)
	}
	if directive != completionDirective {
		t.Fatalf("directive = %v", directive)
	}
}

func containsCompletion(completions []string, want string) bool {
	return slices.Contains(completions, want)
}

func TestCompletionFunctionsAreWiredToCommandsAndFlags(t *testing.T) {
	command := NewRootCommand("test", &bytes.Buffer{}, &bytes.Buffer{})
	for _, name := range []string{"add", "install", "update", "remove"} {
		child, _, err := command.Find([]string{name})
		if err != nil {
			t.Fatalf("find %s: %v", name, err)
		}
		if child.ValidArgsFunction == nil {
			t.Fatalf("%s has no argument completion", name)
		}
	}
	for _, commandName := range []string{"add", "install", "update"} {
		child, _, _ := command.Find([]string{commandName})
		flagNames := []string{"target"}
		if commandName != "update" {
			flagNames = append([]string{"catalog"}, flagNames...)
		}
		for _, flagName := range flagNames {
			if _, exists := child.GetFlagCompletionFunc(flagName); !exists {
				t.Fatalf("%s --%s has no flag completion", commandName, flagName)
			}
		}
	}
	list, _, _ := command.Find([]string{"list"})
	if _, exists := list.GetFlagCompletionFunc("format"); !exists {
		t.Fatal("list --format has no flag completion")
	}
	catalogAdd, _, _ := command.Find([]string{"catalog", "add"})
	catalogUpdate, _, _ := command.Find([]string{"catalog", "update"})
	catalogRemove, _, _ := command.Find([]string{"catalog", "remove"})
	if catalogAdd.ValidArgsFunction == nil {
		t.Fatal("catalog add argument completion is not wired")
	}
	if catalogUpdate.ValidArgsFunction == nil || catalogRemove.ValidArgsFunction == nil {
		t.Fatal("catalog update/remove argument completion is not wired")
	}
	stubGet, _, _ := command.Find([]string{"stub", "get"})
	stubList, _, _ := command.Find([]string{"stub", "list"})
	if stubGet.ValidArgsFunction == nil || stubList.ValidArgsFunction == nil {
		t.Fatal("stub get/list argument completion is not wired")
	}
}

func writeCompletionCatalog(t *testing.T, root, name string, skills map[string]string) string {
	t.Helper()
	manifest := state.NewManifest()
	manifest.Catalog = &state.CatalogDefinition{Name: name, Skills: map[string]state.SkillEntry{}}
	for skill, path := range skills {
		manifest.Catalog.Skills[skill] = state.SkillEntry{Path: path}
	}
	if err := state.SaveManifest(filepath.Join(root, "repertoire.yaml"), manifest); err != nil {
		t.Fatal(err)
	}
	return root
}
