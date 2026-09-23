package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestDigestIgnoresSignatureFile(t *testing.T) {
	t.Parallel()
	skill := skillFixture(t, "demo")
	before, err := Digest(skill)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(skill, DigestSignatureFile), []byte("not a signature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := Digest(skill)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("digest changed from %s to %s when the signature file was added", before, after)
	}
}

func TestVerifyDigestSignature(t *testing.T) {
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg is not installed")
	}
	userHome := gpgHome(t)
	t.Setenv("GNUPGHOME", userHome)
	fingerprint := generateSigningKey(t, userHome)
	otherHome := gpgHome(t)
	otherFingerprint := generateSigningKey(t, otherHome)

	skill := skillFixture(t, "demo")
	digest, err := Digest(skill)
	if err != nil {
		t.Fatal(err)
	}
	signature := filepath.Join(skill, DigestSignatureFile)
	signDetached(t, userHome, digest, signature)

	manifestDir := t.TempDir()
	writePublicKey(t, userHome, filepath.Join(manifestDir, "keys", "company.asc"))
	writePublicKey(t, otherHome, filepath.Join(manifestDir, "keys", "other.asc"))
	deleteKeys(t, userHome, fingerprint)

	source := trustedSkillSource(manifestDir, "keys/company.asc", fingerprint)
	got, err := VerifyDigestSignature(skill, source)
	if err != nil {
		t.Fatal(err)
	}
	if got != fingerprint {
		t.Fatalf("fingerprint = %s, want %s", got, fingerprint)
	}
	if listed := listPublicKeys(t, userHome); strings.Contains(listed, fingerprint) {
		t.Fatalf("declared key was imported into GNUPGHOME:\n%s", listed)
	}

	loose := looseSkillDir(t, skill)
	if _, err = VerifyDigestSignature(loose, source); err != nil {
		t.Fatalf("loose catalog skill: %v", err)
	}

	second := trustedSkillSource(manifestDir, "keys/company.asc", fingerprint)
	second.Registration.Trust.Keys = append(second.Registration.Trust.Keys, state.CatalogTrustKey{
		Path:        "keys/other.asc",
		Fingerprint: otherFingerprint,
	})
	otherSkill := skillFixture(t, "other")
	otherDigest, err := Digest(otherSkill)
	if err != nil {
		t.Fatal(err)
	}
	signDetached(t, otherHome, otherDigest, filepath.Join(otherSkill, DigestSignatureFile))
	otherPrint, err := VerifyDigestSignature(otherSkill, second)
	if err != nil {
		t.Fatalf("second declared key: %v", err)
	}
	if otherPrint != otherFingerprint {
		t.Fatalf("fingerprint = %s, want %s", otherPrint, otherFingerprint)
	}

	if err = os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = VerifyDigestSignature(skill, source); err == nil {
		t.Fatal("expected tampered skill to fail")
	}

	signed := skillFixture(t, "signed")
	signedDigest, err := Digest(signed)
	if err != nil {
		t.Fatal(err)
	}
	keeper := gpgHome(t)
	keeperPrint := generateSigningKey(t, keeper)
	signDetached(t, keeper, signedDigest, filepath.Join(signed, DigestSignatureFile))
	keeperSource := trustedSkillSource(manifestDir, "keys/keeper.asc", keeperPrint)
	writePublicKey(t, keeper, filepath.Join(manifestDir, "keys", "keeper.asc"))
	if err = os.Remove(filepath.Join(signed, DigestSignatureFile)); err != nil {
		t.Fatal(err)
	}
	_, missingErr := VerifyDigestSignature(signed, keeperSource)
	if missingErr == nil || !strings.Contains(missingErr.Error(), DigestSignatureFile) {
		t.Fatalf("missing signature error = %v", missingErr)
	}

	untrusted := skillFixture(t, "untrusted")
	untrustedDigest, err := Digest(untrusted)
	if err != nil {
		t.Fatal(err)
	}
	signDetached(t, otherHome, untrustedDigest, filepath.Join(untrusted, DigestSignatureFile))
	_, untrustedErr := VerifyDigestSignature(untrusted, source)
	if untrustedErr == nil || !strings.Contains(untrustedErr.Error(), "company") {
		t.Fatalf("untrusted signature error = %v", untrustedErr)
	}

	unsigned := catalog.Source{Name: "plain", Registration: state.CatalogRegistration{}}
	plainFingerprint, err := VerifyDigestSignature(skillFixture(t, "plain"), unsigned)
	if err != nil {
		t.Fatalf("catalog without trust: %v", err)
	}
	if plainFingerprint != "" {
		t.Fatalf("catalog without trust returned fingerprint %s", plainFingerprint)
	}
}

func TestVerifyDigestSignatureNamesCatalogWhenGPGIsMissing(t *testing.T) {
	hideGPG(t)
	source := trustedSkillSource(t.TempDir(), "keys/company.asc", "ABCDEF0123456789ABCDEF0123456789ABCDEF01")
	skill := skillFixture(t, "demo")
	_, err := VerifyDigestSignature(skill, source)
	if err == nil {
		t.Fatal("expected verification to fail")
	}
	message := err.Error()
	if !strings.Contains(message, "company") || !strings.Contains(message, skill) || !strings.Contains(message, "gpg") {
		t.Fatalf("error = %q, want catalog, skill, and gpg", message)
	}
}

func trustedSkillSource(manifestDir, keyPath, fingerprint string) catalog.Source {
	return catalog.Source{
		Name:      "company",
		TrustRoot: manifestDir,
		Registration: state.CatalogRegistration{
			Source: "https://example.invalid/skills.git",
			Trust: &state.CatalogTrust{Keys: []state.CatalogTrustKey{{
				Path:        keyPath,
				Fingerprint: fingerprint,
			}}},
		},
	}
}

func looseSkillDir(t *testing.T, signedSkill string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "loose", "skills", "demo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", DigestSignatureFile} {
		content, err := os.ReadFile(filepath.Join(signedSkill, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func gpgHome(t *testing.T) string {
	t.Helper()
	// t.TempDir() includes the test name. On macOS that makes the gpg-agent
	// socket path longer than the 104-byte Unix socket limit.
	home, err := os.MkdirTemp("/tmp", "rgpg-")
	if err != nil {
		home, err = os.MkdirTemp("", "rgpg-")
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	return home
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
	return signingFingerprint(t, home)
}

func signingFingerprint(t *testing.T, home string) string {
	t.Helper()
	list := exec.Command("gpg", "--homedir", home, "--batch", "--with-colons", "--list-secret-keys")
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

func writePublicKey(t *testing.T, home, path string) {
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

func signDetached(t *testing.T, home, payload, output string) {
	t.Helper()
	command := exec.Command("gpg",
		"--homedir", home,
		"--batch",
		"--yes",
		"--pinentry-mode", "loopback",
		"--passphrase", "",
		"--detach-sign",
		"--armor",
		"--output", output,
	)
	command.Stdin = strings.NewReader(payload)
	command.Env = append(os.Environ(), "GNUPGHOME="+home)
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sign digest: %v: %s", err, combined)
	}
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
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list keys: %v: %s", err, output)
	}
	return string(output)
}

func hideGPG(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	t.Setenv("PATH", bin)
}
