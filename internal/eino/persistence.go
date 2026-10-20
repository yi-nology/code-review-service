package eino

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/judge"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/strutil"

	"github.com/oekit/logx"
	"github.com/oekit/pkg/stringx"
)

// 执行器的持久化辅助：状态流转、审计历史、OCR 快照、SHAs、判定转换。
func truncateSHA(sha string) string {
	// stringx.FirstN 按 rune 边界截断（sha 本身是 hex ASCII，但统一走库的
	// 口径，避免各处各写一份「按字节切」的截断）。
	return stringx.FirstN(sha, 8)
}

func truncateForStatus(s string) string {
	if s == "" {
		return "未知原因"
	}
	return strutil.Truncate(s, 100)
}

// appendHistory appends a review history event（基础版，保持兼容）。
func (e *Executor) appendHistory(ctx context.Context, id, stage, status, message string, details map[string]any) {
	e.appendAuditStep(ctx, id, domain.Event{
		Stage:     stage,
		Status:    status,
		Message:   message,
		Details:   details,
		CreatedAt: time.Now().UTC(),
	})
}

// appendAuditStep 追加一个审计步骤到评审历史。
// 每个步骤包含：做了什么（Stage/Message）、怎么做的（How）、
// 输入了什么（Input）、产出了什么（Output）、耗时（DurationMs）。
// 实现统一走 review.AppendHistoryEvent（与 worker/收敛路径同一份）。
func (e *Executor) appendAuditStep(ctx context.Context, id string, event domain.Event) {
	review.AppendHistoryEvent(ctx, e.reviewRepo, id, event)
}

// auditStep 是一个便捷构造器，自动记录起始时间和耗时。
type auditStep struct {
	startTime time.Time
	stage     string
}

func newAuditStep(stage string) *auditStep {
	return &auditStep{startTime: time.Now(), stage: stage}
}

func (s *auditStep) done(message, how string, input, output any) domain.Event {
	return domain.Event{
		Stage:      s.stage,
		Status:     "completed",
		Message:    message,
		How:        how,
		Input:      input,
		Output:     output,
		DurationMs: time.Since(s.startTime).Milliseconds(),
		CreatedAt:  time.Now().UTC(),
	}
}

func (s *auditStep) failed(message, how string, input any, err error) domain.Event {
	return domain.Event{
		Stage:      s.stage,
		Status:     "failed",
		Message:    message,
		How:        how,
		Input:      input,
		Output:     map[string]any{"error": err.Error()},
		DurationMs: time.Since(s.startTime).Milliseconds(),
		CreatedAt:  time.Now().UTC(),
	}
}

func (e *Executor) updateStatus(ctx context.Context, id, status string) error {
	return e.reviewRepo.UpdateInTransaction(ctx, id, func(r *domain.Record) error {
		resp := r.Response
		previousStatus := resp.Status
		previousHistory := append([]domain.Event(nil), resp.History...)
		resp.Status = status
		resp.Summary = "review " + status
		resp.History = previousHistory
		if status != previousStatus {
			resp.History = append(resp.History, domain.NewStatusEvent(status))
		}
		r.Status = status
		r.Summary = resp.Summary
		r.Response = resp
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// updateWithRawOCR writes the final review state + raw OCR payload.
// 实现统一收敛到 review.WriteReviewState（事务内合并 history、写全 Score/Grade/Summary）。
func (e *Executor) updateWithRawOCR(ctx context.Context, id string, req domain.Request, response domain.Response, rawOCR json.RawMessage) error {
	return review.WriteReviewState(ctx, e.reviewRepo, id, req, response, rawOCR)
}

func (e *Executor) updateRequestSHAs(ctx context.Context, id string, req domain.Request) {
	r, err := e.reviewRepo.GetByReviewID(ctx, id)
	if err != nil {
		return
	}
	r.Request = req
	r.UpdatedAt = time.Now().UTC()
	if err := e.reviewRepo.Save(ctx, r); err != nil {
		logx.Warn("failed to update request SHAs", "review_id", id, "error", err)
	}
}

func (e *Executor) updateWithOCRInput(ctx context.Context, id string, ocrInput domain.OCRInput) error {
	err := review.WriteOCRInput(ctx, e.reviewRepo, id, ocrInput)
	if errors.Is(err, repoerr.ErrNotFound) {
		return fmt.Errorf("review not found: %s", id)
	}
	return err
}

// judgmentToDomain 把 judge.Judgment 转为 domain.Judgment（供 API 响应）。
func judgmentToDomain(j *judge.Judgment) *domain.Judgment {
	if j == nil {
		return nil
	}
	findings := make([]domain.FindingJudgment, len(j.PrioritizedFindings))
	for i, f := range j.PrioritizedFindings {
		findings[i] = domain.FindingJudgment{
			Path:         f.Path,
			Line:         f.Line,
			Title:        f.Title,
			Severity:     f.Severity,
			IsLikelyTrue: f.IsLikelyTrue,
			Priority:     f.Priority,
			Reason:       f.Reason,
		}
	}
	return &domain.Judgment{
		OverallAssessment:   j.OverallAssessment,
		Confidence:          j.Confidence,
		RecommendedGate:     j.RecommendedGate,
		GateReason:          j.GateReason,
		PrioritizedFindings: findings,
		SummaryForHuman:     j.SummaryForHuman,
	}
}
