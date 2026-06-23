package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/runner"
)

type OCRRunnerFactory struct {
	baseConfig runner.OpenCodeReviewConfig
}

func NewOCRRunnerFactory(baseConfig runner.OpenCodeReviewConfig) *OCRRunnerFactory {
	return &OCRRunnerFactory{baseConfig: baseConfig}
}

// CreateRunner 按 persona 的 background 与 rule 内容创建独立 runner（路径 A1）。
// ruleContent 为 rule JSON 内容（按 glob 文件模式），将落盘后透传给 OCR 的 --rule。
func (f *OCRRunnerFactory) CreateRunner(background, ruleContent string) reviewRunner {
	config := f.baseConfig
	config.Background = background
	if strings.TrimSpace(ruleContent) != "" {
		if path, err := writeRuleFile(ruleContent); err == nil {
			config.RulePath = path
		}
	}
	return &ocrRunnerAdapter{runner: runner.NewOpenCodeReviewRunner(config)}
}

// writeRuleFile 把 rule JSON 内容按内容哈希幂等落盘到临时目录，返回文件路径。
// 相同内容始终映射到同一文件，服务重启不泄漏、可复用。
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
	runner *runner.OpenCodeReviewRunner
}

func (a *ocrRunnerAdapter) Run(ctx context.Context, workspace reviewWorkspace) (reviewResult, error) {
	reviewWs := review.Workspace{
		Dir:     workspace.Dir,
		BaseSHA: workspace.BaseSHA,
		HeadSHA: workspace.HeadSHA,
		Diff:    workspace.Diff,
	}

	result, err := a.runner.Run(ctx, reviewWs)
	if err != nil {
		return reviewResult{}, err
	}

	findings := make([]reviewFinding, len(result.Findings))
	for i, f := range result.Findings {
		findings[i] = reviewFinding{
			Severity:       string(f.Severity),
			Path:           f.Path,
			Line:           f.Line,
			StartLine:      f.StartLine,
			EndLine:        f.EndLine,
			Title:          f.Title,
			Body:           f.Body,
			ExistingCode:   f.ExistingCode,
			SuggestionCode: f.SuggestionCode,
			Thinking:       f.Thinking,
		}
	}

	return reviewResult{
		Summary:     result.Summary,
		Findings:    findings,
		TotalTokens: result.OCR.TotalTokens,
	}, nil
}
