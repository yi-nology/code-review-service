// kernel.go：评审编排的**阶段推进器**——阶段状态的合法迁移交给 agent-harness/statemachine 裁决。
//
// 重构前：8 个 step 各自散落调用 sessionService.UpdatePhase 与 tracker.UpdatePhase，
// 阶段推进**没有状态机**——任何顺序的阶段写入都会被会话层接受（只查一张手写顺序表），
// 阶段本身的状态（进行中/已完成/失败）更是完全不存在。
//
// 现在：阶段推进一律经 phaseController，状态迁移的合法性由底座的转移表裁决。
// 宿主不另立状态表——两份状态表必然漂移，漂移后没人知道它现在护的是什么。
//
// 底座只取真正生效的部分：
//   - statemachine：环节状态的合法转移表（本包的全部价值所在）
//
// 刻意**不接**的底座件（在本项目的架构里没有触发场景，接了就是假机制）：
//   - checkpoint：进度快照。与 OCR 自身的 `--resume` 重复（那才是真正跑得动的
//     续跑机制），而本包写出的快照无人读，与会话的 phase 列信息重复。
//   - commitgate：终态独占提交闸。管道不重试步骤、每次评审都是新会话，
//     不存在「两条分支都以为赢了终态」的场景。
//   - contract：编排引擎与宿主状态的边界。本项目的编排引擎就是这些 step 本身，
//     没有独立的引擎可对接，接口认证只是形式。
package workflow

import (
	"context"
	"fmt"
	"sync"

	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
	"github.com/yi-nology/code-review-service/internal/service/progress"

	"github.com/yi-nology/agent-harness/statemachine"
)

// 阶段状态机的取值直接用底座词表，不另立别名——
// 别名会让两处都能改同一个概念，改一处忘另一处就漂移。
const (
	phaseCreated   = statemachine.StatusCreated
	phaseRunning   = statemachine.StatusRunning
	phaseSucceeded = statemachine.StatusSucceeded
	phaseFailed    = statemachine.StatusFailed
	phaseSkipped   = statemachine.StatusSkipped
)

// phaseController 收敛「阶段推进」这一件事。
//
// 此前各 step 直连 sessionService 与 tracker，阶段推进散在 8 处；
// 现在推进路径只有这里一条。
type phaseController struct {
	sessionID string
	sessions  sessionService
	tracker   *progress.ProgressTracker

	mu          sync.Mutex
	statuses    map[string]statemachine.StepStatus
	currentStep string
}

func newPhaseController(
	sessionID string,
	sessions sessionService,
	tracker *progress.ProgressTracker,
	currentPhase string,
) *phaseController {
	return &phaseController{
		sessionID:   sessionID,
		sessions:    sessions,
		tracker:     tracker,
		statuses:    map[string]statemachine.StepStatus{},
		currentStep: currentPhase,
	}
}

// begin 开始一个阶段：状态迁移 + 会话推进 + 进度上报。
func (c *phaseController) begin(ctx context.Context, phase, message string) error {
	c.mu.Lock()
	prev, seen := c.statuses[phase]
	if !seen {
		prev = phaseCreated
	}
	if err := statemachine.MustTransition(prev, phaseRunning); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("阶段 %s 状态迁移非法: %w", phase, err)
	}
	c.statuses[phase] = phaseRunning
	prevStep := c.currentStep
	c.mu.Unlock()

	// 阶段推进：只有真正变化时才写会话（会话初始阶段就是首个阶段，
	// 重复写同一阶段会被顺序表判为非法迁移）。
	if err := c.rcSetCurrentStep(phase); err != nil {
		return err
	}
	if prevStep != phase {
		if err := c.sessions.UpdatePhase(ctx, c.sessionID, phase, dsession.PhaseInFlightProgress(phase)); err != nil {
			return err
		}
	}
	if c.tracker != nil {
		c.tracker.UpdatePhase(progress.Phase(phase), "active", 0, message)
	}
	return nil
}

// end 结束一个阶段：状态迁移到终态 + 进度上报。
func (c *phaseController) end(phase, status, message string) error {
	c.mu.Lock()
	prev, seen := c.statuses[phase]
	if !seen {
		prev = phaseCreated
	}
	target := statemachine.StepStatus(status)
	if err := statemachine.MustTransition(prev, target); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("阶段 %s 状态迁移非法: %w", phase, err)
	}
	c.statuses[phase] = target
	c.mu.Unlock()

	if c.tracker != nil {
		switch target {
		case phaseSucceeded, phaseSkipped:
			c.tracker.UpdatePhase(progress.Phase(phase), "completed", 100, message)
		case phaseFailed:
			c.tracker.SetError(message)
		}
	}
	return nil
}

// rcSetCurrentStep 推进当前环节。
//
// 阶段顺序的合法性由领域层的顺序表裁决（单一事实源），
// 当前环节未起步（空）时接受任意已知阶段。
func (c *phaseController) rcSetCurrentStep(step string) error {
	if !dsession.IsKnownPhase(step) {
		return fmt.Errorf("未知评审阶段 %q", step)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.currentStep != "" && c.currentStep != step && !dsession.CanTransitionPhase(c.currentStep, step) {
		return fmt.Errorf("评审阶段迁移非法: %s → %s", c.currentStep, step)
	}
	c.currentStep = step
	return nil
}
