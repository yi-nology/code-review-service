package repository

import (
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
	ContributorStats ContributorStatsRepository
	ReviewerAgent    ReviewerAgentRepository
	ProjectAgent     ProjectAgentRepository
	ReviewMap        ReviewMapRepository
	CustomProvider   CustomProviderRepository
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
		ReviewerAgent:    postgres.NewReviewerAgentRepo(db),
		ProjectAgent:     postgres.NewProjectAgentRepo(db),
		ReviewMap:        postgres.NewReviewMapRepo(db),
		CustomProvider:   postgres.NewCustomProviderRepo(db),
	}
}
