package stub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOptionalAndValidManifest(t *testing.T) {
	t.Parallel()

	empty, err := Load(t.TempDir())
	if err != nil || len(empty.Stubs) != 0 {
		t.Fatalf("optional manifest = %+v, %v", empty, err)
	}

	root := validFixture(t)
	manifest, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	definition := manifest.Stubs["editorconfig"]
	if definition.Path != "assets/.editorconfig" {
		t.Fatalf("definition = %+v", definition)
	}
	path, err := AssetPath(root, definition)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(root, "assets", ".editorconfig") {
		t.Fatalf("asset path = %q", path)
	}
}

func TestLoadRejectsInvalidManifests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest string
		setup    func(t *testing.T, root string)
		want     string
	}{
		{name: "schema", manifest: "schema: 2\nstubs: {}\n", want: "unsupported stub schema"},
		{name: "name", manifest: stubYAML("Bad_Name", "assets/file", "Description", "Instructions"), setup: regularAsset("assets/file"), want: `stub "Bad_Name"`},
		{name: "description", manifest: stubYAML("demo", "assets/file", " ", "Instructions"), setup: regularAsset("assets/file"), want: "description is required"},
		{name: "instructions", manifest: stubYAML("demo", "assets/file", "Description", " "), setup: regularAsset("assets/file"), want: "instructions are required"},
		{name: "traversal", manifest: stubYAML("demo", "../file", "Description", "Instructions"), want: "contained relative path"},
		{name: "missing", manifest: stubYAML("demo", "assets/file", "Description", "Instructions"), want: "no such file"},
		{name: "directory", manifest: stubYAML("demo", "assets", "Description", "Instructions"), setup: func(t *testing.T, root string) {
			t.Helper()
			if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, want: "regular file"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if test.setup != nil {
				test.setup(t, root)
			}
			if err := os.WriteFile(filepath.Join(root, FileName), []byte(test.manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLoadRejectsEscapingSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "assets", "file")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(stubYAML("demo", "assets/file", "Description", "Instructions")), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected escaping symlink error, got %v", err)
	}
}

func TestLoadRejectsEscapingManifestSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(outside, []byte("schema: 1\nstubs: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, FileName)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("expected escaping manifest symlink error")
	}
}

func validFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	regularAsset("assets/.editorconfig")(t, root)
	content := stubYAML("editorconfig", "assets/.editorconfig", "Ensure final newlines.", "Merge it safely.")
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func regularAsset(relative string) func(*testing.T, string) {
	return func(t *testing.T, root string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("asset\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func stubYAML(name, path, description, instructions string) string {
	return "schema: 1\nstubs:\n  " + name + ":\n" +
		"    description: " + description + "\n" +
		"    path: " + path + "\n" +
		"    instructions: " + instructions + "\n"
}
