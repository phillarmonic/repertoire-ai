package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/spf13/cobra"
)

const initDocsURL = "https://phillarmonic.github.io/repertoire-ai/automation/"

func newInitCommand(globalScope, force, dryRun *bool, overrideFlags *[]string) *cobra.Command {
	return &cobra.Command{
		Use:               "init",
		Short:             "Write a starter project repertoire.yaml without installing",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, _ []string) error {
			return runInit(command, *globalScope, *force, *dryRun, overrideFlags)
		},
	}
}

func runInit(command *cobra.Command, globalFlag, force, dryRun bool, overrideFlags *[]string) error {
	if globalFlag {
		return errors.New("init writes a project manifest; --global is not supported")
	}
	projectScope, err := state.ResolveScope(state.ScopeOptions{Project: true})
	if err != nil {
		return err
	}
	manifest, err := state.LoadManifest(projectScope.ManifestPath)
	if err != nil {
		return err
	}
	if len(manifest.Skills) > 0 && !force {
		return fmt.Errorf("%s already declares skills; use --force to rewrite the skills section", filepath.Base(projectScope.ManifestPath))
	}
	if dryRun {
		cloned, cloneErr := previewUncachedBuiltinClone(command, *overrideFlags)
		if cloneErr != nil {
			return cloneErr
		}
		if cloned {
			_, _ = fmt.Fprintf(command.OutOrStdout(), "would create %s\n", filepath.Base(projectScope.ManifestPath))
			return nil
		}
	}
	starter, err := starterBootstrapSkills(*overrideFlags, dryRun)
	if err != nil {
		return err
	}
	manifest.Skills = starter
	_, statErr := os.Stat(projectScope.ManifestPath)
	manifestExists := statErr == nil
	if dryRun {
		out := command.OutOrStdout()
		if manifestExists {
			_, _ = fmt.Fprintf(out, "would update %s\n", filepath.Base(projectScope.ManifestPath))
		} else {
			_, _ = fmt.Fprintf(out, "would create %s\n", filepath.Base(projectScope.ManifestPath))
		}
		return nil
	}
	if err := state.SaveManifest(projectScope.ManifestPath, manifest); err != nil {
		return err
	}
	out := command.OutOrStdout()
	if manifestExists {
		_, _ = fmt.Fprintf(out, "updated %s\n", filepath.Base(projectScope.ManifestPath))
	} else {
		_, _ = fmt.Fprintf(out, "created %s\n", filepath.Base(projectScope.ManifestPath))
	}
	_, _ = fmt.Fprintf(out, "next: edit %s, then run `repertoire bootstrap`\n", filepath.Base(projectScope.ManifestPath))
	_, _ = fmt.Fprintf(out, "see %s\n", initDocsURL)
	return nil
}
