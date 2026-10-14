package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSplitNameListExpandsAndDeduplicates(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{name: "empty", args: nil, want: []string{}},
		{name: "single", args: []string{"alpha"}, want: []string{"alpha"}},
		{name: "space separated", args: []string{"alpha", "beta"}, want: []string{"alpha", "beta"}},
		{name: "comma list", args: []string{"alpha,beta"}, want: []string{"alpha", "beta"}},
		{
			name: "mixed with trim, blanks, and duplicates",
			args: []string{"alpha, beta ", "", "beta,alpha,", " , gamma"},
			want: []string{"alpha", "beta", "gamma"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := splitNameList(test.args); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("splitNameList(%#v) = %#v, want %#v", test.args, got, test.want)
			}
		})
	}
}

func TestSplitCompletionPrefix(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		toComplete string
		prefix     string
		fragment   string
	}{
		{toComplete: "alpha", prefix: "", fragment: "alpha"},
		{toComplete: "alpha,", prefix: "alpha,", fragment: ""},
		{toComplete: "alpha,be", prefix: "alpha,", fragment: "be"},
		{toComplete: "alpha,beta,ga", prefix: "alpha,beta,", fragment: "ga"},
	} {
		prefix, fragment := splitCompletionPrefix(test.toComplete)
		if prefix != test.prefix || fragment != test.fragment {
			t.Fatalf("splitCompletionPrefix(%q) = (%q, %q), want (%q, %q)", test.toComplete, prefix, fragment, test.prefix, test.fragment)
		}
	}
}

func TestCommaAwareCompletesAfterLastComma(t *testing.T) {
	t.Parallel()
	base := func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		all := []string{"alpha\t[one]", "beta\t[two]", "gamma"}
		completions := make([]string, 0, len(all))
		for _, completion := range all {
			value, _, _ := strings.Cut(completion, "\t")
			if strings.HasPrefix(value, toComplete) {
				completions = append(completions, completion)
			}
		}
		return completions, completionDirective
	}
	got, directive := commaAware(base)(nil, nil, "alpha,be")
	want := []string{"alpha,beta\t[two]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commaAware completions = %#v, want %#v", got, want)
	}
	if directive != completionDirective {
		t.Fatalf("directive = %v, want %v", directive, completionDirective)
	}

	// Without a comma the adapter is transparent.
	plain, _ := commaAware(base)(nil, nil, "ga")
	if !reflect.DeepEqual(plain, []string{"gamma"}) {
		t.Fatalf("plain completions = %#v", plain)
	}
}
