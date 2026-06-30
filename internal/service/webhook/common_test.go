package webhook

import "testing"

func TestIntFromAny(t *testing.T) {
	tests := []struct {
		in  any
		out int
	}{
		{float64(42), 42},
		{int(7), 7},
		{int64(100), 100},
		{"15", 15},
		{nil, 0},
		{"abc", 0},
	}
	for _, tt := range tests {
		if got := IntFromAny(tt.in); got != tt.out {
			t.Errorf("IntFromAny(%v) = %d, want %d", tt.in, got, tt.out)
		}
	}
}

func TestStringFromAny(t *testing.T) {
	if got := StringFromAny("hello"); got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
	if got := StringFromAny(nil); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if got := StringFromAny(42); got != "" {
		t.Errorf("expected empty for non-string, got %q", got)
	}
}

func TestParseNormalizedPayload(t *testing.T) {
	raw := []byte(`{"action": "opened", "number": 42, "repository": {"full_name": "owner/repo"}}`)
	np, err := ParseNormalizedPayload(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if np.Raw == nil {
		t.Fatalf("expected Raw to be set")
	}
	if np.Action != "" {
		t.Fatalf("expected action to be empty (ParseNormalizedPayload only sets Raw), got %q", np.Action)
	}
	if np.Raw["action"] != "opened" {
		t.Fatalf("expected Raw[action]=opened, got %v", np.Raw["action"])
	}
}
