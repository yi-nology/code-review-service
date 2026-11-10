package provider

import (
	"strings"
	"testing"

	sdk "github.com/yi-nology/go-git-platform/provider"

	// 平台注册是各后端包的 init 副作用（主程序用 backends/all 统一引入），
	// 测试里按需引入用到的后端即可。
	_ "github.com/yi-nology/go-git-platform/backends/gitcode"
	_ "github.com/yi-nology/go-git-platform/backends/gitea"
	_ "github.com/yi-nology/go-git-platform/backends/github"
	_ "github.com/yi-nology/go-git-platform/backends/gitlab"
)

// cloneURLFixtureCase 一条「平台基址 → 期望 clone 基址」的用例。
// hostRewrite 标记该平台的 API→站点改写是否命中 host 级重写（github/gitcode）。
type cloneURLFixtureCase struct {
	name        string
	platform    string
	baseURL     string
	wantBase    string
	hostRewrite bool // 是否命中 host 级改写（api.*.com → *.com）
}

var cloneBaseURLCases = []cloneURLFixtureCase{
	{
		name:        "github 官方默认 API 基址命中 host 改写",
		platform:    "github",
		baseURL:     "https://api.github.com",
		wantBase:    "https://github.com",
		hostRewrite: true,
	},
	{
		name:        "github 带 v3 路径先去后缀再改 host",
		platform:    "github",
		baseURL:     "https://api.github.com/api/v3",
		wantBase:    "https://github.com",
		hostRewrite: true,
	},
	{
		name:        "gitcode 官方默认 API 基址命中 host 改写",
		platform:    "gitcode",
		baseURL:     "https://api.gitcode.com/api/v5",
		wantBase:    "https://gitcode.com",
		hostRewrite: true,
	},
	{
		name:        "github 企业自建域名不命中 host 改写",
		platform:    "github",
		baseURL:     "https://github.corp.example.com/api/v3",
		wantBase:    "https://github.corp.example.com",
		hostRewrite: false,
	},
	{
		name:        "gitcode 自建域名不命中 host 改写",
		platform:    "gitcode",
		baseURL:     "https://gitcode.corp.example.com/api/v5",
		wantBase:    "https://gitcode.corp.example.com",
		hostRewrite: false,
	},
	{
		name:        "gitlab 仅去 API 路径后缀 host 不变",
		platform:    "gitlab",
		baseURL:     "https://gitlab.example.com/api/v4",
		wantBase:    "https://gitlab.example.com",
		hostRewrite: false,
	},
	{
		name:        "gitea 仅去 API 路径后缀 host 不变",
		platform:    "gitea",
		baseURL:     "https://gitea.example.com/api/v1",
		wantBase:    "https://gitea.example.com",
		hostRewrite: false,
	},
	{
		name:        "gerrit 不做任何改写",
		platform:    "gerrit",
		baseURL:     "https://gerrit.example.com",
		wantBase:    "https://gerrit.example.com",
		hostRewrite: false,
	},
}

// TestCloneBaseURLRewrite 钉住 API 基址 → clone 基址的改写规则（含 host 级
// 改写命中与不命中两态）。
func TestCloneBaseURLRewrite(t *testing.T) {
	for _, tc := range cloneBaseURLCases {
		t.Run(tc.name, func(t *testing.T) {
			got := cloneBaseURL(tc.platform, tc.baseURL)
			if got != tc.wantBase {
				t.Errorf("cloneBaseURL(%q, %q) = %q, 期望 %q", tc.platform, tc.baseURL, got, tc.wantBase)
			}
		})
	}
}

// TestBuildAndValidateCloneURLConsistency 固化不变式（#7）：**平台自己拼出的
// clone URL 必须能通过自己的守卫**——host 改写命中（github/gitcode 官方默认
// 基址）与不命中（自建域名、仅路径改写的 gitlab/gitea/gerrit）两态都要成立。
// 此前 ValidateCloneURL 拿未改写的 cfg.BaseURL 比 host，github/gitcode 默认
// 配置下每次评审都在 clone 前被自己的守卫拦下（clone 目标 host 不匹配）。
func TestBuildAndValidateCloneURLConsistency(t *testing.T) {
	for _, tc := range cloneBaseURLCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newGerritService(t)
			if err := svc.AddOrUpdate(tc.platform, sdk.Config{
				Platform: sdk.Platform(tc.platform),
				BaseURL:  tc.baseURL,
				Token:    "dummy",
			}); err != nil {
				t.Fatalf("注册平台 %s 失败: %v", tc.platform, err)
			}

			cloneURL := svc.BuildCloneURL(tc.platform, "owner", "repo")
			wantClone := tc.wantBase + "/owner/repo.git"
			if tc.platform == "gerrit" {
				wantClone = tc.wantBase + "/a/owner/repo.git"
			}
			if cloneURL != wantClone {
				t.Fatalf("BuildCloneURL = %q, 期望 %q", cloneURL, wantClone)
			}

			if err := svc.ValidateCloneURL(tc.platform, "owner", "repo", cloneURL); err != nil {
				t.Fatalf("平台拼出的 clone URL %q 必须通过自己的守卫, 实际被拒: %v", cloneURL, err)
			}

			// 守卫仍然守得住：换成陌生 host 的同形态 URL 必须被拒——
			// 校验基址换成改写后的值不能放宽 host 白名单本身。
			evil := "https://evil.example.com/owner/repo.git"
			if err := svc.ValidateCloneURL(tc.platform, "owner", "repo", evil); err == nil {
				t.Fatalf("陌生 host 的 clone URL %q 应被守卫拒绝", evil)
			} else if !strings.Contains(err.Error(), "host") {
				t.Fatalf("陌生 host 应报 host 不匹配, 实际: %v", err)
			}
		})
	}
}

// TestValidateCloneURLStillGuardsInjection 守卫的其余判据不因基址同源化而放松。
func TestValidateCloneURLStillGuardsInjection(t *testing.T) {
	svc, _ := newGerritService(t)
	if err := svc.AddOrUpdate("github", sdk.Config{
		Platform: "github",
		BaseURL:  "https://api.github.com",
		Token:    "dummy",
	}); err != nil {
		t.Fatalf("注册 github 平台失败: %v", err)
	}

	// owner 注入仍被 ValidateRepoPath 拦截（host 逃逸 / 路径穿越）。
	for _, tc := range []struct{ name, owner, repo string }{
		{"owner 含 @ 尝试 host 逃逸", "@evil.com", "repo"},
		{"owner 含 .. 路径穿越", "..", "repo"},
		{"owner 含 // host 逃逸", "//evil.com", "repo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cloneURL := svc.BuildCloneURL("github", tc.owner, tc.repo)
			if err := svc.ValidateCloneURL("github", tc.owner, tc.repo, cloneURL); err == nil {
				t.Fatalf("注入形态 (%q, %q) 应被拒绝", tc.owner, tc.repo)
			}
		})
	}
}
