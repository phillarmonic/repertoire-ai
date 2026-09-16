package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDryRunLeavesFilesystemUnchanged(t *testing.T) {
	project, home, environment := bootstrapEnvironment(t)
	catalogRoot := t.TempDir()
	writeSkillCatalog(t, catalogRoot, true, "demo")

	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")
	runCommandWithEnv(t, project, environment, binary, "--project", "catalog", "add", catalogRoot, "--name", "local")

	beforeAdd := snapshotTrees(t, home, project)
	addOutput := runCommandWithEnv(t, project, environment, binary, "--project", "--dry-run", "add", "demo", "--catalog", "local", "--target", "agents")
	assertSnapshotUnchanged(t, "add", beforeAdd, snapshotTrees(t, home, project))
	if !strings.Contains(addOutput, "would install demo to") || !strings.Contains(addOutput, "would write lock entry demo") {
		t.Fatalf("add dry-run output:\n%s", addOutput)
	}

	runCommandWithEnv(t, project, environment, binary, "--project", "add", "demo", "--catalog", "local", "--target", "agents")

	beforeUpdate := snapshotTrees(t, home, project)
	updateOutput := runCommandWithEnv(t, project, environment, binary, "--project", "--dry-run", "update", "demo")
	assertSnapshotUnchanged(t, "update", beforeUpdate, snapshotTrees(t, home, project))
	if !strings.Contains(updateOutput, "would write lock entry demo") {
		t.Fatalf("update dry-run output:\n%s", updateOutput)
	}

	beforeRemove := snapshotTrees(t, home, project)
	removeOutput := runCommandWithEnv(t, project, environment, binary, "--project", "--dry-run", "remove", "demo")
	assertSnapshotUnchanged(t, "remove", beforeRemove, snapshotTrees(t, home, project))
	if !strings.Contains(removeOutput, "would remove demo from") || !strings.Contains(removeOutput, "would remove lock entry demo") {
		t.Fatalf("remove dry-run output:\n%s", removeOutput)
	}

	if err := os.WriteFile(filepath.Join(project, ".agents", "skills", "demo", "SKILL.md"), []byte("locally edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	beforeRefuse := snapshotTrees(t, home, project)
	refuseOutput := runCommandWithEnvError(t, project, environment, binary, "--project", "--dry-run", "update", "demo")
	assertSnapshotUnchanged(t, "refuse", beforeRefuse, snapshotTrees(t, home, project))
	if !strings.Contains(refuseOutput, "would refuse: locally modified copy at") {
		t.Fatalf("modified dry-run output:\n%s", refuseOutput)
	}
}

func TestDryRunCatalogAddDoesNotClone(t *testing.T) {
	project, home, environment := bootstrapEnvironment(t)
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	before := snapshotTrees(t, home, project)
	output := runCommandWithEnv(t, project, environment, binary, "--project", "--dry-run", "catalog", "add", "https://example.invalid/agent-skills.git", "--name", "company")
	assertSnapshotUnchanged(t, "catalog add", before, snapshotTrees(t, home, project))
	if !strings.Contains(output, "would clone https://example.invalid/agent-skills.git") {
		t.Fatalf("catalog add dry-run output:\n%s", output)
	}
}

func TestDryRunBootstrapAndSyncLeaveStateUnchanged(t *testing.T) {
	project, home, environment := bootstrapEnvironment(t)
	catalogRoot := createBootstrapCatalog(t, "local", map[string]string{"demo": "v1"})
	writeBootstrapFile(t, project, `schema: 1
catalogs:
  local:
    source: `+catalogRoot+`
skills:
  demo:
    catalog: local
    scope: project
    targets: [agents]
`)

	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	beforeBootstrap := snapshotTrees(t, home, project)
	bootstrapOutput := runCommandWithEnv(t, project, environment, binary, "--dry-run", "bootstrap")
	assertSnapshotUnchanged(t, "bootstrap", beforeBootstrap, snapshotTrees(t, home, project))
	if !strings.Contains(bootstrapOutput, "would install demo to") {
		t.Fatalf("bootstrap dry-run output:\n%s", bootstrapOutput)
	}

	runCommandWithEnv(t, project, environment, binary, "bootstrap")
	beforeSync := snapshotTrees(t, home, project)
	syncOutput := runCommandWithEnv(t, project, environment, binary, "--dry-run", "sync")
	assertSnapshotUnchanged(t, "sync", beforeSync, snapshotTrees(t, home, project))
	if !strings.Contains(syncOutput, "would write lock entry demo") && !strings.Contains(syncOutput, "would refresh catalog") {
		t.Fatalf("sync dry-run output:\n%s", syncOutput)
	}
}

func TestDryRunIsNoOpNoteForReadCommands(t *testing.T) {
	project, _, environment := bootstrapEnvironment(t)
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	output := runCommandWithEnv(t, project, environment, binary, "--project", "--dry-run", "list")
	if !strings.Contains(output, dryRunNoOpNote) {
		t.Fatalf("list dry-run note missing:\n%s", output)
	}
}

func snapshotTrees(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			key := filepath.ToSlash(filepath.Join(filepath.Base(root), rel))
			if entry.IsDir() {
				snapshot[key] = "dir"
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(content)
			snapshot[key] = hex.EncodeToString(sum[:])
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return snapshot
}

func assertSnapshotUnchanged(t *testing.T, name string, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("%s dry-run changed path count: before %d after %d", name, len(before), len(after))
	}
	for path, digest := range before {
		if after[path] != digest {
			t.Fatalf("%s dry-run changed %s", name, path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Fatalf("%s dry-run created %s", name, path)
		}
	}
}
