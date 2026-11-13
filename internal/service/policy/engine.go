package policy

import (
	"strings"

	"github.com/oekit/pkg/pathx"
	types "github.com/yi-nology/code-review-service/internal/domain/finding"

	"github.com/oekit/logx"
)

type Config struct {
	FailOn      string
	MaxComments int
	SummaryOnly bool
	IgnorePaths []string
}

type Engine struct {
	cfg    Config
	source func() Config // 非 nil 时每次 Evaluate 动态取配置（热更新）
}

type Decision struct {
	Passed bool
	Reason string
}

type Finding struct {
	Severity string
	Path     string
}

func NewEngine(cfg Config) *Engine {
	return &Engine{cfg: cfg}
}

func (e *Engine) Evaluate(findings []Finding) Decision {
	cfg := e.current()
	e.warnInvalidIgnorePaths(cfg.IgnorePaths)
	threshold := types.SeverityRank(cfg.FailOn)
	if threshold == 0 {
		threshold = types.SeverityRank(DefaultFailOn)
	}

	for _, finding := range findings {
		if e.ignored(finding.Path) {
			continue
		}
		if types.SeverityRank(finding.Severity) >= threshold {
			return Decision{
				Passed: false,
				Reason: finding.Severity + " issue found",
			}
		}
	}

	return Decision{Passed: true}
}

func (e *Engine) ignored(path string) bool {
	return pathx.MatchesPath(e.current().IgnorePaths, path)
}

// warnInvalidIgnorePaths 让「非法 glob 被静默吞掉」变成可见事实。
//
// IgnorePaths 存在 DB 里（全局设置与项目策略两条来源），可以被控制台、直接改库或
// 旧版本写入绕过保存校验；pathx.MatchesPath 对 ErrBadPattern 只是当作该条不匹配，
// 于是评审照常跑完、但本该被忽略的文件全部参与门禁，表现为「配了忽略却没用」。
// 这里每条坏 pattern 每次 Evaluate 打一条 WARN——一次评审最多 len(patterns) 条，
// 不随 findings 数量放大。
func (e *Engine) warnInvalidIgnorePaths(patterns []string) {
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		if err := pathx.ValidatePattern(pattern); err != nil {
			logx.Warnf("policy: %v（该条 ignore_paths 不生效，命中它的文件仍会参与门禁）", err)
		}
	}
}

func MatchesPath(patterns []string, path string) bool {
	return pathx.MatchesPath(patterns, path)
}

// WithSource 让 Engine 在每次 Evaluate 时动态取配置（热更新：业务配置存 DB，
// web console 修改后无需重启即对后续评审生效）；source 返回零值时回落启动配置。
// 典型 source：从 OCRSettings 读 PolicyFailOn 等字段构建 Config。
func (e *Engine) WithSource(source func() Config) *Engine {
	e.source = source
	return e
}

// current 返回本次评估应使用的配置。
func (e *Engine) current() Config {
	if e.source != nil {
		if cfg := e.source(); cfg.FailOn != "" || cfg.MaxComments > 0 || cfg.SummaryOnly || len(cfg.IgnorePaths) > 0 {
			return cfg
		}
		// source 失败/为空：回落启动时的兜底配置，保证门禁始终可用。
		return e.cfg
	}
	return e.cfg
}

// WithFailOnOverride 返回以当前配置为基础、覆盖 FailOn 的派生引擎（不改自身）。
// 项目策略的 fail_on 是全局门禁默认值的按项目覆盖，此前两处调用点用裸 Config
// 新建引擎，把全局 IgnorePaths 等默认值一并丢掉了——这里收敛为合并语义。
func (e *Engine) WithFailOnOverride(failOn string) *Engine {
	cfg := e.current()
	if failOn != "" {
		cfg.FailOn = failOn
	}
	return &Engine{cfg: cfg}
}
