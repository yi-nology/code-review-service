package progress_service

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestSameOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		// Origin 缺失放行（非浏览器客户端由 Bearer 认证兜底）。
		{"missing origin allowed", "", "review.example.com", true},
		{"missing origin empty host allowed", "", "", true},

		// 精确同源（各 scheme）。
		{"exact http", "http://review.example.com", "review.example.com", true},
		{"exact https", "https://review.example.com", "review.example.com", true},
		{"exact with port", "http://localhost:8081", "localhost:8081", true},
		{"exact ws", "ws://review.example.com", "review.example.com", true},
		{"exact wss", "wss://review.example.com:9443", "review.example.com:9443", true},

		// 前缀/后缀伪装必须拒绝（CSWSH 修复点）。
		{"attacker subdomain suffix", "http://review.example.com.attacker.io", "review.example.com", false},
		{"attacker prefix", "http://attacker-review.example.com", "review.example.com", false},
		{"attacker contains", "http://review.example.com.attacker.io:8080", "review.example.com", false},

		// host 部分不一致一律拒绝。
		{"different host", "http://evil.io", "review.example.com", false},
		{"port mismatch", "http://review.example.com:9000", "review.example.com", false},

		// 非法输入 fail closed。
		{"missing host header", "http://review.example.com", "", false},
		{"origin without scheme", "review.example.com", "review.example.com", false},
		{"origin null literal", "null", "review.example.com", false},
		{"origin relative", "//review.example.com", "review.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := app.NewContext(8)
			if tt.origin != "" {
				ctx.Request.Header.Set("Origin", tt.origin)
			}
			if tt.host != "" {
				ctx.Request.Header.Set("Host", tt.host)
			}
			if got := sameOrigin(ctx); got != tt.want {
				t.Fatalf("sameOrigin(origin=%q, host=%q) = %v, want %v", tt.origin, tt.host, got, tt.want)
			}
		})
	}
}
