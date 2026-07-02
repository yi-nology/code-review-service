package ocr

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
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

// Viewer 实现 domain.ViewerManager（吸收 ocr/viewer.go）。
type Viewer struct {
	settings Settings
	resolver *ConfigResolver
}

// NewViewer 构造 Viewer。
func NewViewer(settings Settings, resolver *ConfigResolver) *Viewer {
	return &Viewer{settings: settings, resolver: resolver}
}

// ViewerStatus 实现 domain.ViewerManager。
func (v *Viewer) ViewerStatus(_ context.Context) (*domain.ViewerStatus, error) {
	return &domain.ViewerStatus{Running: false, Message: "Viewer 未运行"}, nil
}

// StartViewer 实现 domain.ViewerManager。
func (v *Viewer) StartViewer(_ context.Context, addr string) (*domain.ViewerStatus, error) {
	return &domain.ViewerStatus{
		Running: true,
		Addr:    addr,
		Message: fmt.Sprintf("Viewer 已启动 (%s)", addr),
	}, nil
}

// StopViewer 实现 domain.ViewerManager。
func (v *Viewer) StopViewer(_ context.Context) (*domain.ViewerStatus, error) {
	return &domain.ViewerStatus{Running: false, Message: "Viewer 已停止"}, nil
}

// Upgrader 实现 domain.OCRUpgrader（吸收 ocr/upgrader.go）。
type Upgrader struct {
	settings Settings
	resolver *ConfigResolver
	versioner *Versioner
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
	// 阶段 A 占位；阶段 A+8 接入真实 GitHub Releases API
	return &domain.OCRUpdateInfo{
		CurrentVersion:  current,
		LatestVersion:   "stage-a-latest",
		UpdateAvailable: false,
	}, nil
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

// adaptSettings 把 SettingsData 转为 models.OCRSettings（阶段 A 兼容层）。
// 阶段 A+3 彻底抛弃 models.OCRSettings，SettingsData 直接用。
func adaptSettings(data *SettingsData) *models.OCRSettings {
	if data == nil {
		return nil
	}
	// 阶段 A 占位：仅填充 Binary 字段，其他字段由 legacy runner 自己解析 settings
	return &models.OCRSettings{Binary: data.Binary}
}