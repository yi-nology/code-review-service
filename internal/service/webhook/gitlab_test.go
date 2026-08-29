package webhook

import (
	"testing"
)

func TestFixGitLabTimestamps(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "space-separated + no-colon tz",
			input: `"2026-07-15 21:53:38 +0800"`,
			want:  `"2026-07-15T21:53:38+08:00"`,
		},
		{
			name:  "space-separated + colon tz",
			input: `"2026-07-15 21:53:38 +08:00"`,
			want:  `"2026-07-15T21:53:38+08:00"`,
		},
		{
			name:  "already RFC3339 unchanged",
			input: `"2026-07-15T22:02:48+08:00"`,
			want:  `"2026-07-15T22:02:48+08:00"`,
		},
		{
			name:  "negative tz",
			input: `"2026-07-15 21:53:38 -0500"`,
			want:  `"2026-07-15T21:53:38-05:00"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(fixGitLabTimestamps([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFixGitLabTimestampsInPayload(t *testing.T) {
	raw := []byte(`{"object_attributes":{"created_at":"2026-07-15 21:53:38 +0800","updated_at":"2026-07-15 22:20:58 +0800"},"last_commit":{"timestamp":"2026-07-15T22:02:48+08:00"}}`)
	fixed := string(fixGitLabTimestamps(raw))
	if want := `"created_at":"2026-07-15T21:53:38+08:00"`; !contains(fixed, want) {
		t.Errorf("created_at not fixed: missing %q in %s", want, fixed)
	}
	if want := `"updated_at":"2026-07-15T22:20:58+08:00"`; !contains(fixed, want) {
		t.Errorf("updated_at not fixed: missing %q in %s", want, fixed)
	}
	if want := `"timestamp":"2026-07-15T22:02:48+08:00"`; !contains(fixed, want) {
		t.Errorf("RFC3339 timestamp should be unchanged: missing %q in %s", want, fixed)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
