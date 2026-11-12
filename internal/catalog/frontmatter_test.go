package catalog

import "testing"

// The convention is spelled several ways in the wild. None of them may fail the
// parse: a decode error here is reported as invalid frontmatter, which stops a
// skill from installing at all.
func TestParseSkillFrontmatterReadsTheInvocationFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		header      string
		wantDisable bool
		wantHint    string
	}{
		{
			name:        "quoted hint and boolean flag",
			header:      "argument-hint: \"[task-or-wave]\"\ndisable-model-invocation: true",
			wantDisable: true,
			wantHint:    "[task-or-wave]",
		},
		{
			name:        "bare bracket hint is a sequence, not a string",
			header:      "argument-hint: [target]\ndisable-model-invocation: true",
			wantDisable: true,
			wantHint:    "[target]",
		},
		{
			name:        "flag written as text",
			header:      "disable-model-invocation: \"true\"",
			wantDisable: true,
		},
		{
			name:     "plain hint",
			header:   "argument-hint: <manifest-path>",
			wantHint: "<manifest-path>",
		},
		{
			name:   "neither field present",
			header: "",
		},
		{
			name:        "unrecognised flag value leaves the skill model-invocable",
			header:      "disable-model-invocation: sometimes",
			wantDisable: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := "---\nname: demo\ndescription: Demo\n" + test.header + "\n---\n\nBody\n"
			header, err := ParseSkillFrontmatter([]byte(content))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if header.DisableModelInvocation != test.wantDisable {
				t.Errorf("disable-model-invocation = %v, want %v", header.DisableModelInvocation, test.wantDisable)
			}
			if header.ArgumentHint != test.wantHint {
				t.Errorf("argument-hint = %q, want %q", header.ArgumentHint, test.wantHint)
			}
			if header.Name != "demo" || header.Description != "Demo" {
				t.Errorf("unexpected header: %+v", header)
			}
		})
	}
}
