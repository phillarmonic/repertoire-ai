package catalog

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestSourcesCarryManifestDirectory(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "repertoire.yaml")
	body := "schema: 1\ncatalogs:\n  company:\n    source: https://example.invalid/skills.git\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := state.LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Directory != directory {
		t.Fatalf("manifest directory = %q, want %q", manifest.Directory, directory)
	}
	var company Source
	for _, source := range Sources(manifest) {
		if source.Name == "company" {
			company = source
		}
	}
	if company.TrustRoot != directory {
		t.Fatalf("trust root = %q, want %q", company.TrustRoot, directory)
	}
}

func TestMaterializeWithoutTrustSkipsGPG(t *testing.T) {
	repository := initCatalogRepo(t, false, "")
	hideGPG(t)

	manager, err := NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Materialize(Source{
		Name: "company", Registration: state.CatalogRegistration{Source: repository},
	}, false); err != nil {
		t.Fatal(err)
	}

	manager.Overrides = map[string]string{"company": repository}
	if _, err := manager.Materialize(Source{
		Name: "company",
		Registration: state.CatalogRegistration{
			Source: "https://example.invalid/skills.git",
		},
	}, false); err != nil {
		t.Fatalf("override without trust: %v", err)
	}
}

func TestMaterializeAcceptsCommitSignedByDeclaredKey(t *testing.T) {
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg is not installed")
	}
	userHome := t.TempDir()
	t.Setenv("GNUPGHOME", userHome)
	fingerprint := generateSigningKey(t, userHome)
	repository := initCatalogRepo(t, true, userHome)
	commit := gitCommit(t, repository)
	manifestDir := t.TempDir()
	writeKey(t, userHome, filepath.Join(manifestDir, "keys", "company.asc"))
	deleteKeys(t, userHome, fingerprint)

	manager, err := NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	source := trustedSource("company", repository, manifestDir, "keys/company.asc", fingerprint)
	materialized, err := manager.Materialize(source, false)
	if err != nil {
		t.Fatal(err)
	}
	if materialized.Commit != commit {
		t.Fatalf("commit = %s, want %s", materialized.Commit, commit)
	}
	if materialized.CommitFingerprint != fingerprint {
		t.Fatalf("commit fingerprint = %s, want %s", materialized.CommitFingerprint, fingerprint)
	}
	if listed := listPublicKeys(t, userHome); strings.Contains(listed, fingerprint) {
		t.Fatalf("declared key was imported into GNUPGHOME:\n%s", listed)
	}

	manager.Overrides = map[string]string{"company": repository}
	overridden := source
	overridden.Registration.Source = "https://example.invalid/skills.git"
	if _, err := manager.Materialize(overridden, false); err != nil {
		t.Fatalf("trusted override: %v", err)
	}
}

func TestMaterializeRejectsUntrustedOrMissingSignatures(t *testing.T) {
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg is not installed")
	}
	userHome := t.TempDir()
	t.Setenv("GNUPGHOME", userHome)
	trustedFingerprint := generateSigningKey(t, userHome)
	otherHome := t.TempDir()
	otherFingerprint := generateSigningKey(t, otherHome)

	unsigned := initCatalogRepo(t, false, "")
	signedByOther := initCatalogRepo(t, true, otherHome)
	signedByTrusted := initCatalogRepo(t, true, userHome)
	manifestDir := t.TempDir()
	writeKey(t, userHome, filepath.Join(manifestDir, "keys", "company.asc"))

	manager, err := NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name        string
		repository  string
		fingerprint string
	}{
		{name: "unsigned", repository: unsigned, fingerprint: trustedFingerprint},
		{name: "other-key", repository: signedByOther, fingerprint: trustedFingerprint},
		{name: "fingerprint-mismatch", repository: signedByTrusted, fingerprint: otherFingerprint},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			source := trustedSource("company", testCase.repository, manifestDir, "keys/company.asc", testCase.fingerprint)
			_, err := manager.Materialize(source, false)
			if err == nil {
				t.Fatal("expected materialize to fail")
			}
			message := err.Error()
			if !strings.Contains(message, "company") || !strings.Contains(message, gitCommit(t, testCase.repository)) {
				t.Fatalf("error = %q, want catalog name and commit", message)
			}
		})
	}
}

func TestMaterializeNamesCatalogWhenGPGIsMissing(t *testing.T) {
	repository := initCatalogRepo(t, false, "")
	commit := gitCommit(t, repository)
	hideGPG(t)
	manager, err := NewManager(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	source := trustedSource("company", repository, t.TempDir(), "keys/company.asc", "ABCDEF0123456789ABCDEF0123456789ABCDEF01")
	_, err = manager.Materialize(source, false)
	if err == nil {
		t.Fatal("expected materialize to fail")
	}
	message := err.Error()
	if !strings.Contains(message, "company") || !strings.Contains(message, commit) || !strings.Contains(message, "gpg") {
		t.Fatalf("error = %q, want catalog, commit, and gpg", message)
	}
}

func trustedSource(name, repository, manifestDir, keyPath, fingerprint string) Source {
	return Source{
		Name:      name,
		TrustRoot: manifestDir,
		Registration: state.CatalogRegistration{
			Source: repository,
			Trust: &state.CatalogTrust{Keys: []state.CatalogTrustKey{{
				Path:        keyPath,
				Fingerprint: fingerprint,
			}}},
		},
	}
}

func initCatalogRepo(t *testing.T, sign bool, keyHome string) string {
	t.Helper()
	repository := t.TempDir()
	run(t, repository, "init", "-q", "-b", "main")
	run(t, repository, "config", "user.email", "test@example.test")
	run(t, repository, "config", "user.name", "Test")
	writeCatalog(t, repository)
	run(t, repository, "add", ".")
	if !sign {
		run(t, repository, "commit", "-qm", "initial")
		return repository
	}
	command := exec.Command("git", "-C", repository, "commit", "-S", "-qm", "initial")
	command.Env = append(os.Environ(), "GNUPGHOME="+keyHome)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sign commit: %v: %s", err, output)
	}
	return repository
}

func generateSigningKey(t *testing.T, home string) string {
	t.Helper()
	if err := os.Chmod(home, 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("gpg",
		"--homedir", home,
		"--batch",
		"--pinentry-mode", "loopback",
		"--passphrase", "",
		"--quick-gen-key", "Repertoire Test <test@example.test>",
		"ed25519", "sign", "never",
	)
	command.Env = append(os.Environ(), "GNUPGHOME="+home)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate key: %v: %s", err, output)
	}
	list := exec.Command("gpg", "--homedir", home, "--batch", "--with-colons", "--list-keys")
	list.Env = append(os.Environ(), "GNUPGHOME="+home)
	output, err := list.CombinedOutput()
	if err != nil {
		t.Fatalf("list keys: %v: %s", err, output)
	}
	for line := range strings.SplitSeq(string(output), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) > 9 && fields[0] == "fpr" && fields[9] != "" {
			return fields[9]
		}
	}
	t.Fatalf("no fingerprint in %s", output)
	return ""
}

func writeKey(t *testing.T, home, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("gpg", "--homedir", home, "--armor", "--export")
	command.Env = append(os.Environ(), "GNUPGHOME="+home)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("export key: %v: %s", err, output)
	}
	if err := os.WriteFile(path, output, 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitCommit(t *testing.T, repository string) string {
	t.Helper()
	command := exec.Command("git", "-C", repository, "rev-parse", "HEAD")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse: %v: %s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func hideGPG(t *testing.T) {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

func deleteKeys(t *testing.T, home, fingerprint string) {
	t.Helper()
	for _, action := range []string{"--delete-secret-keys", "--delete-keys"} {
		command := exec.Command("gpg", "--homedir", home, "--batch", "--yes", action, fingerprint)
		command.Env = append(os.Environ(), "GNUPGHOME="+home)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v: %s", action, err, output)
		}
	}
}

func listPublicKeys(t *testing.T, home string) string {
	t.Helper()
	command := exec.Command("gpg", "--homedir", home, "--batch", "--list-keys")
	command.Env = append(os.Environ(), "GNUPGHOME="+home)
	output, _ := command.CombinedOutput()
	return string(output)
}
