package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

// stubSettings 实现 settingsSource
type stubSettings struct {
	settings models.OCRSettings
}

func (s *stubSettings) Get(_ context.Context) (*models.OCRSettings, error) {
	return &s.settings, nil
}

// 验证 PR-22：5xx 自动重试 + 4xx 不重试 + 错误消息走 redact。
func TestDoHTTP_5xxRetriesThenSucceeds(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error": "transient: api_key=secret12345"}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"choices": [{"message": {"content": "ok"}}]}`))
	}))
	defer srv.Close()

	p := NewDynamicProvider(&stubSettings{settings: models.OCRSettings{
		URL:     srv.URL,
		ModelName: "x",
		Enabled: true,
	}})

	req, _ := http.NewRequestWithContext(context.Background(), "POST", srv.URL+"/chat/completions", strings.NewReader("{}"))
	resp, err := p.doHTTP(req, func(b []byte) (CompleteResponse, error) {
		return CompleteResponse{Content: "ok"}, nil
	})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("expected content=ok, got %q", resp.Content)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestDoHTTP_4xxNoRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer srv.Close()

	p := NewDynamicProvider(&stubSettings{settings: models.OCRSettings{
		URL:     srv.URL,
		ModelName: "x",
		Enabled: true,
	}})

	req, _ := http.NewRequestWithContext(context.Background(), "POST", srv.URL+"/chat/completions", strings.NewReader("{}"))
	_, err := p.doHTTP(req, func(b []byte) (CompleteResponse, error) {
		return CompleteResponse{}, fmt.Errorf("parse should not be called")
	})
	if err == nil {
		t.Fatalf("expected err for 400")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (no retry on 4xx), got %d", attempts)
	}
}

func TestDoHTTP_5xxRedactsApiKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error": "auth failed: api_key=sk-1234567890abcdefghij"}`))
	}))
	defer srv.Close()

	p := NewDynamicProvider(&stubSettings{settings: models.OCRSettings{
		URL:     srv.URL,
		ModelName: "x",
		Enabled: true,
	}})

	req, _ := http.NewRequestWithContext(context.Background(), "POST", srv.URL+"/chat/completions", strings.NewReader("{}"))
	_, err := p.doHTTP(req, func(b []byte) (CompleteResponse, error) {
		return CompleteResponse{}, nil
	})
	if err == nil {
		t.Fatalf("expected err for 500")
	}
	if strings.Contains(err.Error(), "sk-1234567890") {
		t.Fatalf("expected api_key in error to be redacted, got %q", err.Error())
	}
}
