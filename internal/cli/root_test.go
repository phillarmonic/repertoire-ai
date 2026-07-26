package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandHelpAndVersion(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "help", args: []string{"--help"}, want: productDescription},
		{name: "version", args: []string{"--version"}, want: "repertoire version test-version"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			command := NewRootCommand("test-version", &stdout, &stdout)
			command.SetArgs(test.args)
			if err := command.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("output %q does not contain %q", stdout.String(), test.want)
			}
		})
	}
}

func TestRootCommandHelpIncludesProductBanner(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	command := NewRootCommand("test-version", &stdout, &stdout)
	command.SetArgs([]string{"--help"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	output := stdout.String()
	for _, want := range []string{productBanner, productDescription, "--self-update"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output does not contain %q:\n%s", want, output)
		}
	}
}
