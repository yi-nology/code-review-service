package repository

// container.go：全部仓储接口的聚合容器。
//
// 容器**类型**住在接口包——它只是一组接口的聚合，消费方（如 handler）依赖它
// 不必直连实现包。
//
// 构造器（NewRepoContainer）住在**实现包** internal/repository/postgres：接口包
// 一旦 import 实现包，依赖方向就倒了（domain → impl → infra），而且会让所有
// 间接依赖仓储接口的包（如 internal/ocr）传递依赖上 GORM 实现。
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
	ArgusReview      ArgusReviewRepository
}
