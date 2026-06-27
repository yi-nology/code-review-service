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
	ListProjects(ctx context.Context, provider, repo, from, to, sort, order string, page, pageSize int) (*stats.ProjectListResult, error)
	GetProject(ctx context.Context, provider, repo, from, to string) (*stats.ProjectDetail, error)
	GetProjectTrend(ctx context.Context, provider, repo, from, to string) ([]stats.ProjectTrendPoint, error)
	GetProjectCategories(ctx context.Context, provider, repo, from, to string) ([]stats.CategoryStat, error)
	GetProjectFiles(ctx context.Context, provider, repo, from, to string) ([]stats.FileRiskStat, error)
	GetProjectContributors(ctx context.Context, provider, repo, from, to string, limit int) ([]stats.ContributorSummary, error)
	GetProjectOCRMetrics(ctx context.Context, provider, repo, from, to string) (*stats.ProjectOCRMetrics, error)
}

type StatsHandler struct {
	statsSvc StatsService
}

func InitHandler(statsSvc StatsService) {
	handlerInst = &StatsHandler{statsSvc: statsSvc}
}

func ListContributors(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.ListContributorsReq](c)
	if !ok {
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
	req, ok := common.Bind[modelStats.GetContributorReq](c)
	if !ok {
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
	req, ok := common.Bind[modelStats.GetContributorTrendReq](c)
	if !ok {
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
	req, ok := common.Bind[modelStats.GetContributorCategoriesReq](c)
	if !ok {
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

func ListProjects(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.ListProjectsReq](c)
	if !ok {
		return
	}

	page, pageSize := common.PageParams(
		convert.Int64ToStr(ptrx.Deref(req.Page, 0)),
		convert.Int64ToStr(ptrx.Deref(req.PageSize, 0)),
	)
	result, err := handlerInst.statsSvc.ListProjects(
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

func GetProject(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectReq](c)
	if !ok {
		return
	}

	detail, err := handlerInst.statsSvc.GetProject(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""))
	if err != nil {
		common.Fail(c, err)
		return
	}
	if detail == nil {
		common.Fail(c, common.NotFound("project not found"))
		return
	}

	common.Success(c, detail)
}

func GetProjectTrend(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectReq](c)
	if !ok {
		return
	}

	trend, err := handlerInst.statsSvc.GetProjectTrend(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""))
	if err != nil {
		common.Fail(c, err)
		return
	}

	common.Success(c, map[string]any{"trend": trend})
}

func GetProjectOCRMetrics(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectReq](c)
	if !ok {
		return
	}
	metrics, err := handlerInst.statsSvc.GetProjectOCRMetrics(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""))
	if err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"metrics": metrics})
}

func GetProjectCategories(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectReq](c)
	if !ok {
		return
	}

	categories, err := handlerInst.statsSvc.GetProjectCategories(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""))
	if err != nil {
		common.Fail(c, err)
		return
	}

	common.Success(c, map[string]any{"categories": categories})
}

func GetProjectFiles(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectReq](c)
	if !ok {
		return
	}

	files, err := handlerInst.statsSvc.GetProjectFiles(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""))
	if err != nil {
		common.Fail(c, err)
		return
	}

	common.Success(c, map[string]any{"files": files})
}

func GetProjectContributors(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectContributorsReq](c)
	if !ok {
		return
	}

	limit := int(ptrx.Deref(req.Limit, 10))
	if limit <= 0 {
		limit = 10
	}
	contributors, err := handlerInst.statsSvc.GetProjectContributors(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""), limit)
	if err != nil {
		common.Fail(c, err)
		return
	}

	common.Success(c, map[string]any{"contributors": contributors})
}
