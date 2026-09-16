package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/spf13/cobra"
)

const (
	showCachePresent      = "present"
	showCacheAbsent       = "absent"
	showCacheUnregistered = "unregistered"
	showCopyIntact        = "intact"
	showCopyModified      = "modified"
	showCopyMissing       = "missing"
)

type skillShowView struct {
	Name         string            `json:"name"`
	Catalog      string            `json:"catalog"`
	Source       string            `json:"source"`
	Ref          string            `json:"ref"`
	Commit       string            `json:"commit"`
	Digest       string            `json:"digest"`
	Origin       string            `json:"origin"`
	CatalogCache string            `json:"catalog_cache"`
	Targets      []skillShowTarget `json:"targets"`
	Hooks        bool              `json:"hooks"`
	Loose        bool              `json:"loose"`
}

type skillShowTarget struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

func newShowCommand(globalScope, projectScope, dryRun *bool, overrideFlags *[]string, format *string) *cobra.Command {
	show := &cobra.Command{
		Use:   "show <skill>",
		Short: "Show where an installed skill came from and whether copies are intact",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			noteDryRunNoOp(command, *dryRun)
			resolvedFormat, err := resolveSkillListFormat(*format, command.OutOrStdout(), false)
			if err != nil {
				return err
			}
			scope, manifest, lock, err := loadInstallationState(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			manager, err := newCatalogManager("", *overrideFlags)
			if err != nil {
				return err
			}
			view, err := buildSkillShowView(scope, manifest, lock, args[0], manager)
			if err != nil {
				return err
			}
			return writeSkillShow(command.OutOrStdout(), view, resolvedFormat)
		},
	}
	show.Flags().StringVar(format, "format", "auto", "output format: auto, table, tsv, or json")
	show.ValidArgsFunction = completeInstalledSkills(globalScope, projectScope)
	_ = show.RegisterFlagCompletionFunc("format", completeListFormats)
	return show
}

func buildSkillShowView(
	scope state.Scope,
	manifest state.Manifest,
	lock state.Lock,
	name string,
	manager *catalog.Manager,
) (skillShowView, error) {
	entry, exists := lock.Skills[name]
	if !exists {
		return skillShowView{}, fmt.Errorf("skill %q is not managed in this scope", name)
	}
	targets, err := installer.ResolveTargets(scope, entry.Targets, "")
	if err != nil {
		return skillShowView{}, err
	}
	view := skillShowView{
		Name:         name,
		Catalog:      entry.Catalog,
		Source:       catalog.RedactSource(entry.Source),
		Ref:          entry.Ref,
		Commit:       entry.Commit,
		Digest:       entry.Digest,
		Origin:       entry.EffectiveOrigin(),
		Hooks:        entry.Hooks,
		CatalogCache: showCacheUnregistered,
		Targets:      make([]skillShowTarget, 0, len(targets)),
	}
	annotateShowCatalog(&view, manifest, manager)
	for _, target := range targets {
		path := installer.SkillInstallPath(name, target)
		status, err := showCopyStatus(path, installer.LockedTargetDigest(entry, target.Name))
		if err != nil {
			return skillShowView{}, err
		}
		view.Targets = append(view.Targets, skillShowTarget{
			Name: target.Name, Path: path, Status: status,
		})
	}
	return view, nil
}

func annotateShowCatalog(view *skillShowView, manifest state.Manifest, manager *catalog.Manager) {
	var source catalog.Source
	found := false
	for _, candidate := range catalog.Sources(manifest) {
		if candidate.Name == view.Catalog {
			source = candidate
			found = true
			break
		}
	}
	if !found {
		return
	}
	if manager == nil {
		view.CatalogCache = showCacheAbsent
		return
	}
	resolved, err := manager.InspectCached(source)
	if err != nil {
		view.CatalogCache = showCacheAbsent
		return
	}
	view.Loose = resolved.Loose
	view.CatalogCache = showCachePresent
}

func showCopyStatus(path, expectedDigest string) (string, error) {
	exists, matches, err := installer.DigestMatches(path, map[string]bool{expectedDigest: true})
	if err != nil {
		return "", err
	}
	switch {
	case !exists:
		return showCopyMissing, nil
	case matches:
		return showCopyIntact, nil
	default:
		return showCopyModified, nil
	}
}

func writeSkillShow(output io.Writer, view skillShowView, format skillListFormat) error {
	switch format {
	case skillListFormatTable:
		return writeSkillShowTable(output, view)
	case skillListFormatTSV:
		return writeSkillShowTSV(output, view)
	case skillListFormatJSON:
		return json.NewEncoder(output).Encode(view)
	default:
		return fmt.Errorf("unsupported list format %q", format)
	}
}

func writeSkillShowTable(output io.Writer, view skillShowView) error {
	table := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	rows := [][2]string{
		{"SKILL", view.Name},
		{"CATALOG", view.Catalog},
		{"SOURCE", view.Source},
		{"REF", view.Ref},
		{"COMMIT", view.Commit},
		{"DIGEST", view.Digest},
		{"ORIGIN", view.Origin},
		{"HOOKS", strconv.FormatBool(view.Hooks)},
		{"LOOSE", strconv.FormatBool(view.Loose)},
		{"CACHE", view.CatalogCache},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(table, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(table, "TARGET\tPATH\tSTATUS"); err != nil {
		return err
	}
	for _, target := range view.Targets {
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\n", target.Name, target.Path, target.Status); err != nil {
			return err
		}
	}
	return table.Flush()
}

func writeSkillShowTSV(output io.Writer, view skillShowView) error {
	rows := [][2]string{
		{"name", view.Name},
		{"catalog", view.Catalog},
		{"source", view.Source},
		{"ref", view.Ref},
		{"commit", view.Commit},
		{"digest", view.Digest},
		{"origin", view.Origin},
		{"hooks", strconv.FormatBool(view.Hooks)},
		{"loose", strconv.FormatBool(view.Loose)},
		{"catalog_cache", view.CatalogCache},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(output, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}
	for _, target := range view.Targets {
		if _, err := fmt.Fprintf(output, "target\t%s\t%s\t%s\n", target.Name, target.Path, target.Status); err != nil {
			return err
		}
	}
	return nil
}
