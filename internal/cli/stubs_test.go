package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestResolveStubUsesIntactCopyAndNamespacedID(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	modified := stubSkillFixture(t, filepath.Join(base, "a"), true)
	intact := stubSkillFixture(t, filepath.Join(base, "b"), true)
	digest, err := installer.Digest(intact)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modified, "local.txt"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}
	name := "github.com/example/skills/common-stubs"
	lock := state.NewLock()
	lock.Skills[name] = state.LockSkill{
		Digest:    digest,
		Locations: []string{intact, modified},
	}

	resolved, err := resolveStub(lock, name+"/editorconfig")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != name+"/editorconfig" {
		t.Fatalf("id = %q", resolved.ID)
	}
	if resolved.AssetPath != filepath.Join(intact, "assets", ".editorconfig") {
		t.Fatalf("asset = %q, want intact copy under %q", resolved.AssetPath, intact)
	}
}

func TestResolveStubErrorsAreActionable(t *testing.T) {
	t.Parallel()
	root := stubSkillFixture(t, filepath.Join(t.TempDir(), "skill"), true)
	digest, err := installer.Digest(root)
	if err != nil {
		t.Fatal(err)
	}
	lock := state.NewLock()
	lock.Skills["common-stubs"] = state.LockSkill{Digest: digest, Locations: []string{root}}

	for _, test := range []struct {
		id   string
		want string
	}{
		{id: "invalid", want: "must be <skill>/<stub>"},
		{id: "missing/editorconfig", want: `skill "missing" is not installed`},
		{id: "common-stubs/missing", want: `stub "missing" is not exposed`},
	} {
		if _, err := resolveStub(lock, test.id); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("resolve %q error = %v, want %q", test.id, err, test.want)
		}
	}

	if err := os.WriteFile(filepath.Join(root, "local.txt"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveStub(lock, "common-stubs/editorconfig"); err == nil ||
		!strings.Contains(err.Error(), `repertoire install common-stubs`) {
		t.Fatalf("modified copy error = %v", err)
	}
}

func TestStubCommandsListAndGetInProjectScope(t *testing.T) {
	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	alpha := stubSkillFixture(t, filepath.Join(project, ".agents", "skills", "alpha"), true)
	plain := stubSkillFixture(t, filepath.Join(project, ".agents", "skills", "plain"), false)
	lock := state.NewLock()
	for name, root := range map[string]string{"alpha": alpha, "plain": plain} {
		digest, err := installer.Digest(root)
		if err != nil {
			t.Fatal(err)
		}
		lock.Skills[name] = state.LockSkill{Digest: digest, Locations: []string{root}}
	}
	if err := state.SaveLock(filepath.Join(project, "repertoire.lock.json"), lock); err != nil {
		t.Fatal(err)
	}

	listOutput := executeStubCommand(t, project, "--project", "stub", "list")
	if strings.TrimSpace(listOutput) != "alpha/editorconfig\tEnsure text files end with a newline." {
		t.Fatalf("list output:\n%s", listOutput)
	}
	getOutput := executeStubCommand(t, project, "--project", "stub", "get", "alpha/editorconfig")
	for _, want := range []string{
		"Stub: alpha/editorconfig",
		"Description: Ensure text files end with a newline.",
		"Asset: " + filepath.Join(alpha, "assets", ".editorconfig"),
		"Instructions:\nCreate or merge the repository-root .editorconfig.",
	} {
		if !strings.Contains(getOutput, want) {
			t.Fatalf("get output missing %q:\n%s", want, getOutput)
		}
	}
	if output := executeStubCommand(t, project, "--project", "stub", "list", "plain"); strings.TrimSpace(output) != "" {
		t.Fatalf("plain list output = %q", output)
	}
}

func TestStubCommandsUseGlobalScopeByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	root := stubSkillFixture(t, filepath.Join(home, ".agents", "skills", "common-stubs"), true)
	scope, err := state.ResolveScope(state.ScopeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := installer.Digest(root)
	if err != nil {
		t.Fatal(err)
	}
	lock := state.NewLock()
	lock.Skills["common-stubs"] = state.LockSkill{Digest: digest, Locations: []string{root}}
	if err := state.SaveLock(scope.LockPath, lock); err != nil {
		t.Fatal(err)
	}

	output := executeStubCommand(t, t.TempDir(), "stub", "get", "common-stubs/editorconfig")
	if !strings.Contains(output, "Asset: "+filepath.Join(root, "assets", ".editorconfig")) {
		t.Fatalf("global output:\n%s", output)
	}
}

func TestInstalledStubCompletion(t *testing.T) {
	t.Parallel()
	root := stubSkillFixture(t, filepath.Join(t.TempDir(), "common-stubs"), true)
	digest, err := installer.Digest(root)
	if err != nil {
		t.Fatal(err)
	}
	lock := state.NewLock()
	lock.Skills["common-stubs"] = state.LockSkill{Digest: digest, Locations: []string{root}}
	completions := installedStubCompletions(lock, "common")
	if len(completions) != 1 || !strings.HasPrefix(completions[0], "common-stubs/editorconfig\t") {
		t.Fatalf("completions = %#v", completions)
	}
}

func stubSkillFixture(t *testing.T, root string, withStub bool) string {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: fixture\ndescription: Test skill\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !withStub {
		return root
	}
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", ".editorconfig"), []byte("root = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "schema: 1\nstubs:\n  editorconfig:\n" +
		"    description: Ensure text files end with a newline.\n" +
		"    path: assets/.editorconfig\n" +
		"    instructions: Create or merge the repository-root .editorconfig.\n"
	if err := os.WriteFile(filepath.Join(root, "stubs.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func executeStubCommand(t *testing.T, directory string, args ...string) string {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()
	var output bytes.Buffer
	command := NewRootCommand("test", &output, &output)
	command.SetArgs(args)
	if err := command.Execute(); err != nil {
		t.Fatalf("execute %v: %v\n%s", args, err, output.String())
	}
	return output.String()
}
