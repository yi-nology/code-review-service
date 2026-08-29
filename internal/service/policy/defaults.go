package policy

// 策略默认值单一真源。historically 这些值散落在 fx 装配层（modules/review.go）
// 和 engine.Evaluate 的内联兜底里，现收敛至此，供装配层与 Engine 共用。

const (
	// DefaultFailOn 是未配置 FailOn 时的严重度门槛。
	DefaultFailOn = "critical"
	// DefaultMaxComments 是未配置时的最大评论数。
	DefaultMaxComments = 20
)

// DefaultIgnorePaths 是未配置 IgnorePaths 时的默认忽略路径列表，
// 屏蔽依赖产物、生成代码与构建输出。
var DefaultIgnorePaths = []string{
	"vendor/**",
	"dist/**",
	"*.pb.go",
	"node_modules/**",
}

// DefaultConfig 返回策略引擎的默认配置。
// 装配层（modules）应以此为基础，再用 DB 配置覆盖各字段，而非在 wiring 代码里硬编码默认值。
func DefaultConfig() Config {
	return Config{
		FailOn:      DefaultFailOn,
		MaxComments: DefaultMaxComments,
		SummaryOnly: false,
		IgnorePaths: append([]string(nil), DefaultIgnorePaths...),
	}
}
