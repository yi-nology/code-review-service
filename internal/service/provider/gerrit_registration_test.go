package provider

import (
	"testing"

	"github.com/yi-nology/go-git-platform/provider"

	"github.com/yi-nology/code-review-service/internal/service/gerrit"
)

// TestAddOrUpdateGerrit 钉住「Gerrit 平台能被注册」这件事。
//
// Gerrit 不在 go-git-platform SDK 的 backend 列表里。若 AddOrUpdate 无条件调
// provider.NewProvider，保存 Gerrit 平台必然失败（"platform not registered"），
// 配置进不了 s.configs → HasProvider 恒为 false → 评审编排层判定「平台未配置」，
// 整条 Gerrit 链路根本不会启动。
//
// 这正是该能力曾经"文档写了但用不了"的根因，用测试钉住以免回退。
func TestAddOrUpdateGerrit(t *testing.T) {
	holder := &gerrit.ClientHolder{}
	svc, err := NewGitProviderService(nil, holder)
	if err != nil {
		t.Fatalf("构造 provider 服务失败: %v", err)
	}

	cfg := provider.Config{
		Platform: provider.Platform("gerrit"),
		BaseURL:  "https://gerrit.example.com",
		Token:    "admin:http-pass",
	}

	// ① 注册不能报错——这是本测试的核心断言
	if err := svc.AddOrUpdate("gerrit", cfg); err != nil {
		t.Fatalf("注册 Gerrit 平台失败（不应调用 SDK 的 NewProvider）: %v", err)
	}

	// ② 必须被判定为「已配置」，否则评审链路不会启动
	if !svc.HasProvider("gerrit") {
		t.Fatal("Gerrit 注册后 HasProvider 应为 true")
	}

	// ③ clone URL：认证 clone 用 /a/ 前缀，多层 project 也要带全
	got := svc.BuildCloneURL("gerrit", "group", "project")
	want := "https://gerrit.example.com/a/group/project.git"
	if got != want {
		t.Fatalf("clone URL 期望 %q，实际 %q", want, got)
	}
}

// TestRemoveGerritClearsClient 钉住「平台被删除/禁用时客户端要一并清掉」。
//
// Gerrit 的客户端独立于 SDK provider map：只删配置不清客户端的话，写回路径
// 还会拿旧客户端继续往已删除的 Gerrit 上发评审。
func TestRemoveGerritClearsClient(t *testing.T) {
	holder := &gerrit.ClientHolder{}
	svc, err := NewGitProviderService(nil, holder)
	if err != nil {
		t.Fatalf("构造 provider 服务失败: %v", err)
	}
	cfg := provider.Config{
		Platform: provider.Platform("gerrit"),
		BaseURL:  "https://gerrit.example.com",
		Token:    "admin:http-pass",
	}
	if err := svc.AddOrUpdate("gerrit", cfg); err != nil {
		t.Fatalf("注册 Gerrit 平台失败: %v", err)
	}
	if holder.Get() == nil {
		t.Fatal("注册后应持有 Gerrit 客户端")
	}

	svc.Remove("gerrit")
	if holder.Get() != nil {
		t.Fatal("平台被移除后 Gerrit 客户端应被清空")
	}
	if svc.HasProvider("gerrit") {
		t.Fatal("平台被移除后 HasProvider 应为 false")
	}
}
