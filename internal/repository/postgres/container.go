package postgres

import (
	"github.com/yi-nology/code-review-service/internal/repository"

	gormDB "gorm.io/gorm"
)

// NewRepoContainer 构造全部仓储实现（装配层入口）。
//
// 它住在**实现包**而不是接口包：接口包一旦 import 实现包，依赖方向就倒了
// （domain → impl → infra），而且会让所有间接依赖仓储接口的包（如 internal/ocr）
// 传递依赖上 GORM 实现。容器**类型**仍留在 internal/repository（它只是接口的
// 聚合），因此消费方继续依赖接口包即可，不必直连实现包。
func NewRepoContainer(db *gormDB.DB) *repository.RepoContainer {
	return &repository.RepoContainer{
		Review:           NewReviewRepo(db),
		OCRSettings:      NewOCRSettingsRepo(db),
		ProjectPolicy:    NewProjectPolicyRepo(db),
		GitPlatform:      NewGitPlatformRepo(db),
		WebhookDelivery:  NewWebhookDeliveryRepo(db),
		ChangeRevision:   NewChangeRevisionRepo(db),
		WritebackItem:    NewWritebackItemRepo(db),
		GlobalRules:      NewGlobalRulesRepo(db),
		OCRRulePackage:   NewOCRRulePackageRepo(db),
		OCRToolPackage:   NewOCRToolPackageRepo(db),
		ContributorStats: NewContributorStatsRepo(db),
		ReviewerAgent:    NewReviewerAgentRepo(db),
		ProjectAgent:     NewProjectAgentRepo(db),
		CustomProvider:   NewCustomProviderRepo(db),
		FindingFeedback:  NewFindingFeedbackRepo(db),
		Notification:     NewNotificationRepo(db),
		Auth:             NewAuthRepo(db),
		ArgusReview:      NewArgusReviewRepo(db),
	}
}
