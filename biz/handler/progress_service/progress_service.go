package progress_service

import (
	"context"
	"net/http"

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

	progress := tracker.GetProgress()
	c.JSON(http.StatusOK, progress)
}

func GetActiveReviews(ctx context.Context, c *app.RequestContext) {
	var req modelProgress.GetActiveReviewsReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	trackers := handlerInst.progressManager.GetActiveTrackers()

	reviews := make([]map[string]interface{}, 0, len(trackers))
	for _, tracker := range trackers {
		progress := tracker.GetProgress()
		reviews = append(reviews, map[string]interface{}{
			"review_id":  progress.ReviewID,
			"session_id": progress.SessionID,
			"round":      progress.Round,
			"phase":      progress.Phase,
			"status":     progress.Status,
			"progress":   progress.Progress,
			"start_time": progress.StartTime,
			"updated_at": progress.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"reviews": reviews,
		"total":   len(reviews),
	})
}
