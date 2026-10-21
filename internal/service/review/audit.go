package review

import (
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/review"
)

// audit.go：审计步骤的构造器。
//
// 审计步骤记录「做了什么（Stage/Message）、怎么做的（How）、输入了什么
// （Input）、产出了什么（Output）、耗时（DurationMs）」——评审历史的统一格式。
//
// 它住在 service/review 而不是引擎适配层：前置编排与两条引擎路径都写审计步骤，
// 格式只有一份实现（此前 eino/persistence.go 持有，两边各写一遍会漂移）。

// AuditStep 是一个审计步骤的构造器，自动记录起始时间与耗时。
type AuditStep struct {
	startTime time.Time
	stage     string
}

// NewAuditStep 开始一个审计步骤。
func NewAuditStep(stage string) *AuditStep {
	return &AuditStep{startTime: time.Now(), stage: stage}
}

// Elapsed 返回自步骤开始到现在的耗时。
//
// 引擎侧要用它给「慢调用」打点——startTime 是未导出字段，跨包读不到。
func (s *AuditStep) Elapsed() time.Duration {
	return time.Since(s.startTime)
}

// Done 收口一个成功步骤。
func (s *AuditStep) Done(message, how string, input, output any) domain.Event {
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

// Failed 收口一个失败步骤。
func (s *AuditStep) Failed(message, how string, input any, err error) domain.Event {
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
