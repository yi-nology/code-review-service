// Package reviewenrich 补全评审请求的派生信息。
//
// 第一项：PR/MR 作者补全——API 直连创建的评审（非 webhook）没有事件体，
// 作者字段为空，用 git 平台 GetCR 回查补上，让列表/详情页能展示头像与姓名。
// git 网络调用属业务编排，收口在 service 层；handler 不直接调平台 API。
package reviewenrich

import (
	"context"

	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/vcs"

	"github.com/oekit/logx"
	"github.com/oekit/pkg/stringx"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// CRSource 是补全作者所需的平台窄接口（*GitProviderService 结构化满足）。
type CRSource interface {
	HasProvider(platform string) bool
	GetCR(ctx context.Context, platform, owner, repo, number string) (*provider.ChangeRequest, error)
}

type Service struct {
	mgr CRSource
}

// New 收具体管理器类型（fx 组合根）；内部仅依赖 CRSource 窄接口。
// *GitProviderService 结构化满足 CRSource。
func New(mgr *gitProvider.GitProviderService) *Service {
	return &Service{mgr: mgr}
}

// FillAuthor 就地补全 req 缺失的作者信息。平台不支持/查询失败时静默跳过
// （作者是展示性字段，不应阻断评审创建）。
func (s *Service) FillAuthor(ctx context.Context, req *domain.Request) {
	if req.Author != "" {
		return
	}
	prID := stringx.TakeOne(req.PullRequestID, req.MergeRequestIID)
	if prID == "" || s == nil || s.mgr == nil || !s.mgr.HasProvider(req.Provider) {
		return
	}
	owner, repo, err := vcs.ParseRepoPath(req.Repo)
	if err != nil {
		return
	}
	cr, err := s.mgr.GetCR(ctx, req.Provider, owner, repo, prID)
	if err != nil {
		logx.CtxWarnf(ctx, "reviewenrich: failed to get CR for %s/%s #%s: %v", owner, repo, prID, err)
		return
	}
	if cr == nil || cr.Author == nil {
		return
	}
	req.Author = cr.Author.Username
	req.AuthorName = stringx.TakeOne(cr.Author.Name, cr.Author.Username)
	req.AuthorAvatar = cr.Author.AvatarURL
}
