// domain/status.go commit status 回写的窄接口：与 *statuscheck.Poster 结构化匹配，
// 让 review 子系统（argus 执行器/轮询器/回报收敛）不直接依赖 statuscheck 包。
// OCR 路径（internal/eino）继续用 *statuscheck.Poster 具体类型，不受影响。
package domain

import (
	"context"
	"strings"

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
)

// 门禁强弱依赖取值（真源在 projectpolicy，此处转出口供执行器/收敛路径使用）。
const (
	GateModeStrong = projectpolicy.GateModeStrong
	GateModeWeak   = projectpolicy.GateModeWeak
)

// WeakGatePrefix 弱门禁 status 描述前缀：放最前，description 被平台截断时标记仍可见。
const WeakGatePrefix = "【弱门禁】"

// WeakGateDesc 拼弱门禁 description（裁决/失败细节 + 弱门禁标记）。
func WeakGateDesc(summary string) string { return WeakGatePrefix + summary }

// commit status 状态词表（git 平台约定）。真源在 domain，statuscheck 包别名引用，
// 消费方（执行器/收敛路径）无需为取一个字符串常量依赖 statuscheck。
const (
	StateSuccess = "success"
	StateFailed  = "failed"
	StatePending = "pending"
	StateError   = "error"
)

// ApplyWeakGate 按门禁强弱收敛 commit status 终态（4 处调用点的共同原语）：
//   - weak（弱门禁）：一律 success 留痕，避免红 status 卡「检查通过后合并」类
//     自动合并；细节放 description 并加弱门禁前缀。
//   - strong：透传调用方按业务算好的 strongState/description。
func ApplyWeakGate(gateMode, engine, strongState, description string) (string, string) {
	if EffectiveGateMode(gateMode, engine) == GateModeWeak {
		return StateSuccess, WeakGateDesc(description)
	}
	return strongState, description
}

// StatusPoster 向 git 平台写 commit status（PR 质量门禁信号）。
type StatusPoster interface {
	// Post 写一条 commit status；sha 为空或平台不支持时实现方应静默跳过，
	// 失败仅记日志、不阻断主流程（status 是辅助信号）。
	Post(ctx context.Context, platform, repoPath, sha, state, description, reviewID string)
}

// PostStatus nil 安全的便捷封装（nil poster = 未装配，静默跳过）。
func PostStatus(p StatusPoster, ctx context.Context, platform, repoPath, sha, state, description, reviewID string) {
	if p == nil {
		return
	}
	p.Post(ctx, platform, repoPath, sha, state, description, reviewID)
}

// EffectiveGateMode 解析生效的门禁模式：显式 strong/weak 生效；”、auto、未知值按
// 引擎默认——OCR 强门禁（事实质量门禁）、argus 弱门禁（AI 审查不阻塞合并）。
func EffectiveGateMode(gateMode, engine string) string {
	switch strings.ToLower(strings.TrimSpace(gateMode)) {
	case GateModeStrong:
		return GateModeStrong
	case GateModeWeak:
		return GateModeWeak
	default:
		if strings.ToLower(strings.TrimSpace(engine)) == EngineArgus {
			return GateModeWeak
		}
		return GateModeStrong
	}
}
