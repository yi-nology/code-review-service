package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/session"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	ekitGorm "github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
	"github.com/oekit/pkg/pagination"
	"github.com/oekit/pkg/uidgen"
	"gorm.io/datatypes"
	gormLib "gorm.io/gorm"
)

// PostgresSessionRepository implements repository.SessionRepository with PostgreSQL
type PostgresSessionRepository struct {
	db *gormLib.DB
}

// NewPostgresSessionRepository creates a new PostgreSQL session repository
func NewPostgresSessionRepository(db *gormLib.DB) *PostgresSessionRepository {
	return &PostgresSessionRepository{db: db}
}

// Create creates a new session
func (r *PostgresSessionRepository) Create(ctx context.Context, s *session.Session) error {
	m := sessionToModel(s)
	if m.SessionID == "" {
		m.SessionID = uidgen.UUID()
	}
	if m.Status == "" {
		m.Status = session.StatusActive
	}
	if m.Phase == "" {
		m.Phase = session.PhaseContext
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	// 回填生成的 SessionID 与默认值给调用方。
	s.SessionID = m.SessionID
	s.Status = m.Status
	s.Phase = m.Phase
	return nil
}

// GetByID returns a session by ID
func (r *PostgresSessionRepository) GetByID(ctx context.Context, sessionID string) (*session.Session, error) {
	var m models.ReviewSession
	err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		First(&m).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	d := sessionToDomain(m)
	return &d, nil
}

// GetByReviewID returns sessions for a review
func (r *PostgresSessionRepository) GetByReviewID(ctx context.Context, reviewID string) ([]*session.Session, error) {
	var ms []models.ReviewSession
	if err := r.db.WithContext(ctx).
		Where("review_id = ?", reviewID).
		Order("round DESC, created_at DESC").
		Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*session.Session, len(ms))
	for i, m := range ms {
		d := sessionToDomain(m)
		out[i] = &d
	}
	return out, nil
}

// GetActiveByReviewID returns the active session for a review
func (r *PostgresSessionRepository) GetActiveByReviewID(ctx context.Context, reviewID string) (*session.Session, error) {
	var m models.ReviewSession
	err := r.db.WithContext(ctx).
		Where("review_id = ? AND status = ?", reviewID, session.StatusActive).
		Order("round DESC").
		First(&m).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	d := sessionToDomain(m)
	return &d, nil
}

// Update updates a session. 按 SessionID 定位现有行（domain Session 不携带 gorm 自增 ID），
// 保留主键与 CreatedAt 后整体 Save，避免 ID=0 时误创建重复行。
func (r *PostgresSessionRepository) Update(ctx context.Context, s *session.Session) error {
	var existing models.ReviewSession
	if err := r.db.WithContext(ctx).Where("session_id = ?", s.SessionID).First(&existing).Error; err != nil {
		return err
	}
	m := sessionToModel(s)
	m.ID = existing.ID
	m.CreatedAt = existing.CreatedAt
	return r.db.WithContext(ctx).Save(&m).Error
}

// UpdatePhase updates the phase of a session
func (r *PostgresSessionRepository) UpdatePhase(ctx context.Context, sessionID string, phase string, progress float64) error {
	return r.db.WithContext(ctx).
		Model(&models.ReviewSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]interface{}{
			"phase":    phase,
			"progress": progress,
		}).Error
}

// UpdateStatus updates the status of a session
func (r *PostgresSessionRepository) UpdateStatus(ctx context.Context, sessionID string, status string) error {
	return r.db.WithContext(ctx).
		Model(&models.ReviewSession{}).
		Where("session_id = ?", sessionID).
		Update("status", status).Error
}

// UpdateAgentResults updates the agent results of a session
func (r *PostgresSessionRepository) UpdateAgentResults(ctx context.Context, sessionID string, agentResults []session.AgentResultData) error {
	data, err := json.Marshal(agentResults)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&models.ReviewSession{}).
		Where("session_id = ?", sessionID).
		Update("agent_results", datatypes.JSON(data)).Error
}

// UpdateDiscussion updates the discussion of a session
func (r *PostgresSessionRepository) UpdateDiscussion(ctx context.Context, sessionID string, discussion session.DiscussionData) error {
	data, err := json.Marshal(discussion)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&models.ReviewSession{}).
		Where("session_id = ?", sessionID).
		Update("discussion", datatypes.JSON(data)).Error
}

// UpdateFinalReport updates the final report of a session
func (r *PostgresSessionRepository) UpdateFinalReport(ctx context.Context, sessionID string, finalReport session.FinalReportData) error {
	data, err := json.Marshal(finalReport)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&models.ReviewSession{}).
		Where("session_id = ?", sessionID).
		Update("final_report", datatypes.JSON(data)).Error
}

// Complete marks a session as completed
func (r *PostgresSessionRepository) Complete(ctx context.Context, sessionID string, finalReport session.FinalReportData) error {
	now := time.Now()
	data, err := json.Marshal(finalReport)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&models.ReviewSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]interface{}{
			"status":       session.StatusCompleted,
			"final_report": datatypes.JSON(data),
			"completed_at": now,
			"progress":     100,
		}).Error
}

// Delete deletes a session
func (r *PostgresSessionRepository) Delete(ctx context.Context, sessionID string) error {
	return r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Delete(&models.ReviewSession{}).Error
}

// CancelActiveByReviewID cancels all active sessions for a review.
func (r *PostgresSessionRepository) CancelActiveByReviewID(ctx context.Context, reviewID string) error {
	return r.db.WithContext(ctx).
		Model(&models.ReviewSession{}).
		Where("review_id = ? AND status = ?", reviewID, session.StatusActive).
		Update("status", session.StatusCancelled).Error
}

// List returns sessions with pagination
func (r *PostgresSessionRepository) List(ctx context.Context, page, pageSize int) (*ekitGorm.PageResponse[session.Session], error) {
	resp, err := ekitGorm.Paginate[models.ReviewSession](
		r.db.WithContext(ctx).Model(&models.ReviewSession{}),
		pagination.NewPageRequest(page, pageSize),
		func(db *gormLib.DB) *gormLib.DB {
			return db.Order("created_at DESC")
		},
	)
	if err != nil {
		return nil, err
	}
	out := &ekitGorm.PageResponse[session.Session]{
		Items: make([]session.Session, len(resp.Items)),
		Total: resp.Total,
	}
	for i, m := range resp.Items {
		out.Items[i] = sessionToDomain(m)
	}
	return out, nil
}
