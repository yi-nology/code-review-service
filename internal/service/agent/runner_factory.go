package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// OCRRunnerFactory 阶段 A15 part 5：完全迁入 ocr 包。
// 原 agent.OCRRunnerFactory 的 baseConfig 类型从 runner.OpenCodeReviewConfig
// 改为 ocr.Config。
//
// baseConfig 与 resolver 二选一：
//   - NewOCRRunnerFactory：固定 Config（启动快照，测试/最小装配用）；
//   - NewResolvingOCRRunnerFactory：每次 CreateRunner 时经 ConfigResolver 从 DB
//     解析最新配置——与主 Runner 的热更新行为一致。此前 modules/agent.go 手工
//     拼了一份启动快照，DB 配置变更后 agent runner 永远看不到。
type OCRRunnerFactory struct {
	baseConfig ocr.Config
	resolver   *ocr.ConfigResolver
}

func NewOCRRunnerFactory(baseConfig ocr.Config) *OCRRunnerFactory {
	return &OCRRunnerFactory{baseConfig: baseConfig}
}

// NewResolvingOCRRunnerFactory 构造走 ConfigResolver 热更新配置的工厂。
func NewResolvingOCRRunnerFactory(resolver *ocr.ConfigResolver) *OCRRunnerFactory {
	return &OCRRunnerFactory{resolver: resolver}
}

// currentBaseConfig 返回本次建 runner 应用的基础配置。
func (f *OCRRunnerFactory) currentBaseConfig() ocr.Config {
	if f.resolver != nil {
		if cfg, err := f.resolver.Resolve(context.Background(), ocr.OCROverrides{}); err == nil && cfg.Binary != "" {
			return cfg
		}
		// Resolve 失败时退回上次成功配置或零值（OCR 会因 Binary 为空快速失败并进入重试），
		// 与主 Runner refreshNow 的容错语义一致。
	}
	return f.baseConfig
}

// CreateRunner 按 persona 的 background 与 rule 内容创建独立 runner。
// ruleContent 为 rule JSON 内容（按 glob 文件模式），将落盘后透传给 OCR 的 --rule。
func (f *OCRRunnerFactory) CreateRunner(background, ruleContent string) reviewRunner {
	config := f.currentBaseConfig()
	config.Background = background
	if strings.TrimSpace(ruleContent) != "" {
		if path, err := writeRuleFile(ruleContent); err == nil {
			config.RulePath = path
		}
	}
	return &ocrRunnerAdapter{runner: ocr.NewRunner(config)}
}

// writeRuleFile 把 rule JSON 内容按内容哈希幂等落盘到临时目录，返回文件路径。
func writeRuleFile(content string) (string, error) {
	sum := sha256.Sum256([]byte(content))
	name := "ocr-rule-" + hex.EncodeToString(sum[:8]) + ".json"
	path := filepath.Join(os.TempDir(), name)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return path, os.WriteFile(path, []byte(content), 0o600)
}

type ocrRunnerAdapter struct {
	runner *ocr.Runner
}

func (a *ocrRunnerAdapter) Run(ctx context.Context, workspace reviewWorkspace) (reviewResult, error) {
	reviewWs := domain.Workspace{
		Dir:      workspace.Dir,
		BaseSHA:  workspace.BaseSHA,
		HeadSHA:  workspace.HeadSHA,
		Diff:     workspace.Diff,
		RulePath: workspace.RulePath,
	}

	result, err := a.runner.Run(ctx, reviewWs)
	if err != nil {
		return reviewResult{}, err
	}

	// domain.Finding = types.Finding = Finding，直接透传，无需逐字段转换。
	return reviewResult{
		Summary:     result.Summary,
		Findings:    result.Findings,
		TotalTokens: result.OCR.TotalTokens,
	}, nil
}
