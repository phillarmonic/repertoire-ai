package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/phillarmonic/repertoire-ai/internal/doctor"
	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/spf13/cobra"
)

func newDoctorCommand(globalScope, projectScope, force *bool, overrideFlags *[]string) *cobra.Command {
	var fix, reset, yes bool
	var format string
	command := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose and repair managed installations",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if *globalScope || *projectScope {
				return errors.New("doctor inspects the current project and global state; --global and --project are not supported")
			}
			if reset {
				if err := runDoctorReset(command, yes, *force, overrideFlags); err != nil {
					return err
				}
			}
			env, err := doctorEnvironment(overrideFlags)
			if err != nil {
				return err
			}
			result, err := doctor.Run(env, fix)
			if err != nil {
				return err
			}
			if fix && hasDoctorCheck(result.Issues, "manifest-drift") {
				if runErr := runBootstrap(command, false, false, true, false, overrideFlags); runErr != nil {
					return runErr
				}
				env, err = doctorEnvironment(overrideFlags)
				if err != nil {
					return err
				}
				result, err = doctor.Run(env, false)
				if err != nil {
					return err
				}
			}
			if err := writeDoctorReport(command, env, result, format); err != nil {
				return err
			}
			if len(result.Issues) > 0 {
				if fix {
					return fmt.Errorf("doctor could not fix %d issue(s); see the report above", len(result.Issues))
				}
				return fmt.Errorf("doctor found %d issue(s); run repertoire doctor --fix to repair", len(result.Issues))
			}
			return nil
		},
	}
	command.Flags().BoolVar(&fix, "fix", false, "repair the issues doctor finds")
	command.Flags().BoolVar(&reset, "reset", false, "remove every managed artifact for the project and reinstall from repertoire.yaml")
	command.Flags().BoolVar(&yes, "yes", false, "skip the --reset confirmation prompt")
	command.Flags().StringVar(&format, "format", "auto", "output format: auto, table, tsv, or json")
	return command
}

// doctorEnvironment loads both scopes. Outside a Git worktree the project
// half stays empty and doctor runs its global checks only.
func doctorEnvironment(overrideFlags *[]string) (*doctor.Env, error) {
	manager, err := newCatalogManager("", *overrideFlags)
	if err != nil {
		return nil, err
	}
	globalScope, err := state.ResolveScope(state.ScopeOptions{Global: true})
	if err != nil {
		return nil, err
	}
	globalManifest, err := state.LoadManifest(globalScope.ManifestPath)
	if err != nil {
		return nil, err
	}
	globalLock, err := state.LoadLock(globalScope.LockPath)
	if err != nil {
		return nil, err
	}
	env := &doctor.Env{
		GlobalScope:    globalScope,
		GlobalManifest: globalManifest,
		GlobalLock:     globalLock,
		Manager:        manager,
	}
	projectScope, err := state.ResolveScope(state.ScopeOptions{Project: true})
	if err != nil {
		//nolint:nilerr // outside a Git worktree the project half stays empty; doctor runs global checks only
		return env, nil
	}
	projectManifest, err := state.LoadManifest(projectScope.ManifestPath)
	if err != nil {
		return nil, err
	}
	projectLock, err := state.LoadLock(projectScope.LockPath)
	if err != nil {
		return nil, err
	}
	env.ProjectRoot = projectScope.Root
	env.HasProject = true
	env.ProjectScope = projectScope
	env.ProjectManifest = projectManifest
	env.ProjectLock = projectLock
	return env, nil
}

func hasDoctorCheck(issues []doctor.Issue, check string) bool {
	for _, issue := range issues {
		if issue.Check == check {
			return true
		}
	}
	return false
}

func runDoctorReset(command *cobra.Command, yes, force bool, overrideFlags *[]string) error {
	projectScope, err := state.ResolveScope(state.ScopeOptions{Project: true})
	if err != nil {
		return err
	}
	if !yes {
		_, _ = fmt.Fprintf(
			command.OutOrStdout(),
			"Remove every managed artifact for %s and reinstall from repertoire.yaml? [y/N] ",
			projectScope.Root,
		)
		answer, readErr := bufio.NewReader(command.InOrStdin()).ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer == "" {
			return errors.New("--reset requires --yes when standard input is not interactive")
		}
		if answer != "y" && answer != "yes" {
			return errors.New("reset aborted")
		}
	}

	projectLock, err := state.LoadLock(projectScope.LockPath)
	if err != nil {
		return err
	}
	globalScope, err := state.ResolveScope(state.ScopeOptions{Global: true, Directory: projectScope.Root})
	if err != nil {
		return err
	}
	globalLock, err := state.LoadLock(globalScope.LockPath)
	if err != nil {
		return err
	}
	for name, entry := range projectLock.Skills {
		if err := installer.RemoveArtifacts(entry.Artifacts, projectScope.Root, true); err != nil {
			return fmt.Errorf("reset %q: %w", name, err)
		}
		targets, err := installer.ResolveTargets(projectScope, entry.Targets, "")
		if err == nil {
			if err := installer.Remove(name, targets, entry, true); err != nil {
				return fmt.Errorf("reset %q: %w", name, err)
			}
		}
		delete(projectLock.Skills, name)
	}
	if err := state.SaveLock(projectScope.LockPath, projectLock); err != nil {
		return err
	}
	for name, entry := range globalLock.Projects[projectScope.Root] {
		if err := installer.RemoveArtifacts(entry.Artifacts, projectScope.Root, true); err != nil {
			return fmt.Errorf("reset %q project pointers: %w", name, err)
		}
	}
	delete(globalLock.Projects, projectScope.Root)
	if err := state.SaveLock(globalScope.LockPath, globalLock); err != nil {
		return err
	}
	_ = force // removal above already uses force semantics
	return runBootstrap(command, false, false, true, false, overrideFlags)
}

func writeDoctorReport(command *cobra.Command, env *doctor.Env, result doctor.Result, format string) error {
	output := command.OutOrStdout()
	resolved, err := resolveDoctorFormat(format, output)
	if err != nil {
		return err
	}
	if resolved == "json" {
		return json.NewEncoder(output).Encode(result)
	}
	if !env.HasProject {
		_, _ = fmt.Fprintln(output, "note: not inside a Git worktree; skipped project checks")
	}
	if len(result.Issues) == 0 && len(result.Fixed) == 0 {
		_, err := fmt.Fprintln(output, "doctor: no issues found")
		return err
	}
	if len(result.Fixed) > 0 {
		if _, err := fmt.Fprintf(output, "fixed %d issue(s)\n", len(result.Fixed)); err != nil {
			return err
		}
		if resolved == "table" {
			if err := writeDoctorTable(output, result.Fixed); err != nil {
				return err
			}
		}
	}
	if len(result.Issues) > 0 {
		if resolved == "table" {
			return writeDoctorTable(output, result.Issues)
		}
		return writeDoctorTSV(output, result.Issues)
	}
	return nil
}

func resolveDoctorFormat(value string, output io.Writer) (string, error) {
	switch value {
	case "auto":
		if isTerminal(output) {
			return "table", nil
		}
		return "tsv", nil
	case "table", "tsv", "json":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported doctor format %q; use auto, table, tsv, or json", value)
	}
}

func writeDoctorTable(output io.Writer, issues []doctor.Issue) error {
	table := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "CHECK\tSCOPE\tSUBJECT\tDETAIL\tREMEDY"); err != nil {
		return err
	}
	for _, issue := range issues {
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
			issue.Check, issue.Scope, issue.Subject, issue.Detail, issue.Remedy); err != nil {
			return err
		}
	}
	return table.Flush()
}

func writeDoctorTSV(output io.Writer, issues []doctor.Issue) error {
	for _, issue := range issues {
		if _, err := fmt.Fprintf(output, "%s\t%s\t%s\t%s\t%s\n",
			issue.Check, issue.Scope, issue.Subject, issue.Detail, issue.Remedy); err != nil {
			return err
		}
	}
	return nil
}
