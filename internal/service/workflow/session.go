package workflow

import (
	"time"

	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/discussion"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/requirements"
)

// ReviewSession 在各步骤间共享的会话数据
type ReviewSession struct {
	ReviewID  string
	SessionID string
	Round     int
	Request   ReviewRequest
	StartTime time.Time

	// 中间结果
	Workspace        agent.Workspace
	ParsedReqs       *requirements.Requirements
	AggregatedResult *agent.AggregatedResult
	DiscussionResult *discussion.DiscussionResult
	Requirements     *requirements.CoverageAssessment
	TechLeadAnalysis *TechLeadAnalysis
	FinalReport      *FinalReport

	// 进度追踪
	Tracker *progress.ProgressTracker
}

// ReviewRequest contains the review request parameters
type ReviewRequest struct {
	ReviewID     string
	Provider     string
	Repo         string
	BaseSHA      string
	HeadSHA      string
	Diff         string
	Dir          string
	Requirements []string
	Round        int
	// EphemeralReviewers 是本次评审临时加入的 reviewer（阶段 5），不入库。
	EphemeralReviewers []agent.EphemeralSpec
}

// ReviewResult contains the final review result
type ReviewResult struct {
	SessionID        string
	Round            int
	AgentResults     []agent.AgentResult
	Discussion       *discussion.DiscussionResult
	Requirements     *requirements.CoverageAssessment
	TechLeadAnalysis *TechLeadAnalysis
	FinalReport      FinalReport
	Duration         time.Duration
}

// FinalReport contains the final review report
type FinalReport struct {
	Verdict             string
	Summary             string
	Blockers            []agent.Finding
	ShouldFix           []agent.Finding
	Suggestions         []agent.Finding
	Score               int
	GatePassed          bool
	GateReason          string
	ClarifyingQuestions []ClarifyingQuestion `json:"clarifying_questions,omitempty"`
}

// ClarifyingQuestion 是需向利益相关者澄清的问题（阶段 4）。
type ClarifyingQuestion struct {
	Category string `json:"category"`
	Question string `json:"question"`
	Context  string `json:"context"`
}

// TechLeadAnalysis 是 Tech Lead（Phase 3）的 LLM 分析产物（B2）。
type TechLeadAnalysis struct {
	Summary         string   `json:"summary"`
	Risks           []string `json:"risks"`
	Recommendations []string `json:"recommendations"`
}
