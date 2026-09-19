package postgres

import (
	"testing"
	"time"

	domstats "github.com/yi-nology/code-review-service/internal/domain/stats"
)

func TestBuildStatsFilter(t *testing.T) {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)

	t.Run("全条件带平台仓库", func(t *testing.T) {
		parts := buildStatsFilter("github", "o/r", from, to, true)
		wantClause := " AND provider = ? AND repo ILIKE ? AND created_at >= ? AND created_at <= ?"
		if parts.clause != wantClause {
			t.Fatalf("clause = %q", parts.clause)
		}
		if len(parts.args) != 4 || parts.args[0] != "github" || parts.args[1] != "%o/r%" {
			t.Fatalf("args = %v", parts.args)
		}
	})

	t.Run("WithoutProject 跳过平台仓库条件", func(t *testing.T) {
		parts := buildStatsFilter("github", "o/r", from, to, false)
		if parts.clause != " AND created_at >= ? AND created_at <= ?" || len(parts.args) != 2 {
			t.Fatalf("clause=%q args=%v", parts.clause, parts.args)
		}
	})

	t.Run("全空无条件", func(t *testing.T) {
		parts := buildStatsFilter("", "", time.Time{}, time.Time{}, true)
		if parts.clause != "" || len(parts.args) != 0 {
			t.Fatalf("clause=%q args=%v", parts.clause, parts.args)
		}
	})

	t.Run("旧委托与核心一致", func(t *testing.T) {
		opts := domstats.ContributorStatsOptions{Provider: "gitea", Repo: "a/b", From: from, To: to}
		if filterClause(opts) != " AND provider = ? AND repo ILIKE ? AND created_at >= ? AND created_at <= ?" {
			t.Fatalf("filterClause = %q", filterClause(opts))
		}
		popts := domstats.ProjectStatsOptions{Provider: "gitea", Repo: "a/b", From: from, To: to}
		c, a := reviewMetricsFilterClause(popts)
		p2 := projectFilterParts(popts, true)
		if c != p2.clause || len(a) != len(p2.args) {
			t.Fatalf("reviewMetricsFilterClause 与核心不一致: %q vs %q", c, p2.clause)
		}
	})
}
