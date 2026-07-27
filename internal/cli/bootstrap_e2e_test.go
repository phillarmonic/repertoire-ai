package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/state"
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
    targets: [agents]
  project-demo:
    catalog: local
    scope: project
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
		if output := runCommandWithEnv(t, project, environment, binary, "--project", "list"); !strings.Contains(output, "project-demo\tlocal\tbootstrap\tagents") {
			t.Fatalf("project list:\n%s", output)
		}
		if output := runCommandWithEnv(t, project, environment, binary, "list"); !strings.Contains(output, "global-demo\tlocal\tbootstrap\tagents") {
			t.Fatalf("global list:\n%s", output)
		}

		runCommandWithEnv(t, project, environment, binary, "--project", "remove", "project-demo")
		runCommandWithEnv(t, project, environment, binary, "remove", "global-demo")
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
    scope: project
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

	t.Run("global skill manages project instructions without a project lock", func(t *testing.T) {
		project, home, environment := bootstrapEnvironment(t)
		catalogRoot := createBootstrapIntegrationCatalog(t, "v1")
		if err := os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte("# Project\n\nKeep me.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeBootstrapFile(t, project, `schema: 1
catalogs:
  graphify:
    source: `+catalogRoot+`
skills:
  graphify:
    catalog: graphify
    scope: global
    targets: [agents, copilot]
`)
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		assertContainsFile(t, filepath.Join(project, "AGENTS.md"), "Keep me.", "Graphify pointer v1")
		assertContainsFile(t, filepath.Join(project, ".github", "copilot-instructions.md"), "Graphify pointer v1")
		if _, err := os.Stat(filepath.Join(project, ".git", "hooks", "post-commit")); !os.IsNotExist(err) {
			t.Fatalf("hooks should remain optional: %v", err)
		}
		if _, err := os.Stat(filepath.Join(project, "repertoire.lock.json")); !os.IsNotExist(err) {
			t.Fatalf("global bootstrap should not create a project lock: %v", err)
		}
		lock, err := state.LoadLock(filepath.Join(globalConfigRoot(home), "repertoire.lock.json"))
		if err != nil {
			t.Fatal(err)
		}
		projectInfo, err := os.Stat(project)
		if err != nil {
			t.Fatal(err)
		}
		foundProject := false
		for projectRoot, skills := range lock.Projects {
			rootInfo, statErr := os.Stat(projectRoot)
			if statErr != nil || !os.SameFile(projectInfo, rootInfo) {
				continue
			}
			foundProject = skills["graphify"].Instructions
			break
		}
		if !foundProject {
			t.Fatalf("global lock does not contain Graphify instructions for project %s: %+v", project, lock.Projects)
		}

		createBootstrapIntegrationCatalogAt(t, catalogRoot, "v2")
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		assertContainsFile(t, filepath.Join(project, "AGENTS.md"), "Keep me.", "Graphify pointer v2")

		writeBootstrapFile(t, project, `schema: 1
catalogs:
  graphify:
    source: `+catalogRoot+`
skills:
  graphify:
    catalog: graphify
    scope: global
    targets: [agents, copilot]
    hooks: true
`)
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		assertFileContent(t, filepath.Join(project, ".git", "hooks", "post-commit"), "#!/bin/sh\ngraphify update .\n")

		writeBootstrapFile(t, project, `schema: 1
catalogs:
  graphify:
    source: `+catalogRoot+`
skills:
  graphify:
    catalog: graphify
    scope: global
    targets: [agents, copilot]
`)
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		if _, err := os.Stat(filepath.Join(project, ".git", "hooks", "post-commit")); !os.IsNotExist(err) {
			t.Fatalf("disabling hooks should remove the managed hook: %v", err)
		}
		assertContainsFile(t, filepath.Join(project, "AGENTS.md"), "Keep me.", "Graphify pointer v2")

		runCommandWithEnv(t, project, environment, binary, "remove", "graphify")
		assertContainsFile(t, filepath.Join(project, "AGENTS.md"), "Keep me.")
		if strings.Contains(readFileForTest(t, filepath.Join(project, "AGENTS.md")), "Graphify pointer") {
			t.Fatal("global removal left the managed project instruction")
		}
		if _, err := os.Stat(filepath.Join(project, ".github", "copilot-instructions.md")); !os.IsNotExist(err) {
			t.Fatalf("global removal left a created project instruction: %v", err)
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
    scope: project
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
    scope: project
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

	t.Run("missing bootstrap creates namespaced global starter", func(t *testing.T) {
		project, home, environment := bootstrapEnvironment(t)
		seedBuiltinCatalogCache(t, home, map[string]string{
			"demo": "demo-v1",
		})
		if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
			t.Fatal(err)
		}
		output := runCommandWithEnv(t, project, environment, binary, "bootstrap")
		if !strings.Contains(output, "created .repertoire.yaml") ||
			!strings.Contains(output, "bootstrapped github.com/phillarmonic/ai-skills/demo (global)") {
			t.Fatalf("bootstrap create output:\n%s", output)
		}
		content, err := os.ReadFile(filepath.Join(project, ".repertoire.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		body := string(content)
		if !strings.Contains(body, "github.com/phillarmonic/ai-skills/demo:") ||
			!strings.Contains(body, "scope: global") {
			t.Fatalf("created bootstrap manifest:\n%s", body)
		}
		if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "demo", "SKILL.md")); err != nil {
			t.Fatalf("expected global install of short skill name: %v", err)
		}
		if _, err := os.Stat(filepath.Join(project, ".codex", "skills", "demo")); !os.IsNotExist(err) {
			t.Fatalf("project should not contain installed skill: %v", err)
		}
		emptyProject, _, emptyEnv := bootstrapEnvironment(t)
		output = runCommandWithEnvError(t, emptyProject, emptyEnv, binary, "sync")
		if !strings.Contains(output, "does not exist") {
			t.Fatalf("sync without manifest:\n%s", output)
		}
	})
}

func bootstrapEnvironment(t *testing.T) (string, string, []string) {
	t.Helper()
	project := t.TempDir()
	runCommand(t, project, "git", "init", "-q")
	home := t.TempDir()
	environment := make([]string, 0, len(os.Environ())+7)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "HOME=") || strings.HasPrefix(value, "XDG_CONFIG_HOME=") ||
			strings.HasPrefix(value, "XDG_CACHE_HOME=") || strings.HasPrefix(value, "CODEX_HOME=") ||
			strings.HasPrefix(value, "APPDATA=") || strings.HasPrefix(value, "LOCALAPPDATA=") ||
			strings.HasPrefix(value, "USERPROFILE=") {
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
		"USERPROFILE="+home,
	)
	return project, home, environment
}

func seedBuiltinCatalogCache(t *testing.T, home string, skills map[string]string) {
	t.Helper()
	root := createBootstrapCatalog(t, "phillarmonic", skills)
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.test")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "seed")
	cacheParent := filepath.Join(userCacheRoot(home), "repertoire", "catalogs")
	if err := os.MkdirAll(cacheParent, 0o755); err != nil {
		t.Fatal(err)
	}
	runCommand(t, cacheParent, "git", "clone", "-q", root, "phillarmonic")
}

func userCacheRoot(home string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Caches")
	case "windows":
		return filepath.Join(home, "cache")
	default:
		return filepath.Join(home, "cache")
	}
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

func createBootstrapIntegrationCatalog(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	createBootstrapIntegrationCatalogAt(t, root, version)
	return root
}

func createBootstrapIntegrationCatalogAt(t *testing.T, root, version string) {
	t.Helper()
	for _, directory := range []string{
		filepath.Join(root, "skills", "graphify"),
		filepath.Join(root, "project-files"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	skill := "---\nname: graphify\ndescription: Test Graphify skill\n---\n"
	if err := os.WriteFile(filepath.Join(root, "skills", "graphify", "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "project-files", "agents.md"),
		[]byte("## Graphify pointer "+version+"\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "project-files", "post-commit"),
		[]byte("#!/bin/sh\ngraphify update .\n"),
		0o755,
	); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
catalog:
  name: graphify
  skills:
    graphify:
      path: skills/graphify
      instructions:
        agents:
          - id: guidance
            source: project-files/agents.md
            destination: AGENTS.md
            mode: markdown-section
        copilot:
          - id: copilot-guidance
            source: project-files/agents.md
            destination: .github/copilot-instructions.md
            mode: markdown-section
      artifacts:
        all:
          - id: post-commit
            source: project-files/post-commit
            destination: .git/hooks/post-commit
            mode: copy
            executable: true
`
	if err := os.WriteFile(filepath.Join(root, "repertoire.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
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
