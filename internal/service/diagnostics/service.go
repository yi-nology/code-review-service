package diagnostics

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/webhook"
	"github.com/yi-nology/code-review-service/internal/domain/writeback"
	"github.com/yi-nology/code-review-service/internal/repository"
)

type Service struct {
	webhooks   repository.WebhookDeliveryRepository
	writebacks repository.WritebackItemRepository
}

func NewService(webhooks repository.WebhookDeliveryRepository, writebacks repository.WritebackItemRepository) *Service {
	return &Service{webhooks: webhooks, writebacks: writebacks}
}

type SnapshotOptions struct {
	Hours    int
	Limit    int
	Provider string
	Status   string
}

type Snapshot struct {
	SinceHours      int                `json:"since_hours"`
	Webhooks        map[string]int64   `json:"webhooks"`
	Writebacks      map[string]int64   `json:"writebacks"`
	RecentWebhooks  []webhook.Delivery `json:"recent_webhooks"`
	RecentWriteback []writeback.Item   `json:"recent_writebacks"`
	Recommendations []Recommendation   `json:"recommendations"`
	GeneratedAt     time.Time          `json:"generated_at"`
}

type Recommendation struct {
	Level   string `json:"level"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Action  string `json:"action,omitempty"`
}

func (s *Service) Snapshot(ctx context.Context, opts SnapshotOptions) (*Snapshot, error) {
	hours := opts.Hours
	if hours <= 0 || hours > 24*30 {
		hours = 24
	}
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	provider := strings.TrimSpace(opts.Provider)
	status := strings.TrimSpace(opts.Status)

	webhookCounts, err := s.webhooks.CountByStatus(ctx, since)
	if err != nil {
		return nil, err
	}
	writebackCounts, err := s.writebacks.CountByStatus(ctx, since)
	if err != nil {
		return nil, err
	}
	webhooks, err := s.webhooks.ListRecent(ctx, provider, status, limit)
	if err != nil {
		return nil, err
	}
	writebacks, err := s.writebacks.ListRecent(ctx, "", status, limit)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		SinceHours:      hours,
		Webhooks:        fillKnownStatuses(webhookCounts, []string{"received", "accepted", "ignored", "error"}),
		Writebacks:      fillKnownStatuses(writebackCounts, []string{"pending", "published", "failed"}),
		RecentWebhooks:  webhooks,
		RecentWriteback: writebacks,
		Recommendations: buildRecommendations(webhookCounts, writebackCounts),
		GeneratedAt:     time.Now().UTC(),
	}, nil
}

func fillKnownStatuses(counts map[string]int64, known []string) map[string]int64 {
	out := make(map[string]int64, len(counts)+len(known))
	for _, status := range known {
		out[status] = counts[status]
	}
	for status, count := range counts {
		out[status] = count
	}
	return out
}

func buildRecommendations(webhooks, writebacks map[string]int64) []Recommendation {
	recs := make([]Recommendation, 0, 4)
	if webhooks["error"] > 0 {
		recs = append(recs, Recommendation{
			Level:   "danger",
			Title:   "Webhook 处理失败",
			Message: "最近窗口内存在 webhook 失败记录，优先检查签名、平台配置和 payload 解析日志。",
			Action:  "打开 Git 平台配置并校验 webhook secret",
		})
	}
	if webhooks["received"] > 0 {
		recs = append(recs, Recommendation{
			Level:   "warning",
			Title:   "Webhook 未完成处理",
			Message: "存在停留在 received 状态的 webhook，可能是处理链路中断或服务重启时未完成。",
			Action:  "查看最近投递详情并确认是否生成审查任务",
		})
	}
	if writebacks["failed"] > 0 {
		recs = append(recs, Recommendation{
			Level:   "danger",
			Title:   "写回失败",
			Message: "有评论或总结未成功发布到代码平台，通常与平台 Token、权限或评论定位有关。",
			Action:  "进入对应审查详情重试发布",
		})
	}
	if writebacks["pending"] > 0 {
		recs = append(recs, Recommendation{
			Level:   "info",
			Title:   "写回等待中",
			Message: "存在待发布项目，请观察 worker 是否正常运行，或在审查详情页手动发布。",
		})
	}
	if len(recs) == 0 {
		recs = append(recs, Recommendation{
			Level:   "success",
			Title:   "诊断窗口内暂无明显故障",
			Message: "Webhook 与写回状态没有暴露高优先级问题。",
		})
	}
	sort.SliceStable(recs, func(i, j int) bool {
		return levelRank(recs[i].Level) < levelRank(recs[j].Level)
	})
	return recs
}

func levelRank(level string) int {
	switch level {
	case "danger":
		return 0
	case "warning":
		return 1
	case "info":
		return 2
	case "success":
		return 3
	default:
		return 4
	}
}
