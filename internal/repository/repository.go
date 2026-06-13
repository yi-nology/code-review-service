package repository

import (
	"context"
	"time"

	ekitGorm "github.com/oekit/gormx"
	"review-service/internal/repository/postgres"
	"review-service/internal/repository/postgres/models"

	gormDB "gorm.io/gorm"
)

type ReviewRepository interface {
	Create(ctx context.Context, review *models.Review) error
	Save(ctx context.Context, review *models.Review) error
	GetByReviewID(ctx context.Context, reviewID string) (*models.Review, error)
	GetByidempotencyKey(ctx context.Context, key string) (*models.Review, error)
	List(ctx context.Context, opts models.ListOptions) (*ekitGorm.PageResponse[models.Review], error)
	AggregateStats(ctx context.Context, opts models.ListOptions) (*models.AggregateStats, error)
	ClaimQueued(ctx context.Context, limit int) ([]models.Review, error)
	RequeueStale(ctx context.Context, before time.Time) (int64, error)
	UpdateInTransaction(ctx context.Context, reviewID string, fn func(review *models.Review) error) error
}

type OCRSettingsRepository interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
	Save(ctx context.Context, settings *models.OCRSettings) error
}

type ProjectPolicyRepository interface {
	List(ctx context.Context) ([]models.ProjectPolicy, error)
	GetByProviderRepo(ctx context.Context, provider, repo string) (*models.ProjectPolicy, error)
	Save(ctx context.Context, policy *models.ProjectPolicy) error
	Delete(ctx context.Context, provider, repo string) error
}

type GitPlatformRepository interface {
	List(ctx context.Context) ([]models.GitPlatform, error)
	Get(ctx context.Context, provider string) (*models.GitPlatform, error)
	Save(ctx context.Context, p *models.GitPlatform) error
	Delete(ctx context.Context, provider string) error
	ListEnabled(ctx context.Context) ([]models.GitPlatform, error)
	GetByProviderAndInstallation(ctx context.Context, provider, installationID string) (*models.GitPlatform, error)
}

type WebhookDeliveryRepository interface {
	Create(ctx context.Context, d *models.WebhookDelivery) error
	Get(ctx context.Context, provider, installationID, deliveryID string) (*models.WebhookDelivery, error)
	MarkProcessed(ctx context.Context, provider, installationID, deliveryID, status, errMsg string) error
}

type ChangeRevisionRepository interface {
	Upsert(ctx context.Context, cr *models.ChangeRevision) error
	GetByChange(ctx context.Context, provider, installationID, repositoryID string, changeNumber int) (*models.ChangeRevision, error)
	GetCurrentHeadSHA(ctx context.Context, provider, installationID, repositoryID string, changeNumber int) (string, error)
}

type WritebackItemRepository interface {
	Create(ctx context.Context, item *models.WritebackItem) error
	UpdateStatus(ctx context.Context, reviewID, findingKey, kind, status, externalID, providerURL, lastErr string) error
	IncrementAttempts(ctx context.Context, reviewID, findingKey, kind string) error
	ListByReview(ctx context.Context, reviewID string) ([]models.WritebackItem, error)
	ListRetryable(ctx context.Context, limit int) ([]models.WritebackItem, error)
}

type GlobalRulesRepository interface {
	Get(ctx context.Context) (*models.GlobalRules, error)
	Save(ctx context.Context, rules *models.GlobalRules) error
}

type ContributorStatsRepository interface {
	ListContributors(ctx context.Context, opts models.ContributorStatsOptions) (*models.ContributorListResult, error)
	GetContributor(ctx context.Context, opts models.ContributorStatsOptions) (*models.ContributorDetail, error)
	GetContributorTrend(ctx context.Context, opts models.ContributorStatsOptions) ([]models.TrendPoint, error)
	GetContributorCategories(ctx context.Context, opts models.ContributorStatsOptions) ([]models.CategoryStat, error)
}

type RepoContainer struct {
	Review          ReviewRepository
	OCRSettings     OCRSettingsRepository
	ProjectPolicy   ProjectPolicyRepository
	GitPlatform     GitPlatformRepository
	WebhookDelivery WebhookDeliveryRepository
	ChangeRevision  ChangeRevisionRepository
	WritebackItem   WritebackItemRepository
	GlobalRules     GlobalRulesRepository
	ContributorStats ContributorStatsRepository
}

func NewRepoContainer(db *gormDB.DB) *RepoContainer {
	return &RepoContainer{
		Review:           postgres.NewReviewRepo(db),
		OCRSettings:      postgres.NewOCRSettingsRepo(db),
		ProjectPolicy:    postgres.NewProjectPolicyRepo(db),
		GitPlatform:      postgres.NewGitPlatformRepo(db),
		WebhookDelivery:  postgres.NewWebhookDeliveryRepo(db),
		ChangeRevision:   postgres.NewChangeRevisionRepo(db),
		WritebackItem:    postgres.NewWritebackItemRepo(db),
		GlobalRules:      postgres.NewGlobalRulesRepo(db),
		ContributorStats: postgres.NewContributorStatsRepo(db),
	}
}
