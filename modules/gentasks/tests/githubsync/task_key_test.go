package githubsync_test

import (
	"slices"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
)

func TestExtractTaskKeys(t *testing.T) {
	cases := []struct {
		title   string
		headRef string
		want    []githubsync.TaskKey
	}{
		{"GEN-42: sign in", "", []githubsync.TaskKey{{Prefix: "GEN", Number: 42}}},
		{"Sign in", "feat/gen-42-sign-in", []githubsync.TaskKey{{Prefix: "GEN", Number: 42}}},
		{"GEN-42 sign in", "feat/GEN-42", []githubsync.TaskKey{{Prefix: "GEN", Number: 42}}},
		{"GEN-1 and OPS-7: share login", "", []githubsync.TaskKey{{Prefix: "GEN", Number: 1}, {Prefix: "OPS", Number: 7}}},
		{"GEN-1-OPS-2", "", []githubsync.TaskKey{{Prefix: "GEN", Number: 1}, {Prefix: "OPS", Number: 2}}},
		{"[pla2-15] fix", "", []githubsync.TaskKey{{Prefix: "PLA2", Number: 15}}},
		{"", "GEN-42_hotfix", []githubsync.TaskKey{{Prefix: "GEN", Number: 42}}},
		{"GEN-42abc and gen_42 and -42", "", nil},
		{"ABCDEFGHIJK-1 too long a prefix", "", nil},
		{"GEN-1234567890 too long a number", "", nil},
		{"Bump to UTF-8", "", []githubsync.TaskKey{{Prefix: "UTF", Number: 8}}},
		{"", "", nil},
	}
	for _, testCase := range cases {
		got := githubsync.ExtractTaskKeys(testCase.title, testCase.headRef)
		if !slices.Equal(got, testCase.want) {
			t.Errorf("ExtractTaskKeys(%q, %q) = %v, want %v", testCase.title, testCase.headRef, got, testCase.want)
		}
	}
}
