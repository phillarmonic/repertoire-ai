package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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
	var overrideFlags []string
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
	// Validate --override syntax eagerly so a malformed pair fails even for
	// commands that never materialize a catalog (for example `list`).
	command.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		for _, pair := range overrideFlags {
			name, path, ok := strings.Cut(pair, "=")
			if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(path) == "" {
				return fmt.Errorf("invalid --override %q: expected name=path or source=path", pair)
			}
		}
		return nil
	}
	command.PersistentFlags().BoolVar(&globalScope, "global", false, "use user-global state (default)")
	command.PersistentFlags().BoolVar(&projectScope, "project", false, "use the current Git project")
	command.PersistentFlags().BoolVar(&force, "force", false, "replace protected managed state")
	command.PersistentFlags().StringArrayVar(&overrideFlags, "override", nil, "resolve a catalog from a local path (name=path or source=path; repeatable)")
	command.Flags().BoolVar(&selfUpdate, "self-update", false, "update Repertoire to the latest stable release")
	command.AddCommand(newCatalogCommand(&globalScope, &projectScope, &force, &overrideFlags))
	command.AddCommand(newCompletionCommand())
	command.AddCommand(newDoctorCommand(&globalScope, &projectScope, &force, &overrideFlags))
	command.AddCommand(newStubCommand(&globalScope, &projectScope))
	for _, child := range newBootstrapCommands(&globalScope, &projectScope, &force, &overrideFlags) {
		command.AddCommand(child)
	}
	for _, child := range newSkillCommands(&globalScope, &projectScope, &force, &overrideFlags) {
		command.AddCommand(child)
	}

	return command
}

func newCatalogCommand(globalScope, projectScope, force *bool, overrideFlags *[]string) *cobra.Command {
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
			manager, err := newCatalogManager("", *overrideFlags)
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
	add.ValidArgsFunction = completeCatalogSources(globalScope, projectScope)

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
			for skill, declaration := range manifest.Skills {
				if declaration.Catalog == args[0] && !*force {
					return fmt.Errorf("catalog %q is required by bootstrap skill %q; use --force to remove it", args[0], skill)
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
			manager, err := newCatalogManager("", *overrideFlags)
			if err != nil {
				return err
			}
			for _, source := range catalog.Sources(manifest) {
				marker := ""
				if source.Builtin {
					marker = " (built-in)"
				}
				if path, overridden := manager.OverrideFor(source); overridden {
					marker += " (overridden -> " + path + ")"
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
			manager, err := newCatalogManager("", *overrideFlags)
			if err != nil {
				return err
			}
			sources, err := refreshCatalogs(manager, manifest, optionalArg(args))
			if err != nil {
				return err
			}
			for _, resolved := range sources {
				_, _ = fmt.Fprintf(command.OutOrStdout(), "%s\t%s\n", resolved.Name, resolved.Commit)
			}
			return nil
		},
	}
	update.ValidArgsFunction = completeCatalogs(globalScope, projectScope, false)
	catalogCommand.AddCommand(add, remove, list, update)
	return catalogCommand
}

func optionalArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// refreshCatalogs materializes every visible catalog, or the single catalog
// named by name. Bulk refreshes tolerate unreachable catalogs (reporting them
// to warningOutput) so one dead source cannot brick updates served by the
// others; an explicitly named catalog still fails hard.
func refreshCatalogs(manager *catalog.Manager, manifest state.Manifest, name string) ([]catalog.Materialized, error) {
	return refreshCatalogsWarn(manager, manifest, name, nil)
}

func refreshCatalogsWarn(manager *catalog.Manager, manifest state.Manifest, name string, warningOutput io.Writer) ([]catalog.Materialized, error) {
	var resolved []catalog.Materialized
	var refreshErrs []error
	for _, source := range catalog.Sources(manifest) {
		if name != "" && source.Name != name {
			continue
		}
		materialized, err := manager.Materialize(source, true)
		if err != nil {
			if name != "" {
				return nil, err
			}
			refreshErrs = append(refreshErrs, err)
			if warningOutput != nil {
				_, _ = fmt.Fprintf(warningOutput, "warning: skipped catalog %s: %v\n", source.Name, err)
			}
			continue
		}
		resolved = append(resolved, materialized)
	}
	if name != "" && len(resolved) == 0 {
		return nil, fmt.Errorf("catalog %q is not visible", name)
	}
	if len(resolved) == 0 && len(refreshErrs) > 0 {
		return nil, refreshErrs[0]
	}
	return resolved, nil
}

// newCatalogManager builds a catalog manager that honors local overrides from
// the REPERTOIRE_OVERRIDES environment variable plus the --override flags
// (flags win over environment values).
func newCatalogManager(cacheRoot string, overrideFlags []string) (*catalog.Manager, error) {
	manager, err := catalog.NewManager(cacheRoot)
	if err != nil {
		return nil, err
	}
	overrides, err := catalog.OverridesFromEnv()
	if err != nil {
		return nil, err
	}
	for _, pair := range overrideFlags {
		name, path, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("invalid --override %q: expected name=path or source=path", pair)
		}
		overrides[strings.TrimSpace(name)] = strings.TrimSpace(path)
	}
	manager.Overrides = overrides
	return manager, nil
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
