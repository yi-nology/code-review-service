package ocr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/runner"

	"github.com/oekit/concurrency/async"
)

type SettingsSource interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
}

type GlobalRulesSource interface {
	Get(ctx context.Context) (*models.GlobalRules, error)
}

type ConfigResolver struct {
	settings SettingsSource
	fallback runner.OpenCodeReviewConfig
}

func NewConfigResolver(settings SettingsSource, fallback runner.OpenCodeReviewConfig) *ConfigResolver {
	return &ConfigResolver{settings: settings, fallback: fallback}
}

func (r *ConfigResolver) Resolve(overrides review.OCROverrides) (runner.OpenCodeReviewConfig, error) {
	s, err := r.settings.Get(context.Background())
	if err != nil {
		return runner.OpenCodeReviewConfig{}, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	return r.buildConfig(s, overrides), nil
}

func (r *ConfigResolver) ResolveFromSettings(s *models.OCRSettings, overrides review.OCROverrides) runner.OpenCodeReviewConfig {
	return r.buildConfig(s, overrides)
}

func (r *ConfigResolver) buildConfig(s *models.OCRSettings, overrides review.OCROverrides) runner.OpenCodeReviewConfig {
	cfg := runner.OpenCodeReviewConfig{
		Binary:          firstNonBlank(s.Binary, r.fallback.Binary),
		Audience:        firstNonBlank(overrides.Audience, s.Audience, r.fallback.Audience, "human"),
		RulePath:        r.fallback.RulePath,
		DefaultSeverity: firstNonBlank(overrides.DefaultSeverity, s.DefaultSeverity, r.fallback.DefaultSeverity, string(review.SeverityMajor)),
		URL:             firstNonBlank(overrides.URL, s.URL),
		Model:           firstNonBlank(overrides.Model, s.ModelName),
		AuthToken:       s.AuthToken,
		AuthHeader:      s.AuthHeader,
		UseAnthropic:    overrides.UseAnthropic || s.UseAnthropic,
		ExtraBody:       firstNonBlank(overrides.ExtraBody, string(s.ExtraBody)),
		Language:        firstNonBlank(overrides.Language, s.Language),
		MaxTools:        firstPositive(overrides.MaxTools, s.MaxTools),
		Background:      firstNonBlank(overrides.Background, s.Background),
		ToolsPath:       firstNonBlank(overrides.ToolsPath, s.ToolsPath),
	}
	if overrides.Concurrency > 0 {
		cfg.Concurrency = overrides.Concurrency
	} else if s.Concurrency > 0 {
		cfg.Concurrency = s.Concurrency
	} else {
		cfg.Concurrency = r.fallback.Concurrency
	}
	if overrides.MaxGitProcs > 0 {
		cfg.MaxGitProcs = overrides.MaxGitProcs
	} else if s.MaxGitProcs > 0 {
		cfg.MaxGitProcs = s.MaxGitProcs
	} else {
		cfg.MaxGitProcs = r.fallback.MaxGitProcs
	}
	if overrides.TimeoutMinutes > 0 {
		cfg.TimeoutMinutes = overrides.TimeoutMinutes
	} else if s.TimeoutMinutes > 0 {
		cfg.TimeoutMinutes = s.TimeoutMinutes
	} else {
		cfg.TimeoutMinutes = r.fallback.TimeoutMinutes
	}
	return cfg
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

type Tester struct {
	settings SettingsSource
	resolver *ConfigResolver
}

func NewTester(settings SettingsSource, resolver *ConfigResolver) *Tester {
	return &Tester{settings: settings, resolver: resolver}
}

func (t *Tester) TestLLM(ctx context.Context) (*review.LLMTestResult, error) {
	s, err := t.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	if !s.Enabled {
		return &review.LLMTestResult{Success: false, Message: "OCR 未启用"}, nil
	}

	cfg := t.resolver.ResolveFromSettings(s, review.OCROverrides{})

	args := []string{"llm", "test"}
	env := os.Environ()
	env = appendLLMEnv(env, cfg)

	result, err := runOCRCommand(ctx, cfg.Binary, args, env)
	if err != nil {
		msg := strings.TrimSpace(result.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return &review.LLMTestResult{Success: false, Message: msg}, nil
	}

	output := strings.TrimSpace(result.stdout)
	if output == "" {
		output = "连接测试成功"
	}
	return &review.LLMTestResult{Success: true, Message: output}, nil
}

type ProviderLister struct {
	settings SettingsSource
	resolver *ConfigResolver
}

func NewProviderLister(settings SettingsSource, resolver *ConfigResolver) *ProviderLister {
	return &ProviderLister{settings: settings, resolver: resolver}
}

func (l *ProviderLister) ListLLMProviders(ctx context.Context) ([]review.LLMProvider, error) {
	s, err := l.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}

	binary := resolveBinaryFrom(s, l.resolver.fallback)

	result, err := runOCRCommand(ctx, binary, []string{"llm", "providers"}, nil)
	if err != nil {
		msg := strings.TrimSpace(result.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("获取 Provider 列表失败: %s", msg)
	}

	output := result.stdout
	var providers []review.LLMProvider
	lines := strings.Split(output, "\n")
	inTable := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "NAME") && strings.Contains(line, "PROTOCOL") {
			inTable = true
			continue
		}
		if inTable && line == "" {
			break
		}
		if inTable && line != "" && !strings.HasPrefix(line, "----") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				p := review.LLMProvider{
					Name:     fields[0],
					Protocol: fields[1],
				}
				if len(fields) >= 3 {
					p.BaseURL = fields[2]
				}
				providers = append(providers, p)
			}
		}
	}

	return providers, nil
}

type RulesChecker struct {
	settings    SettingsSource
	globalRules GlobalRulesSource
	resolver    *ConfigResolver
}

func NewRulesChecker(settings SettingsSource, globalRules GlobalRulesSource, resolver *ConfigResolver) *RulesChecker {
	return &RulesChecker{settings: settings, globalRules: globalRules, resolver: resolver}
}

func (rc *RulesChecker) RulesCheck(ctx context.Context, path string) (*review.RulesCheckResult, error) {
	s, err := rc.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}

	binary := resolveBinaryFrom(s, rc.resolver.fallback)

	args := []string{"rules", "check"}

	if rc.globalRules != nil {
		rulePath, cleanup, ruleErr := resolveGlobalRuleFile(ctx, rc.globalRules)
		if ruleErr == nil && rulePath != "" {
			args = append(args, "--rule", rulePath)
			if cleanup != nil {
				defer cleanup()
			}
		}
	}

	args = append(args, path)

	result, err := runOCRCommand(ctx, binary, args, nil)
	if err != nil {
		msg := strings.TrimSpace(result.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return &review.RulesCheckResult{Applicable: false, Message: msg}, nil
	}

	output := strings.TrimSpace(result.stdout)
	return &review.RulesCheckResult{
		Applicable: output != "",
		Rule:       output,
		Message:    "规则匹配成功",
	}, nil
}

type Versioner struct {
	settings SettingsSource
	resolver *ConfigResolver
}

func NewVersioner(settings SettingsSource, resolver *ConfigResolver) *Versioner {
	return &Versioner{settings: settings, resolver: resolver}
}

func (v *Versioner) OCRVersion(ctx context.Context) (*review.OCRVersion, error) {
	s, err := v.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}

	binary := resolveBinaryFrom(s, v.resolver.fallback)

	cmdResult, err := runOCRCommand(ctx, binary, []string{"version"}, nil)
	if err != nil {
		msg := strings.TrimSpace(cmdResult.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("获取版本失败: %s", msg)
	}

	output := strings.TrimSpace(cmdResult.stdout)
	result := &review.OCRVersion{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "open-code-review ") {
			versionStr := strings.TrimPrefix(line, "open-code-review ")
			parts := strings.SplitN(versionStr, " ", 2)
			if len(parts) >= 1 {
				result.Version = parts[0]
			}
			if len(parts) >= 2 {
				rest := parts[1]
				if strings.HasPrefix(rest, "(") {
					end := strings.Index(rest, ")")
					if end > 0 {
						result.Commit = rest[1:end]
					}
				}
			}
		} else if strings.Contains(line, "darwin") || strings.Contains(line, "linux") || strings.Contains(line, "windows") {
			result.Platform = line
		} else if strings.Contains(line, "built at:") {
			result.BuildTime = strings.TrimSpace(strings.TrimPrefix(line, "built at:"))
		}
	}

	return result, nil
}

type ViewerManager struct {
	process  *exec.Cmd
	addr     string
	mu       sync.Mutex
	done     chan struct{}
	binary   string
	settings SettingsSource
	resolver *ConfigResolver
}

func NewViewerManager(settings SettingsSource, resolver *ConfigResolver) *ViewerManager {
	return &ViewerManager{settings: settings, resolver: resolver}
}

func (vm *ViewerManager) resolveBinary() string {
	if vm.binary != "" {
		return vm.binary
	}
	if vm.settings != nil {
		s, err := vm.settings.Get(context.Background())
		if err == nil && s != nil && s.Binary != "" {
			vm.binary = s.Binary
			return vm.binary
		}
	}
	if vm.resolver != nil {
		vm.binary = vm.resolver.fallback.Binary
	}
	if vm.binary == "" {
		vm.binary = "ocr"
	}
	return vm.binary
}

func (vm *ViewerManager) Status() *review.ViewerStatus {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process == nil {
		return &review.ViewerStatus{Running: false, Message: "Viewer 未运行"}
	}

	select {
	case <-vm.done:
		vm.process = nil
		return &review.ViewerStatus{Running: false, Message: "Viewer 已停止"}
	default:
	}

	return &review.ViewerStatus{
		Running: true,
		Addr:    vm.addr,
		Message: fmt.Sprintf("Viewer 运行中 (%s)", vm.addr),
	}
}

func (vm *ViewerManager) ViewerStatus(ctx context.Context) (*review.ViewerStatus, error) {
	return vm.Status(), nil
}

func (vm *ViewerManager) StartViewer(ctx context.Context, addr string) (*review.ViewerStatus, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process != nil {
		select {
		case <-vm.done:
			vm.process = nil
		default:
			return &review.ViewerStatus{
				Running: true,
				Addr:    vm.addr,
				Message: "Viewer 已在运行",
			}, nil
		}
	}

	binary := vm.resolveBinary()

	args := []string{"viewer"}
	if addr != "" {
		args = append(args, "--addr", addr)
	} else {
		addr = "localhost:5483"
	}

	cmd := exec.Command(binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 Viewer 失败: %w", err)
	}

	vm.process = cmd
	vm.addr = addr
	vm.done = make(chan struct{})

	async.GoSafe(func() {
		_ = cmd.Wait()
		close(vm.done)
	})

	return &review.ViewerStatus{
		Running: true,
		Addr:    addr,
		Message: fmt.Sprintf("Viewer 已启动 (%s)", addr),
	}, nil
}

func (vm *ViewerManager) StopViewer(ctx context.Context) (*review.ViewerStatus, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process == nil {
		return &review.ViewerStatus{Running: false, Message: "Viewer 未运行"}, nil
	}

	select {
	case <-vm.done:
		vm.process = nil
		return &review.ViewerStatus{Running: false, Message: "Viewer 已停止"}, nil
	default:
	}

	if err := syscall.Kill(-vm.process.Process.Pid, syscall.SIGTERM); err != nil {
		_ = syscall.Kill(-vm.process.Process.Pid, syscall.SIGKILL)
	}

	select {
	case <-vm.done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-vm.process.Process.Pid, syscall.SIGKILL)
	}

	addr := vm.addr
	vm.process = nil
	vm.addr = ""

	return &review.ViewerStatus{
		Running: false,
		Addr:    addr,
		Message: "Viewer 已停止",
	}, nil
}

type ocrCommandResult struct {
	stdout string
	stderr string
}

func runOCRCommand(ctx context.Context, binary string, args []string, env []string) (*ocrCommandResult, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if len(env) > 0 {
		cmd.Env = env
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return &ocrCommandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}, err
}

func resolveBinaryFrom(s *models.OCRSettings, fallback runner.OpenCodeReviewConfig) string {
	if s != nil && s.Binary != "" {
		return s.Binary
	}
	if fallback.Binary != "" {
		return fallback.Binary
	}
	return "ocr"
}

func resolveGlobalRuleFile(ctx context.Context, globalRules GlobalRulesSource) (string, func(), error) {
	if globalRules == nil {
		return "", nil, nil
	}

	rules, err := globalRules.Get(ctx)
	if err != nil {
		// Intentionally ignore error - rules are optional
		return "", nil, nil //nolint:nilerr
	}
	if rules == nil || len(rules.Rules) == 0 {
		return "", nil, nil
	}

	f, err := os.CreateTemp("", "ocr-rules-*.json")
	if err != nil {
		return "", nil, err
	}

	ruleJSON := fmt.Sprintf(`{"rules":%s}`, string(rules.Rules))
	if _, err := f.WriteString(ruleJSON); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	_ = f.Close()

	cleanup := func() { _ = os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}

func appendLLMEnv(env []string, cfg runner.OpenCodeReviewConfig) []string {
	if cfg.URL != "" {
		env = append(env, "OCR_LLM_URL="+cfg.URL)
	}
	if cfg.Model != "" {
		env = append(env, "OCR_LLM_MODEL="+cfg.Model)
	}
	if cfg.AuthToken != "" {
		env = append(env, "OCR_LLM_TOKEN="+cfg.AuthToken)
	}
	if cfg.AuthHeader != "" {
		env = append(env, "OCR_LLM_AUTH_HEADER="+cfg.AuthHeader)
	}
	if cfg.UseAnthropic {
		env = append(env, "OCR_USE_ANTHROPIC=true")
	} else {
		env = append(env, "OCR_USE_ANTHROPIC=false")
	}
	if cfg.Language != "" {
		env = append(env, "OCR_LANGUAGE="+cfg.Language)
	}
	if cfg.ExtraBody != "" {
		env = append(env, "OCR_LLM_EXTRA_BODY="+cfg.ExtraBody)
	}
	return env
}

// Compile-time interface checks
var _ review.LLMTester = (*Tester)(nil)
var _ review.LLMProviderLister = (*ProviderLister)(nil)
var _ review.RulesChecker = (*RulesChecker)(nil)
var _ review.OCRVersioner = (*Versioner)(nil)
var _ review.ViewerManager = (*ViewerManager)(nil)
