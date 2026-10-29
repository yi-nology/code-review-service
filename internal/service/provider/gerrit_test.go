package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	domainplatform "github.com/yi-nology/code-review-service/internal/domain/platform"
	"github.com/yi-nology/code-review-service/internal/service/gerrit"

	sdk "github.com/yi-nology/go-git-platform/provider"
)

// newGerritService 造一个带 Gerrit 客户端持有者的服务。
func newGerritService(t *testing.T) (*GitProviderService, *gerrit.ClientHolder) {
	t.Helper()
	holder := &gerrit.ClientHolder{}
	svc, err := NewGitProviderService(nil, holder)
	if err != nil {
		t.Fatalf("构造服务失败: %v", err)
	}
	return svc, holder
}

// TestAddOrUpdateGerrit 钉住「Gerrit 能作为平台被注册」这件事。
//
// Gerrit 不在 go-git-platform SDK 的 backend 列表里。平台注册路径若对 gerrit
// 调 provider.NewProvider，必然拿到 "platform not registered"——web console
// 保存 Gerrit 平台直接失败，Gerrit 整条链路根本起不来。
func TestAddOrUpdateGerrit(t *testing.T) {
	svc, _ := newGerritService(t)

	err := svc.AddOrUpdate(domainplatform.ProviderGerrit, sdk.Config{
		Platform: sdk.Platform(domainplatform.ProviderGerrit),
		BaseURL:  "https://gerrit.example.com",
		Token:    "bot:http-pass",
		SkipTLS:  true,
	})
	if err != nil {
		t.Fatalf("注册 gerrit 平台应成功，实际失败: %v", err)
	}

	// 已配置 → 评审编排层拿得到 Provider。此前判据是 SDK provider map，
	// gerrit 永远被判成「未配置」，整条评审链路不会启动。
	if !svc.HasProvider(domainplatform.ProviderGerrit) {
		t.Fatal("gerrit 应算已配置平台")
	}

	// 但确实没有 SDK provider——Gerrit 的 API 能力走独立客户端。
	if _, err := svc.GetProvider(domainplatform.ProviderGerrit); err == nil {
		t.Fatal("gerrit 不应有 SDK provider")
	}

	// clone 凭据从 configs 读：缺了它 clone 会退化成「无凭据 clone 空 URL」。
	if got := svc.GetToken(domainplatform.ProviderGerrit); got != "bot:http-pass" {
		t.Fatalf("token 不匹配: %q", got)
	}
	if !svc.GetSkipTLS(domainplatform.ProviderGerrit) {
		t.Fatal("skip_tls 期望为 true")
	}
}

// TestGerritBuildCloneURL 钉住 Gerrit 的 clone URL 形态。
//
// 重点是多层 project：Gerrit 里 "group/project" 与 "project" 是两个不同的
// 仓库，只取 repo 会克隆到错的仓库。
func TestGerritBuildCloneURL(t *testing.T) {
	svc, _ := newGerritService(t)
	if err := svc.AddOrUpdate(domainplatform.ProviderGerrit, sdk.Config{
		Platform: sdk.Platform(domainplatform.ProviderGerrit),
		BaseURL:  "https://gerrit.example.com",
		Token:    "bot:http-pass",
	}); err != nil {
		t.Fatalf("注册 gerrit 平台失败: %v", err)
	}

	cases := []struct {
		name  string
		owner string
		repo  string
		want  string
	}{
		{
			name:  "单层 project",
			owner: "",
			repo:  "my-project",
			want:  "https://gerrit.example.com/a/my-project.git",
		},
		{
			name:  "多层 project 保留 group 前缀",
			owner: "group/subgroup",
			repo:  "project",
			want:  "https://gerrit.example.com/a/group/subgroup/project.git",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := svc.BuildCloneURL(domainplatform.ProviderGerrit, tc.owner, tc.repo)
			if got != tc.want {
				t.Fatalf("clone URL 不匹配:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestGerritTestWithToken 钉住 Gerrit 的连接测试走独立客户端。
//
// 此前对 gerrit 调 provider.NewProvider 必然失败，那样的「测试不通过」是假的
// ——凭据本身可能完全正常。
func TestGerritTestWithToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/a/accounts/self" {
			t.Errorf("期望请求 /a/accounts/self，实际 %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(")]}'\n" + `{"_account_id":100,"username":"review-bot"}`))
	}))
	defer srv.Close()

	svc, _ := newGerritService(t)
	res, err := svc.TestWithToken(context.Background(), domainplatform.ProviderGerrit, srv.URL, "bot:http-pass", false)
	if err != nil {
		t.Fatalf("连接测试不应返回 error: %v", err)
	}
	if !res.Connected {
		t.Fatalf("期望连通，实际不通: %s", res.Message)
	}
	if res.UserName != "review-bot" {
		t.Fatalf("用户名不匹配: 期望 review-bot，实际 %q", res.UserName)
	}
}

// TestRemoveGerrit 钉住「平台被禁用/删除后 Gerrit 客户端一并清掉」。
//
// 不清的话写回路径还会拿旧客户端继续发评审——配置改了却不生效。
func TestRemoveGerrit(t *testing.T) {
	svc, holder := newGerritService(t)
	if err := svc.AddOrUpdate(domainplatform.ProviderGerrit, sdk.Config{
		Platform: sdk.Platform(domainplatform.ProviderGerrit),
		BaseURL:  "https://gerrit.example.com",
		Token:    "bot:http-pass",
	}); err != nil {
		t.Fatalf("注册 gerrit 平台失败: %v", err)
	}
	if holder.Get() == nil {
		t.Fatal("注册后应持有 gerrit 客户端")
	}

	svc.Remove(domainplatform.ProviderGerrit)

	if svc.HasProvider(domainplatform.ProviderGerrit) {
		t.Fatal("移除后不应再算已配置平台")
	}
	if holder.Get() != nil {
		t.Fatal("移除后 gerrit 客户端应被清空")
	}
}

// TestGerritHotReload 钉住「改平台配置后客户端热更新」。
//
// 不热更新的话，换 token 后写回仍用旧凭据——只能重启进程。
func TestGerritHotReload(t *testing.T) {
	svc, holder := newGerritService(t)
	cfg := sdk.Config{
		Platform: sdk.Platform(domainplatform.ProviderGerrit),
		BaseURL:  "https://gerrit.example.com",
		Token:    "bot:old-pass",
	}
	if err := svc.AddOrUpdate(domainplatform.ProviderGerrit, cfg); err != nil {
		t.Fatalf("首次注册失败: %v", err)
	}
	first := holder.Get()
	if first == nil {
		t.Fatal("首次注册后应持有 gerrit 客户端")
	}

	cfg.Token = "bot:new-pass"
	if err := svc.AddOrUpdate(domainplatform.ProviderGerrit, cfg); err != nil {
		t.Fatalf("二次注册失败: %v", err)
	}
	if holder.Get() == first {
		t.Fatal("改配置后应重建客户端")
	}
	if got := svc.GetToken(domainplatform.ProviderGerrit); got != "bot:new-pass" {
		t.Fatalf("token 未更新: %q", got)
	}
}
