package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func Skill(resolved ResolvedSkill, targets []Target, previous *state.LockSkill, force bool) ([]string, error) {
	locations, _, err := SkillWithDigests(resolved, targets, previous, force)
	return locations, err
}

func SkillWithDigests(
	resolved ResolvedSkill,
	targets []Target,
	previous *state.LockSkill,
	force bool,
) ([]string, map[string]string, error) {
	locations := make([]string, 0, len(targets))
	targetDigests := make(map[string]string, len(targets))
	installedDestinations := map[string]struct {
		target string
		digest string
	}{}
	for _, target := range targets {
		source := resolved.Root
		digest := resolved.Digest
		if variant, exists := resolved.Variants[target.Name]; exists {
			source = variant.Root
			digest = variant.Digest
		}
		destination := filepath.Join(target.Root, installDirectoryName(resolved.Name))
		if installed, exists := installedDestinations[destination]; exists {
			if installed.digest != digest {
				return nil, nil, fmt.Errorf(
					"targets %s and %s select different variants for the same location %s",
					installed.target, target.Name, destination,
				)
			}
			targetDigests[target.Name] = digest
			continue
		}
		installedDestinations[destination] = struct {
			target string
			digest string
		}{target: target.Name, digest: digest}
		previousDigest := lockedTargetDigest(previous, target.Name)
		if previousDigest == digest {
			if installedDigest, err := Digest(destination); err == nil && installedDigest == digest {
				locations = append(locations, destination)
				targetDigests[target.Name] = digest
				continue
			}
		}
		if err := installOne(source, destination, previousDigest, force); err != nil {
			return nil, nil, fmt.Errorf("install %q to %s: %w", resolved.Name, target.Name, err)
		}
		locations = append(locations, destination)
		targetDigests[target.Name] = digest
	}
	return locations, targetDigests, nil
}

func installOne(source, destination, previousDigest string, force bool) error {
	if _, err := os.Lstat(destination); err == nil {
		existingDigest, digestErr := Digest(destination)
		managedAndUnchanged := digestErr == nil && previousDigest != "" && previousDigest == existingDigest
		if !managedAndUnchanged && !force {
			return errors.New("target is unmanaged or locally modified; use --force to replace it")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	// #nosec G301 -- install directories follow the shared 0755 convention
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(destination), "."+filepath.Base(destination)+".staging-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	if err := copyTree(source, staging); err != nil {
		return err
	}

	backup := destination + ".repertoire-backup"
	_ = os.RemoveAll(backup)
	hadDestination := false
	if _, err := os.Lstat(destination); err == nil {
		if err := os.Rename(destination, backup); err != nil {
			return err
		}
		hadDestination = true
	}
	if err := os.Rename(staging, destination); err != nil {
		if hadDestination {
			_ = os.Rename(backup, destination)
		}
		return err
	}
	if hadDestination {
		if err := os.RemoveAll(backup); err != nil {
			return err
		}
	}
	return nil
}

func Remove(name string, targets []Target, previous state.LockSkill, force bool) error {
	for _, target := range targets {
		destination := filepath.Join(target.Root, installDirectoryName(name))
		if _, err := os.Lstat(destination); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		digest, err := Digest(destination)
		if (err != nil || digest != lockedTargetDigest(&previous, target.Name)) && !force {
			return fmt.Errorf("remove %q from %s: target is locally modified; use --force", name, target.Name)
		}
		root, err := os.OpenRoot(target.Root)
		if err != nil {
			return err
		}
		removeErr := root.RemoveAll(installDirectoryName(name))
		closeErr := root.Close()
		if removeErr != nil {
			return removeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func lockedTargetDigest(previous *state.LockSkill, target string) string {
	if previous == nil {
		return ""
	}
	if digest := previous.TargetDigests[target]; digest != "" {
		return digest
	}
	return previous.Digest
}

func installDirectoryName(name string) string {
	var builder strings.Builder
	previousHyphen := false
	for _, char := range name {
		if ('a' <= char && char <= 'z') || ('0' <= char && char <= '9') {
			builder.WriteRune(char)
			previousHyphen = false
			continue
		}
		if char == '-' {
			if builder.Len() > 0 && !previousHyphen {
				builder.WriteRune(char)
				previousHyphen = true
			}
			continue
		}
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(unicode.ToLower(char))
			previousHyphen = false
			continue
		}
		if builder.Len() > 0 && !previousHyphen {
			builder.WriteRune('-')
			previousHyphen = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "skill"
	}
	return result
}
