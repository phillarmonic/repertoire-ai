package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DigestMatches reports whether location exists and whether its content digest
// is one of the expected lock digests. Missing paths are not an error.
func DigestMatches(location string, expected map[string]bool) (exists, matches bool, err error) {
	_, statErr := os.Lstat(location)
	if os.IsNotExist(statErr) {
		return false, false, nil
	}
	if statErr != nil {
		return false, false, statErr
	}
	digest, digestErr := Digest(location)
	if digestErr != nil {
		return true, false, digestErr
	}
	return true, expected[digest], nil
}

// DigestSignatureFile is a detached signature over Digest. It lives in the
// skill directory and is left out of the hash so the signature can be checked
// against the digest of the skill it sits beside.
const DigestSignatureFile = "REPERTOIRE.digest.asc"

func Digest(root string) (string, error) {
	hash := sha256.New()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if filepath.ToSlash(relative) == DigestSignatureFile {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk skill: %w", err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		relative, _ := filepath.Rel(root, path)
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(hash, filepath.ToSlash(relative)+"\x00"+info.Mode().String()+"\x00")
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := safeSymlinkTarget(root, path)
			if err != nil {
				return "", err
			}
			_, _ = io.WriteString(hash, target)
		} else if info.Mode().IsRegular() {
			// #nosec G304 -- path is walked from a caller-supplied root
			file, err := os.Open(path)
			if err != nil {
				return "", err
			}
			_, copyErr := io.Copy(hash, file)
			closeErr := file.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func DigestFile(path string) (string, error) {
	// #nosec G304 -- path is walked from a caller-supplied root
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func safeSymlinkTarget(root, path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	resolved := target
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(path), resolved)
	}
	resolved = filepath.Clean(resolved)
	root = filepath.Clean(root)
	if resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		return "", fmt.Errorf("symlink %s escapes the skill root", path)
	}
	return target, nil
}

func copyTree(source, destination string) error {
	sourceRoot, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer func() { _ = sourceRoot.Close() }()
	destinationRoot, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer func() { _ = destinationRoot.Close() }()

	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return destinationRoot.MkdirAll(relative, info.Mode().Perm())
		case info.Mode()&os.ModeSymlink != 0:
			if _, err := sourceRoot.Stat(relative); err != nil {
				return fmt.Errorf("unsafe symlink %s: %w", relative, err)
			}
			link, err := sourceRoot.Readlink(relative)
			if err != nil {
				return err
			}
			return destinationRoot.Symlink(link, relative)
		case info.Mode().IsRegular():
			input, err := sourceRoot.Open(relative)
			if err != nil {
				return err
			}
			output, err := destinationRoot.OpenFile(relative, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
			if err != nil {
				_ = input.Close()
				return err
			}
			_, copyErr := io.Copy(output, input)
			inputCloseErr := input.Close()
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if inputCloseErr != nil {
				return inputCloseErr
			}
			return closeErr
		default:
			return fmt.Errorf("unsupported file type at %s", path)
		}
	})
}
