package postgres

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/reviewer"
)

// TestReviewerAgentRepo_CreateBackfillsID 验证 Create 之后 agent.ID 被回填为新行的主键。
// 这是 fc9ba34 的 fix #5：之前只回填 m.ID，caller 的 agent.ID 仍是 0，
// 导致 handler 返回的 response.agent.id=0，前端拿不到新 ID。
func TestReviewerAgentRepo_CreateBackfillsID(t *testing.T) {
	truncateTables(t, "reviewer_agents")
	ctx := context.Background()
	repo := NewReviewerAgentRepo(testDB)

	agent := &reviewer.Agent{
		Name:        "e2e-test-agent",
		Description: "test agent for ID backfill e2e",
		Enabled:     true,
		Priority:    100,
	}
	if agent.ID != 0 {
		t.Fatalf("precondition: agent.ID = %d, want 0", agent.ID)
	}

	if err := repo.Create(ctx, agent); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if agent.ID == 0 {
		t.Fatalf("Create did not backfill agent.ID; got %d, want non-zero", agent.ID)
	}

	// 二次校验：通过 GetByName 查到的 ID 必须一致
	got, err := repo.GetByName(ctx, "e2e-test-agent")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.ID != agent.ID {
		t.Fatalf("GetByName ID = %d, want %d (the backfilled ID)", got.ID, agent.ID)
	}
}

// TestReviewerAgentRepo_Create_ResurrectingSoftDeletedAlsoBackfillsID 验证「同
// 名软删除复活」路径也回填 agent.ID。这条路径在 line 57-65 走 Save 不是 Create，
// 之前漏配会让复活场景的 response 仍返 id=0。
func TestReviewerAgentRepo_Create_ResurrectingSoftDeletedAlsoBackfillsID(t *testing.T) {
	truncateTables(t, "reviewer_agents")
	ctx := context.Background()
	repo := NewReviewerAgentRepo(testDB)

	// 第一次创建
	first := &reviewer.Agent{Name: "resurrect-test", Enabled: true, Priority: 50}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	originalID := first.ID
	if originalID == 0 {
		t.Fatal("first Create should backfill ID")
	}

	// 软删除
	if err := repo.Delete(ctx, originalID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// 再次用同名创建（caller 不指定 ID）→ 走 Unscoped 复活路径
	second := &reviewer.Agent{Name: "resurrect-test", Enabled: true, Priority: 60}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("second Create (resurrect): %v", err)
	}
	if second.ID != originalID {
		t.Fatalf("resurrect should reuse original ID: got %d, want %d", second.ID, originalID)
	}
}
