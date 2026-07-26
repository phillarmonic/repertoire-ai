package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAddInstallAndListEndToEnd(t *testing.T) {
	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	catalogRoot := t.TempDir()
	for _, name := range []string{"demo", "loose"} {
		root := filepath.Join(catalogRoot, "skills", name)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + name + "\ndescription: Test skill\n---\n"
		if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := "schema: 1\ncatalog:\n  name: local\n  skills:\n    demo:\n      path: skills/demo\n    loose:\n      path: skills/loose\n"
	if err := os.WriteFile(filepath.Join(catalogRoot, "repertoire.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")
	runCommand(t, project, binary, "--project", "catalog", "add", catalogRoot, "--name", "local")
	runCommand(t, project, binary, "--project", "add", "demo", "--catalog", "local", "--target", "agents")
	runCommand(t, project, binary, "--project", "install", "loose", "--catalog", "local", "--target", "agents")
	output := runCommand(t, project, binary, "--project", "list")
	if !strings.Contains(output, "demo\tlocal\tdeclared\tagents") ||
		!strings.Contains(output, "loose\tlocal\tad-hoc\tagents") {
		t.Fatalf("unexpected list output:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("installed skill: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(project, ".agents", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	runCommand(t, project, binary, "--project", "install", "--target", "all")
	for _, path := range []string{
		filepath.Join(project, ".agents", "skills", "demo", "SKILL.md"),
		filepath.Join(project, ".codex", "skills", "demo", "SKILL.md"),
		filepath.Join(project, ".windsurf", "skills", "demo", "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("installed declared skill on all targets at %s: %v", path, err)
		}
	}
	if err := os.RemoveAll(filepath.Join(project, ".codex", "skills", "demo")); err != nil {
		t.Fatal(err)
	}
	runCommand(t, project, binary, "--project", "update", "demo", "--target", "all")
	if _, err := os.Stat(filepath.Join(project, ".codex", "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("updated skill on all targets: %v", err)
	}
	runCommand(t, project, binary, "--project", "remove", "demo")
	runCommand(t, project, binary, "--project", "remove", "loose")
	output = runCommand(t, project, binary, "--project", "list")
	if strings.TrimSpace(output) != "" {
		t.Fatalf("expected empty installed list, got %q", output)
	}
}

func testBinaryPath(t *testing.T) string {
	t.Helper()
	name := "repertoire"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(t.TempDir(), name)
}

func runCommand(t *testing.T, directory, name string, arguments ...string) string {
	t.Helper()
	return runCommandWithEnv(t, directory, os.Environ(), name, arguments...)
}

func runCommandWithEnv(t *testing.T, directory string, environment []string, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, arguments, err, output)
	}
	return string(output)
}
