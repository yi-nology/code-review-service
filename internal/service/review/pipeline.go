package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/strutil"
	"github.com/yi-nology/code-review-service/internal/vcs"

	"github.com/oekit/logx"
)

// pipeline.go：评审的**前置编排**——把「一次评审要跑起来之前该做的事」收在一处。
//
// 它归 service/review 而不是 internal/eino 的原因：这段流程是**评审领域的事**
// （克隆工作区 → 项目策略 → 文件过滤 → 规则合并 → 记忆召回 → OCR 执行参数），
// 与「用哪个引擎跑」无关。此前它住在引擎适配层里，于是引擎适配层直接持 6 个
// 仓储、绕过 service 层直读 5 张业务表，又反向调用本包的 13 个导出函数——
// 两套编排互相调用，是耦合的主要来源。
//
// 引擎侧（internal/eino）现在只做三件事：起引擎上下文、跑本管线、把产出交给
// workflow / 单 runner。它不再知道仓储。

// ---- 窄接口（消费方定义、提供方实现）----
//
// 用窄接口而不是直接依赖 service/agent 与 service/memory：这两个包不需要成为
// review 的依赖方向成员——Go 的接口是结构化的，实现方**不需要** import 声明方。

// AgentRegistry 是 Agent 名册的只读窄接口。
//
// 前置编排只需要「当前注册了哪些 Agent」来写审计步骤，不需要执行能力。
type AgentRegistry interface {
	// AgentNames 返回已注册 Agent 的名字列表（按注册顺序）。
	AgentNames() []string
}

// MemoryRecaller 是跨会话长期记忆的召回窄接口。
//
// 参数与返回值都是 review 侧自有类型，review 因此不依赖记忆内核的请求类型。
type MemoryRecaller interface {
	// RecallReviewContext 召回该仓库的历史记忆上下文；空串表示无可召回。
	RecallReviewContext(ctx context.Context, provider, repo, reviewID, query string) (string, error)
}

// StageError 是前置编排的失败，带**失败环节标签**。
//
// 环节标签要跨过管线边界回到引擎侧（失败上报与审计都按环节归类），
// 因此用 error 包装而不是让管线返回一堆具名字段。
type StageError struct {
	// Stage 失败环节（repository / scope / ...）。
	Stage string
	// Err 底层错误。
	Err error
}

func (e *StageError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

// Unwrap 保留底层错误链（errors.Is / errors.As 仍可用）。
func (e *StageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// StageOf 从错误链里取前置编排的失败环节；不是管线失败时返回空串。
func StageOf(err error) string {
	var se *StageError
	if errors.As(err, &se) {
		return se.Stage
	}
	return ""
}

// ---- 管线契约 ----

// PipelineInput 是前置编排的输入。
type PipelineInput struct {
	// ReviewID 评审 ID（审计与状态都挂在它上面）。
	ReviewID string
	// Request 评审请求。
	Request domain.Request
	// Provider git 平台提供者（负责 clone + 解析 PR/MR diff）。
	Provider domain.Provider
	// ProgressReporter 进度上报器（nil = 不上报）。
	ProgressReporter progress.ProgressReporter
	// UseWorkflow 是否走多智能体 workflow 路径（否则走单 runner）。
	UseWorkflow bool
	// ExecutionMode 执行模式的可读描述（写进审计步骤）。
	ExecutionMode string
}

// PipelineOutput 是前置编排的产出。
type PipelineOutput struct {
	// Workspace 克隆出的工作区（含 diff、scope、OCR 覆盖参数）。
	Workspace domain.Workspace
	// ProjectPolicy 本次生效的项目策略。
	ProjectPolicy projectpolicy.ProjectPolicy
	// MemoryContext 召回的跨会话记忆（空串 = 无）。
	MemoryContext string
	// Cleanup 清理本次编排产生的临时资源（工作区目录 + 合并后的规则文件）。
	//
	// 清理责任**留在调用方**：工作区与规则文件在 OCR 执行期间必须存活，
	// 放进管线内部清理会在引擎跑之前就把它们删掉。
	Cleanup func()
}

// PipelineRunner 是前置编排的端口（引擎侧只依赖它）。
type PipelineRunner interface {
	// Run 执行前置编排。
	Run(ctx context.Context, in PipelineInput) (PipelineOutput, error)
}

// Pipeline 是 PipelineRunner 的默认实现。
type Pipeline struct {
	store           ReviewStateStore
	projectRepo     repository.ProjectPolicyRepository
	globalRulesRepo repository.GlobalRulesRepository
	rulePackageRepo repository.OCRRulePackageRepository
	agentRepo       repository.ReviewerAgentRepository
	ocrSettingsRepo repository.OCRSettingsRepository
	agentRegistry   AgentRegistry
	memory          MemoryRecaller
}

// NewPipeline 构造前置编排器。
func NewPipeline(
	store ReviewStateStore,
	projectRepo repository.ProjectPolicyRepository,
	globalRulesRepo repository.GlobalRulesRepository,
	rulePackageRepo repository.OCRRulePackageRepository,
	agentRepo repository.ReviewerAgentRepository,
	ocrSettingsRepo repository.OCRSettingsRepository,
	agentRegistry AgentRegistry,
	memory MemoryRecaller,
) *Pipeline {
	return &Pipeline{
		store:           store,
		projectRepo:     projectRepo,
		globalRulesRepo: globalRulesRepo,
		rulePackageRepo: rulePackageRepo,
		agentRepo:       agentRepo,
		ocrSettingsRepo: ocrSettingsRepo,
		agentRegistry:   agentRegistry,
		memory:          memory,
	}
}

// Run 执行前置编排（Step 0 ~ 2.8）。
//
// 失败语义：任一步失败即返回错误，调用方负责失败上报（统一走 FailureReporter）。
func (p *Pipeline) Run(ctx context.Context, in PipelineInput) (PipelineOutput, error) {
	id := in.ReviewID
	req := in.Request

	if in.ProgressReporter != nil {
		ctx = progress.WithProgressReporter(ctx, in.ProgressReporter)
	}

	// 清理责任在调用方（工作区与规则文件在引擎执行期间必须存活）。
	var cleanups []func()
	defer func() {
		// 管线内部失败时（返回错误），此处不清理——交给调用方的失败路径处理；
		// 正常路径下由 PipelineOutput.Cleanup 统一清理。
		_ = cleanups
	}()

	// Step 0: 初始 OCR 输入（此时工作区还不存在，只落 binary 与标识）。
	initialInput := domain.OCRInput{
		Binary: ResolveOCRBinary(ctx, p.ocrSettingsRepo),
		Workspace: domain.WorkspaceInfo{
			Provider: req.Provider,
			Repo:     req.Repo,
			BaseSHA:  req.BaseSHA,
			HeadSHA:  req.HeadSHA,
		},
		StartedAt: time.Now().UTC(),
	}
	if err := WriteOCRInput(ctx, p.store, id, initialInput); err != nil {
		logx.Warn("failed to save initial OCR input", "review_id", id, "error", err)
	}

	// Step 1: 克隆仓库 + 准备工作区
	stepRepo := NewAuditStep("repository")
	workspace, err := in.Provider.Prepare(ctx, req)
	if err != nil {
		p.appendAuditStep(ctx, id, stepRepo.Failed("仓库准备工作区失败", "git clone + resolve PR diff", req, err))
		return PipelineOutput{}, &StageError{Stage: "repository", Err: err}
	}
	p.appendAuditStep(ctx, id, stepRepo.Done(
		"仓库工作区准备完成",
		"git clone + resolve PR/MR diff",
		map[string]any{"provider": req.Provider, "repo": req.Repo, "base_sha": req.BaseSHA, "head_sha": req.HeadSHA, "pull_request_id": req.PullRequestID},
		map[string]any{"dir": workspace.Dir, "base_sha": workspace.BaseSHA, "head_sha": workspace.HeadSHA, "diff_bytes": len(workspace.Diff), "scope": workspace.Scope},
	))

	if workspace.BaseSHA != "" || workspace.HeadSHA != "" {
		req.BaseSHA = workspace.BaseSHA
		req.HeadSHA = workspace.HeadSHA
		p.updateRequestSHAs(ctx, id, req)
	}

	// Step 2: 加载项目策略
	stepPolicy := NewAuditStep("policy")
	projectPolicy := projectpolicy.NewDefault(req.Provider, req.Repo)
	if p.projectRepo != nil {
		if pol, err := p.projectRepo.GetOrCreateDefault(ctx, req.Provider, req.Repo); err == nil {
			projectPolicy = pol
		} else if !errors.Is(err, repoerr.ErrNotFound) {
			logx.Warn("failed to get/create project policy, using default", "error", err)
		}
	}
	workspace.OCR = ProjectPolicyToOCROverrides(projectPolicy)
	if len(req.Requirements) > 0 {
		workspace.OCR.Background = MergeRequirementsIntoBackground(workspace.OCR.Background, req.Requirements)
	}

	// 记录项目策略配置
	var ignorePaths []string
	if len(projectPolicy.IgnorePaths) > 0 {
		if err := json.Unmarshal(projectPolicy.IgnorePaths, &ignorePaths); err != nil {
			logx.Warnf("[pipeline] failed to parse ignore_paths for %s/%s: %v", req.Provider, req.Repo, err)
		}
	}
	p.appendAuditStep(ctx, id, stepPolicy.Done(
		"项目策略已加载",
		"从 DB 读取 project_policies 表，合并默认值",
		map[string]any{"provider": req.Provider, "repo": req.Repo},
		map[string]any{
			"fail_on":         projectPolicy.FailOn,
			"ignore_paths":    ignorePaths,
			"ocr_language":    workspace.OCR.Language,
			"ocr_background":  strutil.Truncate(workspace.OCR.Background, 200),
			"ocr_concurrency": workspace.OCR.Concurrency,
			"ocr_timeout":     workspace.OCR.TimeoutMinutes,
			"is_default":      projectPolicy.FailOn == "major" && len(ignorePaths) == 0,
		},
	))

	// Step 2.5: 过滤文件（如果配置了 ignore_paths）
	if len(ignorePaths) > 0 {
		stepScope := NewAuditStep("scope")
		workspace, err = ApplyProjectScope(ctx, workspace, ignorePaths)
		if err != nil {
			p.appendAuditStep(ctx, id, stepScope.Failed("应用项目审查策略失败", "apply ignore_paths filter", ignorePaths, err))
			return PipelineOutput{}, &StageError{Stage: "scope", Err: err}
		}
		p.appendAuditStep(ctx, id, stepScope.Done(
			"文件过滤完成",
			fmt.Sprintf("按 ignore_paths 过滤（%d 个规则）", len(ignorePaths)),
			map[string]any{"ignore_paths": ignorePaths, "diff_bytes_before": len(workspace.Diff)},
			map[string]any{"changed_files": workspace.Scope.ChangedFiles, "reviewed_files": workspace.Scope.ReviewedFiles, "excluded_files": workspace.Scope.ExcludedFiles},
		))
	}

	// Step 2.6: 合并规则
	stepRules := NewAuditStep("rules")
	if p.globalRulesRepo != nil {
		rulePath, cleanup, ruleErr := ResolveRuleFileForAgents(ctx, projectPolicy, p.rulePackageRepo, p.globalRulesRepo, AgentRulePackageRefs(ctx, p.agentRepo)...)
		if ruleErr != nil {
			logx.Warn("failed to resolve rule file", "error", ruleErr)
			p.appendAuditStep(ctx, id, stepRules.Failed("规则合并失败", "ResolveRuleFileForAgents", nil, ruleErr))
		} else if rulePath != "" {
			workspace.RulePath = rulePath
			if cleanup != nil {
				cleanups = append(cleanups, cleanup)
			}
			// 读取合并后的规则文件内容（截断显示）
			ruleContent := ""
			if data, readErr := os.ReadFile(rulePath); readErr == nil {
				ruleContent = strutil.Truncate(string(data), 1000)
			}
			p.appendAuditStep(ctx, id, stepRules.Done(
				"审查规则已合并",
				"合并 6 层规则源：项目内联 → 项目规则包 → Agent 规则包 → 全局规则包 → 项目匹配规则包 → 旧版 global_rules",
				map[string]any{
					"project_rules":       strutil.Truncate(string(projectPolicy.Rules), 200),
					"rule_package_ids":    projectPolicy.RulePackageIDs,
					"agent_rule_packages": AgentRulePackageRefs(ctx, p.agentRepo),
				},
				map[string]any{"rule_path": rulePath, "rules_content": ruleContent},
			))
		} else {
			p.appendAuditStep(ctx, id, stepRules.Done("无审查规则", "所有规则源均为空，LLM 自由审查", nil, nil))
		}
	} else {
		p.appendAuditStep(ctx, id, stepRules.Done("规则仓库未配置", "globalRulesRepo 为 nil", nil, nil))
	}

	if workspace.Dir != "" {
		cleanups = append(cleanups, func() {
			if err := os.RemoveAll(workspace.Dir); err != nil {
				logx.Warn("failed to remove review workspace", "dir", workspace.Dir, "error", err)
			}
		})
	}

	ocrInput := p.buildOCRInput(ctx, workspace, req)
	if err := WriteOCRInput(ctx, p.store, id, ocrInput); err != nil {
		logx.Warn("failed to update OCR input", "review_id", id, "error", err)
	}

	// Step 2.65: 召回跨会话长期记忆（底座 agent-memory）。
	// 召回失败/为空都不阻断评审：记忆是加分项，不是前置条件。
	memoryContext := p.recallReviewMemory(ctx, id, req, workspace)
	if memoryContext != "" {
		workspace.OCR.Background = mergeMemoryContext(workspace.OCR.Background, memoryContext)
		p.appendAuditStep(ctx, id, NewAuditStep("memory").Done(
			"已召回历史评审记忆",
			"agent-memory 跨会话长期记忆（按仓库隔离，注入评审背景）",
			map[string]any{"provider": req.Provider, "repo": req.Repo},
			map[string]any{"chars": len(memoryContext)},
		))
	}

	// Step 2.7: Agent 选择
	stepAgent := NewAuditStep("agent")
	if p.agentRegistry != nil {
		agentNames := p.agentRegistry.AgentNames()
		p.appendAuditStep(ctx, id, stepAgent.Done(
			fmt.Sprintf("Agent 选择完成：%d 个注册", len(agentNames)),
			"AgentManager.RunAgentsForProject → 4 层优先级：项目白名单 → 技术栈检测 → is_default → 全部",
			map[string]any{"provider": req.Provider, "repo": req.Repo},
			map[string]any{
				"registered_agents": agentNames,
				"total":             len(agentNames),
				"execution_mode":    in.ExecutionMode,
			},
		))
	} else {
		p.appendAuditStep(ctx, id, stepAgent.Done("Agent 管理器未配置", "使用默认单 agent 模式", nil, nil))
	}

	// OCR resume support
	if !in.UseWorkflow {
		if prior, err := p.store.GetByReviewID(ctx, id); err == nil && prior != nil {
			if sid := strings.TrimSpace(prior.Response.Report.OCR.SessionID); sid != "" {
				workspace.ResumeSessionID = sid
				logx.Infof("[pipeline] review %s 启用 OCR resume，session_id=%s", id, sid)
			}
		}
	}

	// Recover diff from git if empty
	if strings.TrimSpace(workspace.Diff) == "" && workspace.Dir != "" && workspace.BaseSHA != "" && workspace.HeadSHA != "" {
		if git, gitErr := vcs.Open(workspace.Dir, ""); gitErr == nil {
			defer func() { _ = git.Close() }()
			if diff, diffErr := git.Diff(ctx, workspace.BaseSHA, workspace.HeadSHA); diffErr == nil && len(diff) > 0 {
				workspace.Diff = diff
				logx.Infof("recovered empty diff from git for review %s (%d bytes)", id, len(diff))
			} else {
				logx.Warnf("failed to recover diff from git for review %s: %v", id, diffErr)
			}
		}
	}

	// Step 2.8: OCR 执行参数
	stepOCRConfig := NewAuditStep("ocr_config")
	p.appendAuditStep(ctx, id, stepOCRConfig.Done(
		"OCR 执行参数已确定",
		"合并项目策略覆盖 + 全局 OCR 设置",
		map[string]any{
			"ocr_binary":     workspace.OCR.Binary,
			"ocr_language":   workspace.OCR.Language,
			"ocr_audience":   workspace.OCR.Audience,
			"ocr_provider":   workspace.OCR.Provider,
			"ocr_url":        workspace.OCR.URL,
			"ocr_model":      workspace.OCR.Model,
			"ocr_protocol":   workspace.OCR.Protocol,
			"ocr_background": strutil.Truncate(workspace.OCR.Background, 300),
			"concurrency":    workspace.OCR.Concurrency,
			"timeout_min":    workspace.OCR.TimeoutMinutes,
			"max_tools":      workspace.OCR.MaxTools,
			"rule_path":      workspace.RulePath,
			"diff_bytes":     len(workspace.Diff),
			"head_sha":       workspace.HeadSHA,
			"base_sha":       workspace.BaseSHA,
		},
		map[string]any{
			"ready":     workspace.Dir != "" && len(workspace.Diff) > 0,
			"dir":       workspace.Dir,
			"has_rules": workspace.RulePath != "",
		},
	))

	return PipelineOutput{
		Workspace:     workspace,
		ProjectPolicy: projectPolicy,
		MemoryContext: memoryContext,
		Cleanup: func() {
			for _, c := range cleanups {
				if c != nil {
					c()
				}
			}
		},
	}, nil
}

// buildOCRInput 构造 OCR 执行输入快照。
func (p *Pipeline) buildOCRInput(ctx context.Context, workspace domain.Workspace, req domain.Request) domain.OCRInput {
	return domain.OCRInput{
		Binary: ResolveOCRBinary(ctx, p.ocrSettingsRepo),
		Args: []string{
			"review",
			"--repo", workspace.Dir,
			"--from", workspace.BaseSHA,
			"--to", workspace.HeadSHA,
			"--format", "json",
		},
		Workspace: domain.WorkspaceInfo{
			Provider: req.Provider,
			Repo:     req.Repo,
			BaseSHA:  workspace.BaseSHA,
			HeadSHA:  workspace.HeadSHA,
			Dir:      workspace.Dir,
		},
		RulePath:  workspace.RulePath,
		StartedAt: time.Now().UTC(),
	}
}

// recallReviewMemory 召回该仓库的历史记忆，供本次评审注入背景。
//
// 召回失败不阻断评审：记忆是加分项，不是前置条件。
func (p *Pipeline) recallReviewMemory(ctx context.Context, id string, req domain.Request, workspace domain.Workspace) string {
	if p.memory == nil {
		return ""
	}
	recalled, err := p.memory.RecallReviewContext(ctx, req.Provider, req.Repo, id, memoryQuery(req, workspace))
	if err != nil {
		logx.Warnf("[pipeline] recall review memory failed for %s/%s: %v", req.Provider, req.Repo, err)
		return ""
	}
	return recalled
}

// updateRequestSHAs 把解析出的 base/head SHA 回写到评审记录。
func (p *Pipeline) updateRequestSHAs(ctx context.Context, id string, req domain.Request) {
	r, err := p.store.GetByReviewID(ctx, id)
	if err != nil {
		return
	}
	r.Request = req
	r.UpdatedAt = time.Now().UTC()
	if err := p.store.Save(ctx, r); err != nil {
		logx.Warn("failed to update request SHAs", "review_id", id, "error", err)
	}
}

// appendAuditStep 追加一个审计步骤到评审历史。
func (p *Pipeline) appendAuditStep(ctx context.Context, id string, event domain.Event) {
	AppendHistoryEvent(ctx, p.store, id, event)
}

// memoryQuery 构造召回查询词。
//
// 用本次变更触及的文件路径当查询词：召回取数按词面命中，
// 「这次动了哪些文件」正是「哪些历史经验与本次相关」的最强信号。
func memoryQuery(req domain.Request, workspace domain.Workspace) string {
	var sb strings.Builder
	if req.Repo != "" {
		sb.WriteString(req.Repo)
	}
	paths := changedPaths(workspace)
	for _, p := range paths {
		sb.WriteString(" ")
		sb.WriteString(p)
	}
	return strings.TrimSpace(sb.String())
}

// changedPaths 从 diff 里取变更文件路径（最多 20 个，避免查询词过长）。
//
// 变更文件清单只在 diff 里——工作区的 scope 元数据只有计数，没有路径。
func changedPaths(workspace domain.Workspace) []string {
	if strings.TrimSpace(workspace.Diff) == "" {
		return nil
	}
	var out []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(workspace.Diff, "\n") {
		if !strings.HasPrefix(line, "+++ ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "+++"))
		path = strings.TrimSpace(strings.TrimPrefix(path, "b/"))
		if path == "" || path == "/dev/null" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
		if len(out) >= 20 {
			break
		}
	}
	return out
}

// mergeMemoryContext 把召回的记忆并入评审背景。
//
// 记忆放在背景**末尾**：背景是模型读的「本次评审须知」，
// 记忆是历史经验，排在后面不会盖过项目规则与需求。
func mergeMemoryContext(background, memoryContext string) string {
	background = strings.TrimSpace(background)
	memoryContext = strings.TrimSpace(memoryContext)
	switch {
	case memoryContext == "":
		return background
	case background == "":
		return memoryContext
	default:
		return fmt.Sprintf("%s\n\n%s", background, memoryContext)
	}
}
