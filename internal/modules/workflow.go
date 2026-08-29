package modules

import (
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/ocr"
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
) *workflow.ReviewWorkflow {
	wfCfg := workflow.WorkflowConfig{
		EnableDiscussion:   cfg.Agents.Discussion.Enabled,
		EnableRequirements: cfg.Agents.Requirements.Enabled,
		DiscussionRounds:   cfg.Agents.Discussion.MaxRounds,
	}
	if wfCfg.DiscussionRounds <= 0 {
		wfCfg.DiscussionRounds = 1
	}
	return workflow.NewReviewWorkflow(agentMgr, sessionSvc, progressMgr, llmProvider, wfCfg)
}
