package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	installer "github.com/phillarmonic/repertoire-ai/internal/install"
)

type skillListFormat string

const (
	skillListFormatAuto  skillListFormat = "auto"
	skillListFormatTable skillListFormat = "table"
	skillListFormatTSV   skillListFormat = "tsv"
	skillListFormatJSON  skillListFormat = "json"
)

type skillListEntry struct {
	Name    string   `json:"name"`
	Catalog string   `json:"catalog"`
	Origin  string   `json:"origin"`
	Targets []string `json:"targets,omitempty"`
}

func resolveSkillListFormat(value string, output io.Writer, wide bool) (skillListFormat, error) {
	format := skillListFormat(value)
	switch format {
	case skillListFormatAuto:
		if wide || isTerminal(output) {
			return skillListFormatTable, nil
		}
		return skillListFormatTSV, nil
	case skillListFormatTable, skillListFormatTSV, skillListFormatJSON:
		if wide && format != skillListFormatTable {
			return "", errors.New("--wide requires --format table")
		}
		return format, nil
	default:
		return "", fmt.Errorf("unsupported list format %q; use auto, table, tsv, or json", value)
	}
}

func isTerminal(output io.Writer) bool {
	file, ok := output.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func writeSkillList(output io.Writer, entries []skillListEntry, format skillListFormat, wide, available bool) error {
	switch format {
	case skillListFormatTable:
		return writeSkillListTable(output, entries, wide, available)
	case skillListFormatTSV:
		return writeSkillListTSV(output, entries, available)
	case skillListFormatJSON:
		return json.NewEncoder(output).Encode(entries)
	default:
		return fmt.Errorf("unsupported list format %q", format)
	}
}

func writeSkillListTable(output io.Writer, entries []skillListEntry, wide, available bool) error {
	if len(entries) == 0 {
		if available {
			_, err := fmt.Fprintln(output, "No available skills.")
			return err
		}
		_, err := fmt.Fprintln(output, "No installed skills.")
		return err
	}

	table := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	if available {
		if _, err := fmt.Fprintln(table, "SKILL\tCATALOG"); err != nil {
			return err
		}
		for _, entry := range entries {
			if _, err := fmt.Fprintf(table, "%s\t%s\n", entry.Name, entry.Catalog); err != nil {
				return err
			}
		}
		return table.Flush()
	}

	if _, err := fmt.Fprintln(table, "SKILL\tCATALOG\tORIGIN\tTARGETS"); err != nil {
		return err
	}
	for _, entry := range entries {
		targets := summarizeTargets(entry.Targets)
		if wide {
			targets = strings.Join(entry.Targets, ",")
		}
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", entry.Name, entry.Catalog, entry.Origin, targets); err != nil {
			return err
		}
	}
	return table.Flush()
}

func writeSkillListTSV(output io.Writer, entries []skillListEntry, available bool) error {
	for _, entry := range entries {
		if available {
			if _, err := fmt.Fprintf(output, "%s\t%s\tavailable\n", entry.Name, entry.Catalog); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(output, "%s\t%s\t%s\t%s\n", entry.Name, entry.Catalog, entry.Origin, strings.Join(entry.Targets, ",")); err != nil {
			return err
		}
	}
	return nil
}

func summarizeTargets(targets []string) string {
	if sameStringSet(targets, installer.SupportedTargetNames()) {
		return fmt.Sprintf("all (%d)", len(targets))
	}
	if len(targets) <= 3 {
		return strings.Join(targets, ",")
	}
	return fmt.Sprintf("%d targets", len(targets))
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	left = append([]string(nil), left...)
	right = append([]string(nil), right...)
	sort.Strings(left)
	sort.Strings(right)
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
