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
	locations := make([]string, 0, len(targets))
	for _, target := range targets {
		destination := filepath.Join(target.Root, installDirectoryName(resolved.Name))
		if previous != nil && previous.Digest == resolved.Digest {
			if digest, err := Digest(destination); err == nil && digest == resolved.Digest {
				locations = append(locations, destination)
				continue
			}
		}
		if err := installOne(resolved.Root, destination, previous, force); err != nil {
			return nil, fmt.Errorf("install %q to %s: %w", resolved.Name, target.Name, err)
		}
		locations = append(locations, destination)
	}
	return locations, nil
}

func installOne(source, destination string, previous *state.LockSkill, force bool) error {
	if _, err := os.Lstat(destination); err == nil {
		existingDigest, digestErr := Digest(destination)
		managedAndUnchanged := digestErr == nil && previous != nil && previous.Digest == existingDigest
		if !managedAndUnchanged && !force {
			return errors.New("target is unmanaged or locally modified; use --force to replace it")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
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
		if (err != nil || digest != previous.Digest) && !force {
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
