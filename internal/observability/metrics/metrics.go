// Package metrics 定义 review-service 的业务 Prometheus 指标，并注册到 ekit 的 Registry。
//
// 指标体系（namespace=review_service）：
//   - review_duration_seconds：评审耗时分布（按 provider/status 标签），Histogram
//   - review_total：评审计数（按 provider/status 标签），Counter
//   - review_queue_depth：当前队列深度（queued+running），Gauge
//   - writeback_findings_total：回写 finding 计数（按 provider/status 标签），Counter
//   - llm_cost_usd_total：LLM 估算成本累计（按 provider/model 标签），Counter
//
// 所有指标在 init() 注册到 emetrics.Registry，与 HTTP/runtime 指标共用，统一从 /metrics 暴露。
package metrics

import (
	ekitmetrics "github.com/oekit/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "review_service"

var (
	// ReviewDurationSeconds 评审耗时分布（从 queued 到 completed/failed 的墙钟时间）。
	ReviewDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "review_duration_seconds",
			Help:      "评审端到端耗时分布",
			Buckets:   prometheus.DefBuckets, // 0.005s ~ 10s+，覆盖秒级到分钟级评审
		},
		[]string{"provider", "status"},
	)

	// ReviewTotal 评审计数。
	ReviewTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "review_total",
			Help:      "评审总数（按 provider 和最终状态）",
		},
		[]string{"provider", "status"},
	)

	// ReviewQueueDepth 当前队列深度（queued + running 的 review 数）。
	ReviewQueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "review_queue_depth",
			Help:      "当前评审队列深度（queued + running）",
		},
	)

	// WritebackFindingsTotal 回写 finding 计数。
	WritebackFindingsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "writeback_findings_total",
			Help:      "回写到 Git 平台的 finding 总数（按 provider 和状态）",
		},
		[]string{"provider", "status"},
	)

	// LLMCostUSDTotal LLM 估算成本累计（美元）。
	LLMCostUSDTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "llm_cost_usd_total",
			Help:      "LLM 调用估算成本累计（美元，按 provider 和模型）",
		},
		[]string{"provider", "model"},
	)
)

func init() {
	// 注册到 ekit Registry（与 HTTP/runtime 指标共用，统一从 /metrics 暴露）。
	ekitmetrics.Registry.MustRegister(
		ReviewDurationSeconds,
		ReviewTotal,
		ReviewQueueDepth,
		WritebackFindingsTotal,
		LLMCostUSDTotal,
	)
}
