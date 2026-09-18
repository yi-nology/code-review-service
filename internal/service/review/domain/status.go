// domain/status.go commit status 回写的窄接口：与 *statuscheck.Poster 结构化匹配，
// 让 review 子系统（argus 执行器/轮询器/回报收敛）不直接依赖 statuscheck 包。
// OCR 路径（internal/eino）继续用 *statuscheck.Poster 具体类型，不受影响。
package domain

import "context"

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
