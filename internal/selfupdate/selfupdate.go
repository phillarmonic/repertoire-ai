// Package selfupdate updates the running Repertoire executable from GitHub releases.
package selfupdate

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	defaultRepository = "phillarmonic/repertoire-ai"
	checksumsAsset    = "checksums.txt"
)

// Options controls a self-update operation.
type Options struct {
	CurrentVersion string
	In             io.Reader
	Out            io.Writer
	Err            io.Writer

	// These fields make the updater testable and allow alternate GitHub API mirrors.
	HTTPClient     *http.Client
	APIBaseURL     string
	Repository     string
	ExecutablePath string
	GOOS           string
	GOARCH         string
	HomeDir        string
}

type release struct {
	TagName    string  `json:"tag_name"`
	Assets     []asset `json:"assets"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Run checks for and, with confirmation, installs the newest compatible stable release.
func Run(ctx context.Context, options Options) error {
	options = withDefaults(options)

	binaryName, err := releaseBinaryName(options.GOOS, options.GOARCH)
	if err != nil {
		return err
	}
	if options.GOOS != "darwin" && options.GOOS != "linux" && options.GOOS != "windows" {
		return fmt.Errorf("unsupported operating system: %s", options.GOOS)
	}

	_, _ = fmt.Fprintln(options.Out, "Checking for Repertoire updates...")
	releases, err := fetchReleases(ctx, options)
	if err != nil {
		return fmt.Errorf("check for updates: %w", err)
	}
	latest, err := selectLatestCompatibleRelease(releases, binaryName)
	if err != nil {
		return err
	}

	if versionsEqual(options.CurrentVersion, latest.TagName) {
		_, _ = fmt.Fprintf(options.Out, "Already running the latest version: %s\n", latest.TagName)
		return nil
	}

	_, _ = fmt.Fprintf(options.Out, "New version available: %s (current: %s)\n", latest.TagName, options.CurrentVersion)
	confirmed, err := confirm(options.In, options.Out)
	if err != nil {
		return err
	}
	if !confirmed {
		_, _ = fmt.Fprintln(options.Out, "Update cancelled.")
		return nil
	}

	executablePath := options.ExecutablePath
	if executablePath == "" {
		executablePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("locate current executable: %w", err)
		}
	}
	executablePath, err = filepath.Abs(executablePath)
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}

	tempPath, err := downloadAndVerify(ctx, options, latest, binaryName)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tempPath) }()

	backupPath, err := createBackup(executablePath, options.CurrentVersion, options.HomeDir, options.GOOS)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	_, _ = fmt.Fprintf(options.Out, "Backup created: %s\n", backupPath)

	if err := installBinary(tempPath, executablePath, options); err != nil {
		if restoreErr := restoreBackup(backupPath, executablePath); restoreErr != nil {
			return fmt.Errorf("install update: %w; restore backup: %w", err, restoreErr)
		}
		return fmt.Errorf("install update (backup restored): %w", err)
	}

	if err := verifyBinary(ctx, executablePath, latest.TagName); err != nil {
		if restoreErr := restoreBackup(backupPath, executablePath); restoreErr != nil {
			return fmt.Errorf("verify installed update: %w; restore backup: %w", err, restoreErr)
		}
		return fmt.Errorf("verify installed update (backup restored): %w", err)
	}

	_, _ = fmt.Fprintf(options.Out, "Successfully updated to %s\n", latest.TagName)
	_, _ = fmt.Fprintf(options.Out, "Backup retained at: %s\n", backupPath)
	return nil
}

func withDefaults(options Options) Options {
	if options.In == nil {
		options.In = os.Stdin
	}
	if options.Out == nil {
		options.Out = os.Stdout
	}
	if options.Err == nil {
		options.Err = os.Stderr
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}
	if options.APIBaseURL == "" {
		options.APIBaseURL = "https://api.github.com"
	}
	if options.Repository == "" {
		options.Repository = defaultRepository
	}
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}
	if options.GOARCH == "" {
		options.GOARCH = runtime.GOARCH
	}
	return options
}

func fetchReleases(ctx context.Context, options Options) ([]release, error) {
	url := strings.TrimRight(options.APIBaseURL, "/") + "/repos/" + options.Repository + "/releases"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "repertoire-self-update")

	response, err := options.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", response.StatusCode)
	}

	var releases []release
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&releases); err != nil {
		return nil, fmt.Errorf("parse GitHub response: %w", err)
	}
	return releases, nil
}

func selectLatestCompatibleRelease(releases []release, binaryName string) (release, error) {
	for _, candidate := range releases {
		if candidate.Draft || candidate.Prerelease {
			continue
		}
		if findAsset(candidate, binaryName) != nil && findAsset(candidate, checksumsAsset) != nil {
			return candidate, nil
		}
	}
	return release{}, fmt.Errorf("no stable GitHub release contains %q and %q", binaryName, checksumsAsset)
}

func releaseBinaryName(goos, goarch string) (string, error) {
	if goarch != "amd64" && goarch != "arm64" {
		return "", fmt.Errorf("unsupported architecture: %s", goarch)
	}
	name := fmt.Sprintf("repertoire-%s-%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name, nil
}

func findAsset(candidate release, name string) *asset {
	for index := range candidate.Assets {
		if candidate.Assets[index].Name == name {
			return &candidate.Assets[index]
		}
	}
	return nil
}

func versionsEqual(current, latest string) bool {
	current = strings.TrimSpace(current)
	if fields := strings.Fields(current); len(fields) > 0 {
		current = fields[0]
	}
	return strings.TrimPrefix(current, "v") == strings.TrimPrefix(strings.TrimSpace(latest), "v")
}

func confirm(in io.Reader, out io.Writer) (bool, error) {
	_, _ = fmt.Fprint(out, "Update now? (y/N): ")
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("read confirmation: %w", err)
		}
		return false, nil
	}
	response := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return response == "y" || response == "yes", nil
}

func downloadAndVerify(ctx context.Context, options Options, candidate release, binaryName string) (string, error) {
	binaryAsset := findAsset(candidate, binaryName)
	checksumAsset := findAsset(candidate, checksumsAsset)
	if binaryAsset == nil || checksumAsset == nil {
		return "", errors.New("compatible release assets disappeared")
	}

	_, _ = fmt.Fprintf(options.Out, "Downloading %s...\n", binaryName)
	binary, err := download(ctx, options.HTTPClient, binaryAsset.BrowserDownloadURL, 256<<20)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", binaryName, err)
	}
	checksums, err := download(ctx, options.HTTPClient, checksumAsset.BrowserDownloadURL, 1<<20)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", checksumsAsset, err)
	}

	expected, err := checksumForAsset(checksums, binaryName)
	if err != nil {
		return "", err
	}
	actual := sha256.Sum256(binary)
	if !strings.EqualFold(expected, hex.EncodeToString(actual[:])) {
		return "", fmt.Errorf("checksum verification failed for %s", binaryName)
	}

	tempFile, err := os.CreateTemp("", "repertoire-update-*")
	if err != nil {
		return "", fmt.Errorf("create temporary executable: %w", err)
	}
	tempPath := tempFile.Name()
	ok := false
	defer func() {
		_ = tempFile.Close()
		if !ok {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := tempFile.Write(binary); err != nil {
		return "", fmt.Errorf("write temporary executable: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		return "", fmt.Errorf("sync temporary executable: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("close temporary executable: %w", err)
	}
	// #nosec G302 -- the downloaded binary needs the owner execute bit
	if err := os.Chmod(tempPath, 0o700); err != nil {
		return "", fmt.Errorf("make temporary file executable: %w", err)
	}
	if err := verifyBinary(ctx, tempPath, candidate.TagName); err != nil {
		return "", fmt.Errorf("verify downloaded executable: %w", err)
	}
	ok = true
	return tempPath, nil
}

func download(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return body, nil
}

func checksumForAsset(contents []byte, binaryName string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(contents)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if name == binaryName {
			if len(fields[0]) != sha256.Size*2 {
				return "", fmt.Errorf("invalid checksum for %s", binaryName)
			}
			if _, err := hex.DecodeString(fields[0]); err != nil {
				return "", fmt.Errorf("invalid checksum for %s", binaryName)
			}
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}
	return "", fmt.Errorf("checksum not found for %s", binaryName)
}

func verifyBinary(ctx context.Context, binaryPath, expectedVersion string) error {
	command := exec.CommandContext(ctx, binaryPath, "--version")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s --version: %w", filepath.Base(binaryPath), err)
	}
	want := "repertoire version " + expectedVersion
	if !strings.HasPrefix(strings.TrimSpace(string(output)), want) {
		return fmt.Errorf("expected version output beginning %q, got %q", want, strings.TrimSpace(string(output)))
	}
	return nil
}

func createBackup(executablePath, version, homeDir, goos string) (string, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	backupDir := filepath.Join(homeDir, ".repertoire", "backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", err
	}
	version = safeVersion(version)
	backupName := fmt.Sprintf("repertoire_%s_backup_%s", version, time.Now().Format("20060102_150405.000000000"))
	if goos == "windows" {
		backupName += ".exe"
	}
	backupPath := filepath.Join(backupDir, backupName)
	if err := copyFile(executablePath, backupPath, 0o700); err != nil {
		return "", err
	}
	cleanupOldBackups(backupDir)
	return backupPath, nil
}

func safeVersion(version string) string {
	if fields := strings.Fields(strings.TrimSpace(version)); len(fields) > 0 {
		version = fields[0]
	}
	version = strings.TrimPrefix(version, "v")
	var builder strings.Builder
	for _, character := range version {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '.' || character == '-' || character == '_' {
			builder.WriteRune(character)
		}
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}

func cleanupOldBackups(backupDir string) {
	matches, err := filepath.Glob(filepath.Join(backupDir, "repertoire_*_backup_*"))
	if err != nil || len(matches) <= 5 {
		return
	}
	sort.Slice(matches, func(i, j int) bool {
		left, leftErr := os.Stat(matches[i])
		right, rightErr := os.Stat(matches[j])
		if leftErr != nil {
			return true
		}
		if rightErr != nil {
			return false
		}
		return left.ModTime().Before(right.ModTime())
	})
	for _, path := range matches[:len(matches)-5] {
		_ = os.Remove(path)
	}
}

func installBinary(sourcePath, targetPath string, options Options) error {
	if options.GOOS == "windows" {
		return installBinaryWindows(sourcePath, targetPath)
	}
	if options.GOOS != "darwin" && options.GOOS != "linux" {
		return fmt.Errorf("unsupported operating system: %s", options.GOOS)
	}

	targetMode := os.FileMode(0o755)
	if info, err := os.Stat(targetPath); err == nil {
		targetMode = info.Mode().Perm()
	}
	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".update-*")
	if err == nil {
		tempPath := tempFile.Name()
		if closeErr := tempFile.Close(); closeErr != nil {
			_ = os.Remove(tempPath)
			return closeErr
		}
		defer func() { _ = os.Remove(tempPath) }()
		if err := copyFile(sourcePath, tempPath, targetMode); err != nil {
			return err
		}
		if err := os.Rename(tempPath, targetPath); err == nil {
			return nil
		}
	}

	_, _ = fmt.Fprintln(options.Out, "Requesting elevated permissions...")
	// #nosec G204 -- fixed argv; only the mode is formatted in
	command := exec.Command("sudo", "install", "-m", fmt.Sprintf("%03o", targetMode), sourcePath, targetPath)
	command.Stdin = options.In
	command.Stdout = options.Out
	command.Stderr = options.Err
	return command.Run()
}

func installBinaryWindows(sourcePath, targetPath string) error {
	pendingPath := targetPath + ".new"
	_ = os.Remove(pendingPath)
	if err := copyFile(sourcePath, pendingPath, 0o755); err != nil {
		return err
	}
	defer func() { _ = os.Remove(pendingPath) }()
	if err := os.Rename(pendingPath, targetPath); err == nil {
		return nil
	}
	return errors.New("windows cannot replace the running executable; install the downloaded release after exiting Repertoire")
}

func restoreBackup(backupPath, targetPath string) error {
	mode := os.FileMode(0o755)
	if info, err := os.Stat(targetPath); err == nil {
		mode = info.Mode().Perm()
	}
	return copyFile(backupPath, targetPath, mode)
}

func copyFile(sourcePath, targetPath string, mode os.FileMode) error {
	// #nosec G304 -- paths come from the resolved update target
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()

	// #nosec G304 -- paths come from the resolved update target
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	copied := false
	defer func() {
		_ = target.Close()
		if !copied {
			_ = os.Remove(targetPath)
		}
	}()
	if _, err := io.Copy(target, source); err != nil {
		return err
	}
	if err := target.Sync(); err != nil {
		return err
	}
	if err := target.Close(); err != nil {
		return err
	}
	if err := os.Chmod(targetPath, mode); err != nil {
		return err
	}
	copied = true
	return nil
}
