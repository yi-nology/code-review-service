// Package memory 接入 agent-memory：跨会话的长期记忆内核。
//
// 底座分工（记忆是代码，LLM 只做单次提炼）：
//
//	捕获   宿主把「发生了什么」喂进来；落库前先脱敏——那是唯一还来得及的时刻
//	提炼   LLM 只做单次提炼，产物一律是待判定候选，不直接落库
//	晋升   五道检查（价值/证据/实体/重复/矛盾），裁决权在底座
//	召回   按隔离维度取数，预算/去重/多样性由底座收口，注入前一律包成数据区
//
// 本包只做宿主适配：评审域的「仓库」投影成底座的实体，评审产出投影成事件。
// 底座不认识评审域概念，本包也不往底座的类型里塞评审域枚举——
// 实体类型是不透明字符串，由宿主定义。
package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yi-nology/code-review-service/internal/ocr"

	"github.com/yi-nology/agent-memory/application"
	"github.com/yi-nology/agent-memory/pipeline/entity"
	"github.com/yi-nology/agent-memory/pipeline/extractor"
	"github.com/yi-nology/agent-memory/pipeline/promotion"
	"github.com/yi-nology/agent-memory/pipeline/recall"
	"github.com/yi-nology/agent-memory/ports"
	"github.com/yi-nology/agent-memory/store"
	"github.com/yi-nology/agent-memory/types"
)

// entityTypeRepository 是宿主定义的实体类型（底座不解释该字符串）。
const entityTypeRepository = types.EntityType("repository")

// Service 是评审域的长期记忆门面。
type Service struct {
	app   *application.Service
	llm   ports.LLM
	ns    string
	host  string
	limit int

	// resolver 底座实体归一器（惰性构造）。
	//
	// 用 sync.Once 而非裸字段：捕获与召回两条路径都可能触发实体登记，
	// 而它们可能并发（一次评审的捕获与另一次评审的召回同时发生），
	// 裸字段的惰性构造在并发下会写出两个 resolver 指向同一存储，
	// 虽然不至于出错，但那是「靠运气没坏」的写法。
	resolverOnce sync.Once
	resolver     *entity.Resolver
	resolverErr  error

	// namespaces 记本实例碰过的命名空间，供后台维护扫描。
	//
	// 底座存储接口全部按命名空间过滤，没有跨命名空间列举的能力，
	// 所以「有哪些 ns 要维护」只能由宿主自己记。
	nsMu       sync.RWMutex
	namespaces map[store.Namespace]struct{}
}

// Config 是记忆服务的装配配置。
type Config struct {
	// Namespace 记忆库命名空间前缀。底座按它隔离记忆，本服务在后面拼
	// 「平台/仓库」——不同仓库的记忆互不可见。
	Namespace string
	// Host 产出方标识（写进事件的来源字段）。
	Host string
	// RecallLimit 单次召回条数上限（0=用底座默认）。
	RecallLimit int
	// RecallBudget 单次召回的 token 预算（0=用底座默认 1500）。
	RecallBudget int
	// RecallTimeout 单次召回的总时限（0=用底座默认 2s）。
	//
	// 召回在评审**主链**上（internal/eino/executor.go）：PG 挂起或慢查询时，
	// 没有时限就等于评审无限等待。
	RecallTimeout time.Duration
}

// Deps 是装配依赖。
type Deps struct {
	// Store 记忆存储（必需）。
	Store store.Store
	// Redactor 脱敏器（必需）：落库前打码是唯一还来得及的时刻。
	Redactor ports.Redactor
	// Bander 注入防伪（必需）：召回内容是不可信内容。
	Bander ports.Bander
	// LLM 模型能力（可选）：缺失时提炼显式失败，其余功能照常。
	LLM ports.LLM
	// Escalation 实体归一升级通道（可选）：缺失时归一存疑的候选落「送人工」。
	Escalation promotion.Escalation
	// Estimator token 计量（可选）：缺失时退静态代理。
	Estimator ports.TokenEstimator
}

// New 组装记忆服务。
func New(cfg Config, deps Deps) (*Service, error) {
	if deps.Store == nil {
		return nil, errors.New("记忆存储未装配")
	}
	if deps.Redactor == nil {
		return nil, errors.New("记忆脱敏器未装配：落库前打码是唯一还来得及的时刻")
	}
	if deps.Bander == nil {
		return nil, errors.New("记忆注入防伪未装配：召回内容是不可信内容")
	}

	// 召回配置以**底座默认**为基线，只覆盖本仓可配的项。
	//
	// 此前手写 recall.Config{TotalBudget: ...}，其余字段全是零值——而底座的
	// normalize() 只补 Sources/TopK/TotalBudget/MaxRaw，**不补** PerEntityMax
	// 与 RecallTimeout：
	//   - RecallTimeout 0 = 单次召回无时限。召回在评审主链上，PG 挂起时评审
	//     会一直等下去（这是「主链被存储拖死」而非「召回慢一点」）。
	//   - PerEntityMax 0 = 单个实体不限条数。同一实体挂十条事实时，它们说的
	//     是同一件事的十个侧面，全塞进去会把其他实体的事实挤出预算。
	// 改用底座默认（RecallTimeout 2s / PerEntityMax 3），确需放开时再显式配。
	recallCfg := recall.DefaultConfig()
	if cfg.RecallBudget > 0 {
		recallCfg.TotalBudget = cfg.RecallBudget
	}
	if cfg.RecallTimeout > 0 {
		recallCfg.RecallTimeout = cfg.RecallTimeout
	}
	app, err := application.New(application.Config{
		Namespace: store.Namespace(cfg.Namespace),
		Host:      cfg.Host,
		Recall:    recallCfg,
	}, application.Deps{
		Store:      deps.Store,
		Redactor:   deps.Redactor,
		Bander:     deps.Bander,
		LLM:        deps.LLM,
		Escalation: deps.Escalation,
		Estimator:  deps.Estimator,
	})
	if err != nil {
		return nil, fmt.Errorf("装配记忆服务: %w", err)
	}

	return &Service{
		app:   app,
		llm:   deps.LLM,
		ns:    cfg.Namespace,
		host:  cfg.Host,
		limit: cfg.RecallLimit,
	}, nil
}

// Namespace 返回某平台/仓库的记忆库名（底座的隔离维度）。
func (s *Service) Namespace(provider, repo string) store.Namespace {
	return store.Namespace(strings.Join([]string{s.ns, provider, repo}, "/"))
}

// ReviewCapture 是一次评审的记忆捕获输入。
type ReviewCapture struct {
	Provider   string
	Repo       string
	ReviewID   string
	SessionID  string
	BaseSHA    string
	HeadSHA    string
	Verdict    string
	GatePassed bool
	Summary    string
	Findings   []FindingSummary
}

// FindingSummary 是参与记忆的 finding 摘要。
//
// 只带标题与位置，不带正文：记忆是长期资产，正文里常有一次性细节
// （具体行号、临时变量名），进记忆库后既占预算又稀释召回。
type FindingSummary struct {
	Severity string
	Category string
	Path     string
	Title    string
}

// CaptureOutcome 是一次捕获的结果统计。
type CaptureOutcome struct {
	// Accepted 事件是否被接收。
	Accepted bool
	// SkipReason 未接收的原因（底座给出的判定）。
	SkipReason string
	// Candidates 提炼出的候选数。
	Candidates int
	// Promoted 晋升成事实卡的候选数。
	Promoted int
	// Merged 与既有事实合并的候选数。
	Merged int
	// Dropped 被底座判据丢弃的候选数。
	Dropped int
	// NeedsReview 归一存疑、送人工的候选数。
	NeedsReview int
}

// CaptureReview 把一次评审的产出沉淀为长期记忆。
//
// 链路：捕获 → 提炼 → 逐条落库候选 → 晋升。
// 提炼器**不落库**，候选必须先存进库才能晋升——顺序反了晋升会失败。
func (s *Service) CaptureReview(ctx context.Context, in ReviewCapture) (CaptureOutcome, error) {
	var outcome CaptureOutcome
	if strings.TrimSpace(in.Repo) == "" {
		return outcome, errors.New("记忆捕获缺少仓库名")
	}

	ns := s.Namespace(in.Provider, in.Repo)
	// 登记命名空间，供后台维护扫描（记忆库不能只进不出）。
	s.rememberNamespace(ns)
	sessionID := in.SessionID
	if sessionID == "" {
		sessionID = in.ReviewID
	}

	// 仓库实体先落地：晋升的实体检查要求主体能归一到已登记的实体，
	// 而底座**不新建实体**——建实体是宿主的事。
	entityID, err := s.ensureRepositoryEntity(ctx, ns, in.Provider, in.Repo)
	if err != nil {
		return outcome, fmt.Errorf("登记仓库实体: %w", err)
	}

	captured, err := s.app.Capture(ctx, application.CaptureInput{
		Namespace: ns,
		EventID:   eventID(in.ReviewID),
		Host:      s.host,
		SessionID: sessionID,
		ThreadID:  threadID(in.Provider, in.Repo),
		User:      "review-service",
		Type:      types.EventToolResult,
		Content:   renderReviewEvent(in),
		Payload: map[string]any{
			"provider":    in.Provider,
			"repo":        in.Repo,
			"review_id":   in.ReviewID,
			"base_sha":    in.BaseSHA,
			"head_sha":    in.HeadSHA,
			"verdict":     in.Verdict,
			"gate_passed": in.GatePassed,
			"finding_cnt": len(in.Findings),
			"entity_id":   entityID,
		},
		Provenance: types.Provenance{Origin: types.OriginModel, Host: s.host},
	})
	if err != nil {
		return outcome, fmt.Errorf("捕获评审记忆: %w", err)
	}
	outcome.Accepted = captured.Accepted
	outcome.SkipReason = captured.SkipReason
	if !captured.Accepted || captured.Event == nil {
		return outcome, nil
	}

	// 提炼：没有模型就没有候选，绝不静默返回空串假装提炼过。
	ext := extractor.New(s.llm, extractor.WithPrompt(buildExtractPrompt(in.Provider, in.Repo)))
	candidates, err := ext.Extract(ctx, ns, sessionID, []*types.RawEvent{captured.Event})
	if err != nil {
		if errors.Is(err, ports.ErrNoLLM) {
			return outcome, nil
		}
		return outcome, fmt.Errorf("提炼评审记忆: %w", err)
	}
	outcome.Candidates = len(candidates)

	for _, c := range candidates {
		if err := s.app.Store().SaveCandidate(ctx, ns, c); err != nil {
			return outcome, fmt.Errorf("落库记忆候选: %w", err)
		}
		verdict, err := s.app.Promote(ctx, ns, c)
		if err != nil {
			return outcome, fmt.Errorf("晋升记忆候选: %w", err)
		}
		switch verdict.Decision {
		case promotion.DecisionPromote:
			outcome.Promoted++
		case promotion.DecisionMerge:
			outcome.Merged++
		case promotion.DecisionDrop:
			outcome.Dropped++
		case promotion.DecisionReview:
			outcome.NeedsReview++
		}
	}
	return outcome, nil
}

// RecallRequest 是记忆召回请求。
type RecallRequest struct {
	Provider string
	Repo     string
	ReviewID string
	// Query 召回查询词（本次评审的主题）。空表示按实体取全部事实。
	Query string
}

// RecallReview 召回某仓库的历史记忆，返回可注入评审上下文的文本。
//
// 没有记忆时返回空串（调用方照常评审）——底座刻意不返回「没有记忆」这类句子，
// 那会占预算并诱导模型去想记忆这件事。
// RecallReviewContext 是 review.MemoryRecaller 窄接口的实现。
//
// 参数与返回值都是 review 侧自有类型——review 因此不依赖记忆内核的请求类型
// （Go 的接口是结构化的：实现方不需要 import 声明方）。
func (s *Service) RecallReviewContext(ctx context.Context, provider, repo, reviewID, query string) (string, error) {
	return s.RecallReview(ctx, RecallRequest{
		Provider: provider,
		Repo:     repo,
		ReviewID: reviewID,
		Query:    query,
	})
}

func (s *Service) RecallReview(ctx context.Context, req RecallRequest) (string, error) {
	if strings.TrimSpace(req.Repo) == "" {
		return "", nil
	}
	ns := s.Namespace(req.Provider, req.Repo)
	// 召回路径也要登记命名空间：只登记捕获路径的话，
	// 「只被召回过、从没捕获过」的仓库不在维护范围内——
	// 而那正是召回路径长期读取的 ns，其数据同样会无限增长。
	s.rememberNamespace(ns)
	entityID, err := s.ensureRepositoryEntity(ctx, ns, req.Provider, req.Repo)
	if err != nil {
		return "", fmt.Errorf("登记仓库实体: %w", err)
	}

	res, err := s.app.Recall(ctx, recall.Request{
		Namespace: ns,
		Query:     req.Query,
		SessionID: req.ReviewID,
		ThreadID:  threadID(req.Provider, req.Repo),
		Entities:  []string{entityID},
		Limit:     s.limit,
	})
	if err != nil {
		return "", fmt.Errorf("召回评审记忆: %w", err)
	}
	if res.Empty() {
		return "", nil
	}
	return res.Render(), nil
}

// ensureRepositoryEntity 登记仓库实体（幂等）。
//
// 底座**不新建实体**（「新建实体不是选项」），所以宿主必须在捕获前把仓库登记好，
// 否则晋升的实体检查会把候选全部送去人工。
//
// 登记走底座的 entity.Resolver：它比手写版本多两样主机自己拿不到的能力——
//   - **别名冲突检测**：别名归一后若撞上别的实体的规范名，返回冲突而不是默默覆盖。
//     默默覆盖会让其中一个实体从此「查不到」，而它并没有消失。
//   - **名字归一**：按底座的 NormalizeAlias 口径（小写 + 折叠空白），
//     同一仓库用不同大小写登记会归到同一个实体，而不是裂成两个。
func (s *Service) ensureRepositoryEntity(ctx context.Context, ns store.Namespace, provider, repo string) (string, error) {
	name := provider + "/" + repo

	resolver, err := s.entityResolver()
	if err != nil {
		return "", err
	}

	ent, err := resolver.Register(ctx, ns, name, entityTypeRepository, repo, types.DecidedBy(types.DecidedRule))
	if err != nil {
		return "", fmt.Errorf("登记仓库实体 %s: %w", name, err)
	}
	return ent.ID, nil
}

// entityResolver 构造底座归一器（惰性构造，装配失败只影响登记路径）。
func (s *Service) entityResolver() (*entity.Resolver, error) {
	s.resolverOnce.Do(func() {
		s.resolver, s.resolverErr = entity.New(s.app.Store())
	})
	if s.resolverErr != nil {
		return nil, fmt.Errorf("装配实体归一器: %w", s.resolverErr)
	}
	return s.resolver, nil
}

// eventID 由评审 ID 推出稳定的事件标识（可回溯）。
func eventID(reviewID string) string {
	return "review-" + reviewID
}

// threadID 由平台与仓库推出线程标识（活跃实体栈的作用域）。
func threadID(provider, repo string) string {
	return provider + "/" + repo
}

// renderReviewEvent 把评审产出渲染成事件正文。
//
// 正文是提炼器的唯一输入，也是「逐字引用」必须命中的原文——
// 所以这里写什么，决定了将来能提炼出什么样的记忆。
func renderReviewEvent(in ReviewCapture) string {
	var b strings.Builder
	b.WriteString("代码评审产出\n")
	b.WriteString("仓库: " + in.Provider + "/" + in.Repo + "\n")
	if in.ReviewID != "" {
		b.WriteString("评审: " + in.ReviewID + "\n")
	}
	if in.BaseSHA != "" || in.HeadSHA != "" {
		b.WriteString("提交范围: " + in.BaseSHA + ".." + in.HeadSHA + "\n")
	}
	if in.Verdict != "" {
		b.WriteString("结论: " + in.Verdict + "\n")
	}
	b.WriteString("质量门禁: ")
	if in.GatePassed {
		b.WriteString("通过\n")
	} else {
		b.WriteString("未通过\n")
	}
	if summary := strings.TrimSpace(in.Summary); summary != "" {
		b.WriteString("摘要: " + summary + "\n")
	}
	if len(in.Findings) > 0 {
		b.WriteString("问题清单:\n")
		for _, f := range in.Findings {
			b.WriteString("- [" + f.Severity + "]")
			if f.Path != "" {
				b.WriteString(" " + f.Path)
			}
			if f.Title != "" {
				b.WriteString(" " + f.Title)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// buildExtractPrompt 构造评审域的提炼提示词（产品面归宿主）。
//
// 主体名钉死成仓库规范名：评审产出的断言说的都是「这个仓库的什么」，
// 主体若散成文件名或模块名，晋升时归一不到仓库，候选会全部落「送人工」——
// 记忆库于是只进不出。
func buildExtractPrompt(provider, repo string) extractor.PromptFunc {
	name := provider + "/" + repo
	return func(events []*types.RawEvent, maxCandidates int) string {
		var b strings.Builder
		b.WriteString("从下面这次代码评审的产出里抽取值得长期记住的稳定事实。\n")
		b.WriteString("只输出 JSON 数组，每个元素包含：\n")
		b.WriteString("  assertion      断言正文（这条记忆说了什么）\n")
		b.WriteString("  quote          逐字引用（必须原样出现在事件文本里，不得改写）\n")
		b.WriteString("  event_id       引用所在事件的 id\n")
		b.WriteString("  type           fact | decision | preference | procedure\n")
		b.WriteString("  subject_name   固定填 " + name + "\n")
		b.WriteString("  importance     high | medium | low\n")
		b.WriteString("  confidence     high | medium | low\n")
		fmt.Fprintf(&b, "最多 %d 条；没有值得记的就输出空数组。\n", maxCandidates)
		b.WriteString("事件：\n")
		for _, e := range events {
			b.WriteString(fmt.Sprintf("- [%s] %s\n", e.ID, e.Content))
		}
		return b.String()
	}
}

// llmAdapter 把项目的 LLM 门面包装成底座的模型端口。
//
// 底座的约定：没有模型时必须返回 ports.ErrNoLLM，**不许返回空串假装成功**——
// 空串会被当成「提炼出了零条候选」，把「没模型」记成「这次没有值得记的事」。
type llmAdapter struct {
	provider ocr.Provider
}

// NewLLMAdapter 构造模型端口适配器。
func NewLLMAdapter(provider ocr.Provider) ports.LLM {
	return llmAdapter{provider: provider}
}

func (a llmAdapter) Complete(ctx context.Context, req ports.LLMRequest) (string, error) {
	if a.provider == nil || !a.provider.Available(ctx) {
		return "", ports.ErrNoLLM
	}
	resp, err := a.provider.Complete(ctx, ocr.CompleteRequest{
		UserPrompt: req.Prompt,
		MaxTokens:  req.MaxTokens,
		JSONMode:   req.JSON,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.Content) == "" {
		return "", ports.ErrNoLLM
	}
	return resp.Content, nil
}

// repositoryResolver 是实体归一升级通道。
//
// 底座的约定：空串 = 判不了（走送人工），error = 通道故障（上抛，不记成「上下文不足」）。
// 评审域的实体只有一个——仓库本身。主体名与仓库规范名/别名匹配得上就归它，
// 匹配不上就如实说判不了，不硬猜：猜错的断言会挂到别的实体上，
// 日后被召回时又以别人的身份出现，比不记更糟。
type repositoryResolver struct{}

// NewRepositoryResolver 构造实体归一升级通道。
func NewRepositoryResolver() promotion.Escalation {
	return repositoryResolver{}
}

func (repositoryResolver) ResolveEntity(
	_ context.Context,
	_ store.Namespace,
	c *types.Candidate,
	existing []types.Entity,
) (string, error) {
	subject := strings.TrimSpace(c.SubjectName)
	if subject == "" {
		return "", nil
	}
	for _, e := range existing {
		if e.CanonicalName == subject {
			return e.ID, nil
		}
		for _, alias := range e.Aliases {
			if alias == subject {
				return e.ID, nil
			}
		}
	}
	return "", nil
}
