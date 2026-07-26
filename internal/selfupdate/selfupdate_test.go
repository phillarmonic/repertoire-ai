package selfupdate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseBinaryName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos    string
		goarch  string
		want    string
		wantErr string
	}{
		{goos: "darwin", goarch: "arm64", want: "repertoire-darwin-arm64"},
		{goos: "windows", goarch: "amd64", want: "repertoire-windows-amd64.exe"},
		{goos: "linux", goarch: "386", wantErr: "unsupported architecture"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.goos+"-"+test.goarch, func(t *testing.T) {
			t.Parallel()
			got, err := releaseBinaryName(test.goos, test.goarch)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("expected error containing %q, got %v", test.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("releaseBinaryName: %v", err)
			}
			if got != test.want {
				t.Fatalf("releaseBinaryName = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSelectLatestCompatibleRelease(t *testing.T) {
	t.Parallel()

	makeRelease := func(tag string, draft, prerelease bool, names ...string) release {
		candidate := release{TagName: tag, Draft: draft, Prerelease: prerelease}
		for _, name := range names {
			candidate.Assets = append(candidate.Assets, asset{Name: name, BrowserDownloadURL: "https://example.test/" + name})
		}
		return candidate
	}
	releases := []release{
		makeRelease("v4.0.0", false, false, "repertoire-linux-amd64"),
		makeRelease("v3.0.0", false, true, "repertoire-darwin-arm64", checksumsAsset),
		makeRelease("v2.0.0", false, false, "repertoire-darwin-arm64", checksumsAsset),
	}
	got, err := selectLatestCompatibleRelease(releases, "repertoire-darwin-arm64")
	if err != nil {
		t.Fatalf("selectLatestCompatibleRelease: %v", err)
	}
	if got.TagName != "v2.0.0" {
		t.Fatalf("selected %q, want v2.0.0", got.TagName)
	}
}

func TestChecksumForAsset(t *testing.T) {
	t.Parallel()

	checksum := strings.Repeat("a", 64)
	got, err := checksumForAsset([]byte(checksum+"  other\n"+checksum+" *repertoire-linux-amd64\n"), "repertoire-linux-amd64")
	if err != nil {
		t.Fatalf("checksumForAsset: %v", err)
	}
	if got != checksum {
		t.Fatalf("checksum = %q, want %q", got, checksum)
	}

	if _, err := checksumForAsset([]byte("bad  repertoire-linux-amd64\n"), "repertoire-linux-amd64"); err == nil {
		t.Fatal("expected invalid checksum error")
	}
}

func TestVersionsEqualIgnoresBuildMetadata(t *testing.T) {
	t.Parallel()

	if !versionsEqual("v1.2.3 (abcdef0, 2026-07-26 UTC)", "v1.2.3") {
		t.Fatal("expected release build metadata to be ignored")
	}
	if versionsEqual("dev", "v1.2.3") {
		t.Fatal("development version must not equal a release")
	}
}

func TestRunAlreadyCurrent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/phillarmonic/repertoire-ai/releases" {
			http.NotFound(writer, request)
			return
		}
		_, _ = fmt.Fprint(writer, `[{
			"tag_name":"v1.2.3",
			"assets":[
				{"name":"repertoire-linux-amd64","browser_download_url":"https://example.test/binary"},
				{"name":"checksums.txt","browser_download_url":"https://example.test/checksums"}
			]
		}]`)
	}))
	defer server.Close()

	var output strings.Builder
	err := Run(context.Background(), Options{
		CurrentVersion: "v1.2.3 (abcdef0, build date)",
		In:             strings.NewReader(""),
		Out:            &output,
		HTTPClient:     server.Client(),
		APIBaseURL:     server.URL,
		GOOS:           "linux",
		GOARCH:         "amd64",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(output.String(), "Already running the latest version: v1.2.3") {
		t.Fatalf("unexpected output: %s", output.String())
	}
}

func TestRunDownloadsVerifiesBacksUpAndInstalls(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot execute the shell fixture")
	}

	binaryName := "repertoire-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("release builds do not support this architecture")
	}
	newBinary := []byte("#!/bin/sh\necho 'repertoire version v2.0.0 (test build)'\n")
	sum := sha256.Sum256(newBinary)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/phillarmonic/repertoire-ai/releases":
			_, _ = fmt.Fprintf(writer, `[{
				"tag_name":"v2.0.0",
				"assets":[
					{"name":%q,"browser_download_url":%q},
					{"name":"checksums.txt","browser_download_url":%q}
				]
			}]`, binaryName, server.URL+"/binary", server.URL+"/checksums")
		case "/binary":
			_, _ = writer.Write(newBinary)
		case "/checksums":
			_, _ = fmt.Fprintf(writer, "%x  %s\n", sum, binaryName)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	executablePath := filepath.Join(root, "repertoire")
	oldBinary := []byte("#!/bin/sh\necho 'repertoire version v1.0.0 (old build)'\n")
	if err := os.WriteFile(executablePath, oldBinary, 0o755); err != nil {
		t.Fatal(err)
	}

	var output strings.Builder
	err := Run(context.Background(), Options{
		CurrentVersion: "v1.0.0 (old build)",
		In:             strings.NewReader("yes\n"),
		Out:            &output,
		Err:            &output,
		HTTPClient:     server.Client(),
		APIBaseURL:     server.URL,
		ExecutablePath: executablePath,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		HomeDir:        root,
	})
	if err != nil {
		t.Fatalf("Run: %v\n%s", err, output.String())
	}

	installed, err := os.ReadFile(executablePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != string(newBinary) {
		t.Fatalf("installed executable does not match release")
	}
	backups, err := filepath.Glob(filepath.Join(root, ".repertoire", "backups", "repertoire_1.0.0_backup_*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("backups = %v, want one", backups)
	}
	backup, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != string(oldBinary) {
		t.Fatal("backup does not contain the previous executable")
	}
	if !strings.Contains(output.String(), "Successfully updated to v2.0.0") {
		t.Fatalf("unexpected output: %s", output.String())
	}
}
