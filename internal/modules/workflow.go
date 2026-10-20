package modules

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/session"
	"github.com/yi-nology/code-review-service/internal/service/workflow"

	"go.uber.org/fx"
)

var WorkflowModule = fx.Options(
	fx.Provide(NewReviewWorkflow),
)

func NewReviewWorkflow(
	cfg *config.Config,
	agentMgr *agent.AgentManager,
	sessionSvc *session.SessionService,
	progressMgr *progress.ProgressManager,
	llmProvider ocr.Provider,
	repos *repository.RepoContainer,
) *workflow.ReviewWorkflow {
	wfCfg := workflow.WorkflowConfig{
		EnableDiscussion:   cfg.Agents.Discussion.Enabled,
		EnableRequirements: cfg.Agents.Requirements.Enabled,
		DiscussionRounds:   cfg.Agents.Discussion.MaxRounds,
		// 上下文窗口现取而非快照：OCR 设置存 DB 且可热更，
		// 启动时拍一次会让 diff 预算用旧窗口算。
		ContextWindow: func() int {
			if repos == nil || repos.OCRSettings == nil {
				return 0
			}
			settings, err := repos.OCRSettings.Get(context.Background())
			if err != nil || settings == nil {
				return 0
			}
			return int(settings.ContextWindow)
		},
	}
	if wfCfg.DiscussionRounds <= 0 {
		wfCfg.DiscussionRounds = 1
	}
	return workflow.NewReviewWorkflow(agentMgr, sessionSvc, progressMgr, llmProvider, wfCfg)
}
