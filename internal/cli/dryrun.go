package cli

import (
	"fmt"
	"io"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
	"github.com/spf13/cobra"
)

const dryRunNoOpNote = "note: --dry-run has no effect on this command"

func noteDryRunNoOp(command *cobra.Command, dryRun bool) {
	if dryRun {
		_, _ = fmt.Fprintln(command.ErrOrStderr(), dryRunNoOpNote)
	}
}

func writeSkillCopyPlans(out io.Writer, plans []installer.SkillCopyPlan) error {
	var firstRefuse error
	for _, plan := range plans {
		switch plan.Action {
		case installer.SkillCopySkip:
			continue
		case installer.SkillCopyInstall:
			_, _ = fmt.Fprintf(out, "would install %s to %s\n", plan.Skill, plan.Path)
		case installer.SkillCopyReplace:
			_, _ = fmt.Fprintf(out, "would replace %s at %s\n", plan.Skill, plan.Path)
		case installer.SkillCopyRemove:
			_, _ = fmt.Fprintf(out, "would remove %s from %s\n", plan.Skill, plan.Path)
		case installer.SkillCopyRefuseModified:
			_, _ = fmt.Fprintf(out, "would refuse: locally modified copy at %s\n", plan.Path)
			if firstRefuse == nil {
				firstRefuse = installer.InstallConflictError(plan.Skill, plan.Target)
			}
		case installer.SkillCopyRefuseUnmanaged:
			_, _ = fmt.Fprintf(out, "would refuse: unmanaged destination %s\n", plan.Path)
			if firstRefuse == nil {
				firstRefuse = installer.InstallConflictError(plan.Skill, plan.Target)
			}
		}
	}
	return firstRefuse
}

func previewUncachedBuiltinClone(command *cobra.Command, overrideFlags []string) (bool, error) {
	manager, err := newCatalogManager("", overrideFlags)
	if err != nil {
		return false, err
	}
	builtin := catalog.Source{
		Name:         catalog.BuiltinName,
		Builtin:      true,
		Registration: state.CatalogRegistration{Source: catalog.BuiltinSource},
	}
	if manager.HasCachedClone(builtin) {
		return false, nil
	}
	_, _ = fmt.Fprintf(command.OutOrStdout(), "would clone %s\n", catalog.RedactSource(catalog.BuiltinSource))
	return true, nil
}

func dryRunCloneLine(manager *catalog.Manager, manifest state.Manifest, catalogName string) (string, bool) {
	for _, source := range catalog.Sources(manifest) {
		if catalogName != "" && source.Name != catalogName {
			continue
		}
		if manager.HasCachedClone(source) {
			continue
		}
		return "would clone " + catalog.RedactSource(catalog.NormalizeSource(source.Registration.Source)), true
	}
	return "", false
}

func previewCatalogAdd(
	command *cobra.Command,
	manifest *state.Manifest,
	manager *catalog.Manager,
	source catalog.Source,
	force bool,
) error {
	prepared, err := prepareCatalogRegistration(manifest, source, force)
	if err != nil {
		return err
	}
	out := command.OutOrStdout()
	if !manager.HasCachedClone(prepared) {
		_, _ = fmt.Fprintf(out, "would clone %s\n", catalog.RedactSource(prepared.Registration.Source))
		return nil
	}
	_, _ = fmt.Fprintf(out, "would register %s\n", prepared.Name)
	return nil
}

// previewCatalogUpdate prints the catalogs a refresh would touch. With an empty
// names slice it covers every visible catalog; otherwise it covers only the
// named ones and fails when a requested name is not visible.
func previewCatalogUpdate(command *cobra.Command, manager *catalog.Manager, manifest state.Manifest, names []string) error {
	out := command.OutOrStdout()
	requested := make(map[string]struct{}, len(names))
	for _, name := range names {
		requested[name] = struct{}{}
	}
	specific := len(requested) > 0
	found := make(map[string]struct{}, len(requested))
	for _, source := range catalog.Sources(manifest) {
		if specific {
			if _, ok := requested[source.Name]; !ok {
				continue
			}
			found[source.Name] = struct{}{}
		}
		if !manager.HasCachedClone(source) {
			_, _ = fmt.Fprintf(out, "would clone %s\n", catalog.RedactSource(catalog.NormalizeSource(source.Registration.Source)))
			continue
		}
		_, _ = fmt.Fprintf(out, "would refresh catalog %s\n", source.Name)
	}
	if specific {
		for _, name := range names {
			if _, ok := found[name]; !ok {
				return fmt.Errorf("catalog %q is not visible", name)
			}
		}
	}
	return nil
}

func writeRemoveCopyPlans(out io.Writer, plans []installer.SkillCopyPlan) error {
	var firstRefuse error
	for _, plan := range plans {
		switch plan.Action {
		case installer.SkillCopyRemove:
			_, _ = fmt.Fprintf(out, "would remove %s from %s\n", plan.Skill, plan.Path)
		case installer.SkillCopyRefuseModified:
			_, _ = fmt.Fprintf(out, "would refuse: locally modified copy at %s\n", plan.Path)
			if firstRefuse == nil {
				firstRefuse = fmt.Errorf("remove %q from %s: target is locally modified; use --force", plan.Skill, plan.Target)
			}
		}
	}
	return firstRefuse
}
