package session

import (
	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
)

// isValidPhaseTransition 判定阶段迁移是否合法。
//
// 顺序表的单一事实源在 internal/domain/session/phase.go——
// 此前本文件持有一份手写副本，与领域层各写一份，改一处忘另一处就会漂移。
func isValidPhaseTransition(from, to string) bool {
	return dsession.CanTransitionPhase(from, to)
}
