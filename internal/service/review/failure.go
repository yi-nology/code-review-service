package review

import (
	"context"
	"time"

	notifdomain "github.com/yi-nology/code-review-service/internal/domain/notification"
	bizmetrics "github.com/yi-nology/code-review-service/internal/observability/metrics"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/strutil"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
)

// FailureReporter 统一「评审失败」的完整上报管线：
// 历史事件 → 终态落库 → 站内通知 → commit status → 指标。
//
// 此前失败上报有两套且副作用集不同：eino.Executor 的失败会通知 + 写 commit
// status + 打指标，而 review worker 的执行前失败（请求解码失败/扫描被重启
// 打断/provider 不存在）只静默落库——用户在 PR 上和通知里都看不到任何信号。
// 现在两条路径共用本类型，副作用集一致。
//
// notifier/statusPoster 为 nil 时跳过对应副作用（测试或未装配场景）。
type FailureReporter struct {
	store    ReviewStateStore
	notifier domain.Notifier
	poster   domain.StatusPoster
}

func NewFailureReporter(store ReviewStateStore, notifier domain.Notifier, poster domain.StatusPoster) *FailureReporter {
	return &FailureReporter{store: store, notifier: notifier, poster: poster}
}

// Report 上报一次评审失败。stage 标识失败发生的环节（如 workspace/execute/worker）。
func (f *FailureReporter) Report(ctx context.Context, id string, req domain.Request, stage string, runErr error, ocrMeta domain.OCRMetadata) {
	if f == nil || f.store == nil {
		return
	}

	details := map[string]any{
		"error": runErr.Error(),
		"stage": stage,
	}
	if et := classifyReviewErrorType(runErr); et != "" {
		details["error_type"] = et
	}
	AppendHistoryEvent(ctx, f.store, id, domain.Event{
		Stage:     stage,
		Status:    "failed",
		Message:   runErr.Error(),
		Details:   details,
		CreatedAt: time.Now().UTC(),
	})

	response := domain.Response{
		ReviewID: id,
		Status:   "failed",
		Summary:  "review failed",
		Gate:     domain.ErrorGate("review execution failed"),
		Error:    runErr.Error(),
	}
	if ocrMeta.SessionID != "" {
		ocrMeta.Engine = "open-code-review"
		ocrMeta.Status = "failed"
		response.Report.OCR = ocrMeta
	}
	if err := WriteReviewState(ctx, f.store, id, req, response, nil); err != nil {
		logx.Error("failed to persist review failure", "review_id", id, "error", err)
	}

	f.notify(id, req, stage, runErr)
	f.postStatus(id, req, stage, runErr)
	bizmetrics.ReviewTotal.WithLabelValues(req.Provider, "failed").Inc()
}

func (f *FailureReporter) notify(id string, req domain.Request, stage string, runErr error) {
	if f.notifier == nil {
		return
	}
	event := notifdomain.Event{
		ReviewID: id,
		Provider: req.Provider,
		Repo:     req.Repo,
		Ref:      req.PullRequestID + req.MergeRequestIID,
		Kind:     notifdomain.KindFailed,
		Stage:    stage,
		Error:    runErr.Error(),
	}
	async.GoSafe(func() {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := f.notifier.Notify(notifyCtx, event); err != nil {
			logx.CtxWarnf(context.Background(),
				"[notification] review %s kind=%s notify failed: %v", event.ReviewID, event.Kind, err)
		}
	})
}

func (f *FailureReporter) postStatus(id string, req domain.Request, stage string, runErr error) {
	if f.poster == nil || req.HeadSHA == "" {
		return
	}
	// 弱门禁执行失败只 success 留痕（决策收敛在 domain.ApplyWeakGate）。
	state, description := domain.ApplyWeakGate(req.GateMode, req.Engine,
		domain.StateError, "评审执行失败："+truncateForStatusDesc(stage+": "+runErr.Error()))
	f.poster.Post(context.Background(), req.Provider, req.Repo, req.HeadSHA, state, description, id)
}

// truncateForStatusDesc 截断 commit status 描述（对齐 eino 侧 truncateForStatus 语义）。
func truncateForStatusDesc(s string) string {
	if s == "" {
		return "未知原因"
	}
	return strutil.Truncate(s, 100)
}

// classifyReviewErrorType 把 OCR 子进程错误归类为历史明细里的 error_type 标签。
func classifyReviewErrorType(err error) string {
	if ocr.IsBinaryMissing(err) {
		return "binary_missing"
	}
	if ocr.IsTimeout(err) {
		return "timeout"
	}
	if ocr.IsExit(err) {
		return "exit"
	}
	return ""
}
