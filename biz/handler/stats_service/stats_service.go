package stats_service

import (
	"context"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelStats "github.com/yi-nology/code-review-service/biz/model/stats"
	"github.com/yi-nology/code-review-service/internal/service/stats"

	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/ptrx"

	hertzApp "github.com/cloudwego/hertz/pkg/app"
)

var handlerInst *StatsHandler

type StatsService interface {
	ListContributors(ctx context.Context, provider, repo, from, to, sort, order string, page, pageSize int) (*stats.ContributorListResult, error)
	GetContributor(ctx context.Context, author, provider, repo, from, to string) (*stats.ContributorDetail, error)
	GetContributorTrend(ctx context.Context, author, provider, repo, from, to string) ([]stats.TrendPoint, error)
	GetContributorCategories(ctx context.Context, author, provider, repo, from, to string) ([]stats.CategoryStat, error)
}

type StatsHandler struct {
	statsSvc StatsService
}

func InitHandler(statsSvc StatsService) {
	handlerInst = &StatsHandler{statsSvc: statsSvc}
}

func ListContributors(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelStats.ListContributorsReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	page, pageSize := common.PageParams(
		convert.Int64ToStr(ptrx.Deref(req.Page, 0)),
		convert.Int64ToStr(ptrx.Deref(req.PageSize, 0)),
	)

	result, err := handlerInst.statsSvc.ListContributors(
		ctx,
		ptrx.Deref(req.Provider, ""),
		ptrx.Deref(req.Repo, ""),
		ptrx.Deref(req.From, ""),
		ptrx.Deref(req.To, ""),
		ptrx.Deref(req.Sort, ""),
		ptrx.Deref(req.Order, ""),
		page,
		pageSize,
	)
	if err != nil {
		common.Fail(c, err)
		return
	}

	common.Success(c, result)
}

func GetContributor(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelStats.GetContributorReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	detail, err := handlerInst.statsSvc.GetContributor(
		ctx,
		req.Author,
		ptrx.Deref(req.Provider, ""),
		ptrx.Deref(req.Repo, ""),
		ptrx.Deref(req.From, ""),
		ptrx.Deref(req.To, ""),
	)
	if err != nil {
		common.Fail(c, err)
		return
	}
	if detail == nil {
		common.Fail(c, common.NotFound("contributor not found"))
		return
	}

	common.Success(c, detail)
}

func GetContributorTrend(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelStats.GetContributorTrendReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	trend, err := handlerInst.statsSvc.GetContributorTrend(
		ctx,
		req.Author,
		ptrx.Deref(req.Provider, ""),
		ptrx.Deref(req.Repo, ""),
		ptrx.Deref(req.From, ""),
		ptrx.Deref(req.To, ""),
	)
	if err != nil {
		common.Fail(c, err)
		return
	}

	common.Success(c, map[string]any{"trend": trend})
}

func GetContributorCategories(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelStats.GetContributorCategoriesReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	categories, err := handlerInst.statsSvc.GetContributorCategories(
		ctx,
		req.Author,
		ptrx.Deref(req.Provider, ""),
		ptrx.Deref(req.Repo, ""),
		ptrx.Deref(req.From, ""),
		ptrx.Deref(req.To, ""),
	)
	if err != nil {
		common.Fail(c, err)
		return
	}

	common.Success(c, map[string]any{"categories": categories})
}
