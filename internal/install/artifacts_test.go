package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestMarkdownArtifactInstallUpdateAndRemovePreservesUserContent(t *testing.T) {
	project := t.TempDir()
	source := filepath.Join(t.TempDir(), "agents.md")
	if err := os.WriteFile(source, []byte("Graphify v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(project, "AGENTS.md")
	original := "# User instructions\n\nKeep this.\n"
	if err := os.WriteFile(destination, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved := artifactFixture("markdown-section", source, "AGENTS.md")
	targets := []Target{{Name: "codex"}}
	installed, err := InstallArtifacts(resolved, targets, project, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	content := readTestFile(t, destination)
	if !strings.Contains(content, "Keep this.") || !strings.Contains(content, "Graphify v1") ||
		!strings.Contains(content, "repertoire:graphify:codex:guidance:start") {
		t.Fatalf("installed Markdown:\n%s", content)
	}

	if err := os.WriteFile(source, []byte("Graphify v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	updated, err := InstallArtifacts(resolved, targets, project, installed, false)
	if err != nil {
		t.Fatal(err)
	}
	content = readTestFile(t, destination)
	if strings.Contains(content, "Graphify v1") || !strings.Contains(content, "Graphify v2") {
		t.Fatalf("updated Markdown:\n%s", content)
	}
	if err := RemoveArtifacts(updated, project, false); err != nil {
		t.Fatal(err)
	}
	content = readTestFile(t, destination)
	if content != original {
		t.Fatalf("removed Markdown = %q, want original %q", content, original)
	}
}

func TestMarkdownArtifactRemovalRestoresTrailingNewlines(t *testing.T) {
	testCases := map[string]string{
		"empty":                "",
		"no trailing newline":  "User content",
		"one trailing newline": "User content\n",
		"several newlines":     "User content\n\n\n",
		"leading newline":      "\nUser content\n",
	}
	for name, original := range testCases {
		t.Run(name, func(t *testing.T) {
			project := t.TempDir()
			source := filepath.Join(t.TempDir(), "agents.md")
			if err := os.WriteFile(source, []byte("Managed content\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(project, "AGENTS.md")
			if err := os.WriteFile(destination, []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}
			resolved := artifactFixture("markdown-section", source, "AGENTS.md")
			installed, err := InstallArtifacts(resolved, []Target{{Name: "codex"}}, project, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := InstallArtifacts(resolved, []Target{{Name: "codex"}}, project, installed, false); err != nil {
				t.Fatal(err)
			}
			if err := RemoveArtifacts(installed, project, false); err != nil {
				t.Fatal(err)
			}
			if content := readTestFile(t, destination); content != original {
				t.Fatalf("removed Markdown = %q, want original %q", content, original)
			}
		})
	}
}

func TestMarkdownArtifactRemovalSupportsLegacyLock(t *testing.T) {
	project := t.TempDir()
	source := filepath.Join(t.TempDir(), "agents.md")
	if err := os.WriteFile(source, []byte("Managed content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(project, "AGENTS.md")
	original := "User content\n"
	if err := os.WriteFile(destination, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved := artifactFixture("markdown-section", source, "AGENTS.md")
	installed, err := InstallArtifacts(resolved, []Target{{Name: "codex"}}, project, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	installed[0].MarkdownSeparator = ""
	if err := RemoveArtifacts(installed, project, false); err != nil {
		t.Fatal(err)
	}
	if content := readTestFile(t, destination); content != original {
		t.Fatalf("removed Markdown = %q, want original %q", content, original)
	}
}

func TestJSONArtifactMergeUpdateAndRemovePreservesUnrelatedEntries(t *testing.T) {
	project := t.TempDir()
	source := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(source, []byte(`{"hooks":{"PreToolUse":[{"command":"graphify hook-check"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(project, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte(`{
  "user": {"enabled": true},
  "hooks": {"PreToolUse": [{"matcher": "UserTool", "hooks": [{"command": "user-hook"}]}]}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved := artifactFixture("json-merge", source, ".codex/hooks.json")
	targets := []Target{{Name: "codex"}}
	installed, err := InstallArtifacts(resolved, targets, project, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONPath(t, destination, "user")
	assertJSONPath(t, destination, "hooks")
	if !strings.Contains(readTestFile(t, destination), "user-hook") {
		t.Fatalf("unrelated hook missing after merge:\n%s", readTestFile(t, destination))
	}

	if err := os.WriteFile(source, []byte(`{"hooks":{"PreToolUse":[{"command":"graphify hook-check --strict"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	updated, err := InstallArtifacts(resolved, targets, project, installed, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readTestFile(t, destination), "--strict") {
		t.Fatalf("JSON update missing:\n%s", readTestFile(t, destination))
	}
	if err := RemoveArtifacts(updated, project, false); err != nil {
		t.Fatal(err)
	}
	assertJSONPath(t, destination, "user")
	if !strings.Contains(readTestFile(t, destination), "user-hook") {
		t.Fatalf("unrelated hook missing after removal:\n%s", readTestFile(t, destination))
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(readTestFile(t, destination)), &result); err != nil {
		t.Fatal(err)
	}
	hooks := result["hooks"].(map[string]any)
	if strings.Contains(fmt.Sprint(hooks), "graphify") || strings.Contains(fmt.Sprint(hooks), "--strict") {
		t.Fatalf("managed hooks remain after removal: %#v", result)
	}
}

func TestCopiedArtifactRejectsLocalModification(t *testing.T) {
	project := t.TempDir()
	source := filepath.Join(t.TempDir(), "rule.mdc")
	if err := os.WriteFile(source, []byte("managed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved := artifactFixture("copy", source, ".cursor/rules/graphify.mdc")
	installed, err := InstallArtifacts(resolved, []Target{{Name: "cursor"}}, project, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed[0].Destination, []byte("local edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveArtifacts(installed, project, false); err == nil || !strings.Contains(err.Error(), "locally modified") {
		t.Fatalf("expected local modification error, got %v", err)
	}
}

func TestArtifactUpdateRemovesRetiredEntries(t *testing.T) {
	project := t.TempDir()
	source := filepath.Join(t.TempDir(), "agents.md")
	if err := os.WriteFile(source, []byte("Managed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved := artifactFixture("markdown-section", source, "AGENTS.md")
	targets := []Target{{Name: "codex"}}
	installed, err := InstallArtifacts(resolved, targets, project, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	resolved.Artifacts["codex"] = []ResolvedArtifact{{
		ArtifactEntry: state.ArtifactEntry{
			ID: "replacement", Source: filepath.Base(source),
			Destination: "GRAPHIFY.md", Mode: state.ArtifactModeMarkdownSection,
		},
		SourcePath: source,
	}}
	updated, err := InstallArtifacts(resolved, targets, project, installed, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 || updated[0].ID != "replacement" {
		t.Fatalf("updated artifacts = %+v", updated)
	}
	if _, err := os.Stat(filepath.Join(project, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("retired artifact remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, "GRAPHIFY.md")); err != nil {
		t.Fatalf("replacement artifact missing: %v", err)
	}
}

func TestAllArtifactsInstallOnceAcrossTargets(t *testing.T) {
	project := t.TempDir()
	source := filepath.Join(t.TempDir(), "post-commit")
	if err := os.WriteFile(source, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved := ResolvedSkill{
		Name: "graphify",
		Artifacts: map[string][]ResolvedArtifact{
			"all": {{
				ArtifactEntry: state.ArtifactEntry{
					ID:          "git-post-commit",
					Source:      filepath.Base(source),
					Destination: ".git/hooks/post-commit",
					Mode:        state.ArtifactModeCopy,
					Executable:  true,
				},
				SourcePath: source,
			}},
		},
	}
	targets := []Target{{Name: "codex"}, {Name: "copilot"}}
	installed, err := InstallArtifacts(resolved, targets, project, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(installed) != 1 || installed[0].Target != "all" {
		t.Fatalf("installed artifacts = %+v, want one target=all entry", installed)
	}
	info, err := os.Stat(installed[0].Destination)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		t.Fatalf("shared hook mode = %v, want executable", info.Mode())
	}
	updated, err := InstallArtifacts(resolved, targets, project, installed, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 || updated[0].Target != "all" {
		t.Fatalf("updated artifacts = %+v, want one target=all entry", updated)
	}
	if err := RemoveArtifacts(updated, project, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(installed[0].Destination); !os.IsNotExist(err) {
		t.Fatalf("shared hook remains after removal: %v", err)
	}
}

func artifactFixture(mode, source, destination string) ResolvedSkill {
	return ResolvedSkill{
		Name: "graphify",
		Artifacts: map[string][]ResolvedArtifact{
			"codex": {{
				ArtifactEntry: state.ArtifactEntry{
					ID: "guidance", Source: filepath.Base(source), Destination: destination, Mode: mode,
				},
				SourcePath: source,
			}},
			"cursor": {{
				ArtifactEntry: state.ArtifactEntry{
					ID: "guidance", Source: filepath.Base(source), Destination: destination, Mode: mode,
				},
				SourcePath: source,
			}},
		},
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func assertJSONPath(t *testing.T, path, key string) {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(readTestFile(t, path)), &value); err != nil {
		t.Fatal(err)
	}
	if _, exists := value[key]; !exists {
		t.Fatalf("%s missing from %s: %#v", key, path, value)
	}
}
