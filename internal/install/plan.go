package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

// SkillCopyAction is what installing or removing a skill would do at one path.
type SkillCopyAction string

const (
	SkillCopyInstall         SkillCopyAction = "install"
	SkillCopyReplace         SkillCopyAction = "replace"
	SkillCopySkip            SkillCopyAction = "skip"
	SkillCopyRemove          SkillCopyAction = "remove"
	SkillCopyRefuseModified  SkillCopyAction = "refuse-modified"
	SkillCopyRefuseUnmanaged SkillCopyAction = "refuse-unmanaged"
)

// SkillCopyPlan is one destination a mutating command would touch.
type SkillCopyPlan struct {
	Skill  string
	Target string
	Path   string
	Action SkillCopyAction
}

// PlanSkill classifies each install destination without writing.
func PlanSkill(resolved ResolvedSkill, targets []Target, previous *state.LockSkill, force bool) ([]SkillCopyPlan, error) {
	plans := make([]SkillCopyPlan, 0, len(targets))
	installedDestinations := map[string]struct {
		target string
		digest string
	}{}
	for _, target := range targets {
		sourceDigest := resolved.Digest
		if variant, exists := resolved.Variants[target.Name]; exists {
			sourceDigest = variant.Digest
		}
		destination := filepath.Join(target.Root, installDirectoryName(resolved.Name))
		if installed, exists := installedDestinations[destination]; exists {
			if installed.digest != sourceDigest {
				return nil, fmt.Errorf(
					"targets %s and %s select different variants for the same location %s",
					installed.target, target.Name, destination,
				)
			}
			continue
		}
		installedDestinations[destination] = struct {
			target string
			digest string
		}{target: target.Name, digest: sourceDigest}
		action, err := classifyInstallCopy(destination, lockedTargetDigest(previous, target.Name), sourceDigest, force)
		if err != nil {
			return nil, fmt.Errorf("install %q to %s: %w", resolved.Name, target.Name, err)
		}
		plans = append(plans, SkillCopyPlan{
			Skill: resolved.Name, Target: target.Name, Path: destination, Action: action,
		})
	}
	return plans, nil
}

// PlanRemove classifies each managed copy remove would touch, without deleting.
func PlanRemove(name string, targets []Target, previous state.LockSkill, force bool) ([]SkillCopyPlan, error) {
	plans := make([]SkillCopyPlan, 0, len(targets))
	for _, target := range targets {
		destination := filepath.Join(target.Root, installDirectoryName(name))
		if _, err := os.Lstat(destination); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		action := SkillCopyRemove
		digest, err := Digest(destination)
		if (err != nil || digest != lockedTargetDigest(&previous, target.Name)) && !force {
			action = SkillCopyRefuseModified
		}
		plans = append(plans, SkillCopyPlan{
			Skill: name, Target: target.Name, Path: destination, Action: action,
		})
	}
	return plans, nil
}

func classifyInstallCopy(destination, previousDigest, desiredDigest string, force bool) (SkillCopyAction, error) {
	_, err := os.Lstat(destination)
	if os.IsNotExist(err) {
		return SkillCopyInstall, nil
	}
	if err != nil {
		return "", err
	}
	existing, digestErr := Digest(destination)
	managedUnchanged := digestErr == nil && previousDigest != "" && previousDigest == existing
	if managedUnchanged && existing == desiredDigest {
		return SkillCopySkip, nil
	}
	if force {
		return SkillCopyReplace, nil
	}
	if managedUnchanged {
		return SkillCopyReplace, nil
	}
	if previousDigest != "" {
		return SkillCopyRefuseModified, nil
	}
	return SkillCopyRefuseUnmanaged, nil
}

// InstallConflictError is the error a real install returns when a destination
// is unmanaged or locally modified.
func InstallConflictError(name, target string) error {
	return fmt.Errorf("install %q to %s: %w", name, target, errors.New("target is unmanaged or locally modified; use --force to replace it"))
}
