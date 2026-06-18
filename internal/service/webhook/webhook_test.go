package webhook

import (
	"testing"
)

func TestIsSupportedAction(t *testing.T) {
	tests := []struct {
		provider string
		action   string
		want     bool
	}{
		{"github", "opened", true},
		{"github", "reopened", true},
		{"github", "synchronize", true},
		{"github", "ready_for_review", true},
		{"github", "closed", false},
		{"gitlab", "open", true},
		{"gitlab", "reopen", true},
		{"gitlab", "update", true},
		{"gitlab", "close", false},
		{"gitea", "opened", true},
		{"gitea", "reopened", true},
		{"gitea", "synchronized", true},
		{"gitcode", "open", true},
		{"tencent_code", "open", true},
		{"unsupported", "opened", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider+"/"+tt.action, func(t *testing.T) {
			got := IsSupportedAction(tt.provider, tt.action)
			if got != tt.want {
				t.Errorf("IsSupportedAction(%q, %q) = %v, want %v", tt.provider, tt.action, got, tt.want)
			}
		})
	}
}

func TestIsDraftEvent(t *testing.T) {
	tests := []struct {
		name    string
		provider string
		payload map[string]any
		want    bool
	}{
		{
			name:     "github draft PR",
			provider: "github",
			payload:  map[string]any{"pull_request": map[string]any{"draft": true}},
			want:     true,
		},
		{
			name:     "github non-draft PR",
			provider: "github",
			payload:  map[string]any{"pull_request": map[string]any{"draft": false}},
			want:     false,
		},
		{
			name:     "gitlab WIP",
			provider: "gitlab",
			payload:  map[string]any{"object_attributes": map[string]any{"work_in_progress": true}},
			want:     true,
		},
		{
			name:     "gitea WIP label",
			provider: "gitea",
			payload:  map[string]any{"pull_request": map[string]any{"labels": []any{map[string]any{"name": "wip"}}}},
			want:     true,
		},
		{
			name:     "empty payload",
			provider: "github",
			payload:  map[string]any{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDraftEvent(tt.provider, tt.payload)
			if got != tt.want {
				t.Errorf("IsDraftEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBotSender(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		payload  map[string]any
		botName  string
		want     bool
	}{
		{
			name:     "github bot sender",
			provider: "github",
			payload:  map[string]any{"sender": map[string]any{"login": "dependabot[bot]"}},
			botName:  "dependabot[bot]",
			want:     true,
		},
		{
			name:     "github human sender",
			provider: "github",
			payload:  map[string]any{"sender": map[string]any{"login": "human"}},
			botName:  "dependabot[bot]",
			want:     false,
		},
		{
			name:     "empty bot name",
			provider: "github",
			payload:  map[string]any{"sender": map[string]any{"login": "bot"}},
			botName:  "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBotSender(tt.provider, tt.payload, tt.botName)
			if got != tt.want {
				t.Errorf("IsBotSender() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitHubNormalizerNormalize(t *testing.T) {
	n := &GitHubNormalizer{}

	t.Run("unsupported event returns nil", func(t *testing.T) {
		result, err := n.Normalize([]byte(`{}`), "issues")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil for unsupported event")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		_, err := n.Normalize([]byte(`not json`), "pull_request")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestGitLabNormalizerNormalize(t *testing.T) {
	n := &GitLabNormalizer{}

	t.Run("unsupported event returns nil", func(t *testing.T) {
		result, err := n.Normalize([]byte(`{}`), "issues")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil for unsupported event")
		}
	})
}

func TestGiteaNormalizerNormalize(t *testing.T) {
	n := &GiteaNormalizer{}

	t.Run("unsupported event returns nil", func(t *testing.T) {
		result, err := n.Normalize([]byte(`{}`), "issues")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil for unsupported event")
		}
	})
}

func TestWebhookServiceMaxPayloadBytes(t *testing.T) {
	svc := NewService(ServiceConfig{
		MaxPayloadBytes: 0,
	})
	if svc.MaxPayloadBytes() != 1<<20 {
		t.Errorf("expected default 1MB, got %d", svc.MaxPayloadBytes())
	}

	svc2 := NewService(ServiceConfig{
		MaxPayloadBytes: 5 << 20,
	})
	if svc2.MaxPayloadBytes() != 5<<20 {
		t.Errorf("expected 5MB, got %d", svc2.MaxPayloadBytes())
	}
}
