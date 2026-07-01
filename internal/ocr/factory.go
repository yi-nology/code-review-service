package ocr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// Factory 是 agent 包注入的 Runner 工厂（吸收 agent.OCRRunnerFactory）。
type Factory struct {
	baseConfig Config
	runner     *Runner
}

// NewFactory 构造 Factory。
// 阶段 A：仍委托给 Runner（薄壳 wrapper）。
func NewFactory(cfg Config, runner *Runner) *Factory {
	return &Factory{baseConfig: cfg, runner: runner}
}

// CreateRunner 按 persona background + rule content 构造独立 runner。
//
// 阶段 A：实现与原 agent.OCRRunnerFactory 等价；
// 阶段 A+3 把 ruleContent 写盘逻辑搬入内部 helper。
func (f *Factory) CreateRunner(background, ruleContent string) *Runner {
	cfg := f.baseConfig
	cfg.Background = background
	if strings.TrimSpace(ruleContent) != "" {
		if path, err := writeRuleFile(ruleContent); err == nil {
			cfg.RulePath = path
		}
	}
	return NewRunner(cfg)
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

// 防止 unused import 警告（domain 在 stage A+3 真正使用）。
var _ = domain.Result{}
var _ = context.Background