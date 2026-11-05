package progress

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
)

// PhaseSnapshot 粗粒度进度快照：DB 持久层视角的会话进度（phase/status/progress
// 每次阶段流转都落库，跨副本可见——与进程内 tracker 的细粒度事件互补）。
type PhaseSnapshot struct {
	SessionID string
	Phase     string
	Status    string
	Progress  float64
	UpdatedAt time.Time
}

// PhaseReader 跨副本进度降级读数端口。
//
// 多副本部署下，细粒度进度事件只存在于「跑这个评审的副本」的进程内 tracker；
// WS 连到其他副本时 tracker 为 nil。此时降级为轮询持久层（粗粒度但全局一致），
// 而不是直接报「no active review found」。
type PhaseReader interface {
	// LatestPhase 返回该评审活跃会话的粗粒度快照；nil = 无活跃会话。
	LatestPhase(ctx context.Context, reviewID string) (*PhaseSnapshot, error)
}

// NewRepoPhaseReader 以 session 仓储实现 PhaseReader（装配层入口）。
func NewRepoPhaseReader(sess repository.SessionRepository) PhaseReader {
	if sess == nil {
		return nil
	}
	return &repoPhaseReader{sess: sess}
}

type repoPhaseReader struct {
	sess repository.SessionRepository
}

func (r *repoPhaseReader) LatestPhase(ctx context.Context, reviewID string) (*PhaseSnapshot, error) {
	s, err := r.sess.GetActiveByReviewID(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}
	return &PhaseSnapshot{
		SessionID: s.SessionID,
		Phase:     s.Phase,
		Status:    s.Status,
		Progress:  s.Progress,
		UpdatedAt: s.UpdatedAt,
	}, nil
}
