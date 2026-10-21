package review

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"

	//lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
	"github.com/oekit/logx"
)

// service.go：Service 装配（配置/构造/生命周期）——业务方法按职责分布在同包文件中。
type Executor interface {
	Execute(ctx context.Context, id string, req domain.Request, provider domain.Provider, progressReporter progress.ProgressReporter) (domain.Response, error)
}

type SessionRepo interface {
	repository.SessionCanceller
}

type ServiceConfig struct {
	ReviewRepo      repository.ReviewRepository
	OCRSettingsRepo repository.OCRSettingsRepository
	ProjectRepo     repository.ProjectPolicyRepository
	GlobalRulesRepo repository.GlobalRulesRepository
	RulePackageRepo repository.OCRRulePackageRepository
	AgentRepo       repository.ReviewerAgentRepository
	SessionRepo     SessionRepo
	Policy          *policy.Engine
	Providers       domain.Providers
	Runner          domain.Runner
	PreviewRunner   domain.PreviewRunner
	ScanRunner      domain.ScanRunner
	// Delegator 暴露 OCR v1.7.11+ 的 delegate 子命令（host-agent 模式）。
	// 与 PreviewRunner/ScanRunner 不同，delegate 不调 LLM。
	// 可为 nil（OCR binary < v1.7.11 或测试桩），使用方需判空。
	Delegator       domain.Delegator
	ProgressManager *progress.ProgressManager
	Executor        Executor
	// FailureReporter 统一失败上报（历史/落库/通知/status/指标）。
	// nil 时构造仅落库的默认实现（不通知、不写 status），保证 worker 路径
	// 在最小装配（如测试）下仍有终态落库。
	FailureReporter *FailureReporter
	// Concurrency 限制同时进行的 review 数（worker 与 scan 共享）。
	// <=0 表示不限制。在构造期一次性创建信号量，避免运行期懒初始化导致的数据竞争。
	Concurrency int
	// HeartbeatInterval 是执行期心跳间隔：执行中定期刷新 updated_at，
	// 防止长评审被 stale 重排误判为孤儿而重复执行。<=0 时禁用心跳。
	// 调用方应保证其小于 worker_stale_after（约定取 stale_after/3）。
	HeartbeatInterval time.Duration
	// ArgusEnabled 表示 argus 引擎可用（argus.bot_url 已配置）。
	// false 时 engine=argus 的创建请求 fail-fast 拒绝，而非入队后在 worker 失败。
	ArgusEnabled bool
}

type Service struct {
	reviewRepo      repository.ReviewRepository
	ocrSettingsRepo repository.OCRSettingsRepository
	projectRepo     repository.ProjectPolicyRepository
	globalRulesRepo repository.GlobalRulesRepository
	rulePackageRepo repository.OCRRulePackageRepository
	agentRepo       repository.ReviewerAgentRepository
	sessionRepo     SessionRepo
	policy          *policy.Engine
	Providers       domain.Providers
	runner          domain.Runner
	PreviewRunner   domain.PreviewRunner
	ScanRunner      domain.ScanRunner
	Delegator       domain.Delegator
	progressManager *progress.ProgressManager
	Executor        Executor
	argusEnabled    bool

	// runtime 聚合 worker/scan 共享的进程内生命周期设施
	//（并发信号量、in-flight、扫描取消注册表）。
	runtime *scanRuntime

	// heartbeatInterval 执行期心跳间隔（<=0 禁用）。
	heartbeatInterval time.Duration

	// Refactored components
	scanOrchestrator *ScanOrchestrator
	failureReporter  *FailureReporter
}

func NewService(cfg ServiceConfig) *Service {
	s := &Service{
		reviewRepo:        cfg.ReviewRepo,
		ocrSettingsRepo:   cfg.OCRSettingsRepo,
		projectRepo:       cfg.ProjectRepo,
		globalRulesRepo:   cfg.GlobalRulesRepo,
		rulePackageRepo:   cfg.RulePackageRepo,
		agentRepo:         cfg.AgentRepo,
		sessionRepo:       cfg.SessionRepo,
		policy:            cfg.Policy,
		Providers:         cfg.Providers,
		runner:            cfg.Runner,
		PreviewRunner:     cfg.PreviewRunner,
		ScanRunner:        cfg.ScanRunner,
		Delegator:         cfg.Delegator,
		progressManager:   cfg.ProgressManager,
		Executor:          cfg.Executor,
		argusEnabled:      cfg.ArgusEnabled,
		heartbeatInterval: cfg.HeartbeatInterval,
	}
	// 构造期一次性创建共享运行时（含并发信号量），消除运行期懒初始化的数据竞争。
	// 信号量 nil 表示不限制（Concurrency<=0）。
	s.runtime = &scanRuntime{}
	if cfg.Concurrency > 0 {
		s.runtime.sem = make(chan struct{}, cfg.Concurrency)
		logx.Infof("review concurrency limit=%d (worker & scan shared)", cfg.Concurrency)
	}

	// Initialize refactored components
	s.scanOrchestrator = NewScanOrchestrator(s)

	// 失败上报：未注入完整 reporter 时退化为仅落库（不通知、不写 status），
	// 保证执行前失败也有终态记录。
	if cfg.FailureReporter != nil {
		s.failureReporter = cfg.FailureReporter
	} else {
		s.failureReporter = NewFailureReporter(cfg.ReviewRepo, nil, nil)
	}

	return s
}
