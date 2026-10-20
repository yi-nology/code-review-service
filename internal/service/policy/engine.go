package policy

import (
	"github.com/oekit/pkg/pathx"
	types "github.com/yi-nology/code-review-service/internal/domain/finding"
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
