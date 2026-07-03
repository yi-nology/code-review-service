package session_service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSession "github.com/yi-nology/code-review-service/biz/model/session"
	dsession "github.com/yi-nology/code-review-service/internal/domain/session"

	"github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/ptrx"

	"github.com/oekit/logx"
	"github.com/cloudwego/hertz/pkg/app"
)

var handlerInst *SessionHandler

type SessionService interface {
	GetSessionsByReview(ctx context.Context, reviewID string) ([]*dsession.Session, error)
	GetSession(ctx context.Context, sessionID string) (*dsession.Session, error)
	GetActiveSession(ctx context.Context, reviewID string) (*dsession.Session, error)
	GetNextRound(ctx context.Context, reviewID string) (int, error)
	CreateSession(ctx context.Context, reviewID string, round int) (*dsession.Session, error)
	UpdatePhase(ctx context.Context, sessionID string, phase string, progress float64) error
	UpdateStatus(ctx context.Context, sessionID string, status string) error
	UpdateAgentResults(ctx context.Context, sessionID string, agentResults []dsession.AgentResultData) error
	UpdateDiscussion(ctx context.Context, sessionID string, discussion dsession.DiscussionData) error
	UpdateFinalReport(ctx context.Context, sessionID string, finalReport dsession.FinalReportData) error
	CompleteSession(ctx context.Context, sessionID string, finalReport dsession.FinalReportData) error
	FailSession(ctx context.Context, sessionID string, reason string) error
	CancelSession(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, page, pageSize int) (*gormx.PageResponse[dsession.Session], error)
}

type SessionHandler struct {
	sessionService SessionService
}

func InitHandler(sessionService SessionService) {
	handlerInst = &SessionHandler{sessionService: sessionService}
}

// sessionInfo 是 ReviewSession 的可序列化投影：datatypes.JSON 转 json.RawMessage，
// 避免 sonic 序列化 named []byte（datatypes.JSON）类型报 "unsupported data type"（sessions 500 根因）。
type sessionInfo struct {
	SessionID    string          `json:"session_id"`
	ReviewID     string          `json:"review_id"`
	Round        int             `json:"round"`
	Phase        string          `json:"phase"`
	Status       string          `json:"status"`
	Progress     float64         `json:"progress"`
	AgentResults json.RawMessage `json:"agent_results,omitempty"`
	Discussion   json.RawMessage `json:"discussion,omitempty"`
	FinalReport  json.RawMessage `json:"final_report,omitempty"`
	Requirements json.RawMessage `json:"requirements,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func toSessionInfo(s dsession.Session) sessionInfo {
	info := sessionInfo{
		SessionID: s.SessionID, ReviewID: s.ReviewID, Round: s.Round,
		Phase: s.Phase, Status: s.Status, Progress: s.Progress,
		CompletedAt: s.CompletedAt, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
	if len(s.AgentResults) > 0 {
		info.AgentResults = json.RawMessage(s.AgentResults)
	}
	if len(s.Discussion) > 0 {
		info.Discussion = json.RawMessage(s.Discussion)
	}
	if len(s.FinalReport) > 0 {
		info.FinalReport = json.RawMessage(s.FinalReport)
	}
	if len(s.Requirements) > 0 {
		info.Requirements = json.RawMessage(s.Requirements)
	}
	if len(s.Metadata) > 0 {
		info.Metadata = json.RawMessage(s.Metadata)
	}
	return info
}

func ListSessions(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.ListSessionsReq](c)
	if !ok {
		return
	}

	page, pageSize := common.PageParams(
		convert.Int64ToStr(ptrx.Deref(req.Page, 0)),
		convert.Int64ToStr(ptrx.Deref(req.PageSize, 0)),
	)

	result, err := handlerInst.sessionService.ListSessions(ctx, page, pageSize)
	if err != nil {
		logx.Errorf("list sessions failed: %v", err)
		common.Fail(c, err)
		return
	}

	sessions := make([]sessionInfo, len(result.List))
	for i, s := range result.List {
		sessions[i] = toSessionInfo(s)
	}
	common.Success(c, map[string]interface{}{
		"sessions":    sessions,
		"total":       result.Total,
		"page":        result.Page,
		"page_size":   result.PageSize,
		"total_pages": result.Pages,
	})
}

func GetSessions(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.GetSessionsReq](c)
	if !ok {
		return
	}

	rawSessions, err := handlerInst.sessionService.GetSessionsByReview(ctx, req.ID)
	if err != nil {
		logx.Errorf("get sessions failed: %v", err)
		common.Fail(c, err)
		return
	}
	sessions := make([]sessionInfo, len(rawSessions))
	for i, s := range rawSessions {
		sessions[i] = toSessionInfo(*s)
	}
	common.Success(c, map[string]interface{}{
		"sessions": sessions,
	})
}

func CreateSession(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.CreateSessionReq](c)
	if !ok {
		return
	}

	round, err := handlerInst.sessionService.GetNextRound(ctx, req.ID)
	if err != nil {
		logx.Errorf("get next round failed: %v", err)
		common.Fail(c, err)
		return
	}

	session, err := handlerInst.sessionService.CreateSession(ctx, req.ID, round)
	if err != nil {
		logx.Errorf("create session failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, session)
}

func GetActiveSession(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.GetActiveSessionReq](c)
	if !ok {
		return
	}

	session, err := handlerInst.sessionService.GetActiveSession(ctx, req.ID)
	if err != nil {
		logx.Errorf("get active session failed: %v", err)
		common.Fail(c, err)
		return
	}

	if session == nil {
		common.Fail(c, common.NotFound("no active session found"))
		return
	}

	common.Success(c, session)
}

func GetSession(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.GetSessionReq](c)
	if !ok {
		return
	}

	session, err := handlerInst.sessionService.GetSession(ctx, req.SessionID)
	if err != nil {
		logx.Errorf("get session failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, session)
}

func UpdateSessionPhase(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.UpdateSessionPhaseReq](c)
	if !ok {
		return
	}

	progress := ptrx.Deref(req.Progress, 0)
	if err := handlerInst.sessionService.UpdatePhase(ctx, req.SessionID, req.Phase, progress); err != nil {
		logx.Errorf("update session phase failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.UpdateSessionPhaseResp{
		Message: "session phase updated",
	})
}

func UpdateSessionStatus(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.UpdateSessionStatusReq](c)
	if !ok {
		return
	}

	if err := handlerInst.sessionService.UpdateStatus(ctx, req.SessionID, req.Status); err != nil {
		logx.Errorf("update session status failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.UpdateSessionStatusResp{
		Message: "session status updated",
	})
}

func UpdateSessionAgentResults(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.UpdateSessionAgentResultsReq](c)
	if !ok {
		return
	}

	var agentResults []dsession.AgentResultData
	if err := json.Unmarshal([]byte(req.AgentResults), &agentResults); err != nil {
		common.Fail(c, common.BadRequest("invalid agent_results format"))
		return
	}

	if err := handlerInst.sessionService.UpdateAgentResults(ctx, req.SessionID, agentResults); err != nil {
		logx.Errorf("update session agent results failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.UpdateSessionAgentResultsResp{
		Message: "session agent results updated",
	})
}

func UpdateSessionDiscussion(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.UpdateSessionDiscussionReq](c)
	if !ok {
		return
	}

	var discussion dsession.DiscussionData
	if err := json.Unmarshal([]byte(req.Discussion), &discussion); err != nil {
		common.Fail(c, common.BadRequest("invalid discussion format"))
		return
	}

	if err := handlerInst.sessionService.UpdateDiscussion(ctx, req.SessionID, discussion); err != nil {
		logx.Errorf("update session discussion failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.UpdateSessionDiscussionResp{
		Message: "session discussion updated",
	})
}

func UpdateSessionFinalReport(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.UpdateSessionFinalReportReq](c)
	if !ok {
		return
	}

	var finalReport dsession.FinalReportData
	if err := json.Unmarshal([]byte(req.FinalReport), &finalReport); err != nil {
		common.Fail(c, common.BadRequest("invalid final_report format"))
		return
	}

	if err := handlerInst.sessionService.UpdateFinalReport(ctx, req.SessionID, finalReport); err != nil {
		logx.Errorf("update session final report failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.UpdateSessionFinalReportResp{
		Message: "session final report updated",
	})
}

func CompleteSession(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.CompleteSessionReq](c)
	if !ok {
		return
	}

	var finalReport dsession.FinalReportData
	if err := json.Unmarshal([]byte(req.FinalReport), &finalReport); err != nil {
		common.Fail(c, common.BadRequest("invalid final_report format"))
		return
	}

	if err := handlerInst.sessionService.CompleteSession(ctx, req.SessionID, finalReport); err != nil {
		logx.Errorf("complete session failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.CompleteSessionResp{
		Message: "session completed",
	})
}

func FailSession(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.FailSessionReq](c)
	if !ok {
		return
	}

	if err := handlerInst.sessionService.FailSession(ctx, req.SessionID, req.Reason); err != nil {
		logx.Errorf("fail session failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.FailSessionResp{
		Message: "session failed",
	})
}

func CancelSession(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSession.CancelSessionReq](c)
	if !ok {
		return
	}

	if err := handlerInst.sessionService.CancelSession(ctx, req.SessionID); err != nil {
		logx.Errorf("cancel session failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.CancelSessionResp{
		Message: "session cancelled",
	})
}
