package state

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteFileAtomic(path string, content []byte, mode os.FileMode) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	temporary, err := os.CreateTemp(parent, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("create temporary state: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set state permissions: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary state: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}

func SaveManifest(path string, manifest Manifest) error {
	content, err := manifest.Marshal()
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, content, 0o644)
}

func SaveBootstrapManifest(path string, manifest BootstrapManifest) error {
	content, err := manifest.Marshal()
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, content, 0o644)
}

func SaveLock(path string, lock Lock) error {
	content, err := lock.Marshal()
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, content, 0o644)
}
