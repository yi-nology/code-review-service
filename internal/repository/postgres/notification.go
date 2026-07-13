package postgres

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/notification"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type NotificationRepo struct {
	db *gormLib.DB
}

func NewNotificationRepo(db *gormLib.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

func (r *NotificationRepo) Create(ctx context.Context, item *notification.Item) error {
	m := notificationToModel(item)
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	item.ID = m.ID
	item.CreatedAt = m.CreatedAt
	return nil
}

func (r *NotificationRepo) List(ctx context.Context, opts notification.ListOptions) ([]notification.Item, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Notification{})
	if opts.OnlyUnread {
		q = q.Where("read = false")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	var rows []models.Notification
	if err := q.Order("created_at DESC").Limit(limit).Offset(opts.Offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]notification.Item, len(rows))
	for i, m := range rows {
		out[i] = notificationToDomain(m)
	}
	return out, total, nil
}

func (r *NotificationRepo) MarkRead(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ?", id).Update("read", true).Error
}

func (r *NotificationRepo) MarkAllRead(ctx context.Context) error {
	return r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("read = false").Update("read", true).Error
}

func (r *NotificationRepo) UnreadCount(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("read = false").Count(&n).Error
	return n, err
}

func notificationToModel(item *notification.Item) models.Notification {
	return models.Notification{
		ID:        item.ID,
		ReviewID:  item.ReviewID,
		Kind:      string(item.Kind),
		Title:     item.Title,
		Body:      item.Body,
		Read:      item.Read,
		Provider:  item.Provider,
		Repo:      item.Repo,
		CreatedAt: item.CreatedAt,
	}
}

func notificationToDomain(m models.Notification) notification.Item {
	return notification.Item{
		ID:        m.ID,
		ReviewID:  m.ReviewID,
		Kind:      notification.Kind(m.Kind),
		Title:     m.Title,
		Body:      m.Body,
		Read:      m.Read,
		Provider:  m.Provider,
		Repo:      m.Repo,
		CreatedAt: m.CreatedAt,
	}
}
