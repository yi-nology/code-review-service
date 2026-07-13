package postgres

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
)

// TestProjectPolicyRepo_DeleteIsHardDeleteAndListVisible 验证 Delete 走 Unscoped
// 硬删除（不是软删），List 看到的是「真删后」的状态。
//
// fc9ba34 fix #6 之前用软删，List 看不到但 GetOrCreateDefault 走 NotFound 路径会
// 创建新记录，造成「UI 显示暂无 + DB 里有软删僵尸行 + 还能造新行」数据割裂。
// 硬删后整行消失，List/GetOrCreateDefault 行为一致。
func TestProjectPolicyRepo_DeleteIsHardDeleteAndListVisible(t *testing.T) {
	truncateTables(t, "project_policies")
	ctx := context.Background()
	repo := NewProjectPolicyRepo(testDB)

	// 准备 3 个策略：A 留着，B 删，C 留着
	mustSave(t, ctx, repo, "github", "repo-A", projectpolicy.NewDefault("github", "repo-A"))
	mustSave(t, ctx, repo, "github", "repo-B", projectpolicy.NewDefault("github", "repo-B"))
	mustSave(t, ctx, repo, "github", "repo-C", projectpolicy.NewDefault("github", "repo-C"))

	// 删 B
	if err := repo.Delete(ctx, "github", "repo-B"); err != nil {
		t.Fatalf("Delete B: %v", err)
	}

	// List 只能看到 A 和 C
	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	names := make(map[string]bool)
	for _, p := range got {
		names[p.Provider+"/"+p.Repo] = true
	}
	if names["github/repo-B"] {
		t.Fatal("List still contains deleted B — Delete is soft delete, must be hard")
	}
	if !names["github/repo-A"] || !names["github/repo-C"] {
		t.Fatalf("List missing A or C: %+v", names)
	}
	if len(got) != 2 {
		t.Fatalf("List len = %d, want 2 (A,C)", len(got))
	}
}

// TestProjectPolicyRepo_DeleteThenGetOrCreateDefault_CreatesFresh 验证硬删
// 后 GetOrCreateDefault 走「NotFound → 新建」路径而不是返回软删僵尸行。
// 这是 fc9ba34 fix #6 修复的真正问题：之前的软删会让 GetByProviderRepo 找不到，
// 走 NotFound → Create 新行，但 Delete 行还在 DB 里软躺着，UI 看「暂无」但 DB 有僵尸。
func TestProjectPolicyRepo_DeleteThenGetOrCreateDefault_CreatesFresh(t *testing.T) {
	truncateTables(t, "project_policies")
	ctx := context.Background()
	repo := NewProjectPolicyRepo(testDB)

	// 创建 → 删
	mustSave(t, ctx, repo, "gitlab", "evict-target", projectpolicy.NewDefault("gitlab", "evict-target"))
	if err := repo.Delete(ctx, "gitlab", "evict-target"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// GetOrCreateDefault 应该真 NotFound，然后新建
	got, err := repo.GetOrCreateDefault(ctx, "gitlab", "evict-target")
	if err != nil {
		t.Fatalf("GetOrCreateDefault: %v", err)
	}
	if got.Provider != "gitlab" || got.Repo != "evict-target" {
		t.Fatalf("got policy = %+v", got)
	}
	// List 现在应该 1 条
	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List len = %d, want 1 (no zombie)", len(all))
	}
}

// TestProjectPolicyRepo_DeleteNonExistent_ReturnsNotFound 验证删不存在的策略返
// 回 ErrRecordNotFound（不让调用方以为成功了）。
func TestProjectPolicyRepo_DeleteNonExistent_ReturnsNotFound(t *testing.T) {
	truncateTables(t, "project_policies")
	ctx := context.Background()
	repo := NewProjectPolicyRepo(testDB)

	err := repo.Delete(ctx, "github", "never-existed")
	if err == nil {
		t.Fatal("Delete non-existent should return error")
	}
}

func mustSave(t *testing.T, ctx context.Context, repo *ProjectPolicyRepo, provider, repoName string, p projectpolicy.ProjectPolicy) {
	t.Helper()
	p.Provider = provider
	p.Repo = repoName
	if err := repo.Save(ctx, &p); err != nil {
		t.Fatalf("Save %s/%s: %v", provider, repoName, err)
	}
}
