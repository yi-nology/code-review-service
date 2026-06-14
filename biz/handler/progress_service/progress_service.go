package progress_service

import (
	"context"
	"net/http"
	"time"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelProgress "github.com/yi-nology/code-review-service/biz/model/progress"
	"github.com/yi-nology/code-review-service/internal/service/progress"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
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
	var req modelProgress.HandleProgressWSReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	var conn *websocket.Conn
	err := handlerInst.upgrader.Upgrade(c, func(ws *websocket.Conn) {
		conn = ws
	})
	if err != nil {
		hlog.Errorf("failed to upgrade websocket connection: %v", err)
		return
	}
	defer conn.Close()

	tracker := handlerInst.progressManager.GetTracker(req.ID)
	if tracker == nil {
		conn.WriteJSON(map[string]interface{}{
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
		hlog.Errorf("failed to send initial progress: %v", err)
		return
	}

	hlog.Infof("websocket client connected for review: %s", req.ID)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				hlog.Errorf("websocket error: %v", err)
			}
			break
		}
	}

	hlog.Infof("websocket client disconnected for review: %s", req.ID)
}

func GetProgress(ctx context.Context, c *app.RequestContext) {
	var req modelProgress.GetProgressReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	tracker := handlerInst.progressManager.GetTracker(req.ID)
	if tracker == nil {
		common.Fail(c, common.NotFound("no active review found"))
		return
	}

	p := tracker.GetProgress()
	progress := &modelProgress.ReviewProgress{
		ReviewID:  p.ReviewID,
		SessionID: p.SessionID,
		Round:     int64(p.Round),
		Phase:     string(p.Phase),
		Status:    p.Status,
		Progress:  p.Progress,
		StartTime: p.StartTime.Format(time.RFC3339),
		UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
	}
	if p.Error != "" {
		progress.Error = &p.Error
	}

	c.JSON(http.StatusOK, &modelProgress.GetProgressResp{Progress: progress})
}

func GetActiveReviews(ctx context.Context, c *app.RequestContext) {
	var req modelProgress.GetActiveReviewsReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
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
			StartTime: p.StartTime.Format(time.RFC3339),
			UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, &modelProgress.GetActiveReviewsResp{
		Reviews: reviews,
		Total:   int64(len(reviews)),
	})
}
