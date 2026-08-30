package progress_service

import (
	"context"
	"net/url"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelProgress "github.com/yi-nology/code-review-service/biz/model/progress"
	"github.com/yi-nology/code-review-service/internal/service/progress"

	"github.com/oekit/logx"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/hertz-contrib/websocket"
)

// ProgressWSHandler 通过 fx Provide 注入。
// PR-16 修复：原实现用 var handlerInst 全局单例，测试无法 mock。
// 改为 NewProgressHandler 构造器 + 方法接收者，router 改为 h.Method。
var handlerInst *ProgressWSHandler

type ProgressWSHandler struct {
	progressManager *progress.ProgressManager
	upgrader        websocket.HertzUpgrader
}

// NewProgressHandler 构造进度 WebSocket handler。
// 通过 fx Provide 注入，可在测试中传入 mock 替换。
func NewProgressHandler(progressManager *progress.ProgressManager) *ProgressWSHandler {
	return &ProgressWSHandler{
		progressManager: progressManager,
		upgrader: websocket.HertzUpgrader{
			// 仅允许同源连接，避免跨站 WebSocket 劫持（CSRF/CSWSH）。
			// 反向代理部署时需保证转发了正确的 Host/Origin 头。
			CheckOrigin:     sameOrigin,
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

func InitHandler(progressManager *progress.ProgressManager) {
	handlerInst = NewProgressHandler(progressManager)
}

// sameOrigin 校验请求 Origin 与当前 Host 同源。
// 必须解析后严格比较 host 部分：前缀匹配（strings.HasPrefix(origin, scheme+host)）
// 会被 http://<host>.attacker.com 这类域名绕过，导致跨站 WebSocket 劫持（CSWSH）。
// Origin 缺失（部分非浏览器客户端）时放行，由 Bearer 认证保障安全；
// Host 头缺失或 Origin 非法时拒绝（fail closed）。
func sameOrigin(r *app.RequestContext) bool {
	origin := string(r.GetHeader("Origin"))
	if origin == "" {
		return true
	}
	host := string(r.GetHeader("Host"))
	if host == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "http", "https", "ws", "wss":
	default:
		return false
	}
	return u.Host == host
}

func HandleProgressWS(ctx context.Context, c *app.RequestContext) {
	handlerInst.HandleProgressWSMethod(ctx, c)
}

// HandleProgressWSMethod 是 (h *ProgressWSHandler) 的方法实现，
// 便于 fx Provide 注入后通过 h.HandleProgressWSMethod 调用。
func (h *ProgressWSHandler) HandleProgressWSMethod(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelProgress.HandleProgressWSReq](c)
	if !ok {
		return
	}

	err := h.upgrader.Upgrade(c, func(conn *websocket.Conn) {
		// 整个会话逻辑必须在 Upgrade 回调内执行：回调在独立 goroutine 运行，
		// conn 作为参数保证非 nil 且仅在此 goroutine 访问，消除此前
		// 「回调写 conn + 主 goroutine 读 conn」的数据竞争与 nil panic。
		defer func() {
			_ = conn.Close()
			logx.Infof("websocket client disconnected for review: %s", req.ID)
		}()

		tracker := h.progressManager.GetTracker(req.ID)
		if tracker == nil {
			_ = conn.WriteJSON(map[string]string{
				"type":    "error",
				"error":   "no active review found",
				"message": "No active review found for this ID",
			})
			return
		}

		roomID := tracker.GetRoomID()
		broadcaster := h.progressManager.GetBroadcaster()
		broadcaster.Register(roomID, conn)
		defer broadcaster.Unregister(roomID, conn)

		progress := tracker.GetProgress()
		if err := conn.WriteJSON(progress); err != nil {
			logx.Errorf("failed to send initial progress: %v", err)
			return
		}

		logx.Infof("websocket client connected for review: %s", req.ID)

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					logx.Errorf("websocket error: %v", err)
				}
				return
			}
		}
	})
	if err != nil {
		logx.Errorf("failed to upgrade websocket connection: %v", err)
		return
	}
}

func GetProgress(ctx context.Context, c *app.RequestContext) {
	handlerInst.GetProgressMethod(ctx, c)
}

func (h *ProgressWSHandler) GetProgressMethod(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelProgress.GetProgressReq](c)
	if !ok {
		return
	}

	tracker := h.progressManager.GetTracker(req.ID)
	if tracker == nil {
		common.Fail(c, common.NotFound("no active review found"))
		return
	}

	p := tracker.GetProgress()
	common.Success(c, &modelProgress.GetProgressResp{Progress: convertProgress(p)})
}

func GetActiveReviews(ctx context.Context, c *app.RequestContext) {
	handlerInst.GetActiveReviewsMethod(ctx, c)
}

func (h *ProgressWSHandler) GetActiveReviewsMethod(ctx context.Context, c *app.RequestContext) {
	_, ok := common.Bind[modelProgress.GetActiveReviewsReq](c)
	if !ok {
		return
	}

	trackers := h.progressManager.GetActiveTrackers()

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
