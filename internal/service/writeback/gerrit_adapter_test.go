package writeback

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/gerrit"
)

// TestGerritAdapterWithoutClient 钉住「Gerrit 未配置时写回返回明确错误，而不是 panic」。
//
// 适配器改为持有 getter（为了热更新），取到的 client 可能是 nil——
// 若不判空，Publish 会在第一次调 client 的地方空指针崩溃，把一次配置
// 缺失变成整个评审流程的 panic。
func TestGerritAdapterWithoutClient(t *testing.T) {
	adapter := NewGerritAdapter(func() *gerrit.Client { return nil })

	_, err := adapter.Publish(context.Background(), Review{
		ChangeRequest: ChangeRequest{
			Provider:   "gerrit",
			Repository: "/my-project",
			Number:     "123",
			HeadSHA:    "abc123",
		},
		Summary: "评审摘要",
	})
	if err == nil {
		t.Fatal("未配置 gerrit 客户端时应返回错误")
	}
}

// TestGerritAdapterWithoutGetter 钉住「适配器没接入客户端来源时也不 panic」。
func TestGerritAdapterWithoutGetter(t *testing.T) {
	adapter := NewGerritAdapter(nil)

	if _, err := adapter.Publish(context.Background(), Review{
		ChangeRequest: ChangeRequest{Repository: "/my-project", Number: "123"},
	}); err == nil {
		t.Fatal("未接入客户端来源时应返回错误")
	}
}

// TestGerritAdapterCapabilities 钉住 Gerrit 的能力声明。
//
// Gerrit 用 label 投票代替 commit status，也不支持 GitHub 式 suggestion。
func TestGerritAdapterCapabilities(t *testing.T) {
	adapter := NewGerritAdapter(func() *gerrit.Client { return nil })

	caps, err := adapter.Capabilities(context.Background(), ChangeRequest{})
	if err != nil {
		t.Fatalf("取能力不应失败: %v", err)
	}
	if !caps.BatchReview || !caps.MultiLineComments {
		t.Fatalf("Gerrit 应支持批量评审与行内评论: %+v", caps)
	}
	if caps.Suggestions {
		t.Fatal("Gerrit 不支持 GitHub-style suggestion")
	}
}
