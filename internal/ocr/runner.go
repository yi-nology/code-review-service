package ocr

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/progress"
	"github.com/yi-nology/code-review-service/internal/domain/review"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/concurrency/retry"
	"github.com/oekit/logx"
	"github.com/oekit/pkg/convert"
)

// runner.go：OCR Runner 核心——配置热载生命周期与执行主流程（Run/runOnce）。
type Runner struct {
	// baseCfg 是从 DB 解析的全局默认配置（不含 per-execution overrides）。
	// 后台 goroutine 定时刷新；执行时通过 effectiveConfig() 读快照 + 叠加 overrides。
	baseCfg   Config
	baseCfgMu sync.RWMutex

	resolver *ConfigResolver

	// refreshCh 收到信号时立即刷新（不等定时器），用于 PUT settings 后加速生效。
	refreshCh chan struct{}

	// stopCh 关闭后后台 goroutine 退出。
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewRunner 从 Config 构造 Runner。
func NewRunner(cfg Config) *Runner {
	return &Runner{
		baseCfg:   cfg,
		refreshCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}
}

// WithResolver 注入 ConfigResolver 并启动后台刷新 goroutine。
// 注入后 Runner 会每 runnerRefreshInterval 从 DB 读最新全局配置更新 baseCfg；
// 执行时再叠加 per-execution overrides（不读 DB）。
func (r *Runner) WithResolver(resolver *ConfigResolver) *Runner {
	r.resolver = resolver
	// 立即同步刷新一次，避免首次执行用 fallback。
	r.refreshNow()
	// async.GoSafe：后台刷新循环里的 panic 会终止整个进程。
	async.GoSafe(r.refreshLoop)
	return r
}

// NotifyRefresh 触发一次异步刷新（非阻塞）。PUT settings handler 调它加速生效。
func (r *Runner) NotifyRefresh() {
	select {
	case r.refreshCh <- struct{}{}:
	default: // channel 已有待处理信号，不重复入队
	}
}

// Stop 停止后台刷新 goroutine（幂等，供 fx OnStop 调用）。
func (r *Runner) Stop() {
	r.stopOnce.Do(func() { close(r.stopCh) })
}

// refreshLoop 定时 + 信号驱动刷新 baseCfg。
func (r *Runner) refreshLoop() {
	ticker := time.NewTicker(runnerRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.refreshNow()
		case <-r.refreshCh:
			r.refreshNow()
		}
	}
}

// refreshNow 同步从 DB 读取最新全局配置并更新 baseCfg。
// 无 resolver 时为 no-op（保持 fallback 不变）。
func (r *Runner) refreshNow() {
	if r.resolver == nil {
		return
	}
	cfg, err := r.resolver.Resolve(context.Background(), OCROverrides{})
	if err != nil || cfg.Binary == "" {
		return
	}
	r.baseCfgMu.Lock()
	r.baseCfg = cfg
	r.baseCfgMu.Unlock()
}

// effectiveConfig 返回本次执行应使用的 Config：baseCfg 快照 + workspace overrides。
// 纯内存操作（不读 DB），用 BuildConfig 把 overrides 叠加到已缓存的 baseCfg 上。
func (r *Runner) effectiveConfig(overrides OCROverrides) Config {
	r.baseCfgMu.RLock()
	base := r.baseCfg
	r.baseCfgMu.RUnlock()
	return mergeOverrides(base, overrides)
}

// Cfg 返回当前全局默认 Config 的快照（不含 overrides）。
func (r *Runner) Cfg() Config {
	r.baseCfgMu.RLock()
	defer r.baseCfgMu.RUnlock()
	return r.baseCfg
}

// Refresh 替换 baseCfg（供外部强制注入，如测试）。生产路径走后台自动刷新。
func (r *Runner) Refresh(cfg Config) {
	r.baseCfgMu.Lock()
	r.baseCfg = cfg
	r.baseCfgMu.Unlock()
}

// mergeOverrides 把 per-execution overrides 叠加到已从 DB 解析的 base Config 上。
// 语义与 BuildConfig 中 overrides 层一致（overrides 非零值优先），但不再读 DB。
func mergeOverrides(base Config, ov OCROverrides) Config {
	out := base
	if ov.Audience != "" {
		out.Audience = ov.Audience
	}
	if ov.DefaultSeverity != "" {
		out.DefaultSeverity = ov.DefaultSeverity
	}
	if ov.URL != "" {
		out.URL = ov.URL
	}
	if ov.Model != "" {
		out.Model = ov.Model
	}
	if ov.AuthToken != "" {
		out.AuthToken = ov.AuthToken
	}
	if ov.AuthHeader != "" {
		out.AuthHeader = ov.AuthHeader
	}
	if ov.ExtraHeaders != "" {
		out.ExtraHeaders = ov.ExtraHeaders
	}
	if ov.UseAnthropic {
		out.UseAnthropic = true
	}
	if ov.Protocol != "" {
		out.Protocol = ov.Protocol
	}
	if ov.ExtraBody != "" {
		out.ExtraBody = ov.ExtraBody
	}
	if ov.MaxTools > 0 {
		out.MaxTools = ov.MaxTools
	}
	if ov.Background != "" {
		out.Background = ov.Background
	}
	if ov.BackgroundFile != "" {
		out.BackgroundFile = ov.BackgroundFile
	}
	if ov.Language != "" {
		out.Language = ov.Language
	}
	if ov.ToolsPath != "" {
		out.ToolsPath = ov.ToolsPath
	}
	return out
}

// Previewer / Scanner 包装 Runner。
func (r *Runner) Previewer() domain.PreviewRunner { return r }

func (r *Runner) Scanner() domain.ScanRunner { return r }

// Run 实现 domain.Runner。
//
// 直接调 ocr 二进制，保留原 runner.OpenCodeReviewRunner 的所有行为：
//   - 超时 + 重试（默认 1 次，配置可调）+ 指数退避
//   - ctx cancel → SIGKILL 三段式
//   - stderr 行级回调（用于 progress 解析）
//   - JSON 输出解析
func (r *Runner) Run(ctx context.Context, workspace domain.Workspace) (domain.Result, error) {
	if strings.TrimSpace(workspace.Dir) == "" {
		return domain.Result{}, fmt.Errorf("open-code-review requires a checked out workspace")
	}

	gc := r.Cfg()
	timeout := time.Duration(gc.TimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	// MaxRetries <= 0 表示「未显式配置」，用默认 1 次重试。
	// 注意：Config.MaxRetries 目前不暴露到 settings/overrides，BuildConfig 不设置它，
	// 因此零值必然是「未配置」而非「显式禁用」。若未来要支持显式禁用重试，
	// 应给 Config 加一个独立的 disable 标志，而非复用零值语义。
	maxRetries := gc.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultOCRMaxRetries
	}
	attempts := maxRetries + 1

	reporter := progress.ProgressReporterFromContext(ctx)

	var (
		lastErr error
		// lastPartial 是最后一次失败的 partial result（含 OCR session_id，供上层 retry resume）。
		lastPartial domain.Result
		// canceledDuringRun 标记 ctx 是在**运行期间**被取消的（而非两次尝试之间）。
		// 这两种取消在旧实现里返回不同结果，见下方 switch。
		canceledDuringRun bool
	)

	// 重试与退避委托 oekit/concurrency/retry：退避序列、ctx 取消感知、
	// 「可重试与否」的判定门控只有一份实现，不在本地再写一遍循环。
	//
	// 退避参数与旧实现逐字对齐：第 i 次失败后等 retryBaseBackoff * 2^(i-1)
	//（2s / 4s / 8s …），且**不设上限**（旧实现也没有）。
	retryCfg := retry.Config{
		Attempts:    attempts,
		Delay:       retryBaseBackoff,
		Exponential: true,
		Retryable: func(err error) bool {
			// ctx 已取消：不再重试。
			if ctx.Err() != nil {
				return false
			}
			// 二进制缺失：重试无意义（ocr 不存在，再试也是不存在）。
			if IsBinaryMissing(err) {
				return false
			}
			// 值得重试吗：判据来自 harness/toolfail 的策略表，宿主不再自己判。
			return ocrRetryable(err, stderrOf(err))
		},
	}

	result, runErr := retry.DoValue(ctx, retryCfg, func(attempt int) (domain.Result, error) {
		if err := ctx.Err(); err != nil {
			return domain.Result{}, err
		}

		attemptCtx, cancel := context.WithTimeout(ctx, timeout+30*time.Second)
		if reporter != nil {
			attemptCtx = progress.WithProgressReporter(attemptCtx, reporter)
		}

		if attempt > 1 {
			logx.CtxWarnf(ctx, "ocr run failed, retrying attempt %d/%d: %v", attempt-1, maxRetries, lastErr)
			if reporter != nil {
				reporter.ReportAgentStatus("ocr", "retrying", fmt.Sprintf("重试第 %d 次", attempt-1))
			}
		}

		result, err := r.runOnce(attemptCtx, workspace, timeout)
		cancel()
		if err == nil {
			return result, nil
		}
		lastErr = err
		// runOnce 失败时仍可能返回 partial result（OCR v1.7.6+ 崩溃前已 flush 的部分，含 session_id）。
		// 保留最新一份，全部 attempt 失败后返回给 executor，让 retry 时传 --resume 复用 checkpoint。
		if result.OCR.SessionID != "" {
			lastPartial = result
		}
		if ctx.Err() != nil {
			canceledDuringRun = true
		}
		// 不可重试且需人工介入的失败单独留痕（认证失败/权限不足这类：
		// 不会进入退避环，但运维必须看到）。这里在 attempt 内部判，
		// 而不是在 Retryable 门控里——后者在最后一次尝试时不会被调用，
		// 那样这条留痕会在「恰好是最后一次失败」时漏掉。
		if ctx.Err() == nil && !IsBinaryMissing(err) && !ocrRetryable(err, stderrOf(err)) &&
			ocrNeedsOperator(err, stderrOf(err)) {
			logx.CtxErrorf(ctx, "ocr run failed with a failure that needs operator intervention: %v", err)
		}
		return domain.Result{}, err
	})
	if runErr == nil {
		// 某次尝试成功：直接返回它的结果。
		return result, nil
	}
	if lastErr == nil {
		// ctx 在两次尝试之间被取消（退避等待期）：错误来自 ctx 而非 OCR 调用。
		lastErr = runErr
	}

	switch {
	case ctx.Err() != nil && !canceledDuringRun:
		// ctx 在 attempt 之间被取消：与旧实现一致——只回 ctx 错误，不带 partial。
		return domain.Result{}, ctx.Err()
	case lastPartial.OCR.SessionID != "":
		// 全部 attempt 失败：返回 lastPartial（可能含 session_id）+ lastErr。
		// executor 的 recordFailure 应把 lastPartial.OCR.SessionID 持久化，供下次 retry --resume 使用。
		return lastPartial, lastErr
	default:
		return domain.Result{}, lastErr
	}
}

func (r *Runner) runOnce(ctx context.Context, workspace domain.Workspace, timeout time.Duration) (domain.Result, error) {
	fromRef := workspace.BaseSHA
	toRef := workspace.HeadSHA
	if fromRef == "" {
		fromRef = "HEAD"
	}
	// OCR v1.7.6+ resume 模式下，OCR 会校验 (from, to) SHAs 与 session 中存储的是否一致。
	// PR-mode 的 workspace.HeadSHA 可能是合成提交（restoreDeletedFiles / ApplyProjectScope 用
	// 当前时间戳 commit），跨 retry 不稳定——会导致 OCR 报 "resume session range mismatch"。
	// 改用 workspace.OriginalHeadSHA（平台 API 返回的真实 PR head，稳定）作为 --to：
	//   - SHA 仍然能在 workspace.Dir 内解析（cloneAndResolvePR 已 fetch PR head ref）
	//   - OCR 计算 diff against 真实 head，正是我们要审的 PR diff
	//   - 文件正文仍从 workspace.Dir（已 restored deleted files）读取，不丢上下文
	if workspace.ResumeSessionID != "" && workspace.OriginalHeadSHA != "" {
		toRef = workspace.OriginalHeadSHA
	}

	// 取全局默认配置快照 + 叠加 workspace 的项目级 overrides（纯内存，不读 DB）。
	// 全局配置由后台 goroutine 定时从 DB 刷新到 baseCfg。
	cfg := r.effectiveConfig(workspace.OCR)

	audience := cfg.Audience
	if audience == "" {
		audience = "agent"
	}

	var args []string
	// OCR v1.9.5+ --commit：单 commit 评审模式，对比 commit 与其 parent。
	// 与 --from/--to 互斥。
	if workspace.CommitSHA != "" {
		args = []string{
			"review",
			"--repo", workspace.Dir,
			"--commit", workspace.CommitSHA,
			"--format", "json",
			"--concurrency", convert.IntToStr(cfg.Concurrency),
			"--max-git-procs", convert.IntToStr(cfg.MaxGitProcs),
			"--timeout", convert.IntToStr(cfg.TimeoutMinutes),
			"--audience", audience,
		}
	} else {
		args = []string{
			"review",
			"--repo", workspace.Dir,
			"--from", fromRef,
			"--to", toRef,
			"--format", "json",
			"--concurrency", convert.IntToStr(cfg.Concurrency),
			"--max-git-procs", convert.IntToStr(cfg.MaxGitProcs),
			"--timeout", convert.IntToStr(cfg.TimeoutMinutes),
			"--audience", audience,
		}
	}
	// OCR v1.7.6+ --resume：复用上次 session 的已完成文件 checkpoint，跳过重复 LLM 调用。
	// 由 executor 在 retry 路径填入（读上次 review 的 response.report.ocr.session_id）。
	if sid := strings.TrimSpace(workspace.ResumeSessionID); sid != "" {
		args = append(args, "--resume", sid)
		logx.Infof("[ocr] review 启用 resume，复用 session_id=%s", sid)
	}
	rulePath := cfg.RulePath
	if strings.TrimSpace(workspace.RulePath) != "" {
		rulePath = workspace.RulePath
	}
	if strings.TrimSpace(rulePath) != "" {
		args = append(args, "--rule", rulePath)
	}
	if cfg.MaxTools > 0 {
		args = append(args, "--max-tools", convert.IntToStr(cfg.MaxTools))
	}
	toolsArgs, cleanupTools, err := appendToolsArgs(args, cfg)
	if err != nil {
		return domain.Result{}, err
	}
	args = toolsArgs
	if cleanupTools != nil {
		defer cleanupTools()
	}
	// 将语言要求注入 background，确保 LLM 用指定语言输出。
	lang := cfg.Language
	if lang == "" {
		lang = "中文"
	}
	langPrompt := languagePrompt(lang)
	background := strings.TrimSpace(cfg.Background)
	if langPrompt != "" {
		if background != "" {
			background = background + "\n\n" + langPrompt
		} else {
			background = langPrompt
		}
	}
	if background != "" {
		args = append(args, "--background", background)
	}
	// OCR v1.7.6+ 支持 --background-file 读取 Markdown 文件，规避 inline --background 的 8000 字符硬上限。
	// 路径必须解析后落在 workspace.Dir 内（防路径穿越），否则忽略并记日志（不阻断评审）。
	if bgFile, ok := safeBackgroundFile(workspace.Dir, cfg.BackgroundFile); ok {
		args = append(args, "--background-file", bgFile)
	} else if strings.TrimSpace(cfg.BackgroundFile) != "" {
		logx.Warnf("[ocr] background_file %q 不在工作区 %q 内或不存在，已忽略", cfg.BackgroundFile, workspace.Dir)
	}

	cmd := exec.CommandContext(ctx, cfg.Binary, args...)
	cmd.Dir = workspace.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	env := childEnv(os.Environ(), cfg.hasExplicitLLMCredential())
	env = cfg.AppendEnv(env)
	cmd.Env = env

	// MCP server 配置只能通过 OCR 全局 config.json 的 mcp_servers 键传递(上游限制:
	// review 路径忽略 OCR_CONFIG_PATH,无 --mcp flag)。在 spawn 前同步一次。
	// MCP 是全局慢变配置(所有并发评审用同一套 server),用进程锁串行化即可,
	// 与 per-review 的 language 不同(last-writer-wins 在这里无害)。
	syncMCPConfigIfSet(cfg.McpServers)

	stdout := newCappedBuffer(maxSubprocessOutput)
	cmd.Stdout = stdout

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return domain.Result{}, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	progressReporter := progress.ProgressReporterFromContext(ctx)

	if err := cmd.Start(); err != nil {
		return domain.Result{}, &BinaryMissingError{Binary: cfg.Binary, Err: err}
	}

	var stderrLines []string
	var stderrMu sync.Mutex
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)
	async.GoSafe(func() {
		defer stderrWg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			stderrMu.Lock()
			stderrLines = append(stderrLines, line)
			stderrMu.Unlock()

			if progressReporter != nil {
				parseOCRProgress(line, progressReporter)
			}
		}
		if err := scanner.Err(); err != nil {
			logx.Warnf("ocr stderr scanner error: %v", err)
		}
	})

	done := make(chan struct{})
	async.GoSafe(func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
	})

	err = cmd.Wait()
	close(done)
	stderrWg.Wait()
	if err != nil {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		stderrMu.Lock()
		message := strings.Join(stderrLines, "\n")
		stderrMu.Unlock()
		// 失败时仍尝试 parse stdout：OCR v1.7.6+ 在崩溃前可能已 flush 部分 JSON，
		// 其中包含 session_id。把它附在 Result 上返回，让上层 retry 时能传 --resume 复用 checkpoint。
		// parseOpenCodeReviewOutput 对非 JSON / 空 stdout 安全（返回零值 Result）。
		partial := parseOpenCodeReviewOutput(stdout.Bytes(), cfg.DefaultSeverity)
		partial.OCR.Model = cfg.Model
		partial.OCR.Status = "failed"
		return partial, classifyExitError(ctx, err, timeout, message)
	}

	result := parseOpenCodeReviewOutput(stdout.Bytes(), cfg.DefaultSeverity)
	// 填充当前配置的模型名（用于成本归因，parseOpenCodeReviewOutput 无配置上下文）。
	result.OCR.Model = cfg.Model
	return result, nil
}

// appendToolsArgs 把 tools_path 或 tools_config 加到 args。
func appendToolsArgs(args []string, cfg Config) ([]string, func(), error) {
	if strings.TrimSpace(cfg.ToolsPath) != "" {
		args = append(args, "--tools", cfg.ToolsPath)
		return args, nil, nil
	}
	if strings.TrimSpace(cfg.ToolsConfig) != "" {
		f, err := os.CreateTemp("", "ocr-tools-*.json")
		if err != nil {
			return args, nil, fmt.Errorf("create tools config temp file: %w", err)
		}
		if _, err := f.WriteString(cfg.ToolsConfig); err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
			return args, nil, fmt.Errorf("write tools config: %w", err)
		}
		_ = f.Close()
		args = append(args, "--tools", f.Name())
		return args, func() { _ = os.Remove(f.Name()) }, nil
	}
	return args, nil, nil
}
