package modules

import (
	"github.com/yi-nology/code-review-service/biz/handler/argus_service"
	"github.com/yi-nology/code-review-service/biz/handler/auth_service"
	"github.com/yi-nology/code-review-service/biz/handler/cost_service"
	"github.com/yi-nology/code-review-service/biz/handler/diagnostics_service"
	"github.com/yi-nology/code-review-service/biz/handler/feedback_service"
	"github.com/yi-nology/code-review-service/biz/handler/notification_service"
	"github.com/yi-nology/code-review-service/biz/handler/progress_service"
	"github.com/yi-nology/code-review-service/biz/handler/project_agent_service"
	"github.com/yi-nology/code-review-service/biz/handler/publisher_service"
	"github.com/yi-nology/code-review-service/biz/handler/report_service"
	"github.com/yi-nology/code-review-service/biz/handler/review_service"
	"github.com/yi-nology/code-review-service/biz/handler/session_service"
	"github.com/yi-nology/code-review-service/biz/handler/settings_service"
	"github.com/yi-nology/code-review-service/biz/handler/stats_service"
	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/cost"
	"github.com/yi-nology/code-review-service/internal/service/diagnostics"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"
	reviewdomain "github.com/yi-nology/code-review-service/internal/service/review/domain"
	reviewenrich "github.com/yi-nology/code-review-service/internal/service/reviewenrich"
	"github.com/yi-nology/code-review-service/internal/service/session"
	statsSvc "github.com/yi-nology/code-review-service/internal/service/stats"
	"github.com/yi-nology/code-review-service/internal/service/statuscheck"
	"github.com/yi-nology/code-review-service/internal/service/suggestion"
	"github.com/yi-nology/code-review-service/internal/service/webhook"
	"github.com/yi-nology/code-review-service/internal/service/writeback"

	"go.uber.org/fx"
)

var HandlerModule = fx.Options(
	fx.Provide(NewSuggestionService),
	// NewArgusClient 已上提至 CoreServiceModule（mcp-server 等不拉 Handler 的入口也需要它）。
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
	fx.Invoke(InitArgusHandler),
	fx.Invoke(InitNotificationHandler),
	fx.Invoke(InitAuthHandler),
	fx.Invoke(InitCostHandler),
	fx.Invoke(InitReportHandler),
)

func InitReviewHandler(reviewSvc *svcReview.Service, gitMgr *gitProvider.GitProviderService, enrich *reviewenrich.Service, suggestionSvc *suggestion.Service) {
	// 角色窄接口：同一 *GitProviderService 结构化满足 PlatformReader/PlatformCRWriter。
	review_service.InitHandler(reviewSvc, gitMgr, gitMgr, enrich, suggestionSvc)
}

// NewSuggestionService 构造评审建议补丁服务（#8 拆分：从 review.Service 抽离）。
// 依赖窄接口：reviewSvc 提供 Get（满足 suggestion.ReviewFetcher），
// providers 满足 domain.Providers，gitMgr 满足 suggestion.GitTokenSource。
func NewSuggestionService(reviewSvc *svcReview.Service, providers *gitProvider.GitProviderProviders, gitMgr *gitProvider.GitProviderService) *suggestion.Service {
	return suggestion.NewService(reviewSvc, providers, gitMgr)
}

func InitWebhookHandler(webhookSvc *webhook.Service, reviewSvc *svcReview.Service) {
	review_service.InitWebhookHandler(webhookSvc, reviewSvc)
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

// NewArgusClient argus 出站调用唯一收口（触发审查任务/状态查询/误报抑制代理），
// base URL 与凭证来自 config.Argus（argus.bot_url / argus.bot_token）。
// bot_url 为空时 Enabled()=false：argus 引擎与反馈回流整体关闭。
func NewArgusClient(cfg *config.Config) *argusclient.Client {
	return argusclient.New(cfg.Argus.BotURL, cfg.Argus.BotToken)
}

// InitArgusHandler Argus 审查结果承接（push 模型，见 biz/handler/argus_service）；
// push 落库后经 CompleteFromArgusReport 关联收敛 engine=argus 的 active 评审，
// 并发应用内通知（notifier 与 OCR 路径同源）+ PR 门禁 commit status（statusPoster）。
func InitArgusHandler(repos *repository.RepoContainer, gitSvc *gitProvider.GitProviderService,
	argus *argusclient.Client, notifier reviewdomain.Notifier) {
	argus_service.InitHandler(repos.ArgusReview, repos.Review, argus, notifier,
		statuscheck.NewPoster(gitSvc))
}

func InitNotificationHandler(repos *repository.RepoContainer) {
	notification_service.InitHandler(repos.Notification)
}

func InitAuthHandler(repos *repository.RepoContainer) {
	auth_service.InitHandler(repos.Auth)
}

func InitCostHandler(calc *cost.Calculator) {
	cost_service.InitHandler(calc)
}

func InitReportHandler(repos *repository.RepoContainer) {
	report_service.InitHandler(repos.Review, repos.ContributorStats)
}
