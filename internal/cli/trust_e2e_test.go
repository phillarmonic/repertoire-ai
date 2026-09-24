package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

func TestTrustedCatalogCommandsLeaveLockUnchanged(t *testing.T) {
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg is not installed")
	}
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	t.Run("bad digest on a local catalog", func(t *testing.T) {
		fixture := newTrustCatalog(t, true, true)
		tamperTrustSkill(t, fixture.repo)
		assertCommandsLeaveLock(t, binary, fixture, fixture.repo, "digest signature")
	})
	t.Run("bad commit on a local catalog", func(t *testing.T) {
		fixture := newTrustCatalog(t, false, true)
		assertCommandsLeaveLock(t, binary, fixture, fixture.repo, "commit signature")
	})
	t.Run("override cannot skip a bad digest", func(t *testing.T) {
		fixture := newTrustCatalog(t, true, true)
		tamperTrustSkill(t, fixture.repo)
		assertOverrideRejected(t, binary, fixture, "digest signature")
	})
	t.Run("override cannot skip a bad commit", func(t *testing.T) {
		fixture := newTrustCatalog(t, false, true)
		assertOverrideRejected(t, binary, fixture, "commit signature")
	})
	t.Run("catalog without trust still installs", func(t *testing.T) {
		fixture := newTrustCatalog(t, false, false)
		project, _, environment := bootstrapEnvironment(t)
		writeTrustManifest(t, project, fixture, fixture.repo, false, false)
		runCommandWithEnv(t, project, environment, binary, "--project", "add", "demo", "--catalog", "company", "--target", "agents")
		lock := readFileForTest(t, filepath.Join(project, "repertoire.lock.json"))
		if !strings.Contains(lock, "demo") {
			t.Fatalf("lock did not record the unsigned catalog skill:\n%s", lock)
		}
		if strings.Contains(lock, "fingerprint") {
			t.Fatalf("unsigned catalog recorded fingerprints:\n%s", lock)
		}
		installed := filepath.Join(project, ".agents", "skills", "demo", "SKILL.md")
		if _, err := os.Stat(installed); err != nil {
			t.Fatalf("installed skill: %v", err)
		}
	})
}

func assertCommandsLeaveLock(t *testing.T, binary string, fixture trustCatalog, source, phrase string) {
	t.Helper()
	for _, command := range []string{"add", "install", "bootstrap"} {
		t.Run(command, func(t *testing.T) {
			project, _, environment := bootstrapEnvironment(t)
			writeTrustManifest(t, project, fixture, source, true, command == "bootstrap")
			beforeLock := readOptionalFile(t, filepath.Join(project, "repertoire.lock.json"))
			beforeManifest := readFileForTest(t, filepath.Join(project, "repertoire.yaml"))
			var output string
			if command == "bootstrap" {
				output = runCommandWithEnvError(t, project, environment, binary, "bootstrap")
			} else {
				output = runCommandWithEnvError(t, project, environment, binary, "--project", command, "demo", "--catalog", "company", "--target", "agents")
			}
			assertTrustFailure(t, output, fixture.commit, "demo", phrase)
			assertBytesUnchanged(t, filepath.Join(project, "repertoire.lock.json"), beforeLock)
			if after := readFileForTest(t, filepath.Join(project, "repertoire.yaml")); after != beforeManifest {
				t.Fatalf("%s changed the manifest", command)
			}
			if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "demo", "SKILL.md")); !os.IsNotExist(err) {
				t.Fatalf("%s wrote the skill: %v", command, err)
			}
		})
	}
	t.Run("update", func(t *testing.T) {
		good := newTrustCatalog(t, true, true)
		project, _, environment := bootstrapEnvironment(t)
		writeTrustManifest(t, project, good, good.repo, true, false)
		runCommandWithEnv(t, project, environment, binary, "--project", "add", "demo", "--catalog", "company", "--target", "agents")
		lockPath := filepath.Join(project, "repertoire.lock.json")
		installed := filepath.Join(project, ".agents", "skills", "demo", "SKILL.md")
		beforeLock := readFileForTest(t, lockPath)
		beforeSkill := readFileForTest(t, installed)
		if phrase == "digest signature" {
			tamperTrustSkill(t, good.repo)
		} else {
			commitUnsignedTrustChange(t, good.repo)
		}
		output := runCommandWithEnvError(t, project, environment, binary, "--project", "update", "demo", "--target", "agents")
		commit := good.commit
		if phrase == "commit signature" {
			commit = trustHead(t, good.repo)
		}
		assertTrustFailure(t, output, commit, "demo", phrase)
		if after := readFileForTest(t, lockPath); after != beforeLock {
			t.Fatalf("update changed the lock:\n%s", after)
		}
		if after := readFileForTest(t, installed); after != beforeSkill {
			t.Fatalf("update changed the installed skill:\n%s", after)
		}
	})
}

func assertOverrideRejected(t *testing.T, binary string, fixture trustCatalog, phrase string) {
	t.Helper()
	project, _, environment := bootstrapEnvironment(t)
	source := "https://example.invalid/company-skills.git"
	writeTrustManifest(t, project, fixture, source, true, false)
	environment = append(environment, "REPERTOIRE_OVERRIDES=company="+fixture.repo)
	output := runCommandWithEnvError(t, project, environment, binary, "--project", "install", "demo", "--catalog", "company", "--target", "agents")
	assertTrustFailure(t, output, fixture.commit, "demo", phrase)
	if _, err := os.Stat(filepath.Join(project, "repertoire.lock.json")); !os.IsNotExist(err) {
		t.Fatalf("override install wrote a lock: %v", err)
	}
}

func assertTrustFailure(t *testing.T, output, commit, skill, phrase string) {
	t.Helper()
	for _, want := range []string{"company", skill, commit, phrase} {
		if !strings.Contains(output, want) {
			t.Fatalf("error %q missing %q\n%s", phrase, want, output)
		}
	}
}

type trustCatalog struct {
	repo        string
	keyHome     string
	fingerprint string
	commit      string
}

func TestTrustedInstallRecordsFingerprints(t *testing.T) {
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg is not installed")
	}
	binary := testBinaryPath(t)
	moduleRoot := filepath.Clean(filepath.Join("..", ".."))
	runCommand(t, moduleRoot, "go", "build", "-o", binary, "./cmd/repertoire")

	t.Run("skill entry", func(t *testing.T) {
		fixture := newTrustCatalog(t, true, true)
		project, _, environment := bootstrapEnvironment(t)
		writeTrustManifest(t, project, fixture, fixture.repo, true, false)
		runCommandWithEnv(t, project, environment, binary, "--project", "add", "demo", "--catalog", "company", "--target", "agents")
		entry := loadTrustSkill(t, filepath.Join(project, "repertoire.lock.json"))
		assertTrustFingerprints(t, entry.CommitFingerprint, entry.DigestFingerprint, fixture.fingerprint)
	})

	t.Run("project artifact entry", func(t *testing.T) {
		fixture := newTrustCatalogWithInstruction(t)
		project, home, environment := bootstrapEnvironment(t)
		writeTrustManifest(t, project, fixture, fixture.repo, true, true)
		manifestPath := filepath.Join(project, "repertoire.yaml")
		manifest := strings.Replace(readFileForTest(t, manifestPath), "scope: project", "scope: global", 1)
		if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		runCommandWithEnv(t, project, environment, binary, "bootstrap")
		lock, err := state.LoadLock(filepath.Join(globalConfigRoot(home), "repertoire.lock.json"))
		if err != nil {
			t.Fatal(err)
		}
		entry := lock.Skills["demo"]
		assertTrustFingerprints(t, entry.CommitFingerprint, entry.DigestFingerprint, fixture.fingerprint)
		projectEntry, ok := lock.Projects[findLockedProject(t, lock, project)]["demo"]
		if !ok {
			t.Fatalf("project artifact entry missing:\n%#v", lock.Projects)
		}
		assertTrustFingerprints(t, projectEntry.CommitFingerprint, projectEntry.DigestFingerprint, fixture.fingerprint)
	})
}

func loadTrustSkill(t *testing.T, path string) state.LockSkill {
	t.Helper()
	lock, err := state.LoadLock(path)
	if err != nil {
		t.Fatal(err)
	}
	return lock.Skills["demo"]
}

func assertTrustFingerprints(t *testing.T, commitFingerprint, digestFingerprint, want string) {
	t.Helper()
	if !strings.EqualFold(commitFingerprint, want) || !strings.EqualFold(digestFingerprint, want) {
		t.Fatalf("fingerprints = %s %s, want %s", commitFingerprint, digestFingerprint, want)
	}
}

func newTrustCatalog(t *testing.T, signCommit, signDigest bool) trustCatalog {
	t.Helper()
	return makeTrustCatalog(t, signCommit, signDigest, false)
}

func newTrustCatalogWithInstruction(t *testing.T) trustCatalog {
	t.Helper()
	return makeTrustCatalog(t, true, true, true)
}

func makeTrustCatalog(t *testing.T, signCommit, signDigest, instruction bool) trustCatalog {
	t.Helper()
	keyHome := gpgHome(t)
	fingerprint := generateTrustKey(t, keyHome)
	repo := t.TempDir()
	runCommand(t, repo, "git", "init", "-q", "-b", "main")
	runCommand(t, repo, "git", "config", "user.email", "test@example.test")
	runCommand(t, repo, "git", "config", "user.name", "Test")
	skill := filepath.Join(repo, "skills", "demo")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: demo\ndescription: Test skill\n---\nv1\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "schema: 1\ncatalog:\n  name: company\n  skills:\n    demo:\n      path: skills/demo\n"
	if instruction {
		guidance := filepath.Join(repo, "project-files")
		if err := os.MkdirAll(guidance, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(guidance, "agents.md"), []byte("## Demo\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		manifest += "      instructions:\n        agents:\n          - id: guidance\n            source: project-files/agents.md\n            destination: AGENTS.md\n            mode: markdown-section\n"
	}
	if err := os.WriteFile(filepath.Join(repo, "repertoire.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if signDigest {
		digest, err := installer.Digest(skill)
		if err != nil {
			t.Fatal(err)
		}
		signTrustDetached(t, keyHome, digest, filepath.Join(skill, installer.DigestSignatureFile))
	}
	runCommand(t, repo, "git", "add", ".")
	if signCommit {
		signTrustCommit(t, repo, keyHome)
	} else {
		runCommand(t, repo, "git", "commit", "-qm", "initial")
	}
	return trustCatalog{repo: repo, keyHome: keyHome, fingerprint: fingerprint, commit: trustHead(t, repo)}
}

func writeTrustManifest(t *testing.T, project string, fixture trustCatalog, source string, withTrust, bootstrap bool) {
	t.Helper()
	var body strings.Builder
	body.WriteString("schema: 1\ncatalogs:\n  company:\n    source: '")
	body.WriteString(strings.ReplaceAll(source, "'", "''"))
	body.WriteString("'\n")
	if withTrust {
		exportTrustKey(t, fixture.keyHome, filepath.Join(project, "keys", "company.asc"))
		body.WriteString("    trust:\n      keys:\n        - path: keys/company.asc\n          fingerprint: ")
		body.WriteString(fixture.fingerprint)
		body.WriteString("\n")
	}
	if bootstrap {
		body.WriteString("skills:\n  demo:\n    catalog: company\n    scope: project\n    targets: [agents]\n")
	}
	if err := os.WriteFile(filepath.Join(project, "repertoire.yaml"), []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func tamperTrustSkill(t *testing.T, repo string) {
	t.Helper()
	body := "---\nname: demo\ndescription: Test skill\n---\nv2\n"
	if err := os.WriteFile(filepath.Join(repo, "skills", "demo", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitUnsignedTrustChange(t *testing.T, repo string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("unsigned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCommand(t, repo, "git", "add", "README.md")
	runCommand(t, repo, "git", "commit", "-qm", "unsigned")
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

func generateTrustKey(t *testing.T, home string) string {
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

func exportTrustKey(t *testing.T, home, path string) {
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

func signTrustDetached(t *testing.T, home, payload, output string) {
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

func signTrustCommit(t *testing.T, repo, home string) {
	t.Helper()
	command := exec.Command("git", "-C", repo, "commit", "-S", "-qm", "initial")
	command.Env = append(os.Environ(), "GNUPGHOME="+home)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sign commit: %v: %s", err, output)
	}
}

func trustHead(t *testing.T, repo string) string {
	t.Helper()
	command := exec.Command("git", "-C", repo, "rev-parse", "HEAD")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse: %v: %s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func readOptionalFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func assertBytesUnchanged(t *testing.T, path, before string) {
	t.Helper()
	after := readOptionalFile(t, path)
	if after != before {
		t.Fatalf("lock changed from %q to %q", before, after)
	}
}
