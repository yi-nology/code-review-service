package repository

import (
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository/postgres"

	gormDB "gorm.io/gorm"
)

type RepoContainer struct {
	Review           ReviewRepository
	OCRSettings      OCRSettingsRepository
	ProjectPolicy    ProjectPolicyRepository
	GitPlatform      GitPlatformRepository
	WebhookDelivery  WebhookDeliveryRepository
	ChangeRevision   ChangeRevisionRepository
	WritebackItem    WritebackItemRepository
	GlobalRules      GlobalRulesRepository
	OCRRulePackage   OCRRulePackageRepository
	OCRToolPackage   OCRToolPackageRepository
	ContributorStats ContributorStatsRepository
	ReviewerAgent    ReviewerAgentRepository
	ProjectAgent     ProjectAgentRepository
	CustomProvider   CustomProviderRepository
	FindingFeedback  FindingFeedbackRepository
	Notification     NotificationRepository
	Auth             AuthRepository
}

func NewRepoContainer(db *gormDB.DB, cfg *config.Config) *RepoContainer {
	return &RepoContainer{
		Review:           postgres.NewReviewRepo(db),
		OCRSettings:      postgres.NewOCRSettingsRepo(db, defaultOCRSettings(cfg)),
		ProjectPolicy:    postgres.NewProjectPolicyRepo(db),
		GitPlatform:      postgres.NewGitPlatformRepo(db),
		WebhookDelivery:  postgres.NewWebhookDeliveryRepo(db),
		ChangeRevision:   postgres.NewChangeRevisionRepo(db),
		WritebackItem:    postgres.NewWritebackItemRepo(db),
		GlobalRules:      postgres.NewGlobalRulesRepo(db),
		OCRRulePackage:   postgres.NewOCRRulePackageRepo(db),
		OCRToolPackage:   postgres.NewOCRToolPackageRepo(db),
		ContributorStats: postgres.NewContributorStatsRepo(db),
		ReviewerAgent:    postgres.NewReviewerAgentRepo(db),
		ProjectAgent:     postgres.NewProjectAgentRepo(db),
		CustomProvider:   postgres.NewCustomProviderRepo(db),
		FindingFeedback:  postgres.NewFindingFeedbackRepo(db),
		Notification:     postgres.NewNotificationRepo(db),
		Auth:             postgres.NewAuthRepo(db),
	}
}

func defaultOCRSettings(cfg *config.Config) postgres.OCRSettingsDefaults {
	defaults := postgres.OCRSettingsDefaults{}
	if cfg == nil {
		return defaults
	}
	defaults.Enabled = cfg.OCR.Enabled
	defaults.Binary = cfg.OCR.Binary
	defaults.ContextWindow = cfg.OCR.ContextWindow
	defaults.MaxOutputTokens = cfg.OCR.MaxOutputTokens
	return defaults
}
