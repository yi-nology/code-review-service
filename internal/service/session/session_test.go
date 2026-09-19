package session

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/page"

	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
)

// mockSessionRepo 实现 repository.SessionRepository，用于单元测试
type mockSessionRepo struct {
	sessions  map[string]*dsession.Session   // sessionID -> session
	byReview  map[string][]*dsession.Session // reviewID -> sessions
	getErr    error
	createErr error
	updateErr error
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{
		sessions: make(map[string]*dsession.Session),
		byReview: make(map[string][]*dsession.Session),
	}
}

func (m *mockSessionRepo) GetByID(_ context.Context, sessionID string) (*dsession.Session, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, repoerr.ErrNotFound
	}
	return s, nil
}

func (m *mockSessionRepo) GetByReviewID(_ context.Context, reviewID string) ([]*dsession.Session, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.byReview[reviewID], nil
}

func (m *mockSessionRepo) GetActiveByReviewID(_ context.Context, reviewID string) (*dsession.Session, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, s := range m.sessions {
		if s.ReviewID == reviewID && s.Status == string(dsession.StatusActive) {
			return s, nil
		}
	}
	return nil, repoerr.ErrNotFound
}

func (m *mockSessionRepo) Create(_ context.Context, session *dsession.Session) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.sessions[session.SessionID] = session
	m.byReview[session.ReviewID] = append(m.byReview[session.ReviewID], session)
	return nil
}

func (m *mockSessionRepo) Update(_ context.Context, session *dsession.Session) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.sessions[session.SessionID] = session
	return nil
}

func (m *mockSessionRepo) UpdatePhase(_ context.Context, sessionID string, phase string, progress float64) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if s, ok := m.sessions[sessionID]; ok {
		s.Phase = phase
		s.Progress = progress
	}
	return nil
}

func (m *mockSessionRepo) UpdateStatus(_ context.Context, sessionID string, status string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if s, ok := m.sessions[sessionID]; ok {
		s.Status = status
	}
	return nil
}

func (m *mockSessionRepo) UpdateAgentResults(_ context.Context, _ string, _ []dsession.AgentResultData) error {
	return m.updateErr
}

func (m *mockSessionRepo) UpdateDiscussion(_ context.Context, _ string, _ dsession.DiscussionData) error {
	return m.updateErr
}

func (m *mockSessionRepo) UpdateFinalReport(_ context.Context, _ string, _ dsession.FinalReportData) error {
	return m.updateErr
}

func (m *mockSessionRepo) Complete(_ context.Context, sessionID string, _ dsession.FinalReportData) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if s, ok := m.sessions[sessionID]; ok {
		s.Status = string(dsession.StatusCompleted)
	}
	return nil
}

func (m *mockSessionRepo) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *mockSessionRepo) CancelActiveByReviewID(_ context.Context, _ string) error {
	return nil
}

func (m *mockSessionRepo) List(_ context.Context, _, _ int) (*page.Page[dsession.Session], error) {
	return nil, nil
}

func newTestSession(reviewID, sessionID, status, phase string) *dsession.Session {
	return &dsession.Session{
		SessionID: sessionID,
		ReviewID:  reviewID,
		Round:     1,
		Phase:     phase,
		Status:    status,
		Progress:  0,
		Metadata:  json.RawMessage([]byte("{}")),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ==================== CreateSession Tests ====================

func TestCreateSession_Success(t *testing.T) {
	repo := newMockSessionRepo()
	svc := NewSessionService(repo)

	session, err := svc.CreateSession(context.Background(), "review-1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ReviewID != "review-1" {
		t.Errorf("expected reviewID review-1, got %s", session.ReviewID)
	}
	if session.Round != 1 {
		t.Errorf("expected round 1, got %d", session.Round)
	}
	if session.Status != string(dsession.StatusActive) {
		t.Errorf("expected status active, got %s", session.Status)
	}
	if session.Phase != string(dsession.PhaseContext) {
		t.Errorf("expected phase context, got %s", session.Phase)
	}
}

func TestCreateSession_AutoCancelsActiveSession(t *testing.T) {
	repo := newMockSessionRepo()
	// 预先创建一个 active session
	repo.sessions["existing"] = newTestSession("review-1", "existing", string(dsession.StatusActive), string(dsession.PhaseContext))

	svc := NewSessionService(repo)
	newSess, err := svc.CreateSession(context.Background(), "review-1", 1)
	if err != nil {
		t.Fatalf("expected success (auto-cancel old session), got error: %v", err)
	}
	if newSess == nil {
		t.Fatal("expected new session to be created")
	}
	// 旧 session 应被取消
	old := repo.sessions["existing"]
	if old.Status != dsession.StatusCancelled {
		t.Errorf("expected old session status=cancelled, got %s", old.Status)
	}
}

func TestCreateSession_ErrNotFoundFromGetActive(t *testing.T) {
	// 这是修复的关键测试：GetActiveByReviewID 返回 ErrNotFound 不应阻塞创建
	repo := newMockSessionRepo()
	svc := NewSessionService(repo)

	session, err := svc.CreateSession(context.Background(), "review-1", 1)
	if err != nil {
		t.Fatalf("ErrNotFound from GetActiveByReviewID should not block CreateSession: %v", err)
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
}

func TestCreateSession_GetActiveReturnsOtherError(t *testing.T) {
	repo := newMockSessionRepo()
	repo.getErr = fmt.Errorf("database connection lost")

	svc := NewSessionService(repo)
	_, err := svc.CreateSession(context.Background(), "review-1", 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "check active session: database connection lost" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateSession_CreateFails(t *testing.T) {
	repo := newMockSessionRepo()
	repo.createErr = fmt.Errorf("insert failed")

	svc := NewSessionService(repo)
	_, err := svc.CreateSession(context.Background(), "review-1", 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "create session: insert failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== GetSession Tests ====================

func TestGetSession_Success(t *testing.T) {
	repo := newMockSessionRepo()
	repo.sessions["s1"] = newTestSession("review-1", "s1", string(dsession.StatusActive), string(dsession.PhaseContext))

	svc := NewSessionService(repo)
	session, err := svc.GetSession(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.SessionID != "s1" {
		t.Errorf("expected sessionID s1, got %s", session.SessionID)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	repo := newMockSessionRepo()
	svc := NewSessionService(repo)

	_, err := svc.GetSession(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "session not found: nonexistent" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetSession_OtherError(t *testing.T) {
	repo := newMockSessionRepo()
	repo.getErr = fmt.Errorf("db error")

	svc := NewSessionService(repo)
	_, err := svc.GetSession(context.Background(), "s1")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "db error" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== GetActiveSession Tests ====================

func TestGetActiveSession_Success(t *testing.T) {
	repo := newMockSessionRepo()
	repo.sessions["s1"] = newTestSession("review-1", "s1", string(dsession.StatusActive), string(dsession.PhaseContext))

	svc := NewSessionService(repo)
	session, err := svc.GetActiveSession(context.Background(), "review-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.SessionID != "s1" {
		t.Errorf("expected s1, got %s", session.SessionID)
	}
}

func TestGetActiveSession_NotFound_ReturnsNil(t *testing.T) {
	repo := newMockSessionRepo()
	svc := NewSessionService(repo)

	session, err := svc.GetActiveSession(context.Background(), "review-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session != nil {
		t.Errorf("expected nil, got %v", session)
	}
}

func TestGetActiveSession_OtherError(t *testing.T) {
	repo := newMockSessionRepo()
	repo.getErr = fmt.Errorf("connection timeout")

	svc := NewSessionService(repo)
	_, err := svc.GetActiveSession(context.Background(), "review-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "connection timeout" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== UpdatePhase Tests ====================

func TestUpdatePhase_Success(t *testing.T) {
	repo := newMockSessionRepo()
	repo.sessions["s1"] = newTestSession("review-1", "s1", string(dsession.StatusActive), string(dsession.PhaseContext))

	svc := NewSessionService(repo)
	err := svc.UpdatePhase(context.Background(), "s1", dsession.PhaseChangeContext, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.sessions["s1"].Phase != string(dsession.PhaseChangeContext) {
		t.Errorf("expected phase change_context, got %s", repo.sessions["s1"].Phase)
	}
}

func TestUpdatePhase_SessionNotFound(t *testing.T) {
	repo := newMockSessionRepo()
	svc := NewSessionService(repo)

	err := svc.UpdatePhase(context.Background(), "nonexistent", dsession.PhaseContext, 50)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "session not found: nonexistent" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdatePhase_InvalidTransition(t *testing.T) {
	repo := newMockSessionRepo()
	repo.sessions["s1"] = newTestSession("review-1", "s1", string(dsession.StatusActive), string(dsession.PhaseContext))

	svc := NewSessionService(repo)
	// context → synthesis 是无效跳转（需要经过 change_context → analysis → reviews → aggregation）
	err := svc.UpdatePhase(context.Background(), "s1", dsession.PhaseSynthesis, 100)
	if err == nil {
		t.Fatal("expected error for invalid phase transition")
	}
	expected := "invalid phase transition from context to synthesis"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestUpdatePhase_AllValidTransitions(t *testing.T) {
	transitions := []struct {
		from string
		to   string
	}{
		{dsession.PhaseContext, dsession.PhaseChangeContext},
		{dsession.PhaseChangeContext, dsession.PhaseAnalysis},
		{dsession.PhaseAnalysis, dsession.PhaseReviews},
		{dsession.PhaseReviews, dsession.PhaseAggregation},
		{dsession.PhaseAggregation, dsession.PhaseDiscussion},
		{dsession.PhaseAggregation, dsession.PhaseSynthesis},
		{dsession.PhaseDiscussion, dsession.PhaseSynthesis},
		{dsession.PhaseSynthesis, dsession.PhaseComplete},
	}

	for _, tt := range transitions {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			repo := newMockSessionRepo()
			repo.sessions["s1"] = newTestSession("review-1", "s1", string(dsession.StatusActive), string(tt.from))

			svc := NewSessionService(repo)
			err := svc.UpdatePhase(context.Background(), "s1", tt.to, 50)
			if err != nil {
				t.Fatalf("unexpected error for %s -> %s: %v", tt.from, tt.to, err)
			}
		})
	}
}

// ==================== FailSession Tests ====================

func TestFailSession_Success(t *testing.T) {
	repo := newMockSessionRepo()
	repo.sessions["s1"] = newTestSession("review-1", "s1", string(dsession.StatusActive), string(dsession.PhaseContext))

	svc := NewSessionService(repo)
	err := svc.FailSession(context.Background(), "s1", "review execution failed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.sessions["s1"].Status != string(dsession.StatusFailed) {
		t.Errorf("expected status failed, got %s", repo.sessions["s1"].Status)
	}
}

func TestFailSession_NotFound(t *testing.T) {
	repo := newMockSessionRepo()
	svc := NewSessionService(repo)

	err := svc.FailSession(context.Background(), "nonexistent", "reason")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "session not found: nonexistent" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== GetNextRound Tests ====================

func TestGetNextRound_NoSessions(t *testing.T) {
	repo := newMockSessionRepo()
	svc := NewSessionService(repo)

	round, err := svc.GetNextRound(context.Background(), "review-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if round != 1 {
		t.Errorf("expected round 1, got %d", round)
	}
}

func TestGetNextRound_MultipleSessions(t *testing.T) {
	repo := newMockSessionRepo()
	repo.byReview["review-1"] = []*dsession.Session{
		newTestSession("review-1", "s1", string(dsession.StatusCompleted), string(dsession.PhaseContext)),
		newTestSession("review-1", "s2", string(dsession.StatusCompleted), string(dsession.PhaseContext)),
	}
	repo.byReview["review-1"][0].Round = 1
	repo.byReview["review-1"][1].Round = 2

	svc := NewSessionService(repo)
	round, err := svc.GetNextRound(context.Background(), "review-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if round != 3 {
		t.Errorf("expected round 3, got %d", round)
	}
}

// ==================== Phase Validation Tests ====================

func TestIsValidPhaseTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{dsession.PhaseContext, dsession.PhaseChangeContext, true},
		{dsession.PhaseContext, dsession.PhaseAnalysis, false},
		{dsession.PhaseContext, dsession.PhaseReviews, false},
		{dsession.PhaseChangeContext, dsession.PhaseAnalysis, true},
		{dsession.PhaseAnalysis, dsession.PhaseReviews, true},
		{dsession.PhaseReviews, dsession.PhaseAggregation, true},
		{dsession.PhaseAggregation, dsession.PhaseDiscussion, true},
		{dsession.PhaseAggregation, dsession.PhaseSynthesis, true},
		{dsession.PhaseDiscussion, dsession.PhaseSynthesis, true},
		{dsession.PhaseSynthesis, dsession.PhaseComplete, true},
		{dsession.PhaseComplete, dsession.PhaseContext, false},
		{dsession.PhaseComplete, dsession.PhaseSynthesis, false},
		{"unknown", dsession.PhaseContext, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			got := isValidPhaseTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("isValidPhaseTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
