package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	ekitGorm "github.com/oekit/gormx"
	"github.com/google/uuid"
	gormLib "gorm.io/gorm"
)

// SessionRepository defines the interface for session repository operations
type SessionRepository interface {
	// Create creates a new session
	Create(ctx context.Context, session *models.ReviewSession) error

	// GetByID returns a session by ID
	GetByID(ctx context.Context, sessionID string) (*models.ReviewSession, error)

	// GetByReviewID returns sessions for a review
	GetByReviewID(ctx context.Context, reviewID string) ([]*models.ReviewSession, error)

	// GetActiveByReviewID returns the active session for a review
	GetActiveByReviewID(ctx context.Context, reviewID string) (*models.ReviewSession, error)

	// Update updates a session
	Update(ctx context.Context, session *models.ReviewSession) error

	// UpdatePhase updates the phase of a session
	UpdatePhase(ctx context.Context, sessionID string, phase string, progress float64) error

	// UpdateStatus updates the status of a session
	UpdateStatus(ctx context.Context, sessionID string, status string) error

	// UpdateAgentResults updates the agent results of a session
	UpdateAgentResults(ctx context.Context, sessionID string, agentResults interface{}) error

	// UpdateDiscussion updates the discussion of a session
	UpdateDiscussion(ctx context.Context, sessionID string, discussion interface{}) error

	// UpdateFinalReport updates the final report of a session
	UpdateFinalReport(ctx context.Context, sessionID string, finalReport interface{}) error

	// Complete marks a session as completed
	Complete(ctx context.Context, sessionID string, finalReport interface{}) error

	// Delete deletes a session
	Delete(ctx context.Context, sessionID string) error

	// List returns sessions with pagination
	List(ctx context.Context, page, pageSize int) (*ekitGorm.PageResponse[models.ReviewSession], error)
}

// PostgresSessionRepository implements SessionRepository with PostgreSQL
type PostgresSessionRepository struct {
	db *gormLib.DB
}

// NewPostgresSessionRepository creates a new PostgreSQL session repository
func NewPostgresSessionRepository(db *gormLib.DB) *PostgresSessionRepository {
	return &PostgresSessionRepository{db: db}
}

// Create creates a new session
func (r *PostgresSessionRepository) Create(ctx context.Context, session *models.ReviewSession) error {
	if session.SessionID == "" {
		session.SessionID = uuid.New().String()
	}
	if session.Status == "" {
		session.Status = string(models.SessionStatusActive)
	}
	if session.Phase == "" {
		session.Phase = string(models.SessionPhaseContext)
	}
	return r.db.WithContext(ctx).Create(session).Error
}

// GetByID returns a session by ID
func (r *PostgresSessionRepository) GetByID(ctx context.Context, sessionID string) (*models.ReviewSession, error) {
	var session models.ReviewSession
	err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		First(&session).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, err
	}
	return &session, nil
}

// GetByReviewID returns sessions for a review
func (r *PostgresSessionRepository) GetByReviewID(ctx context.Context, reviewID string) ([]*models.ReviewSession, error) {
	var sessions []*models.ReviewSession
	err := r.db.WithContext(ctx).
		Where("review_id = ?", reviewID).
		Order("round DESC, created_at DESC").
		Find(&sessions).Error
	return sessions, err
}

// GetActiveByReviewID returns the active session for a review
func (r *PostgresSessionRepository) GetActiveByReviewID(ctx context.Context, reviewID string) (*models.ReviewSession, error) {
	var session models.ReviewSession
	err := r.db.WithContext(ctx).
		Where("review_id = ? AND status = ?", reviewID, models.SessionStatusActive).
		Order("round DESC").
		First(&session).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

// Update updates a session
func (r *PostgresSessionRepository) Update(ctx context.Context, session *models.ReviewSession) error {
	return r.db.WithContext(ctx).Save(session).Error
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
func (r *PostgresSessionRepository) UpdateAgentResults(ctx context.Context, sessionID string, agentResults interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.ReviewSession{}).
		Where("session_id = ?", sessionID).
		Update("agent_results", agentResults).Error
}

// UpdateDiscussion updates the discussion of a session
func (r *PostgresSessionRepository) UpdateDiscussion(ctx context.Context, sessionID string, discussion interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.ReviewSession{}).
		Where("session_id = ?", sessionID).
		Update("discussion", discussion).Error
}

// UpdateFinalReport updates the final report of a session
func (r *PostgresSessionRepository) UpdateFinalReport(ctx context.Context, sessionID string, finalReport interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.ReviewSession{}).
		Where("session_id = ?", sessionID).
		Update("final_report", finalReport).Error
}

// Complete marks a session as completed
func (r *PostgresSessionRepository) Complete(ctx context.Context, sessionID string, finalReport interface{}) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.ReviewSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]interface{}{
			"status":       models.SessionStatusCompleted,
			"final_report": finalReport,
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

// List returns sessions with pagination
func (r *PostgresSessionRepository) List(ctx context.Context, page, pageSize int) (*ekitGorm.PageResponse[models.ReviewSession], error) {
	return ekitGorm.Paginate[models.ReviewSession](
		r.db.WithContext(ctx),
		ekitGorm.NewPageRequest(page, pageSize),
		func(db *gormLib.DB) *gormLib.DB {
			return db.Order("created_at DESC")
		},
	)
}
