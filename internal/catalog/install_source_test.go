package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseInstallSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repo := filepath.Join(root, "acme-skills")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		arg  string
		want InstallSource
		ok   bool
	}{
		{arg: repo, want: InstallSource{Source: repo}, ok: true},
		{arg: filepath.Join(repo, "code-reviewer"), want: InstallSource{Source: repo, Skill: "code-reviewer"}, ok: true},
		{arg: "https://github.com/acme/skills.git", want: InstallSource{Source: "https://github.com/acme/skills.git"}, ok: true},
		{arg: "github.com/acme/skills", want: InstallSource{Source: "https://github.com/acme/skills.git"}, ok: true},
		{arg: "github.com/acme/skills/code-reviewer", want: InstallSource{Source: "https://github.com/acme/skills.git", Skill: "code-reviewer"}, ok: true},
		{
			arg:  "https://github.com/acme/skills/tree/main/skills/code-reviewer",
			want: InstallSource{Source: "https://github.com/acme/skills.git", Ref: "main", Skill: "code-reviewer"},
			ok:   true,
		},
		{arg: "acme/skills", want: InstallSource{Source: "github.com/acme/skills"}, ok: true},
		{arg: "git@github.com:acme/skills.git", want: InstallSource{Source: "git@github.com:acme/skills.git"}, ok: true},
		{arg: "zensical", ok: false},
		{arg: "github.com/acme", ok: false},
	}
	for _, test := range cases {
		got, ok := ParseInstallSource(test.arg)
		if ok != test.ok {
			t.Fatalf("ParseInstallSource(%q) ok=%v, want %v", test.arg, ok, test.ok)
		}
		if got != test.want {
			t.Fatalf("ParseInstallSource(%q) = %+v, want %+v", test.arg, got, test.want)
		}
	}
}

func TestDefaultCatalogNameAndSameSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repo := filepath.Join(root, "Agent_Skills")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := DefaultCatalogName(repo)
	if err != nil {
		t.Fatal(err)
	}
	if name != "agent-skills" {
		t.Fatalf("DefaultCatalogName(local) = %q", name)
	}
	name, err = DefaultCatalogName("https://github.com/Acme/My_Repo.git")
	if err != nil {
		t.Fatal(err)
	}
	if name != "my-repo" {
		t.Fatalf("DefaultCatalogName(url) = %q", name)
	}
	if !SameSource(repo, repo+"/.") {
		t.Fatal("expected local paths to match")
	}
	if !SameSource("github.com/acme/skills", "https://github.com/acme/skills.git") {
		t.Fatal("expected github shorthand to match clone URL")
	}
	if SameSource("https://github.com/acme/one.git", "https://github.com/acme/two.git") {
		t.Fatal("distinct remotes should not match")
	}
}
