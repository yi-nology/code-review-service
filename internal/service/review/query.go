package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	//lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
	"github.com/oekit/logx"
)

// query.go：查询/预览/委托/扫描入口与共享辅助。
func (s *Service) Get(ctx context.Context, reviewID string) (domain.Record, bool, error) {
	review, err := s.reviewRepo.GetByReviewID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return domain.Record{}, false, nil
		}
		return domain.Record{}, false, err
	}

	// review 已是 domain.Record（Request/Response 强类型），仅按需补 Analysis。
	if len(review.RawOCR) > 0 && review.Response.Status == "success" {
		result := domain.Result{
			OCR:      review.Response.Report.OCR,
			Scope:    review.Response.Report.Scope,
			Warnings: review.Response.Report.Warnings,
			Findings: review.Response.Findings,
		}
		review.Response.Analysis = BuildAnalysisFromRaw(json.RawMessage(review.RawOCR), result)
	}

	return *review, true, nil
}

func (s *Service) List(ctx context.Context, options ListOptions) (ListResult, error) {
	if options.Limit <= 0 || options.Limit > 200 {
		options.Limit = 50
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	page := options.Offset/options.Limit + 1
	result, err := s.reviewRepo.List(ctx, domain.ListOptions{
		Status:     options.Status,
		GateStatus: options.GateStatus,
		Provider:   options.Provider,
		Repo:       options.Repo,
		Page:       page,
		PageSize:   options.Limit,
		SortBy:     options.SortBy,
		SortOrder:  options.SortOrder,
	})
	if err != nil {
		return ListResult{}, err
	}

	items := make([]ListItem, 0, len(result.Items))
	for _, review := range result.Items {
		// review 已是 domain.Record，Request/Response 强类型，无需 unmarshal。
		items = append(items, ListItem{
			ReviewID:        review.ID,
			Provider:        review.Request.Provider,
			Repo:            review.Request.Repo,
			Status:          review.Response.Status,
			Summary:         review.Response.Summary,
			Gate:            review.Response.Gate,
			Findings:        len(review.Response.Findings),
			Score:           review.Response.Report.Score.Value,
			Grade:           review.Response.Report.Score.Grade,
			ReportGenerated: review.Response.Report.Generated,
			RetryCount:      review.RetryCount,
			Author:          review.Author,
			AuthorName:      review.AuthorName,
			AuthorAvatar:    review.AuthorAvatar,
			CreatedAt:       review.CreatedAt,
			UpdatedAt:       review.UpdatedAt,
		})
	}

	agg, err := s.reviewRepo.AggregateStats(ctx, domain.ListOptions{
		Status:     options.Status,
		GateStatus: options.GateStatus,
		Provider:   options.Provider,
		Repo:       options.Repo,
	})
	if err != nil {
		logx.Warn("failed to compute aggregate stats", "error", err)
		agg = &domain.AggregateStats{}
	}

	return ListResult{
		Items:      items,
		Total:      int(result.Total),
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.Pages,
		Stats: ListStats{
			Total:       int(result.Total),
			Running:     agg.Running,
			GateFailed:  agg.GateFailed,
			Findings:    agg.Findings,
			TotalTokens: agg.TotalTokens,
		},
	}, nil
}

func (s *Service) Preview(ctx context.Context, req domain.Request) ([]string, error) {
	return s.previewCore(ctx, req, nil, nil, true)
}

// PreviewWithScope 同 Preview，但支持限定扫描范围（paths/exclude）。
// scan preview 用它把用户在 scan 页指定的 --path/--exclude 传给 ocr scan --preview。
func (s *Service) PreviewWithScope(ctx context.Context, req domain.Request, paths, exclude []string) ([]string, error) {
	return s.previewCore(ctx, req, paths, exclude, true)
}

// previewCore 是 Preview / PreviewWithScope / scanPreview 共享的单一实现。
// includeAgentRules=false 时跳过审查员引用的规则包（仅 scan preview 用：scan 不调 LLM，
// 收集 agent 规则是浪费一次 DB 查询，且会污染 ocr scan --preview 的输入）。
func (s *Service) previewCore(ctx context.Context, req domain.Request, paths, exclude []string, includeAgentRules bool) ([]string, error) {
	return withPreviewWorkspace(s, ctx, req, paths, exclude, includeAgentRules, func(workspace domain.Workspace) ([]string, error) {
		if s.PreviewRunner == nil {
			return nil, fmt.Errorf("preview feature not configured")
		}
		result, err := s.PreviewRunner.Preview(ctx, workspace)
		if err != nil {
			return nil, err
		}
		return result.Files, nil
	})
}

// withPreviewWorkspace 准备一个 review-preview 风格的 workspace（clone + project scope +
// 规则文件），然后调用 run 回调。所有需要 workspace.Dir 的轻量端点（Preview / Delegate）
// 都应通过此 helper 复用 workspace 准备逻辑。
//
// 是包级泛型函数（Go 不允许 method 上带类型参数）。paths/exclude 仅写入
// workspace.ScanPaths/ScanExclude，由具体 runner 决定如何使用。
// includeAgentRules=false 时跳过审查员规则包收集（scan / delegate rule 等不调 LLM 的场景）。
func withPreviewWorkspace[T any](
	s *Service,
	ctx context.Context,
	req domain.Request,
	paths, exclude []string,
	includeAgentRules bool,
	run func(workspace domain.Workspace) (T, error),
) (T, error) {
	var zero T
	if err := validate(req); err != nil {
		return zero, err
	}

	p, ok := s.Providers.Get(req.Provider)
	if !ok {
		logx.Errorf("Preview: provider not found: requested=%q", req.Provider)
		return zero, fmt.Errorf("%w: %s", domain.ErrUnsupportedProvider, req.Provider)
	}

	workspace, err := p.Prepare(ctx, req)
	if err != nil {
		return zero, err
	}
	if workspace.Dir != "" {
		defer func() { _ = os.RemoveAll(workspace.Dir) }()
	}

	projectPolicy := projectpolicy.NewDefault(req.Provider, req.Repo)
	if s.projectRepo != nil {
		if policyRecord, err := s.projectRepo.GetOrCreateDefault(ctx, req.Provider, req.Repo); err == nil {
			projectPolicy = policyRecord
		} else if !errors.Is(err, repoerr.ErrNotFound) {
			logx.Warn("failed to get/create project policy, using default", "error", err)
		}
	}

	var ignorePaths []string
	if len(projectPolicy.IgnorePaths) > 0 {
		if err := json.Unmarshal(projectPolicy.IgnorePaths, &ignorePaths); err != nil {
			logx.Warn("failed to decode ignore_paths", "error", err)
		}
	}
	if len(ignorePaths) > 0 {
		workspace, err = ApplyProjectScope(ctx, workspace, ignorePaths)
		if err != nil {
			return zero, err
		}
	}

	workspace.ScanPaths = paths
	workspace.ScanExclude = exclude

	if s.globalRulesRepo != nil {
		var agentRefs [][]byte
		if includeAgentRules {
			agentRefs = AgentRulePackageRefs(ctx, s.agentRepo)
		}
		rulePath, cleanup, ruleErr := ResolveRuleFileForAgents(ctx, projectPolicy, s.rulePackageRepo, s.globalRulesRepo, agentRefs...)
		if ruleErr != nil {
			logx.Warn("failed to resolve rule file", "error", ruleErr)
		} else if rulePath != "" {
			workspace.RulePath = rulePath
			if cleanup != nil {
				defer cleanup()
			}
		}
	}

	return run(workspace)
}

// DelegatePreview 调 OCR v1.7.11+ `ocr delegate preview`，返回将参与评审的文件列表 +
// mode/from/to/merge_base/total_*/Files 元数据（host-agent 范式，不调 LLM）。
// workspace 由 Providers.Prepare 内部 clone，Dir 在调用结束后自动清理。
func (s *Service) DelegatePreview(ctx context.Context, req domain.Request, exclude []string) (*domain.DelegatePreviewResult, error) {
	if s.Delegator == nil {
		return nil, fmt.Errorf("delegate feature not configured (OCR < v1.7.11 或未注入 Delegator)")
	}
	return withPreviewWorkspace(s, ctx, req, nil, exclude, false, func(workspace domain.Workspace) (*domain.DelegatePreviewResult, error) {
		return s.Delegator.DelegatePreview(ctx, workspace)
	})
}

// DelegateRule 调 OCR v1.7.11+ `ocr delegate rule <paths...>`，返回按内容分组的解析规则 markdown。
// paths 为空时返回全量规则。workspace 由 Providers.Prepare 内部 clone，Dir 在调用结束后自动清理。
func (s *Service) DelegateRule(ctx context.Context, req domain.Request, paths []string) (*domain.DelegateRuleResult, error) {
	if s.Delegator == nil {
		return nil, fmt.Errorf("delegate feature not configured (OCR < v1.7.11 或未注入 Delegator)")
	}
	return withPreviewWorkspace(s, ctx, req, paths, nil, false, func(workspace domain.Workspace) (*domain.DelegateRuleResult, error) {
		return s.Delegator.DelegateRule(ctx, workspace, paths)
	})
}

func (s *Service) Scan(ctx context.Context, req domain.ScanRequest) (*domain.ScanResult, error) {
	return s.scanOrchestrator.Scan(ctx, req)
}

func (s *Service) updateReviewFields(ctx context.Context, reviewID string, fn func(review *domain.Record)) error {
	return s.reviewRepo.UpdateInTransaction(ctx, reviewID, func(review *domain.Record) error {
		fn(review)
		return nil
	})
}

// ProjectPolicyToOCROverrides 将项目策略中的 OCR 配置项映射为 review.OCROverrides。
// 作为 scan 路径与 executor 路径的单一实现，消除重复。
func ProjectPolicyToOCROverrides(p projectpolicy.ProjectPolicy) domain.OCROverrides {
	return domain.OCROverrides{
		Enabled:                 p.OCREnabled,
		Concurrency:             p.OCRConcurrency,
		TimeoutMinutes:          p.OCRTimeoutMinutes,
		MaxTools:                p.OCRMaxTools,
		DefaultSeverity:         p.OCRDefaultSeverity,
		Background:              p.OCRBackground,
		BackgroundFile:          p.OCRBackgroundFile,
		Audience:                p.OCRAudience,
		Provider:                p.OCRProvider,
		URL:                     p.OCRURL,
		Model:                   p.OCRModel,
		MaxGitProcs:             p.OCRMaxGitProcs,
		UseAnthropic:            p.OCRUseAnthropic,
		Protocol:                p.OCRProtocol,
		ExtraBody:               p.OCRExtraBody,
		Language:                p.OCRLanguage,
		Binary:                  p.OCRBinary,
		ToolsPath:               p.OCRToolsPath,
		TelemetryEnabled:        p.OCRTelemetryEnabled,
		TelemetryExporter:       p.OCRTelemetryExporter,
		TelemetryOTLPEndpoint:   p.OCRTelemetryOTLPEndpoint,
		TelemetryContentLogging: p.OCRTelemetryContentLogging,
	}
}

// MergeRequirementsIntoBackground 把需求/spec 追加进 OCR --background 上下文。
//
// OCR 的 --background 让审查 agent 拿到 diff 之外的背景（需求、约束、验收标准），
// 官方明确建议「有 PR/需求上下文时始终传 --background，质量提升显著」。
// 此前 req.Requirements 仅在 workflow 多 agent 路径（嵌入 LLM prompt）里被使用，
// 单 runner 路径直接调 OCR 时丢失了需求上下文 —— 本函数补上这个缺口。
//
// 合并规则：项目策略的 Background 在前，requirements 追加其后（已有内容则换行分隔），
// 两者都保留，让 OCR 同时看到项目级背景和本次评审需求。
func MergeRequirementsIntoBackground(existing string, requirements []string) string {
	var parts []string
	if b := strings.TrimSpace(existing); b != "" {
		parts = append(parts, b)
	}
	for _, r := range requirements {
		if r = strings.TrimSpace(r); r != "" {
			parts = append(parts, r)
		}
	}
	return strings.Join(parts, "\n")
}
