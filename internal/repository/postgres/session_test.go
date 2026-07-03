package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"

	"gorm.io/datatypes"
)

func newSession(reviewID string, round int) *models.ReviewSession {
	return &models.ReviewSession{
		ReviewID: reviewID,
		Round:    round,
	}
}

func TestSessionRepo_CreateAndGetByID(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewPostgresSessionRepository(testDB)

	s := newSession("rev-1", 1)
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.SessionID == "" {
		t.Fatal("Create should auto-generate SessionID")
	}
	if s.Status != string(models.SessionStatusActive) {
		t.Fatalf("default status = %q, want %q", s.Status, models.SessionStatusActive)
	}
	if s.Phase != string(models.SessionPhaseContext) {
		t.Fatalf("default phase = %q, want %q", s.Phase, models.SessionPhaseContext)
	}

	got, err := repo.GetByID(ctx, s.SessionID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ReviewID != "rev-1" || got.Round != 1 {
		t.Fatalf("unexpected session: %+v", got)
	}
}

func TestSessionRepo_GetByID_NotFound(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewPostgresSessionRepository(testDB)

	if _, err := repo.GetByID(ctx, "missing"); !errors.Is(err, repoerr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSessionRepo_GetByReviewID_Ordering(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewPostgresSessionRepository(testDB)

	for _, round := range []int{1, 2, 3} {
		if err := repo.Create(ctx, newSession("rev-x", round)); err != nil {
			t.Fatalf("Create round %d: %v", round, err)
		}
	}

	sessions, err := repo.GetByReviewID(ctx, "rev-x")
	if err != nil {
		t.Fatalf("GetByReviewID: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}
	// 应按 round DESC 排序。
	if sessions[0].Round != 3 {
		t.Fatalf("expected round 3 first, got %d", sessions[0].Round)
	}
}

func TestSessionRepo_GetActiveByReviewID(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewPostgresSessionRepository(testDB)

	active := newSession("rev-a", 1)
	if err := repo.Create(ctx, active); err != nil {
		t.Fatalf("Create active: %v", err)
	}
	completed := newSession("rev-a", 2)
	if err := repo.Create(ctx, completed); err != nil {
		t.Fatalf("Create completed: %v", err)
	}
	if err := repo.UpdateStatus(ctx, completed.SessionID, string(models.SessionStatusCompleted)); err != nil {
		t.Fatalf("mark completed: %v", err)
	}

	got, err := repo.GetActiveByReviewID(ctx, "rev-a")
	if err != nil {
		t.Fatalf("GetActiveByReviewID: %v", err)
	}
	if got.SessionID != active.SessionID {
		t.Fatalf("expected active session, got %s", got.SessionID)
	}

	// 全部完成后应 NotFound。
	if err := repo.UpdateStatus(ctx, active.SessionID, string(models.SessionStatusCompleted)); err != nil {
		t.Fatalf("complete active: %v", err)
	}
	if _, err := repo.GetActiveByReviewID(ctx, "rev-a"); !errors.Is(err, repoerr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound when no active session, got %v", err)
	}
}

// TestSessionRepo_JSONBPayloads 验证 4 个 interface{} payload 方法的 JSONB 往返。
// 这是 Phase 5 要强类型化的方法，集成测试先锁住其当前行为。
func TestSessionRepo_JSONBPayloads(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewPostgresSessionRepository(testDB)

	s := newSession("rev-p", 1)
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	agentResults := []map[string]any{{"agent": "security", "findings": 3}}
	discussion := map[string]any{"consensus": 2, "disagreements": 1}
	finalReport := map[string]any{"summary": "ok", "score": 90}

	if err := repo.UpdateAgentResults(ctx, s.SessionID, agentResults); err != nil {
		t.Fatalf("UpdateAgentResults: %v", err)
	}
	if err := repo.UpdateDiscussion(ctx, s.SessionID, discussion); err != nil {
		t.Fatalf("UpdateDiscussion: %v", err)
	}

	// Complete 同时写 final_report + 置状态。
	if err := repo.Complete(ctx, s.SessionID, finalReport); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got, err := repo.GetByID(ctx, s.SessionID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != string(models.SessionStatusCompleted) {
		t.Fatalf("status = %q, want completed", got.Status)
	}
	if got.Progress != 100 {
		t.Fatalf("progress = %v, want 100", got.Progress)
	}
	if got.CompletedAt == nil {
		t.Fatal("CompletedAt should be set")
	}
	// final_report JSONB 应可反序列化回结构体。
	if len(got.FinalReport) == 0 || string(got.FinalReport) == "null" {
		t.Fatalf("final_report not persisted: %s", got.FinalReport)
	}
	if len(got.AgentResults) == 0 || string(got.AgentResults) == "null" {
		t.Fatalf("agent_results not persisted: %s", got.AgentResults)
	}
}

func TestSessionRepo_UpdatePhaseAndStatus(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewPostgresSessionRepository(testDB)

	s := newSession("rev-u", 1)
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdatePhase(ctx, s.SessionID, string(models.SessionPhaseAnalysis), 55.5); err != nil {
		t.Fatalf("UpdatePhase: %v", err)
	}
	got, _ := repo.GetByID(ctx, s.SessionID)
	if got.Phase != string(models.SessionPhaseAnalysis) || got.Progress != 55.5 {
		t.Fatalf("phase/progress not updated: phase=%q progress=%v", got.Phase, got.Progress)
	}

	if err := repo.UpdateStatus(ctx, s.SessionID, string(models.SessionStatusCancelled)); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, _ = repo.GetByID(ctx, s.SessionID)
	if got.Status != string(models.SessionStatusCancelled) {
		t.Fatalf("status = %q, want cancelled", got.Status)
	}
}

func TestSessionRepo_CancelActiveByReviewID(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewPostgresSessionRepository(testDB)

	a1 := newSession("rev-c", 1)
	a2 := newSession("rev-c", 2)
	if err := repo.Create(ctx, a1); err != nil {
		t.Fatalf("Create a1: %v", err)
	}
	if err := repo.Create(ctx, a2); err != nil {
		t.Fatalf("Create a2: %v", err)
	}

	// 取消该 review 的所有 active session。
	if err := repo.CancelActiveByReviewID(ctx, "rev-c"); err != nil {
		t.Fatalf("CancelActiveByReviewID: %v", err)
	}
	sessions, _ := repo.GetByReviewID(ctx, "rev-c")
	for _, s := range sessions {
		if s.Status != string(models.SessionStatusCancelled) {
			t.Fatalf("session %s status = %q, want cancelled", s.SessionID, s.Status)
		}
	}
	// 取消后不应再有 active。
	if _, err := repo.GetActiveByReviewID(ctx, "rev-c"); !errors.Is(err, repoerr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after cancel, got %v", err)
	}
}

func TestSessionRepo_Update(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewPostgresSessionRepository(testDB)

	s := newSession("rev-upd", 1)
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	s.Progress = 80
	s.AgentResults = datatypes.JSON([]byte(`{"agents":["a","b"]}`))
	if err := repo.Update(ctx, s); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(ctx, s.SessionID)
	if got.Progress != 80 {
		t.Fatalf("progress = %v, want 80", got.Progress)
	}
	// postgres jsonb 会规范化空白/键序，解析后语义校验。
	var ar struct {
		Agents []string `json:"agents"`
	}
	if err := json.Unmarshal(got.AgentResults, &ar); err != nil {
		t.Fatalf("agent_results not valid JSON: %v", err)
	}
	if len(ar.Agents) != 2 || ar.Agents[0] != "a" || ar.Agents[1] != "b" {
		t.Fatalf("agent_results agents = %v", ar.Agents)
	}
}
