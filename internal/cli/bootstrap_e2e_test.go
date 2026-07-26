package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBootstrapAndSyncEndToEnd(t *testing.T) {
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	t.Run("mixed scopes repair additive lifecycle and origins", func(t *testing.T) {
		project, home, environment := bootstrapEnvironment(t)
		catalogRoot := createBootstrapCatalog(t, "local", map[string]string{
			"global-demo":  "global-v1",
			"project-demo": "project-v1",
		})
		writeBootstrapFile(t, project, `schema: 1
catalogs:
  local:
    source: `+catalogRoot+`
skills:
  global-demo:
    catalog: local
    scope: global
    targets: [agents]
  project-demo:
    catalog: local
    targets: [agents]
`)
		output := runCommandWithEnv(t, project, environment, binary, "bootstrap")
		if !strings.Contains(output, "bootstrapped global-demo (global)") ||
			!strings.Contains(output, "bootstrapped project-demo (project)") {
			t.Fatalf("bootstrap output:\n%s", output)
		}
		projectSkill := filepath.Join(project, ".agents", "skills", "project-demo", "SKILL.md")
		globalSkill := filepath.Join(home, ".agents", "skills", "global-demo", "SKILL.md")
		for _, path := range []string{projectSkill, globalSkill} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("installed skill %s: %v", path, err)
			}
		}
		if output := runCommandWithEnv(t, project, environment, binary, "list"); !strings.Contains(output, "project-demo\tlocal\tbootstrap\tagents") {
			t.Fatalf("project list:\n%s", output)
		}
		if output := runCommandWithEnv(t, project, environment, binary, "--global", "list"); !strings.Contains(output, "global-demo\tlocal\tbootstrap\tagents") {
			t.Fatalf("global list:\n%s", output)
		}

		runCommandWithEnv(t, project, environment, binary, "remove", "project-demo")
		runCommandWithEnv(t, project, environment, binary, "--global", "remove", "global-demo")
		if _, err := os.Stat(filepath.Join(project, "repertoire.yaml")); !os.IsNotExist(err) {
			t.Fatalf("removing bootstrap skill changed project requirements manifest: %v", err)
		}
		if _, err := os.Stat(filepath.Join(globalConfigRoot(home), "repertoire.yaml")); !os.IsNotExist(err) {
			t.Fatalf("removing bootstrap skill changed global requirements manifest: %v", err)
		}
		runCommandWithEnv(t, project, environment, binary, "bootstrap")

		if err := os.RemoveAll(filepath.Dir(projectSkill)); err != nil {
			t.Fatal(err)
		}
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		if _, err := os.Stat(projectSkill); err != nil {
			t.Fatalf("bootstrap did not repair project skill: %v", err)
		}

		writeBootstrapFile(t, project, `schema: 1
catalogs:
  local:
    source: `+catalogRoot+`
skills:
  global-demo:
    catalog: local
    scope: global
    targets: [agents]
`)
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		if _, err := os.Stat(projectSkill); err != nil {
			t.Fatalf("omitted project skill was removed: %v", err)
		}
	})

	t.Run("modified targets and shared global conflicts stay protected", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		first := createBootstrapCatalog(t, "first", map[string]string{"demo": "first"})
		second := createBootstrapCatalog(t, "second", map[string]string{"demo": "second"})
		writeBootstrapFile(t, project, `schema: 1
catalogs:
  first:
    source: `+first+`
skills:
  demo:
    catalog: first
    scope: global
    targets: [agents]
`)
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		writeBootstrapFile(t, project, `schema: 1
catalogs:
  second:
    source: `+second+`
skills:
  demo:
    catalog: second
    scope: global
    targets: [agents]
`)
		output := runCommandWithEnvError(t, project, environment, binary, "bootstrap")
		if !strings.Contains(output, "different catalog source or ref") {
			t.Fatalf("global conflict output:\n%s", output)
		}
		runCommandWithEnv(t, project, environment, binary, "bootstrap", "--force")

		writeBootstrapFile(t, project, `schema: 1
catalogs:
  second:
    source: `+second+`
skills:
  demo:
    catalog: second
    targets: [agents]
`)
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		target := filepath.Join(project, ".agents", "skills", "demo", "local.txt")
		if err := os.WriteFile(target, []byte("modified"), 0o644); err != nil {
			t.Fatal(err)
		}
		output = runCommandWithEnvError(t, project, environment, binary, "bootstrap")
		if !strings.Contains(output, "locally modified") {
			t.Fatalf("modified target output:\n%s", output)
		}
	})

	t.Run("bootstrap stays cached sync refreshes and commit refs stay pinned", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		work, remote, firstCommit := createTrackingCatalog(t)
		writeBootstrapFile(t, project, `schema: 1
catalogs:
  tracking:
    source: file://`+remote+`
    ref: main
skills:
  tracked:
    catalog: tracking
    targets: [agents]
`)
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		versionPath := filepath.Join(project, ".agents", "skills", "tracked", "version.txt")
		assertFileContent(t, versionPath, "v1")

		if err := os.WriteFile(filepath.Join(work, "skills", "tracked", "version.txt"), []byte("v2"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, work, "add", ".")
		runGit(t, work, "commit", "-qm", "v2")
		runGit(t, work, "push", "-q", "origin", "main")

		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		assertFileContent(t, versionPath, "v1")
		runCommandWithEnv(t, project, environment, binary, "sync")
		assertFileContent(t, versionPath, "v2")

		writeBootstrapFile(t, project, `schema: 1
catalogs:
  tracking:
    source: file://`+remote+`
    ref: `+firstCommit+`
skills:
  tracked:
    catalog: tracking
    targets: [agents]
`)
		runCommandWithEnv(t, project, environment, binary, "sync")
		assertFileContent(t, versionPath, "v1")
	})

	t.Run("scope override flags are rejected", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		output := runCommandWithEnvError(t, project, environment, binary, "bootstrap", "--global")
		if !strings.Contains(output, "--global and --project are not supported") {
			t.Fatalf("scope flag output:\n%s", output)
		}
	})
}

func bootstrapEnvironment(t *testing.T) (string, string, []string) {
	t.Helper()
	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	home := t.TempDir()
	environment := make([]string, 0, len(os.Environ())+6)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "HOME=") || strings.HasPrefix(value, "XDG_CONFIG_HOME=") ||
			strings.HasPrefix(value, "XDG_CACHE_HOME=") || strings.HasPrefix(value, "CODEX_HOME=") ||
			strings.HasPrefix(value, "APPDATA=") || strings.HasPrefix(value, "LOCALAPPDATA=") {
			continue
		}
		environment = append(environment, value)
	}
	environment = append(environment,
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, "config"),
		"XDG_CACHE_HOME="+filepath.Join(home, "cache"),
		"APPDATA="+filepath.Join(home, "config"),
		"LOCALAPPDATA="+filepath.Join(home, "cache"),
	)
	return project, home, environment
}

func createBootstrapCatalog(t *testing.T, name string, skills map[string]string) string {
	t.Helper()
	root := t.TempDir()
	var manifest strings.Builder
	manifest.WriteString("schema: 1\ncatalog:\n  name: " + name + "\n  skills:\n")
	for skill, marker := range skills {
		directory := filepath.Join(root, "skills", skill)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + skill + "\ndescription: Test skill\n---\n\n" + marker + "\n"
		if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		manifest.WriteString("    " + skill + ":\n      path: skills/" + skill + "\n")
	}
	if err := os.WriteFile(filepath.Join(root, "repertoire.yaml"), []byte(manifest.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func createTrackingCatalog(t *testing.T) (string, string, string) {
	t.Helper()
	work := t.TempDir()
	runGit(t, work, "init", "-q", "-b", "main")
	runGit(t, work, "config", "user.email", "test@example.test")
	runGit(t, work, "config", "user.name", "Test")
	skill := filepath.Join(work, "skills", "tracked")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: tracked\ndescription: Test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "version.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "schema: 1\ncatalog:\n  name: tracking\n  skills:\n    tracked:\n      path: skills/tracked\n"
	if err := os.WriteFile(filepath.Join(work, "repertoire.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "add", ".")
	runGit(t, work, "commit", "-qm", "v1")
	firstCommit := strings.TrimSpace(runCommand(t, work, "git", "rev-parse", "HEAD"))
	remote := filepath.Join(t.TempDir(), "tracking.git")
	runCommand(t, work, "git", "clone", "-q", "--bare", ".", remote)
	runGit(t, work, "remote", "add", "origin", remote)
	return work, remote, firstCommit
}

func writeBootstrapFile(t *testing.T, project, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(project, ".repertoire.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runCommandWithEnvError(t *testing.T, directory string, environment []string, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = environment
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("%s %v unexpectedly succeeded\n%s", name, arguments, output)
	}
	return string(output)
}

func runGit(t *testing.T, directory string, arguments ...string) {
	t.Helper()
	runCommand(t, directory, "git", arguments...)
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s = %q, want %q", path, content, want)
	}
}

func globalConfigRoot(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "repertoire")
	}
	return filepath.Join(home, "config", "repertoire")
}
