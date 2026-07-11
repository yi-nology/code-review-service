package ocr

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"github.com/google/go-github/v69/github"
)

// Versioner 实现 domain.OCRVersioner（吸收 ocr/versioner.go）。
type Versioner struct {
	settings Settings
	resolver *ConfigResolver
}

// NewVersioner 构造 Versioner。
func NewVersioner(settings Settings, resolver *ConfigResolver) *Versioner {
	return &Versioner{settings: settings, resolver: resolver}
}

// OCRVersion 实现 domain.OCRVersioner：真实执行 `<binary> --version` 并解析输出。
//
// 预期输出：
//
//	open-code-review v1.6.4 (3f41551) darwin/arm64
//	built at: 2026-06-26T13:24:22Z
//	https://github.com/alibaba/open-code-review
func (v *Versioner) OCRVersion(ctx context.Context) (*domain.OCRVersion, error) {
	data, err := v.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	binary := resolveBinaryFrom(adaptSettings(data), v.resolver.fallback)

	stdout, stderr, err := Run(ctx, binary, "--version")
	if err != nil {
		return nil, fmt.Errorf("执行 %s --version 失败: %w; stderr: %s",
			binary, err, strings.TrimSpace(string(stderr)))
	}
	return parseOCRVersion(stdout), nil
}

// ocrVersionRe 匹配 `ocr --version` 第一行的 "version (commit) platform"。
var ocrVersionRe = regexp.MustCompile(`(v\d+\.\d+\.\d+(?:[-.\w]*)?)\s*\(([0-9a-fA-F]+)\)\s+(\S+)`)

// parseOCRVersion 解析 `ocr --version` 输出。
func parseOCRVersion(out []byte) *domain.OCRVersion {
	ver := &domain.OCRVersion{}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return ver
	}
	lines := strings.Split(text, "\n")

	if len(lines) > 0 {
		if m := ocrVersionRe.FindStringSubmatch(strings.TrimSpace(lines[0])); m != nil {
			ver.Version = m[1]
			ver.Commit = m[2]
			ver.Platform = m[3]
		}
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "built at:") {
			ver.BuildTime = strings.TrimSpace(strings.TrimPrefix(line, "built at:"))
			break
		}
	}
	return ver
}

// ProviderLister 实现 domain.LLMProviderLister（吸收 ocr/provider_lister.go）。
type ProviderLister struct {
	settings Settings
	resolver *ConfigResolver
}

// NewProviderLister 构造 ProviderLister。
func NewProviderLister(settings Settings, resolver *ConfigResolver) *ProviderLister {
	return &ProviderLister{settings: settings, resolver: resolver}
}

// ListLLMProviders 实现 domain.LLMProviderLister。
func (l *ProviderLister) ListLLMProviders(_ context.Context) ([]domain.LLMProvider, error) {
	// 阶段 A 占位：从 presets 列出；阶段 A+5 接 internal.Run "ocr llm providers" 命令
	return []domain.LLMProvider{
		{Name: "openai", Protocol: "openai"},
		{Name: "anthropic", Protocol: "anthropic"},
	}, nil
}

// RulesChecker 实现 domain.RulesChecker（吸收 ocr/rules_checker.go）。
type RulesChecker struct {
	settings Settings
	rules    GlobalRules
	resolver *ConfigResolver
}

// GlobalRules 是 GlobalRulesSource 在 internal/ocr 包的别名。
type GlobalRules = settingsGlobalRules

type settingsGlobalRules interface {
	Get(ctx context.Context) (*struct {
		Rules []byte
	}, error)
}

// NewRulesChecker 构造 RulesChecker。
func NewRulesChecker(settings Settings, rules GlobalRules, resolver *ConfigResolver) *RulesChecker {
	return &RulesChecker{settings: settings, rules: rules, resolver: resolver}
}

// RulesCheck 实现 domain.RulesChecker。
func (rc *RulesChecker) RulesCheck(_ context.Context, _ string) (*domain.RulesCheckResult, error) {
	// 阶段 A 占位
	return &domain.RulesCheckResult{Applicable: true, Rule: "stage-a-placeholder", Message: "规则匹配成功"}, nil
}

// Viewer 实现 domain.ViewerManager。
//
// 真正通过 exec 拉起 `ocr viewer` 子进程（默认监听 localhost:5483），
// 进程状态由 mu 保护；cmd.Wait 在独立 goroutine 中回收，退出时清空状态。
type Viewer struct {
	settings Settings
	resolver *ConfigResolver

	mu   sync.Mutex
	cmd  *exec.Cmd
	addr string
}

// NewViewer 构造 Viewer。
func NewViewer(settings Settings, resolver *ConfigResolver) *Viewer {
	return &Viewer{settings: settings, resolver: resolver}
}

// ViewerStatus 实现 domain.ViewerManager。
func (v *Viewer) ViewerStatus(_ context.Context) (*domain.ViewerStatus, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.runningLocked() {
		return &domain.ViewerStatus{Running: false, Message: "Viewer 未运行"}, nil
	}
	return &domain.ViewerStatus{Running: true, Addr: v.addr, Message: fmt.Sprintf("Viewer 运行中 (%s)", v.addr)}, nil
}

// StartViewer 实现 domain.ViewerManager。
//
// 幂等：已有运行中进程时直接返回当前状态。addr 为空时使用默认 localhost:5483。
func (v *Viewer) StartViewer(ctx context.Context, addr string) (*domain.ViewerStatus, error) {
	if strings.TrimSpace(addr) == "" {
		addr = "localhost:5483"
	}

	v.mu.Lock()
	if v.runningLocked() {
		cur := &domain.ViewerStatus{Running: true, Addr: v.addr, Message: fmt.Sprintf("Viewer 已在运行 (%s)", v.addr)}
		v.mu.Unlock()
		return cur, nil
	}
	v.mu.Unlock()

	binary, err := v.resolveBinary(ctx)
	if err != nil {
		return &domain.ViewerStatus{Running: false, Message: err.Error()}, err
	}

	cmd := exec.Command(binary, "viewer", "--addr", addr)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		msg := fmt.Sprintf("启动 Viewer 失败: %v", err)
		return &domain.ViewerStatus{Running: false, Message: msg}, fmt.Errorf("%s", msg)
	}

	v.mu.Lock()
	v.cmd = cmd
	v.addr = addr
	v.mu.Unlock()

	pid := cmd.Process.Pid
	async.GoSafe(func() {
		waitErr := cmd.Wait()
		v.mu.Lock()
		// 仅当仍是同一个 cmd 时才清空（避免 Stop 后竞态覆盖）。
		if v.cmd == cmd {
			v.cmd = nil
			v.addr = ""
		}
		v.mu.Unlock()
		if waitErr != nil {
			logx.Warn("ocr viewer process exited", "pid", pid, "error", waitErr)
		} else {
			logx.Info("ocr viewer process exited", "pid", pid)
		}
	})

	logx.Info("ocr viewer started", "pid", pid, "addr", addr)
	return &domain.ViewerStatus{
		Running: true,
		Addr:    addr,
		Message: fmt.Sprintf("Viewer 已启动 (%s)", addr),
	}, nil
}

// StopViewer 实现 domain.ViewerManager。
//
// 向进程组发送 SIGTERM；进程已退出或不存在时幂等返回未运行状态。
func (v *Viewer) StopViewer(_ context.Context) (*domain.ViewerStatus, error) {
	v.mu.Lock()
	cmd := v.cmd
	v.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return &domain.ViewerStatus{Running: false, Message: "Viewer 未运行"}, nil
	}

	// Setpgid 让子进程成为独立进程组，用负 pid 向整组发信号（含 viewer spawn 的子进程）。
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)

	// 等待进程退出（最多 5s），随后兜底 SIGKILL。
	done := make(chan struct{})
	go func() { _, _ = cmd.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
	}

	v.mu.Lock()
	v.cmd = nil
	v.addr = ""
	v.mu.Unlock()

	return &domain.ViewerStatus{Running: false, Message: "Viewer 已停止"}, nil
}

// runningLocked 报告当前是否有一个尚未退出的 viewer 进程。调用方需持锁。
func (v *Viewer) runningLocked() bool {
	if v.cmd == nil || v.cmd.ProcessState != nil {
		// ProcessState 非 nil 说明 Wait 已返回（进程已退出）。
		return false
	}
	return true
}

// resolveBinary 从 settings + fallback 解析 OCR 二进制路径。
func (v *Viewer) resolveBinary(ctx context.Context) (string, error) {
	data, err := v.settings.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	return resolveBinaryFrom(adaptSettings(data), v.resolver.fallback), nil
}

// Upgrader 实现 domain.OCRUpgrader（吸收 ocr/upgrader.go）。
type Upgrader struct {
	settings   Settings
	resolver   *ConfigResolver
	versioner  *Versioner
	installDir string
	repo       string
}

// NewUpgrader 构造 Upgrader。
func NewUpgrader(settings Settings, resolver *ConfigResolver, versioner *Versioner, installDir, repo string) *Upgrader {
	if strings.TrimSpace(repo) == "" {
		repo = "alibaba/open-code-review"
	}
	return &Upgrader{
		settings:   settings,
		resolver:   resolver,
		versioner:  versioner,
		installDir: installDir,
		repo:       repo,
	}
}

// CheckUpdate 实现 domain.OCRUpgrader。
func (u *Upgrader) CheckUpdate(ctx context.Context) (*domain.OCRUpdateInfo, error) {
	current := ""
	if u.versioner != nil {
		if v, err := u.versioner.OCRVersion(ctx); err == nil && v != nil {
			current = v.Version
		}
	}

	client := github.NewClient(nil)
	repoParts := strings.SplitN(u.repo, "/", 2)
	if len(repoParts) != 2 {
		return &domain.OCRUpdateInfo{
			CurrentVersion:  current,
			LatestVersion:   current,
			UpdateAvailable: false,
		}, nil
	}

	release, _, err := client.Repositories.GetLatestRelease(ctx, repoParts[0], repoParts[1])
	if err != nil {
		if current != "" {
			return &domain.OCRUpdateInfo{
				CurrentVersion:  current,
				LatestVersion:   current,
				UpdateAvailable: false,
			}, nil
		}
		return nil, fmt.Errorf("获取最新版本失败: %w", err)
	}

	latest := strings.TrimSpace(release.GetTagName())
	releaseNotes := release.GetBody()
	releaseURL := release.GetHTMLURL()

	hasUpdate := false
	if current != "" && latest != "" {
		currentVer := normalizeVersion(current)
		latestVer := normalizeVersion(latest)
		if currentVer != latestVer {
			hasUpdate = true
		}
	}

	return &domain.OCRUpdateInfo{
		CurrentVersion:  current,
		LatestVersion:   latest,
		UpdateAvailable: hasUpdate,
		ReleaseNotes:    releaseNotes,
		ReleaseURL:      releaseURL,
	}, nil
}

// normalizeVersion 去掉版本号前面的 v 前缀。
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// Upgrade 实现 domain.OCRUpgrader。
func (u *Upgrader) Upgrade(_ context.Context, targetVersion string) (*domain.OCRUpgradeStatus, error) {
	return &domain.OCRUpgradeStatus{
		State:   domain.UpgradeIdle,
		Version: targetVersion,
	}, nil
}

// UpgradeStatus 实现 domain.OCRUpgrader。
func (u *Upgrader) UpgradeStatus(_ context.Context) (*domain.OCRUpgradeStatus, error) {
	return &domain.OCRUpgradeStatus{State: domain.UpgradeIdle}, nil
}

// adaptSettings 把 SettingsData 转为 domain ocrsettings.Settings（#6：不再依赖 models）。
func adaptSettings(data *SettingsData) *ocrsettings.Settings {
	if data == nil {
		return nil
	}
	// 阶段 A 占位：仅填充 Binary 字段，其他字段由 legacy runner 自己解析 settings
	return &ocrsettings.Settings{Binary: data.Binary}
}
