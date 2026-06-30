package modules

import (
	"github.com/yi-nology/code-review-service/biz/handler/diagnostics_service"
	"github.com/yi-nology/code-review-service/biz/handler/feedback_service"
	"github.com/yi-nology/code-review-service/biz/handler/progress_service"
	"github.com/yi-nology/code-review-service/biz/handler/project_agent_service"
	"github.com/yi-nology/code-review-service/biz/handler/publisher_service"
	"github.com/yi-nology/code-review-service/biz/handler/review_service"
	"github.com/yi-nology/code-review-service/biz/handler/session_service"
	"github.com/yi-nology/code-review-service/biz/handler/settings_service"
	"github.com/yi-nology/code-review-service/biz/handler/stats_service"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/diagnostics"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/session"
	statsSvc "github.com/yi-nology/code-review-service/internal/service/stats"
	"github.com/yi-nology/code-review-service/internal/service/webhook"
	"github.com/yi-nology/code-review-service/internal/service/writeback"

	"go.uber.org/fx"
)

var HandlerModule = fx.Options(
	fx.Invoke(InitReviewHandler),
	fx.Invoke(InitWebhookHandler),
	fx.Invoke(InitSettingsHandler),
	fx.Invoke(InitStatsHandler),
	fx.Invoke(InitSessionHandler),
	fx.Invoke(InitProgressHandler),
	fx.Invoke(InitPublisherHandler),
	fx.Invoke(InitProjectAgentHandler),
	fx.Invoke(InitDiagnosticsHandler),
	fx.Invoke(InitFeedbackHandler),
)

func InitReviewHandler(reviewSvc *svcReview.Service, gitMgr *gitProvider.GitProviderService) {
	review_service.InitHandler(reviewSvc, gitMgr)
}

func InitWebhookHandler(webhookSvc *webhook.Service, reviewSvc *svcReview.Service, repos *repository.RepoContainer) {
	review_service.InitWebhookHandler(webhookSvc, reviewSvc, repos.GitPlatform)
}

// InitSettingsHandler 收口为 3 个参数：repos + gitMgr + *ocr.Registry。
// 之前 14 个参数；OCR 子能力（tester/lister/checker/versioner/viewer/upgrader）
// 全部从 *ocr.Registry 拿。
func InitSettingsHandler(
	repos *repository.RepoContainer,
	gitMgr *gitProvider.GitProviderService,
	ocrReg *ocr.Registry,
) {
	settings_service.InitHandler(repos, gitMgr, ocrReg)
}

func InitStatsHandler(repos *repository.RepoContainer) {
	stats_service.InitHandler(statsSvc.NewService(repos.ContributorStats, repos.Review))
}

func InitSessionHandler(sessionSvc *session.SessionService) {
	session_service.InitHandler(sessionSvc)
}

func InitProgressHandler(progressMgr *progress.ProgressManager) {
	progress_service.InitHandler(progressMgr)
}

func InitPublisherHandler(writebackOrch *writeback.Orchestrator, reviewSvc *svcReview.Service, providerSvc *gitProvider.GitProviderService) {
	publisher_service.InitHandler(writebackOrch, reviewSvc, providerSvc)
}

func InitProjectAgentHandler(repos *repository.RepoContainer) {
	project_agent_service.InitHandler(repos.ProjectAgent, repos.ReviewerAgent)
}

func InitDiagnosticsHandler(repos *repository.RepoContainer) {
	diagnostics_service.InitHandler(diagnostics.NewService(repos.WebhookDelivery, repos.WritebackItem))
}

func InitFeedbackHandler(repos *repository.RepoContainer) {
	feedback_service.InitHandler(repos.FindingFeedback)
}
