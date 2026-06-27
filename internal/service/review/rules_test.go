package review

import "testing"

func TestMatchesProjectRefs(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		provider string
		repo     string
		want     bool
	}{
		{name: "empty refs match all", raw: `[]`, provider: "github", repo: "org/repo", want: true},
		{name: "provider repo match", raw: `["github/org/repo"]`, provider: "github", repo: "org/repo", want: true},
		{name: "repo only match", raw: `["org/repo"]`, provider: "gitlab", repo: "org/repo", want: true},
		{name: "mismatch", raw: `["github/other/repo"]`, provider: "github", repo: "org/repo", want: false},
		{name: "invalid json is permissive", raw: `{`, provider: "github", repo: "org/repo", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesProjectRefs([]byte(tt.raw), tt.provider, tt.repo)
			if got != tt.want {
				t.Fatalf("matchesProjectRefs() = %v, want %v", got, tt.want)
			}
		})
	}
}
