package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/selfupdate"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/spf13/cobra"
)

const (
	productDescription = "Install and manage portable AI agent skills"
	productBanner      = `                                          __            _
   _____  ___     ____   ___    _____  / /_  ____    (_)   _____  ___
  / ___/ / _ \   / __ \ / _ \  / ___/ / __/ / __ \  / /   / ___/ / _ \
 / /    /  __/  / /_/ //  __/ / /    / /_  / /_/ / / /   / /    /  __/
/_/     \___/  / .___/ \___/ /_/     \__/  \____/ /_/   /_/     \___/
              /_/`
)

// NewRootCommand builds the repertoire command tree.
func NewRootCommand(version string, stdout, stderr io.Writer) *cobra.Command {
	var globalScope bool
	var projectScope bool
	var force bool
	var selfUpdate bool
	command := &cobra.Command{
		Use:           "repertoire",
		Short:         productDescription,
		Long:          productBanner + "\n\n" + productDescription,
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version,
		Args:          cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if !selfUpdate {
				return command.Help()
			}
			return selfupdate.Run(command.Context(), selfupdate.Options{
				CurrentVersion: version,
				In:             command.InOrStdin(),
				Out:            command.OutOrStdout(),
				Err:            command.ErrOrStderr(),
			})
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetVersionTemplate("repertoire version {{.Version}}\n")
	command.PersistentFlags().BoolVar(&globalScope, "global", false, "use user-global state (default)")
	command.PersistentFlags().BoolVar(&projectScope, "project", false, "use the current Git project")
	command.PersistentFlags().BoolVar(&force, "force", false, "replace protected managed state")
	command.Flags().BoolVar(&selfUpdate, "self-update", false, "update Repertoire to the latest stable release")
	command.AddCommand(newCatalogCommand(&globalScope, &projectScope, &force))
	command.AddCommand(newCompletionCommand())
	for _, child := range newBootstrapCommands(&globalScope, &projectScope, &force) {
		command.AddCommand(child)
	}
	for _, child := range newSkillCommands(&globalScope, &projectScope, &force) {
		command.AddCommand(child)
	}

	return command
}

func newCatalogCommand(globalScope, projectScope, force *bool) *cobra.Command {
	catalogCommand := &cobra.Command{Use: "catalog", Short: "Manage skill catalogs"}
	var name, ref string
	add := &cobra.Command{
		Use:   "add <source>",
		Short: "Register a catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, manifest, err := loadScope(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			manager, err := catalog.NewManager("")
			if err != nil {
				return err
			}
			source := catalog.Source{Name: name, Registration: state.CatalogRegistration{Source: args[0], Ref: ref}}
			normalized := catalog.NormalizeSource(source.Registration.Source)
			if catalog.RedactSource(normalized) != normalized {
				return errors.New("catalog URLs must not contain embedded credentials; use system Git credentials")
			}
			source.Registration.Source = normalized
			if source.Name == "" {
				if catalog.IsLocal(args[0]) {
					resolved, err := manager.Materialize(catalog.Source{Name: "probe", Registration: source.Registration}, false)
					if err != nil {
						return err
					}
					source.Name = resolved.Manifest.Catalog.Name
				} else {
					return errors.New("--name is required for remote catalogs")
				}
			}
			if _, exists := manifest.Catalogs[source.Name]; exists && !*force {
				return fmt.Errorf("catalog %q already exists; use --force to replace it", source.Name)
			}
			if _, err := manager.Materialize(source, true); err != nil {
				return err
			}
			manifest.Catalogs[source.Name] = source.Registration
			if err := state.SaveManifest(scope.ManifestPath, manifest); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(command.OutOrStdout(), "registered %s\n", source.Name)
			return nil
		},
	}
	add.Flags().StringVar(&name, "name", "", "catalog name")
	add.Flags().StringVar(&ref, "ref", "", "branch, tag, or commit")

	remove := &cobra.Command{
		Use:   "remove <name>",
		Short: "Unregister a catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, manifest, err := loadScope(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			if _, exists := manifest.Catalogs[args[0]]; !exists {
				return fmt.Errorf("catalog %q is not registered", args[0])
			}
			for skill, requirement := range manifest.Requirements {
				if requirement.Catalog == args[0] && !*force {
					return fmt.Errorf("catalog %q is required by %q; use --force to remove it", args[0], skill)
				}
			}
			delete(manifest.Catalogs, args[0])
			return state.SaveManifest(scope.ManifestPath, manifest)
		},
	}
	remove.ValidArgsFunction = completeCatalogs(globalScope, projectScope, true)
	list := &cobra.Command{
		Use:   "list",
		Short: "List visible catalogs",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_, manifest, err := loadScope(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			for _, source := range catalog.Sources(manifest) {
				marker := ""
				if source.Builtin {
					marker = " (built-in)"
				}
				_, _ = fmt.Fprintf(command.OutOrStdout(), "%s\t%s%s\n", source.Name, catalog.RedactSource(source.Registration.Source), marker)
			}
			return nil
		},
	}
	update := &cobra.Command{
		Use:   "update [name]",
		Short: "Refresh catalog caches",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			_, manifest, err := loadScope(*globalScope, *projectScope)
			if err != nil {
				return err
			}
			manager, err := catalog.NewManager("")
			if err != nil {
				return err
			}
			for _, source := range catalog.Sources(manifest) {
				if len(args) == 1 && source.Name != args[0] {
					continue
				}
				resolved, err := manager.Materialize(source, true)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(command.OutOrStdout(), "%s\t%s\n", source.Name, resolved.Commit)
			}
			return nil
		},
	}
	update.ValidArgsFunction = completeCatalogs(globalScope, projectScope, false)
	catalogCommand.AddCommand(add, remove, list, update)
	return catalogCommand
}

func loadScope(globalScope, projectScope bool) (state.Scope, state.Manifest, error) {
	scope, err := state.ResolveScope(state.ScopeOptions{Global: globalScope, Project: projectScope})
	if err != nil {
		return state.Scope{}, state.Manifest{}, err
	}
	manifest, err := state.LoadManifest(scope.ManifestPath)
	return scope, manifest, err
}

// Execute runs the repertoire CLI and returns a process exit code.
func Execute(version string) int {
	command := NewRootCommand(version, os.Stdout, os.Stderr)

	if err := command.Execute(); err != nil {
		_, _ = fmt.Fprintln(command.ErrOrStderr(), "error:", err)
		return 1
	}
	return 0
}
