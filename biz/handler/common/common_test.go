package common

import (
	"strings"
	"testing"
	"time"
)

func TestPageParams(t *testing.T) {
	tests := []struct {
		name               string
		page, size         string
		wantPage           int
		wantSizeOrFallback bool // size 由上游决定 fallback，这里只验证 clamp
	}{
		{"normal", "3", "50", 3, true},
		{"zero page falls to 1", "0", "50", 1, true},
		{"negative page falls to 1", "-2", "50", 1, true},
		{"garbage page falls to 1", "abc", "50", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, _ := PageParams(tt.page, tt.size)
			if page != tt.wantPage {
				t.Fatalf("page = %d, want %d", page, tt.wantPage)
			}
		})
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// 契约：短 token（≤8）整段打码；长 token 保首尾 4 位。
		{"", "***"},
		{"short", "***"},
		{"ghp_1234567890abcdef", "ghp_****cdef"},
	}
	for _, tt := range tests {
		if got := MaskToken(tt.in); got != tt.want {
			t.Fatalf("MaskToken(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMaskWebhookSecret(t *testing.T) {
	// 契约：空 secret 返 nil（响应 omit），非空打码且不明文透出。
	if got := MaskWebhookSecret(""); got != nil {
		t.Fatalf("空 secret 应返 nil: %v", got)
	}
	if got := MaskWebhookSecret("s3cret"); got == nil || strings.Contains(*got, "s3cret") {
		t.Fatalf("secret 不得明文透出: %v", got)
	}
}

func TestParseUintID(t *testing.T) {
	if id, err := ParseUintID("42"); err != nil || id != 42 {
		t.Fatalf("ParseUintID(42) = %v, %v", id, err)
	}
	if _, err := ParseUintID("-1"); err == nil {
		t.Fatal("负数应报错")
	}
	if _, err := ParseUintID("abc"); err == nil {
		t.Fatal("非数字应报错")
	}
}

func TestSafeInt64UintRoundTrip(t *testing.T) {
	if got := SafeInt64(42); got != 42 {
		t.Fatalf("SafeInt64 = %d", got)
	}
	if got := SafeUint(-7); got != 0 {
		t.Fatalf("负数应钳到 0, got %d", got)
	}
}

func TestFormatTime(t *testing.T) {
	// 契约：统一输出 UTC RFC3339（前端展示层负责转本地时区）；零值返空串。
	utc := time.Date(2026, 9, 20, 8, 5, 0, 0, time.UTC)
	if got := FormatTime(utc); got != "2026-09-20T08:05:00Z" {
		t.Fatalf("FormatTime = %q", got)
	}
	if got := FormatTime(time.Time{}); got != "" {
		t.Fatalf("零值时间应返空串: %q", got)
	}
}
