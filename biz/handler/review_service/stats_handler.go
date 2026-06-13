package review_service

import (
	"context"

	"review-service/internal/service/stats"

	hertzApp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type StatsHandler struct {
	statsSvc StatsService
}

type StatsService interface {
	ListContributors(ctx context.Context, provider, repo, from, to, sort, order string, page, pageSize int) (*stats.ContributorListResult, error)
	GetContributor(ctx context.Context, author, provider, repo, from, to string) (*stats.ContributorDetail, error)
	GetContributorTrend(ctx context.Context, author, provider, repo, from, to string) ([]stats.TrendPoint, error)
	GetContributorCategories(ctx context.Context, author, provider, repo, from, to string) ([]stats.CategoryStat, error)
}

func NewStatsHandler(statsSvc StatsService) *StatsHandler {
	return &StatsHandler{statsSvc: statsSvc}
}

func (h *StatsHandler) ListContributors(ctx context.Context, c *hertzApp.RequestContext) {
	page, pageSize := pageParams(
		string(c.Query("page")),
		string(c.Query("page_size")),
	)

	result, err := h.statsSvc.ListContributors(
		ctx,
		cleanQuery(string(c.Query("provider"))),
		cleanQuery(string(c.Query("repo"))),
		cleanQuery(string(c.Query("from"))),
		cleanQuery(string(c.Query("to"))),
		cleanQuery(string(c.Query("sort"))),
		cleanQuery(string(c.Query("order"))),
		page,
		pageSize,
	)
	if err != nil {
		fail(c, err)
		return
	}

	c.JSON(consts.StatusOK, result)
}

func (h *StatsHandler) GetContributor(ctx context.Context, c *hertzApp.RequestContext) {
	author := c.Param("author")
	if author == "" {
		fail(c, badRequest("author is required"))
		return
	}

	detail, err := h.statsSvc.GetContributor(
		ctx,
		author,
		cleanQuery(string(c.Query("provider"))),
		cleanQuery(string(c.Query("repo"))),
		cleanQuery(string(c.Query("from"))),
		cleanQuery(string(c.Query("to"))),
	)
	if err != nil {
		fail(c, err)
		return
	}
	if detail == nil {
		fail(c, notFound("contributor not found"))
		return
	}

	c.JSON(consts.StatusOK, detail)
}

func (h *StatsHandler) GetContributorTrend(ctx context.Context, c *hertzApp.RequestContext) {
	author := c.Param("author")
	if author == "" {
		fail(c, badRequest("author is required"))
		return
	}

	trend, err := h.statsSvc.GetContributorTrend(
		ctx,
		author,
		cleanQuery(string(c.Query("provider"))),
		cleanQuery(string(c.Query("repo"))),
		cleanQuery(string(c.Query("from"))),
		cleanQuery(string(c.Query("to"))),
	)
	if err != nil {
		fail(c, err)
		return
	}

	c.JSON(consts.StatusOK, map[string]any{"trend": trend})
}

func (h *StatsHandler) GetContributorCategories(ctx context.Context, c *hertzApp.RequestContext) {
	author := c.Param("author")
	if author == "" {
		fail(c, badRequest("author is required"))
		return
	}

	categories, err := h.statsSvc.GetContributorCategories(
		ctx,
		author,
		cleanQuery(string(c.Query("provider"))),
		cleanQuery(string(c.Query("repo"))),
		cleanQuery(string(c.Query("from"))),
		cleanQuery(string(c.Query("to"))),
	)
	if err != nil {
		fail(c, err)
		return
	}

	c.JSON(consts.StatusOK, map[string]any{"categories": categories})
}
