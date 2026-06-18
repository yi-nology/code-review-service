package review_service

import (
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/webhook"
)

func TestGetNormalizerFromService(t *testing.T) {
	svc := webhook.NewService(webhook.ServiceConfig{
		Normalizers: map[string]webhook.Normalizer{
			"github":       &webhook.GitHubNormalizer{},
			"gitlab":       &webhook.GitLabNormalizer{},
			"gitea":        &webhook.GiteaNormalizer{},
			"gitcode":      &webhook.GitCodeNormalizer{},
			"tencent_code": &webhook.TencentCodeNormalizer{},
		},
	})

	tests := []struct {
		provider string
		wantOK   bool
	}{
		{"github", true},
		{"gitlab", true},
		{"gitea", true},
		{"gitcode", true},
		{"tencent_code", true},
		{"unsupported", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			_, ok := svc.GetNormalizer(tt.provider)
			if ok != tt.wantOK {
				t.Errorf("GetNormalizer(%q) ok = %v, want %v", tt.provider, ok, tt.wantOK)
			}
		})
	}
}

func TestNormalizerEventTypeHeader(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"github", "X-GitHub-Event"},
		{"gitlab", "X-GitLab-Event"},
		{"gitea", "X-Gitea-Event"},
		{"gitcode", "X-GitLab-Event"},
		{"tencent_code", "X-GitLab-Event"},
		{"unknown", "X-Event-Type"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := normalizerEventTypeHeader(tt.provider)
			if got != tt.want {
				t.Errorf("normalizerEventTypeHeader(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestNormalizerDeliveryIDHeader(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"github", "X-GitHub-Delivery"},
		{"gitlab", "X-Gitlab-Event-UUID"},
		{"gitea", "X-Gitea-Delivery"},
		{"unknown", "X-Delivery-ID"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := normalizerDeliveryIDHeader(tt.provider)
			if got != tt.want {
				t.Errorf("normalizerDeliveryIDHeader(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestNormalizerSignatureHeader(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"github", "X-Hub-Signature-256"},
		{"gitlab", "X-Gitlab-Token"},
		{"gitea", "X-Gitea-Signature"},
		{"unknown", "X-Signature"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := normalizerSignatureHeader(tt.provider)
			if got != tt.want {
				t.Errorf("normalizerSignatureHeader(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}
