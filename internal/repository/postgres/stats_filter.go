package postgres

import (
	"time"

	domstats "github.com/yi-nology/code-review-service/internal/domain/stats"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// statsFilterParts 是统计查询 WHERE 片段的 clause+args 组合。
type statsFilterParts struct {
	clause string
	args   []any
}

// buildStatsFilter 统一统计查询的过滤构建：provider 精确 / repo ILIKE 模糊 /
// created_at 时间窗。此前同一逻辑以 4 组近似的 clause/args 函数散落在
// contributor_stats.go 与 review.go（reviewMetricsFilterClause），仅入参类型不同。
// withProviderRepo=false 时跳过 provider/repo 条件（明细聚合外层已过滤，
// 只保留时间窗，即原 projectFilterClauseWithoutProject 语义）。
func buildStatsFilter(provider, repo string, from, to time.Time, withProviderRepo bool) statsFilterParts {
	var f statsFilterParts
	if withProviderRepo {
		if provider != "" {
			f.clause += " AND provider = ?"
			f.args = append(f.args, provider)
		}
		if repo != "" {
			f.clause += " AND repo ILIKE ?"
			f.args = append(f.args, "%"+repo+"%")
		}
	}
	if !from.IsZero() {
		f.clause += " AND created_at >= ?"
		f.args = append(f.args, from)
	}
	if !to.IsZero() {
		f.clause += " AND created_at <= ?"
		f.args = append(f.args, to)
	}
	return f
}

// 以下为旧函数名的一行委托（30 处调用点零改动），单一实现在 buildStatsFilter。

func filterParts(opts domstats.ContributorStatsOptions) statsFilterParts {
	return buildStatsFilter(opts.Provider, opts.Repo, opts.From, opts.To, true)
}

func filterClause(opts domstats.ContributorStatsOptions) string { return filterParts(opts).clause }

func filterArgs(opts domstats.ContributorStatsOptions) []any { return filterParts(opts).args }

func projectFilterParts(opts domain.ProjectStatsOptions, withProviderRepo bool) statsFilterParts {
	return buildStatsFilter(opts.Provider, opts.Repo, opts.From, opts.To, withProviderRepo)
}

func projectFilterClause(opts domain.ProjectStatsOptions) string {
	return projectFilterParts(opts, true).clause
}

func projectFilterArgs(opts domain.ProjectStatsOptions) []any {
	return projectFilterParts(opts, true).args
}

func projectFilterClauseWithoutProject(opts domain.ProjectStatsOptions) string {
	return projectFilterParts(opts, false).clause
}

func projectFilterArgsWithoutProject(opts domain.ProjectStatsOptions) []any {
	return projectFilterParts(opts, false).args
}
