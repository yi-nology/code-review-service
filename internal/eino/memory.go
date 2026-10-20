package eino

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/memory"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
)

// memory.go：把评审结果沉淀进跨会话长期记忆（底座 agent-memory）。
//
// 记忆是**副产物**：评审成败不依赖它。所以捕获失败只记日志，
// 绝不因记忆写不进去而把一次成功的评审判成失败。
//
// 捕获含一次 LLM 提炼（把评审产出蒸馏成候选事实），耗时不可忽略，
// 因此异步执行：评审结果不该为记忆买单。

// memoryCaptureTimeout 是后台记忆捕获的时限。
const memoryCaptureTimeout = 2 * time.Minute

// captureReviewMemoryAsync 异步把一次评审的产出捕获进长期记忆。
//
// 异步跑：捕获含一次 LLM 提炼，评审结果不该为记忆买单。
// ctx 与请求解绑（WithoutCancel）：请求返回后捕获仍要跑完。
func (e *Executor) captureReviewMemoryAsync(
	ctx context.Context,
	id string,
	req domain.Request,
	workspace domain.Workspace,
	result domain.Result,
	gatePassed bool,
) {
	if e.memory == nil {
		return
	}
	// 快照必要字段：后台 goroutine 不该再读被后续步骤改写的结构。
	capture := memory.ReviewCapture{
		Provider:   req.Provider,
		Repo:       req.Repo,
		ReviewID:   id,
		SessionID:  id,
		BaseSHA:    workspace.BaseSHA,
		HeadSHA:    workspace.HeadSHA,
		Verdict:    result.Summary,
		GatePassed: gatePassed,
		Summary:    result.Summary,
	}
	for _, f := range result.Findings {
		capture.Findings = append(capture.Findings, memory.FindingSummary{
			Severity: f.Severity,
			Category: f.Category,
			Path:     f.Path,
			Title:    f.Title,
		})
	}

	async.GoSafe(func() {
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), memoryCaptureTimeout)
		defer cancel()
		outcome, err := e.memory.CaptureReview(bgCtx, capture)
		if err != nil {
			logx.Warnf("[executor] capture review memory failed for %s/%s: %v", req.Provider, req.Repo, err)
			return
		}
		logx.Infof("[executor] review memory captured for %s/%s: %d candidates, %d promoted, %d merged, %d dropped, %d need review",
			req.Provider, req.Repo, outcome.Candidates, outcome.Promoted, outcome.Merged, outcome.Dropped, outcome.NeedsReview)
	})
}

// recallReviewMemory 召回该仓库的历史记忆，供本次评审注入背景。
//
// 召回失败不阻断评审：记忆是加分项，不是前置条件。
// 召回为空时返回空串——调用方据此不改动背景。
func (e *Executor) recallReviewMemory(
	ctx context.Context,
	id string,
	req domain.Request,
	workspace domain.Workspace,
) string {
	if e.memory == nil {
		return ""
	}
	recalled, err := e.memory.RecallReview(ctx, memory.RecallRequest{
		Provider: req.Provider,
		Repo:     req.Repo,
		ReviewID: id,
		Query:    memoryQuery(req, workspace),
	})
	if err != nil {
		logx.Warnf("[executor] recall review memory failed for %s/%s: %v", req.Provider, req.Repo, err)
		return ""
	}
	return recalled
}

// memoryQuery 构造召回查询词。
//
// 用本次变更触及的文件路径当查询词：召回取数按词面命中，
// 「这次动了哪些文件」正是「哪些历史经验与本次相关」的最强信号。
func memoryQuery(req domain.Request, workspace domain.Workspace) string {
	var sb strings.Builder
	if req.Repo != "" {
		sb.WriteString(req.Repo)
	}
	paths := changedPaths(workspace)
	for _, p := range paths {
		sb.WriteString(" ")
		sb.WriteString(p)
	}
	return strings.TrimSpace(sb.String())
}

// changedPaths 从 diff 里取变更文件路径（最多 20 个，避免查询词过长）。
//
// 变更文件清单只在 diff 里——工作区的 scope 元数据只有计数，没有路径。
func changedPaths(workspace domain.Workspace) []string {
	if strings.TrimSpace(workspace.Diff) == "" {
		return nil
	}
	var out []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(workspace.Diff, "\n") {
		if !strings.HasPrefix(line, "+++ ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "+++"))
		path = strings.TrimSpace(strings.TrimPrefix(path, "b/"))
		if path == "" || path == "/dev/null" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
		if len(out) >= 20 {
			break
		}
	}
	return out
}

// mergeMemoryContext 把召回的记忆并入评审背景。
//
// 记忆放在背景**末尾**：背景是模型读的「本次评审须知」，
// 记忆是历史经验，排在后面不会盖过项目规则与需求。
func mergeMemoryContext(background, memoryContext string) string {
	background = strings.TrimSpace(background)
	memoryContext = strings.TrimSpace(memoryContext)
	switch {
	case memoryContext == "":
		return background
	case background == "":
		return memoryContext
	default:
		return fmt.Sprintf("%s\n\n%s", background, memoryContext)
	}
}
