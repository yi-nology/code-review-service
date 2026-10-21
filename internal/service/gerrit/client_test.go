package gerrit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	domainplatform "github.com/yi-nology/code-review-service/internal/domain/platform"
)

// TestTestConnection 钉住 Gerrit 连接测试的行为。
//
// Gerrit 不走 SDK 的 Provider.TestConnection，连接测试打的是 REST API 的
// accounts/self —— 它是 Gerrit 上唯一一个「凭据对就 200」的端点，
// 顺带把认证用户名带回来（web console 要显示 user_name）。
func TestTestConnection(t *testing.T) {
	t.Run("连通且带出用户名", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// go-gerrit 在已认证时自动加 "a/" 前缀——Gerrit 的认证 REST API
			// 约定就在 /a/ 下（与认证 clone 的 /a/ 同一前缀）。
			if r.URL.Path != "/a/accounts/self" {
				t.Errorf("期望请求 /a/accounts/self，实际 %q", r.URL.Path)
			}
			user, pass, ok := r.BasicAuth()
			if !ok || user != "bot" || pass != "http-pass" {
				t.Errorf("期望 Basic Auth bot:http-pass，实际 %q:%q (ok=%v)", user, pass, ok)
			}
			w.Header().Set("Content-Type", "application/json")
			// Gerrit 的 REST API 带 magic prefix，客户端会剥掉它。
			_, _ = w.Write([]byte(")]}'\n" + `{"_account_id":100,"username":"review-bot"}`))
		}))
		defer srv.Close()

		res, err := TestConnection(context.Background(), srv.URL, "bot:http-pass", false)
		if err != nil {
			t.Fatalf("连接测试不应返回 error: %v", err)
		}
		if !res.Connected {
			t.Fatalf("期望连通，实际不通: %s", res.Message)
		}
		if res.UserName != "review-bot" {
			t.Fatalf("用户名不匹配: 期望 review-bot，实际 %q", res.UserName)
		}
		if res.Platform != domainplatform.ProviderGerrit {
			t.Fatalf("平台标识不匹配: %q", res.Platform)
		}
	})

	t.Run("凭据不对报连通失败而不是 error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(")]}'\n" + `{"message":"Invalid username or password"}`))
		}))
		defer srv.Close()

		res, err := TestConnection(context.Background(), srv.URL, "bot:wrong", false)
		if err != nil {
			t.Fatalf("连通失败不应返回 error（走 Connected=false + Message）: %v", err)
		}
		if res.Connected {
			t.Fatal("凭据错误时不应报连通")
		}
		if res.Message == "" {
			t.Fatal("连通失败必须带原因说明")
		}
	})

	t.Run("base URL 非法报初始化失败", func(t *testing.T) {
		res, err := TestConnection(context.Background(), "", "bot:pass", false)
		if err != nil {
			t.Fatalf("初始化失败不应返回 error: %v", err)
		}
		if res.Connected {
			t.Fatal("base URL 为空时不应报连通")
		}
		if res.Message == "" {
			t.Fatal("初始化失败必须带原因说明")
		}
	})
}

// TestBuildCloneURL 钉住 Gerrit 认证 clone URL 的形态。
func TestBuildCloneURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		project string
		want    string
	}{
		{
			name:    "补 /a/ 前缀",
			baseURL: "https://gerrit.example.com",
			project: "my-project",
			want:    "https://gerrit.example.com/a/my-project.git",
		},
		{
			name:    "多层 project 保持原样",
			baseURL: "https://gerrit.example.com",
			project: "group/subgroup/project",
			want:    "https://gerrit.example.com/a/group/subgroup/project.git",
		},
		{
			name:    "base 已含 /a 不重复追加",
			baseURL: "https://gerrit.example.com/a",
			project: "my-project",
			want:    "https://gerrit.example.com/a/my-project.git",
		},
		{
			name:    "base 末尾斜杠不产生双斜杠",
			baseURL: "https://gerrit.example.com/",
			project: "my-project",
			want:    "https://gerrit.example.com/a/my-project.git",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildCloneURL(tc.baseURL, tc.project); got != tc.want {
				t.Fatalf("clone URL 不匹配:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestParseToken 钉住 token 字段的两种写法。
func TestParseToken(t *testing.T) {
	cases := []struct {
		token    string
		wantUser string
		wantPass string
	}{
		{"bot:http-pass", "bot", "http-pass"},
		{"http-pass", "admin", "http-pass"},
		{"", "admin", ""},
	}
	for _, tc := range cases {
		user, pass := parseToken(tc.token)
		if user != tc.wantUser || pass != tc.wantPass {
			t.Fatalf("token %q 解析为 %q:%q，期望 %q:%q", tc.token, user, pass, tc.wantUser, tc.wantPass)
		}
	}
}
