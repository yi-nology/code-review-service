package session_service

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSession "github.com/yi-nology/code-review-service/biz/model/session"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/ptrx"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var handlerInst *SessionHandler

type SessionService interface {
	GetSessionsByReview(ctx context.Context, reviewID string) ([]*models.ReviewSession, error)
	GetSession(ctx context.Context, sessionID string) (*models.ReviewSession, error)
	GetActiveSession(ctx context.Context, reviewID string) (*models.ReviewSession, error)
	GetNextRound(ctx context.Context, reviewID string) (int, error)
	CreateSession(ctx context.Context, reviewID string, round int) (*models.ReviewSession, error)
	UpdatePhase(ctx context.Context, sessionID string, phase models.SessionPhase, progress float64) error
	UpdateStatus(ctx context.Context, sessionID string, status models.SessionStatus) error
	UpdateAgentResults(ctx context.Context, sessionID string, agentResults []models.AgentResultData) error
	UpdateDiscussion(ctx context.Context, sessionID string, discussion models.DiscussionData) error
	UpdateFinalReport(ctx context.Context, sessionID string, finalReport models.FinalReportData) error
	CompleteSession(ctx context.Context, sessionID string, finalReport models.FinalReportData) error
	FailSession(ctx context.Context, sessionID string, reason string) error
	CancelSession(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, page, pageSize int) ([]*models.ReviewSession, int64, error)
}

type SessionHandler struct {
	sessionService SessionService
}

func InitHandler(sessionService SessionService) {
	handlerInst = &SessionHandler{sessionService: sessionService}
}

func ListSessions(ctx context.Context, c *app.RequestContext) {
	var req modelSession.ListSessionsReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	page, pageSize := common.PageParams(
		convert.Int64ToStr(ptrx.Deref(req.Page, 0)),
		convert.Int64ToStr(ptrx.Deref(req.PageSize, 0)),
	)

	sessions, total, err := handlerInst.sessionService.ListSessions(ctx, page, pageSize)
	if err != nil {
		hlog.Errorf("list sessions failed: %v", err)
		common.Fail(c, err)
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"sessions":    sessions,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

func GetSessions(ctx context.Context, c *app.RequestContext) {
	var req modelSession.GetSessionsReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	sessions, err := handlerInst.sessionService.GetSessionsByReview(ctx, req.ID)
	if err != nil {
		hlog.Errorf("get sessions failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, map[string]interface{}{
		"sessions": sessions,
	})
}

func CreateSession(ctx context.Context, c *app.RequestContext) {
	var req modelSession.CreateSessionReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	round, err := handlerInst.sessionService.GetNextRound(ctx, req.ID)
	if err != nil {
		hlog.Errorf("get next round failed: %v", err)
		common.Fail(c, err)
		return
	}

	session, err := handlerInst.sessionService.CreateSession(ctx, req.ID, round)
	if err != nil {
		hlog.Errorf("create session failed: %v", err)
		common.Fail(c, err)
		return
	}

	c.JSON(consts.StatusCreated, session)
}

func GetActiveSession(ctx context.Context, c *app.RequestContext) {
	var req modelSession.GetActiveSessionReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	session, err := handlerInst.sessionService.GetActiveSession(ctx, req.ID)
	if err != nil {
		hlog.Errorf("get active session failed: %v", err)
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
	var req modelSession.GetSessionReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	session, err := handlerInst.sessionService.GetSession(ctx, req.SessionID)
	if err != nil {
		hlog.Errorf("get session failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, session)
}

func UpdateSessionPhase(ctx context.Context, c *app.RequestContext) {
	var req modelSession.UpdateSessionPhaseReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	progress := ptrx.Deref(req.Progress, 0)
	if err := handlerInst.sessionService.UpdatePhase(ctx, req.SessionID, models.SessionPhase(req.Phase), progress); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.UpdateSessionPhaseResp{
		Message: "session phase updated",
	})
}

func UpdateSessionStatus(ctx context.Context, c *app.RequestContext) {
	var req modelSession.UpdateSessionStatusReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	if err := handlerInst.sessionService.UpdateStatus(ctx, req.SessionID, models.SessionStatus(req.Status)); err != nil {
		hlog.Errorf("update session status failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.UpdateSessionStatusResp{
		Message: "session status updated",
	})
}

func UpdateSessionAgentResults(ctx context.Context, c *app.RequestContext) {
	var req modelSession.UpdateSessionAgentResultsReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	var agentResults []models.AgentResultData
	if err := json.Unmarshal([]byte(req.AgentResults), &agentResults); err != nil {
		common.Fail(c, common.BadRequest("invalid agent_results format"))
		return
	}

	if err := handlerInst.sessionService.UpdateAgentResults(ctx, req.SessionID, agentResults); err != nil {
		hlog.Errorf("update session agent results failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.UpdateSessionAgentResultsResp{
		Message: "session agent results updated",
	})
}

func UpdateSessionDiscussion(ctx context.Context, c *app.RequestContext) {
	var req modelSession.UpdateSessionDiscussionReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	var discussion models.DiscussionData
	if err := json.Unmarshal([]byte(req.Discussion), &discussion); err != nil {
		common.Fail(c, common.BadRequest("invalid discussion format"))
		return
	}

	if err := handlerInst.sessionService.UpdateDiscussion(ctx, req.SessionID, discussion); err != nil {
		hlog.Errorf("update session discussion failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.UpdateSessionDiscussionResp{
		Message: "session discussion updated",
	})
}

func UpdateSessionFinalReport(ctx context.Context, c *app.RequestContext) {
	var req modelSession.UpdateSessionFinalReportReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	var finalReport models.FinalReportData
	if err := json.Unmarshal([]byte(req.FinalReport), &finalReport); err != nil {
		common.Fail(c, common.BadRequest("invalid final_report format"))
		return
	}

	if err := handlerInst.sessionService.UpdateFinalReport(ctx, req.SessionID, finalReport); err != nil {
		hlog.Errorf("update session final report failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.UpdateSessionFinalReportResp{
		Message: "session final report updated",
	})
}

func CompleteSession(ctx context.Context, c *app.RequestContext) {
	var req modelSession.CompleteSessionReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	var finalReport models.FinalReportData
	if err := json.Unmarshal([]byte(req.FinalReport), &finalReport); err != nil {
		common.Fail(c, common.BadRequest("invalid final_report format"))
		return
	}

	if err := handlerInst.sessionService.CompleteSession(ctx, req.SessionID, finalReport); err != nil {
		hlog.Errorf("complete session failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.CompleteSessionResp{
		Message: "session completed",
	})
}

func FailSession(ctx context.Context, c *app.RequestContext) {
	var req modelSession.FailSessionReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	if err := handlerInst.sessionService.FailSession(ctx, req.SessionID, req.Reason); err != nil {
		hlog.Errorf("fail session failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.FailSessionResp{
		Message: "session failed",
	})
}

func CancelSession(ctx context.Context, c *app.RequestContext) {
	var req modelSession.CancelSessionReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	if err := handlerInst.sessionService.CancelSession(ctx, req.SessionID); err != nil {
		hlog.Errorf("cancel session failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelSession.CancelSessionResp{
		Message: "session cancelled",
	})
}
