package llm

import (
	"context"
	"errors"
	"testing"
)

func TestNoopProviderAlwaysDegraded(t *testing.T) {
	p := NewNoopProvider()
	if p.Available(context.Background()) {
		t.Fatal("NoopProvider should not be available")
	}
	if _, err := p.Complete(context.Background(), CompleteRequest{}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestCompleteJSONDegradesOnError(t *testing.T) {
	// NoopProvider 返回错误 → CompleteJSON 必须返回错误（降级路径）
	var out map[string]any
	if err := CompleteJSON(context.Background(), NewNoopProvider(), CompleteRequest{}, &out); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
	if out != nil {
		t.Fatalf("out should remain nil on error")
	}
}

func TestParseJSONLoose(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantKey string
		wantVal string
	}{
		{"plain json", `{"a":"b"}`, "a", "b"},
		{"fenced json", "```json\n{\"a\":\"b\"}\n```", "a", "b"},
		{"fenced no lang", "```\n{\"a\":\"b\"}\n```", "a", "b"},
		{"with prefix noise", "Here is the result: {\"a\":\"b\"}", "a", "b"},
		{"with suffix noise", "{\"a\":\"b\"}\n\nHope this helps.", "a", "b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]string
			if err := parseJSONLoose(tc.input, &got); err != nil {
				t.Fatalf("parseJSONLoose failed: %v", err)
			}
			if got[tc.wantKey] != tc.wantVal {
				t.Fatalf("got %q, want %q", got[tc.wantKey], tc.wantVal)
			}
		})
	}
}

func TestParseJSONLooseInvalid(t *testing.T) {
	var got map[string]any
	if err := parseJSONLoose("not json at all", &got); err == nil {
		t.Fatal("expected error for invalid json")
	}
}
