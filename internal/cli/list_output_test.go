package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	installer "github.com/phillarmonic/repertoire-ai/internal/install"
)

func TestResolveSkillListFormat(t *testing.T) {
	var output bytes.Buffer

	format, err := resolveSkillListFormat("auto", &output, false)
	if err != nil || format != skillListFormatTSV {
		t.Fatalf("auto format = %q, %v; want tsv", format, err)
	}

	format, err = resolveSkillListFormat("auto", &output, true)
	if err != nil || format != skillListFormatTable {
		t.Fatalf("wide auto format = %q, %v; want table", format, err)
	}

	if _, err := resolveSkillListFormat("json", &output, true); err == nil {
		t.Fatal("wide JSON format unexpectedly succeeded")
	}
	if _, err := resolveSkillListFormat("yaml", &output, false); err == nil {
		t.Fatal("unknown format unexpectedly succeeded")
	}
}

func TestWriteInstalledSkillListTableSummarizesTargets(t *testing.T) {
	entries := []skillListEntry{
		{Name: "all-targets", Catalog: "local", Origin: "declared", Targets: installer.SupportedTargetNames()},
		{Name: "several-targets", Catalog: "local", Origin: "ad-hoc", Targets: []string{"claude", "codex", "cursor", "windsurf"}},
		{Name: "one-target", Catalog: "local", Origin: "declared", Targets: []string{"codex"}},
	}
	var output bytes.Buffer

	if err := writeSkillList(&output, entries, skillListFormatTable, false, false); err != nil {
		t.Fatal(err)
	}
	result := output.String()
	for _, expected := range []string{
		"SKILL", "CATALOG", "ORIGIN", "TARGETS",
		"all-targets", fmt.Sprintf("all (%d)", len(installer.SupportedTargetNames())),
		"several-targets", "4 targets",
		"one-target", "codex",
	} {
		if !strings.Contains(result, expected) {
			t.Fatalf("table missing %q:\n%s", expected, result)
		}
	}
	if strings.Contains(result, "claude,codex,cursor,windsurf") {
		t.Fatalf("compact table contains expanded targets:\n%s", result)
	}
}

func TestWriteInstalledSkillListWideTableShowsTargets(t *testing.T) {
	entries := []skillListEntry{{
		Name: "demo", Catalog: "local", Origin: "declared",
		Targets: []string{"claude", "codex", "cursor", "windsurf"},
	}}
	var output bytes.Buffer

	if err := writeSkillList(&output, entries, skillListFormatTable, true, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "claude,codex,cursor,windsurf") {
		t.Fatalf("wide table did not expand targets:\n%s", output.String())
	}
}

func TestWriteAvailableSkillListTableOmitsRedundantStatus(t *testing.T) {
	entries := []skillListEntry{
		{Name: "demo", Catalog: "local", Origin: "available"},
	}
	var output bytes.Buffer

	if err := writeSkillList(&output, entries, skillListFormatTable, false, true); err != nil {
		t.Fatal(err)
	}
	result := output.String()
	if !strings.Contains(result, "SKILL") || !strings.Contains(result, "CATALOG") {
		t.Fatalf("available table is missing headers:\n%s", result)
	}
	if strings.Contains(result, "ORIGIN") || strings.Contains(result, "available") {
		t.Fatalf("available table contains redundant status:\n%s", result)
	}
}

func TestWriteSkillListJSON(t *testing.T) {
	entries := []skillListEntry{{
		Name: "demo", Catalog: "local", Origin: "declared", Targets: []string{"codex"},
	}}
	var output bytes.Buffer

	if err := writeSkillList(&output, entries, skillListFormatJSON, false, false); err != nil {
		t.Fatal(err)
	}
	expected := `[{"name":"demo","catalog":"local","origin":"declared","targets":["codex"]}]`
	if strings.TrimSpace(output.String()) != expected {
		t.Fatalf("JSON output = %q, want %q", strings.TrimSpace(output.String()), expected)
	}
}
