package suggestion

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// ReviewFetcher 解耦对 review.Service 的依赖：suggestion 只需要按 ID 读取评审记录。
type ReviewFetcher interface {
	Get(ctx context.Context, reviewID string) (domain.Record, bool, error)
}

// GitTokenSource 解耦对 GitProviderService 的依赖：仅需要按平台取 token / skipTLS。
type GitTokenSource interface {
	GetToken(platform string) string
	GetSkipTLS(platform string) bool
}

// Service 负责评审建议补丁的生成、可用性检查与应用（git apply/commit/push）。
//
// 从 review.Service 抽离（#8 上帝对象拆分）：原 review.Service 同时承担 CRUD、worker 池、
// scan 编排与 suggestion 补丁应用，职责过载。suggestion 是自包含特性——只需读取评审记录、
// 准备工作区、执行 git——因此独立成包，仅依赖窄接口。
type Service struct {
	reviews   ReviewFetcher
	providers domain.Providers
	gitSvc    GitTokenSource
}

func NewService(reviews ReviewFetcher, providers domain.Providers, gitSvc GitTokenSource) *Service {
	return &Service{reviews: reviews, providers: providers, gitSvc: gitSvc}
}

func (s *Service) CheckSuggestionPatch(ctx context.Context, reviewID string, indexes []int) (*SuggestionPatchCheck, bool, error) {
	record, found, err := s.reviews.Get(ctx, reviewID)
	if err != nil || !found {
		return nil, found, err
	}
	preview := BuildSuggestionPatch(record.ID, record.Response.Findings, indexes)
	result := &SuggestionPatchCheck{SuggestionPatchPreview: preview}
	if strings.TrimSpace(preview.Patch) == "" {
		result.Applicable = false
		result.Error = "no patch generated"
		return result, true, nil
	}
	if s.providers == nil {
		result.Applicable = false
		result.Error = "providers not configured"
		return result, true, nil
	}
	provider, ok := s.providers.Get(record.Request.Provider)
	if !ok {
		result.Applicable = false
		result.Error = fmt.Sprintf("unsupported provider %q", record.Request.Provider)
		return result, true, nil
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	workspace, err := provider.Prepare(checkCtx, record.Request)
	if err != nil {
		result.Applicable = false
		result.Error = err.Error()
		return result, true, nil
	}
	if workspace.Dir != "" {
		defer func() { _ = os.RemoveAll(workspace.Dir) }()
	}
	output, err := runGitApplyCheck(checkCtx, workspace.Dir, preview.Patch)
	result.Output = output
	result.Applicable = err == nil
	if err != nil {
		result.Error = err.Error()
	}
	return result, true, nil
}

func (s *Service) ApplySuggestionPatch(ctx context.Context, reviewID string, opts SuggestionPatchApplyOptions) (*SuggestionPatchApplyResult, bool, error) {
	record, found, err := s.reviews.Get(ctx, reviewID)
	if err != nil || !found {
		return nil, found, err
	}
	preview := BuildSuggestionPatch(record.ID, record.Response.Findings, opts.Indexes)
	result := &SuggestionPatchApplyResult{SuggestionPatchPreview: preview}
	if strings.TrimSpace(preview.Patch) == "" {
		result.Error = "no patch generated"
		return result, true, nil
	}
	if s.providers == nil {
		result.Error = "providers not configured"
		return result, true, nil
	}
	provider, ok := s.providers.Get(record.Request.Provider)
	if !ok {
		result.Error = fmt.Sprintf("unsupported provider %q", record.Request.Provider)
		return result, true, nil
	}

	applyCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	workspace, err := provider.Prepare(applyCtx, record.Request)
	if err != nil {
		result.Error = err.Error()
		return result, true, nil
	}
	keepWorkspace := false
	if workspace.Dir != "" {
		defer func() {
			if !keepWorkspace {
				_ = os.RemoveAll(workspace.Dir)
			}
		}()
	}

	branch := normalizeSuggestionPatchBranch(opts.Branch, record.ID)
	pushAuth := suggestionPatchPushAuth{}
	if opts.Push && s.gitSvc != nil {
		pushAuth.Token = s.gitSvc.GetToken(record.Request.Provider)
		pushAuth.SkipTLS = s.gitSvc.GetSkipTLS(record.Request.Provider)
	}
	output, commitSHA, err := applyPatchCommit(applyCtx, workspace.Dir, branch, preview.Patch, opts.Push, pushAuth)
	result.Branch = branch
	result.CommitSHA = commitSHA
	result.Workspace = workspace.Dir
	result.Output = output
	result.Applied = err == nil
	result.Pushed = err == nil && opts.Push
	if err != nil {
		result.Error = err.Error()
		return result, true, nil
	}
	keepWorkspace = true
	return result, true, nil
}
