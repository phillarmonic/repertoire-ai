package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/doctor"
	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestDoctorEndToEnd(t *testing.T) {
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	writeDemo := func(t *testing.T, project, catalogRoot string) {
		t.Helper()
		writeBootstrapFile(t, project, `schema: 1
catalogs:
  demo:
    source: `+catalogRoot+`
skills:
  demo:
    catalog: demo
    scope: global
    targets: [agents, codex]
`)
	}

	t.Run("healthy project passes with one deduplicated section", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		writeDemo(t, project, createDoctorCatalog(t))
		runCommandWithEnv(t, project, environment, binary, "bootstrap")

		agents := readFileForTest(t, filepath.Join(project, "AGENTS.md"))
		if strings.Count(agents, ":start -->") != 1 || !strings.Contains(agents, "repertoire:demo:all:guidance:start") {
			t.Fatalf("bootstrap should install one deduplicated section:\n%s", agents)
		}
		output := runCommandWithEnv(t, project, environment, binary, "doctor")
		if !strings.Contains(output, "no issues found") {
			t.Fatalf("doctor output:\n%s", output)
		}
	})

	t.Run("missing destination is repaired", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		writeDemo(t, project, createDoctorCatalog(t))
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		if err := os.Remove(filepath.Join(project, "AGENTS.md")); err != nil {
			t.Fatal(err)
		}

		output := runCommandWithEnvError(t, project, environment, binary, "doctor")
		if !strings.Contains(output, "missing-destination") {
			t.Fatalf("doctor output:\n%s", output)
		}
		runCommandWithEnv(t, project, environment, binary, "doctor", "--fix")
		if content := readFileForTest(t, filepath.Join(project, "AGENTS.md")); !strings.Contains(content, "Demo pointer v1") {
			t.Fatalf("repaired AGENTS.md:\n%s", content)
		}
		runCommandWithEnv(t, project, environment, binary, "doctor")
	})

	t.Run("locally modified section is repaired", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		writeDemo(t, project, createDoctorCatalog(t))
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		destination := filepath.Join(project, "AGENTS.md")
		content := readFileForTest(t, destination)
		if err := os.WriteFile(destination, []byte(strings.Replace(content, "Demo pointer v1", "local edit", 1)), 0o644); err != nil {
			t.Fatal(err)
		}

		output := runCommandWithEnvError(t, project, environment, binary, "doctor")
		if !strings.Contains(output, "modified-managed-content") {
			t.Fatalf("doctor output:\n%s", output)
		}
		runCommandWithEnv(t, project, environment, binary, "doctor", "--fix")
		if repaired := readFileForTest(t, destination); !strings.Contains(repaired, "Demo pointer v1") || strings.Contains(repaired, "local edit") {
			t.Fatalf("repaired AGENTS.md:\n%s", repaired)
		}
	})

	t.Run("duplicated per-target sections collapse to the shared marker", func(t *testing.T) {
		project, home, environment := bootstrapEnvironment(t)
		writeDemo(t, project, createDoctorCatalog(t))
		runCommandWithEnv(t, project, environment, binary, "bootstrap")

		// Replicate the pre-dedup layout: one identical block per target,
		// each with its own marker and lock entry.
		lockPath := filepath.Join(globalConfigRoot(home), "repertoire.lock.json")
		lock, err := state.LoadLock(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		projectKey := findLockedProject(t, lock, project)
		entry := lock.Projects[projectKey]["demo"]
		if len(entry.Artifacts) != 1 {
			t.Fatalf("bootstrap artifacts = %+v", entry.Artifacts)
		}
		destination := entry.Artifacts[0].Destination
		var content strings.Builder
		var artifacts []state.LockArtifact
		for index, target := range []string{"agents", "codex"} {
			marker := "repertoire:demo:" + target + ":guidance"
			separator := "\n"
			if index == 0 {
				separator = ""
			}
			content.WriteString(separator)
			content.Write(installer.RenderMarkedSection(marker, []byte("## Demo pointer v1\n")))
			artifacts = append(artifacts, state.LockArtifact{
				ID: "guidance", Target: target, Destination: destination,
				Mode: state.ArtifactModeMarkdownSection, Marker: marker,
				Created: true, MarkdownSeparator: separator,
			})
		}
		if writeErr := os.WriteFile(destination, []byte(content.String()), 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
		for index := range artifacts {
			digest, found, markdownErr := installer.MarkdownSectionDigest(destination, artifacts[index].Marker)
			if markdownErr != nil || !found {
				t.Fatalf("digest %s: %v", artifacts[index].Marker, markdownErr)
			}
			artifacts[index].Digest = digest
		}
		entry.Artifacts = artifacts
		lock.Projects[projectKey]["demo"] = entry
		if saveErr := state.SaveLock(lockPath, lock); saveErr != nil {
			t.Fatal(saveErr)
		}

		output := runCommandWithEnvError(t, project, environment, binary, "doctor")
		if !strings.Contains(output, "duplicate-sections") {
			t.Fatalf("doctor output:\n%s", output)
		}
		runCommandWithEnv(t, project, environment, binary, "doctor", "--fix")
		collapsed := readFileForTest(t, destination)
		if strings.Count(collapsed, ":start -->") != 1 || !strings.Contains(collapsed, "repertoire:demo:all:guidance:start") {
			t.Fatalf("collapsed AGENTS.md:\n%s", collapsed)
		}
		lock, err = state.LoadLock(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		artifacts = lock.Projects[projectKey]["demo"].Artifacts
		if len(artifacts) != 1 || artifacts[0].Target != "all" {
			t.Fatalf("collapsed lock artifacts = %+v", artifacts)
		}
	})

	t.Run("orphaned section is removed", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		writeDemo(t, project, createDoctorCatalog(t))
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		destination := filepath.Join(project, "AGENTS.md")
		content := readFileForTest(t, destination)
		orphan := "\n<!-- repertoire:ghost:codex:x:start -->\nBoo\n<!-- repertoire:ghost:codex:x:end -->\n"
		if err := os.WriteFile(destination, []byte(content+orphan), 0o644); err != nil {
			t.Fatal(err)
		}

		output := runCommandWithEnvError(t, project, environment, binary, "doctor")
		if !strings.Contains(output, "orphaned-markers") {
			t.Fatalf("doctor output:\n%s", output)
		}
		runCommandWithEnv(t, project, environment, binary, "doctor", "--fix")
		repaired := readFileForTest(t, destination)
		if strings.Contains(repaired, "ghost") || !strings.Contains(repaired, "Demo pointer v1") {
			t.Fatalf("repaired AGENTS.md:\n%s", repaired)
		}
	})

	t.Run("manifest drift is reported and reconciled by sync", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		writeDemo(t, project, createDoctorCatalog(t))

		output := runCommandWithEnvError(t, project, environment, binary, "doctor")
		if !strings.Contains(output, "manifest-drift") || !strings.Contains(output, "run repertoire sync") {
			t.Fatalf("doctor output:\n%s", output)
		}
		runCommandWithEnv(t, project, environment, binary, "doctor", "--fix")
		if content := readFileForTest(t, filepath.Join(project, "AGENTS.md")); !strings.Contains(content, "Demo pointer v1") {
			t.Fatalf("reconciled AGENTS.md:\n%s", content)
		}
	})

	t.Run("stale global project entries are pruned", func(t *testing.T) {
		project, home, environment := bootstrapEnvironment(t)
		writeDemo(t, project, createDoctorCatalog(t))
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		lockPath := filepath.Join(globalConfigRoot(home), "repertoire.lock.json")
		lock, err := state.LoadLock(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		vanished := filepath.Join(project, "vanished")
		lock.Projects[vanished] = map[string]state.LockProjectArtifacts{"demo": {Catalog: "demo"}}
		if saveErr := state.SaveLock(lockPath, lock); saveErr != nil {
			t.Fatal(saveErr)
		}

		output := runCommandWithEnvError(t, project, environment, binary, "doctor")
		if !strings.Contains(output, "stale-project-entries") {
			t.Fatalf("doctor output:\n%s", output)
		}
		runCommandWithEnv(t, project, environment, binary, "doctor", "--fix")
		lock, err = state.LoadLock(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := lock.Projects[vanished]; exists {
			t.Fatalf("stale entry remains: %+v", lock.Projects)
		}
	})

	t.Run("reset requires confirmation and reinstalls from scratch", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		writeDemo(t, project, createDoctorCatalog(t))
		runCommandWithEnv(t, project, environment, binary, "bootstrap")

		output := runCommandWithEnvError(t, project, environment, binary, "doctor", "--reset")
		if !strings.Contains(output, "--reset requires --yes") {
			t.Fatalf("reset guard output:\n%s", output)
		}
		if err := os.Remove(filepath.Join(project, "AGENTS.md")); err != nil {
			t.Fatal(err)
		}
		runCommandWithEnv(t, project, environment, binary, "doctor", "--reset", "--yes")
		if content := readFileForTest(t, filepath.Join(project, "AGENTS.md")); !strings.Contains(content, "Demo pointer v1") {
			t.Fatalf("reset AGENTS.md:\n%s", content)
		}
	})

	t.Run("json output parses while the exit code signals issues", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		writeDemo(t, project, createDoctorCatalog(t))
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		destination := filepath.Join(project, "AGENTS.md")
		content := readFileForTest(t, destination)
		orphan := "\n<!-- repertoire:ghost:codex:x:start -->\nBoo\n<!-- repertoire:ghost:codex:x:end -->\n"
		if err := os.WriteFile(destination, []byte(content+orphan), 0o644); err != nil {
			t.Fatal(err)
		}

		command := exec.Command(binary, "doctor", "--format", "json")
		command.Dir = project
		command.Env = environment
		stdout, err := command.Output()
		if err == nil {
			t.Fatalf("doctor unexpectedly succeeded:\n%s", stdout)
		}
		var report doctor.Result
		if err := json.Unmarshal(stdout, &report); err != nil {
			t.Fatalf("json output: %v\n%s", err, stdout)
		}
		if len(report.Issues) != 1 || report.Issues[0].Check != "orphaned-markers" {
			t.Fatalf("json report = %+v", report.Issues)
		}
	})

	t.Run("scope flags are rejected", func(t *testing.T) {
		project, _, environment := bootstrapEnvironment(t)
		output := runCommandWithEnvError(t, project, environment, binary, "doctor", "--global")
		if !strings.Contains(output, "--global and --project are not supported") {
			t.Fatalf("scope flag output:\n%s", output)
		}
	})
}

func createDoctorCatalog(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{
		filepath.Join(root, "skills", "demo"),
		filepath.Join(root, "project-files"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(
		filepath.Join(root, "skills", "demo", "SKILL.md"),
		[]byte("---\nname: demo\ndescription: Test demo skill\n---\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "project-files", "agents.md"),
		[]byte("## Demo pointer v1\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
catalog:
  name: demo
  skills:
    demo:
      path: skills/demo
      instructions:
        agents:
          - id: guidance
            source: project-files/agents.md
            destination: AGENTS.md
            mode: markdown-section
        codex:
          - id: guidance
            source: project-files/agents.md
            destination: AGENTS.md
            mode: markdown-section
`
	if err := os.WriteFile(filepath.Join(root, "repertoire.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// findLockedProject locates the lock key for project, tolerating the
// symlink-resolved roots git reports on macOS.
func findLockedProject(t *testing.T, lock state.Lock, project string) string {
	t.Helper()
	info, err := os.Stat(project)
	if err != nil {
		t.Fatal(err)
	}
	for root := range lock.Projects {
		rootInfo, err := os.Stat(root)
		if err == nil && os.SameFile(info, rootInfo) {
			return root
		}
	}
	t.Fatalf("project %s missing from lock: %+v", project, lock.Projects)
	return ""
}
