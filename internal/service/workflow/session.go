package workflow

import (
	"time"

	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/discussion"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/requirements"
	"github.com/yi-nology/code-review-service/internal/service/reviewmap"
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
	ReviewMap        *reviewmap.ReviewMap
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
}

// ReviewResult contains the final review result
type ReviewResult struct {
	SessionID    string
	Round        int
	AgentResults []agent.AgentResult
	Discussion   *discussion.DiscussionResult
	Requirements *requirements.CoverageAssessment
	ReviewMap    *reviewmap.ReviewMap
	FinalReport  FinalReport
	Duration     time.Duration
}

// FinalReport contains the final review report
type FinalReport struct {
	Verdict     string
	Summary     string
	Blockers    []agent.Finding
	ShouldFix   []agent.Finding
	Suggestions []agent.Finding
	Score       int
	GatePassed  bool
	GateReason  string
}
