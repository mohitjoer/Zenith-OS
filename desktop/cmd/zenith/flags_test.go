package main

import "testing"

// TestParseFlagsAcceptsFlagsAfterPositionals pins a bug this tool actually had.
//
// The standard flag package stops at the first non-flag argument, so
// "zenith reset ghostty/config --root DIR" dropped the --root flag entirely and
// the command reported "needs exactly one path" while the flag it printed was
// silently the default. A person typing a command by hand hits that every time.
func TestParseFlagsAcceptsFlagsAfterPositionals(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantPositional []string
		wantRoot       string
	}{
		{
			name:           "flags first",
			args:           []string{"--root", "/a", "ghostty/config"},
			wantPositional: []string{"ghostty/config"},
			wantRoot:       "/a",
		},
		{
			name:           "flags last",
			args:           []string{"ghostty/config", "--root", "/a"},
			wantPositional: []string{"ghostty/config"},
			wantRoot:       "/a",
		},
		{
			name:           "flags around the positional",
			args:           []string{"--root", "/a", "ghostty/config", "--strict"},
			wantPositional: []string{"ghostty/config"},
			wantRoot:       "/a",
		},
		{
			name:           "no positionals",
			args:           []string{"--root", "/a"},
			wantPositional: nil,
			wantRoot:       "/a",
		},
		{
			name:           "several positionals",
			args:           []string{"a", "b", "--root", "/a"},
			wantPositional: []string{"a", "b"},
			wantRoot:       "/a",
		},
		{
			name:           "boolean flag after a positional",
			args:           []string{"a", "--strict", "--root", "/a"},
			wantPositional: []string{"a"},
			wantRoot:       "/a",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			set := newFlags("test", "test\n")
			root := set.String("root", "", "")
			strict := set.Bool("strict", false, "")

			positional, err := parseFlags(set, tc.args)
			if err != nil {
				t.Fatalf("parseFlags(%v): %v", tc.args, err)
			}

			if len(positional) != len(tc.wantPositional) {
				t.Fatalf("positionals = %v, want %v", positional, tc.wantPositional)
			}
			for i := range tc.wantPositional {
				if positional[i] != tc.wantPositional[i] {
					t.Errorf("positional[%d] = %q, want %q",
						i, positional[i], tc.wantPositional[i])
				}
			}
			if *root != tc.wantRoot {
				t.Errorf("--root = %q, want %q", *root, tc.wantRoot)
			}
			if tc.name == "flags around the positional" && !*strict {
				t.Error("--strict after a positional was not set")
			}
		})
	}
}

// TestParseFlagsRejectsBadFlags checks a typo is an error, not a default.
func TestParseFlagsRejectsBadFlags(t *testing.T) {
	set := newFlags("test", "test\n")
	set.String("root", "", "")
	if _, err := parseFlags(set, []string{"--nonsense"}); err == nil {
		t.Error("an unknown flag was accepted")
	}
}
