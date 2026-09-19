// Package ingest 承接外部审查引擎（argus）的上报业务：payload 校验/归一、
// 误报抑制代理、报告懒拉取。HTTP 层（biz/handler/argus_service）只做绑定与
// 状态码映射，本包持有全部业务规则——argus 回报的入站业务单一实现。
package ingest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/domain/argusingest"
)

// MaxFindingsPerIngest 单次上报意见条数上限（防异常客户端刷库）。
const MaxFindingsPerIngest = 500

// ErrUnknownFP fp/意见不属于该任务（客户端错误，映射 400）。
var ErrUnknownFP = errors.New("fp 不属于该任务的任何意见")

// FindingInput 是单条意见的中立入站形态（与 biz/model 生成的 ArgusFindingItem
// 字段一致；HTTP 层负责从 IDL 模型转换，业务层不 import 生成代码）。
type FindingInput struct {
	FP           string
	File         string
	Line         int
	Severity     string
	Comment      string
	SuggestedFix string
	By           string
}

// IngestPayload 是上报请求的中立入站形态。
type IngestPayload struct {
	ArgusTaskID  string
	Platform     string
	Owner        string
	Repo         string
	Number       string
	Title        string
	HeadSha      string
	BaseBranch   string
	Verdict      string
	Findings     []*FindingInput
	Plugins      []string
	DurationMs   int64
	TokenUsage   int
	OccurredAt   string
	ErrorMessage string
}

// ReviewStore 是承接落库所需的最小仓储面。
type ReviewStore interface {
	UpsertReview(ctx context.Context, rev *argusingest.Review) (uint, error)
	GetReview(ctx context.Context, id uint) (*argusingest.Review, error)
}

// ArgusProxy 是 argus 出站调用的窄接口（*argusclient.Client 的子集，
// 测试可用 httptest 桩替）。方法语义与 argusclient 一致。
type ArgusProxy interface {
	SuppressFinding(ctx context.Context, platform, owner, repo, number, fp string) (bool, error)
	SuppressFindingByComment(ctx context.Context, platform, owner, repo, number, file, comment string) (bool, error)
}

// BuildReview 把上报 payload 归一为领域实体并做全部业务校验。
func BuildReview(req *IngestPayload) (*argusingest.Review, error) {
	trim := func(s string) string { return strings.TrimSpace(s) }
	platform, owner, repo := trim(req.Platform), trim(req.Owner), trim(req.Repo)
	number, headSHA, verdict := trim(req.Number), trim(req.HeadSha), trim(req.Verdict)
	for _, f := range []struct{ v, name string }{
		{platform, "platform"}, {owner, "owner"}, {repo, "repo"},
		{number, "number"}, {headSHA, "head_sha"},
	} {
		if f.v == "" {
			return nil, fmt.Errorf("%s 必填", f.name)
		}
	}
	switch verdict {
	case argusingest.VerdictApprove, argusingest.VerdictComment,
		argusingest.VerdictRequestChanges, argusingest.VerdictError:
	default:
		return nil, errors.New("verdict 仅支持 approve、comment、request_changes、error")
	}
	if len(req.Findings) > MaxFindingsPerIngest {
		return nil, fmt.Errorf("findings 超过单次上限 %d", MaxFindingsPerIngest)
	}
	occurredAt := time.Now()
	if raw := strings.TrimSpace(req.OccurredAt); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, errors.New("occurred_at 须为 RFC3339 时间")
		}
		occurredAt = t
	}
	rev := &argusingest.Review{
		ArgusTaskID: trim(req.ArgusTaskID),
		Platform:    platform, Owner: owner, Repo: repo,
		Number: number, Title: strings.TrimSpace(req.Title),
		HeadSHA: headSHA, BaseBranch: trim(req.BaseBranch),
		Verdict: verdict, Plugins: req.Plugins,
		DurationMS: req.DurationMs, TokenUsage: req.TokenUsage,
		ErrorMessage: trim(req.ErrorMessage),
		OccurredAt:   occurredAt,
	}
	// 同 payload 内非空 fp 去重（last-wins）：重复 fp 直接落库会撞
	// (review_id, fp) 部分唯一索引、整组入库失败；计数在去重后聚合。
	seenFP := make(map[string]struct{})
	for _, f := range req.Findings {
		if f == nil {
			continue
		}
		file, comment, severity := trim(f.File), strings.TrimSpace(f.Comment), trim(f.Severity)
		if file == "" || comment == "" {
			return nil, errors.New("finding 的 file 与 comment 必填")
		}
		switch severity {
		case argusingest.SeverityHigh, argusingest.SeverityMedium, argusingest.SeverityLow:
		default:
			return nil, errors.New("finding.severity 仅支持 high、medium、low")
		}
		finding := argusingest.Finding{
			FP: strings.TrimSpace(f.FP), File: file, Line: f.Line,
			Severity: severity, Comment: comment,
			SuggestedFix: strings.TrimSpace(f.SuggestedFix), By: strings.TrimSpace(f.By),
		}
		if finding.FP != "" {
			if _, dup := seenFP[finding.FP]; dup {
				for i := range rev.Findings {
					if rev.Findings[i].FP == finding.FP {
						rev.Findings[i] = finding
						break
					}
				}
				continue
			}
			seenFP[finding.FP] = struct{}{}
		}
		rev.Findings = append(rev.Findings, finding)
	}
	for _, f := range rev.Findings {
		switch f.Severity {
		case argusingest.SeverityHigh:
			rev.FindingsHigh++
		case argusingest.SeverityMedium:
			rev.FindingsMedium++
		case argusingest.SeverityLow:
			rev.FindingsLow++
		}
	}
	rev.FindingsTotal = len(rev.Findings)
	return rev, nil
}

// SuppressFinding 误报抑制：意见归属校验 → 代理调用 argus → 落本地状态。
// fp 模式：fp 必须属于该任务的意见；argus 未命中且带 file+comment 时自动降级
// 兜底模式（措辞漂移场景）。comment 模式：按 file+意见原文匹配归属。
// 本地状态按命中的意见 fp 标记；被命中意见本身无 fp（旧数据）时跳过本地标记。
// matched=false（argus 未命中）不落状态，由调用方告知前端。
func SuppressFinding(ctx context.Context, store ReviewStore, argus ArgusProxy, reviewID uint, fp, file, comment string) (bool, error) {
	rev, err := store.GetReview(ctx, reviewID)
	if err != nil {
		return false, err
	}

	var (
		matched    bool
		markedFP   string // 命中意见的本地 fp（用于落 suppress_state）
		matchedByC = func(f argusingest.Finding) bool { return f.File == file && f.Comment == comment }
	)
	if fp != "" {
		known := false
		for _, f := range rev.Findings {
			if f.FP == fp {
				known, markedFP = true, f.FP
				break
			}
		}
		if !known {
			return false, fmt.Errorf("fp %q 不属于任务 %d 的任何意见: %w", fp, reviewID, ErrUnknownFP)
		}
		matched, err = argus.SuppressFinding(ctx, rev.Platform, rev.Owner, rev.Repo, rev.Number, fp)
		if err != nil {
			return false, err
		}
		if !matched && file != "" && comment != "" {
			// 指纹未命中（argus 侧重审覆盖后措辞漂移等）→ file+comment 兜底。
			matched, err = argus.SuppressFindingByComment(ctx, rev.Platform, rev.Owner, rev.Repo, rev.Number, file, comment)
			if err != nil {
				return false, err
			}
		}
	} else {
		known := false
		for _, f := range rev.Findings {
			if matchedByC(f) {
				known, markedFP = true, f.FP
				break
			}
		}
		if !known {
			return false, fmt.Errorf("意见（%s）不属于任务 %d 的任何意见: %w", file, reviewID, ErrUnknownFP)
		}
		matched, err = argus.SuppressFindingByComment(ctx, rev.Platform, rev.Owner, rev.Repo, rev.Number, file, comment)
		if err != nil {
			return false, err
		}
	}
	if !matched {
		return false, nil
	}
	if markedFP != "" {
		if setter, ok := store.(interface {
			SetFindingSuppressState(ctx context.Context, reviewID uint, fp, state string) error
		}); ok {
			if err := setter.SetFindingSuppressState(ctx, reviewID, markedFP, "suppressed"); err != nil {
				return false, fmt.Errorf("落抑制状态失败: %w", err)
			}
		}
	}
	return true, nil
}

// 编译期确认 *argusclient.Client 满足 ArgusProxy（防两接口漂移）。
var _ ArgusProxy = (*argusclient.Client)(nil)
