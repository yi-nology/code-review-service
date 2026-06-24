package progress_service

import (
	"context"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelProgress "github.com/yi-nology/code-review-service/biz/model/progress"
	"github.com/yi-nology/code-review-service/internal/service/progress"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/oekit/logx"
	"github.com/hertz-contrib/websocket"
)

var handlerInst *ProgressWSHandler

type ProgressWSHandler struct {
	progressManager *progress.ProgressManager
	upgrader        websocket.HertzUpgrader
}

func InitHandler(progressManager *progress.ProgressManager) {
	handlerInst = &ProgressWSHandler{
		progressManager: progressManager,
		upgrader: websocket.HertzUpgrader{
			CheckOrigin: func(r *app.RequestContext) bool {
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

func HandleProgressWS(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelProgress.HandleProgressWSReq](c)
	if !ok {
		return
	}

	var conn *websocket.Conn
	err := handlerInst.upgrader.Upgrade(c, func(ws *websocket.Conn) {
		conn = ws
	})
	if err != nil {
		logx.Errorf("failed to upgrade websocket connection: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	tracker := handlerInst.progressManager.GetTracker(req.ID)
	if tracker == nil {
		_ = conn.WriteJSON(map[string]string{
			"type":    "error",
			"error":   "no active review found",
			"message": "No active review found for this ID",
		})
		return
	}

	roomID := tracker.GetRoomID()
	broadcaster := handlerInst.progressManager.GetBroadcaster()
	broadcaster.Register(roomID, conn)
	defer broadcaster.Unregister(roomID, conn)

	progress := tracker.GetProgress()
	if err := conn.WriteJSON(progress); err != nil {
		logx.Errorf("failed to send initial progress: %v", err)
		return
	}

	logx.Infof("websocket client connected for review: %s", req.ID)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logx.Errorf("websocket error: %v", err)
			}
			break
		}
	}

	logx.Infof("websocket client disconnected for review: %s", req.ID)
}

func GetProgress(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelProgress.GetProgressReq](c)
	if !ok {
		return
	}

	tracker := handlerInst.progressManager.GetTracker(req.ID)
	if tracker == nil {
		common.Fail(c, common.NotFound("no active review found"))
		return
	}

	p := tracker.GetProgress()
	common.Success(c, &modelProgress.GetProgressResp{Progress: convertProgress(p)})
}

func GetActiveReviews(ctx context.Context, c *app.RequestContext) {
	_, ok := common.Bind[modelProgress.GetActiveReviewsReq](c)
	if !ok {
		return
	}

	trackers := handlerInst.progressManager.GetActiveTrackers()

	reviews := make([]*modelProgress.ActiveReviewItem, 0, len(trackers))
	for _, tracker := range trackers {
		p := tracker.GetProgress()
		reviews = append(reviews, &modelProgress.ActiveReviewItem{
			ReviewID:  p.ReviewID,
			SessionID: p.SessionID,
			Round:     int64(p.Round),
			Phase:     string(p.Phase),
			Status:    p.Status,
			Progress:  p.Progress,
			StartTime: common.FormatTime(p.StartTime),
			UpdatedAt: common.FormatTime(p.UpdatedAt),
		})
	}

	common.Success(c, &modelProgress.GetActiveReviewsResp{
		Reviews: reviews,
		Total:   int64(len(reviews)),
	})
}

func convertProgress(p *progress.ReviewProgress) *modelProgress.ReviewProgress {
	progress := &modelProgress.ReviewProgress{
		ReviewID:    p.ReviewID,
		SessionID:   p.SessionID,
		Round:       int64(p.Round),
		Phase:       string(p.Phase),
		Status:      p.Status,
		Progress:    p.Progress,
		Phases:      convertPhases(p.Phases),
		AgentStatus: convertAgentStatus(p.AgentStatus),
		StartTime:   common.FormatTime(p.StartTime),
		UpdatedAt:   common.FormatTime(p.UpdatedAt),
	}
	if p.Error != "" {
		progress.Error = &p.Error
	}
	return progress
}

func convertPhases(phases []progress.PhaseInfo) []*modelProgress.PhaseInfo {
	items := make([]*modelProgress.PhaseInfo, 0, len(phases))
	for _, phase := range phases {
		item := &modelProgress.PhaseInfo{
			Name:        phase.Name,
			Title:       phase.Title,
			Description: phase.Description,
			Status:      phase.Status,
			Progress:    phase.Progress,
		}
		if !phase.StartTime.IsZero() {
			startTime := common.FormatTime(phase.StartTime)
			item.StartTime = &startTime
		}
		if !phase.EndTime.IsZero() {
			endTime := common.FormatTime(phase.EndTime)
			item.EndTime = &endTime
		}
		if phase.Duration > 0 {
			duration := phase.Duration
			item.DurationMs = &duration
		}
		items = append(items, item)
	}
	return items
}

func convertAgentStatus(status map[string]progress.AgentStatus) map[string]*modelProgress.AgentStatus {
	if len(status) == 0 {
		return nil
	}
	items := make(map[string]*modelProgress.AgentStatus, len(status))
	for key, agent := range status {
		item := &modelProgress.AgentStatus{
			Name:     agent.Name,
			Status:   agent.Status,
			Progress: agent.Progress,
			Message:  agent.Message,
		}
		if agent.Duration > 0 {
			duration := agent.Duration
			item.DurationMs = &duration
		}
		if agent.Error != "" {
			errMessage := agent.Error
			item.Error = &errMessage
		}
		items[key] = item
	}
	return items
}
