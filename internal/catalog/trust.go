package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

// verifyTrustedCommit checks a materialized commit against the catalog's
// declared public keys. Catalogs with no trust block skip the check and do
// not invoke gpg. The returned fingerprint is the declared key that signed
// the commit. The keyring is a temporary directory, never the caller's
// GNUPGHOME, and it is removed before this function returns.
func verifyTrustedCommit(source Source, root, commit string) (string, error) {
	if source.Registration.Trust == nil {
		return "", nil
	}
	if strings.TrimSpace(commit) == "" {
		return "", trustErrorf(source, "commit", commit, "signed commit is required")
	}
	home, err := trustedKeyring(source, "commit", commit)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(home) }()
	if verifyErr := verifyCommit(home, root, commit); verifyErr != nil {
		return "", trustErrorf(source, "commit", commit, "%s", verifyErr.Error())
	}
	fingerprints, fingerprintErr := commitSigningFingerprints(home, root, commit)
	if fingerprintErr != nil {
		return "", trustErrorf(source, "commit", commit, "%s", fingerprintErr.Error())
	}
	matched, ok := declaredFingerprint(source, fingerprints...)
	if !ok {
		return "", trustErrorf(source, "commit", commit, "signature fingerprint does not match a declared key")
	}
	return matched, nil
}

// VerifyDetachedSignature checks that signaturePath is a detached signature
// over payload from one of the catalog's declared public keys. A registration
// with no trust block skips the check and returns an empty fingerprint. The
// keyring is temporary and is removed before this function returns.
func VerifyDetachedSignature(source Source, subject, signaturePath string, payload []byte) (string, error) {
	if source.Registration.Trust == nil {
		return "", nil
	}
	home, err := trustedKeyring(source, "skill", subject)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(home) }()
	fingerprint, verifyErr := verifyDetached(home, signaturePath, payload)
	if verifyErr != nil {
		return "", trustErrorf(source, "skill", subject, "%s", verifyErr.Error())
	}
	matched, ok := declaredFingerprint(source, fingerprint)
	if !ok {
		return "", trustErrorf(source, "skill", subject, "signature fingerprint does not match a declared key")
	}
	return matched, nil
}

// TrustFailure is a catalog trust check that did not pass. Kind is "commit"
// or "skill". Subject is the commit id or the skill directory.
type TrustFailure struct {
	Catalog string
	Kind    string
	Subject string
	Detail  string
}

func (e *TrustFailure) Error() string {
	return fmt.Sprintf("catalog %q %s %s: %s", e.Catalog, e.Kind, e.Subject, e.Detail)
}

func trustErrorf(source Source, kind, subject, format string, args ...any) error {
	return &TrustFailure{
		Catalog: source.Name,
		Kind:    kind,
		Subject: subject,
		Detail:  fmt.Sprintf(format, args...),
	}
}

func trustedKeyring(source Source, kind, subject string) (string, error) {
	trust := source.Registration.Trust
	if _, err := exec.LookPath("gpg"); err != nil {
		return "", trustErrorf(source, kind, subject, "gpg is not available")
	}
	if trust == nil || len(trust.Keys) == 0 {
		return "", trustErrorf(source, kind, subject, "trust block has no keys")
	}

	home, err := newIsolatedKeyHome()
	if err != nil {
		return "", trustErrorf(source, kind, subject, "create keyring: %s", err.Error())
	}
	for index, key := range trust.Keys {
		label := fmt.Sprintf("trust key %d", index+1)
		path, pathErr := trustKeyPath(source, key)
		if pathErr != nil {
			_ = os.RemoveAll(home)
			return "", trustErrorf(source, kind, subject, "%s: %s", label, pathErr.Error())
		}
		fingerprints, showErr := keyFingerprints(home, path)
		if showErr != nil {
			_ = os.RemoveAll(home)
			return "", trustErrorf(source, kind, subject, "%s: %s", label, showErr.Error())
		}
		declared := normalizeFingerprint(key.Fingerprint)
		if declared == "" || !slices.Contains(fingerprints, declared) {
			_ = os.RemoveAll(home)
			return "", trustErrorf(source, kind, subject, "%s fingerprint does not match %s", label, key.Path)
		}
		if importErr := importKey(home, path); importErr != nil {
			_ = os.RemoveAll(home)
			return "", trustErrorf(source, kind, subject, "%s: %s", label, importErr.Error())
		}
	}
	return home, nil
}

// newIsolatedKeyHome creates a keyring directory whose path stays short
// enough for the gpg-agent socket. macOS rejects Unix socket names longer
// than 104 bytes, and the default temp directory under /var/folders is
// already most of that budget.
func newIsolatedKeyHome() (string, error) {
	parent := os.TempDir()
	if runtime.GOOS != "windows" {
		parent = "/tmp"
	}
	return os.MkdirTemp(parent, "rgpg-")
}

func trustKeyPath(source Source, key state.CatalogTrustKey) (string, error) {
	cleaned := strings.TrimSpace(key.Path)
	if cleaned == "" {
		return "", errors.New("empty path")
	}
	if filepath.IsAbs(cleaned) {
		return cleaned, nil
	}
	if source.TrustRoot == "" {
		return "", fmt.Errorf("path %s is not anchored to a manifest directory", key.Path)
	}
	return filepath.Join(source.TrustRoot, cleaned), nil
}

func keyFingerprints(home, path string) ([]string, error) {
	output, err := runGPG(home, "--with-colons", "--show-keys", path)
	if err != nil {
		return nil, err
	}
	var fingerprints []string
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) > 9 && fields[0] == "fpr" && fields[9] != "" {
			fingerprints = append(fingerprints, normalizeFingerprint(fields[9]))
		}
	}
	if len(fingerprints) == 0 {
		return nil, fmt.Errorf("no fingerprint in %s", path)
	}
	return fingerprints, nil
}

func normalizeFingerprint(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, char := range value {
		if unicode.Is(unicode.ASCII_Hex_Digit, char) {
			builder.WriteRune(unicode.ToUpper(char))
		}
	}
	return builder.String()
}

func importKey(home, path string) error {
	_, err := runGPG(home, "--import", path)
	return err
}

func declaredFingerprint(source Source, candidates ...string) (string, bool) {
	trust := source.Registration.Trust
	if trust == nil {
		return "", false
	}
	declared := make([]string, 0, len(trust.Keys))
	for _, key := range trust.Keys {
		declared = append(declared, normalizeFingerprint(key.Fingerprint))
	}
	for _, candidate := range candidates {
		normalized := normalizeFingerprint(candidate)
		if normalized != "" && slices.Contains(declared, normalized) {
			return normalized, true
		}
	}
	return "", false
}

func verifyDetached(home, signaturePath string, payload []byte) (string, error) {
	info, err := os.Lstat(signaturePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("missing %s", filepath.Base(signaturePath))
		}
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", filepath.Base(signaturePath))
	}
	payloadPath := filepath.Join(home, "payload")
	if writeErr := os.WriteFile(payloadPath, payload, 0o600); writeErr != nil {
		return "", fmt.Errorf("write signature payload: %w", writeErr)
	}
	output, err := runGPG(home, "--status-fd", "1", "--verify", signaturePath, payloadPath)
	if err != nil {
		return "", err
	}
	fingerprint := verifiedFingerprint(output)
	if fingerprint == "" {
		return "", errors.New("verified signature did not report a fingerprint")
	}
	return fingerprint, nil
}

// verifiedFingerprint reads GnuPG VALIDSIG status. The primary-key
// fingerprint is preferred when GnuPG reports one, so a subkey signature
// still names the key declared in the trust block.
func verifiedFingerprint(output string) string {
	var signing, primary string
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "[GNUPG:]" || fields[1] != "VALIDSIG" {
			continue
		}
		signing = fields[2]
		if len(fields) >= 12 {
			primary = fields[11]
		}
	}
	if primary != "" {
		return primary
	}
	return signing
}

func verifyCommit(home, root, commit string) error {
	// #nosec G204 -- argv is the catalog checkout and resolved commit, not a shell
	command := exec.Command("git", gitArguments(root, []string{"-c", "gpg.program=gpg", "verify-commit", commit})...)
	command.Env = isolatedGPGEnvironment(home)
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = "signature verification failed"
		}
		return fmt.Errorf("%s", detail)
	}
	return nil
}

func commitSigningFingerprints(home, root, commit string) ([]string, error) {
	// %GP is the primary key when a subkey signed. %GF is the key that signed.
	// #nosec G204 -- argv is the catalog checkout and resolved commit, not a shell
	command := exec.Command("git", gitArguments(root, []string{
		"-c", "gpg.program=gpg", "log", "-1", "--format=%GP%n%GF", commit,
	})...)
	command.Env = isolatedGPGEnvironment(home)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("%s", detail)
	}
	var fingerprints []string
	for line := range strings.SplitSeq(stdout.String(), "\n") {
		normalized := normalizeFingerprint(line)
		if normalized != "" {
			fingerprints = append(fingerprints, normalized)
		}
	}
	if len(fingerprints) == 0 {
		return nil, errors.New("verified commit did not report a fingerprint")
	}
	return fingerprints, nil
}

func runGPG(home string, arguments ...string) (string, error) {
	argv := append([]string{
		"--homedir", home,
		"--batch",
		"--no-tty",
		"--no-auto-key-retrieve",
		"--keyserver-options", "no-auto-key-retrieve",
	}, arguments...)
	// #nosec G204 -- argv is the temporary keyring and a declared public key file
	command := exec.Command("gpg", argv...)
	command.Env = isolatedGPGEnvironment(home)
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("%s", detail)
	}
	return string(output), nil
}

func isolatedGPGEnvironment(home string) []string {
	filtered := make([]string, 0, len(gitEnvironment())+1)
	for _, pair := range gitEnvironment() {
		if strings.HasPrefix(pair, "GNUPGHOME=") || strings.HasPrefix(pair, "GPG_AGENT_INFO=") {
			continue
		}
		filtered = append(filtered, pair)
	}
	return append(filtered, "GNUPGHOME="+home)
}
