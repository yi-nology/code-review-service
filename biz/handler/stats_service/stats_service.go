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
	GetProjectOCRTrend(ctx context.Context, provider, repo, from, to string) ([]stats.ProjectOCRTrendPoint, error)
}

type StatsHandler struct {
	statsSvc StatsService
}

func InitHandler(statsSvc StatsService) {
	handlerInst = &StatsHandler{statsSvc: statsSvc}
}

func (h *StatsHandler) listContributors(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.ListContributorsReq](c)
	if !ok {
		return
	}

	page, pageSize := common.PageParams(
		convert.Int64ToStr(ptrx.Deref(req.Page, 0)),
		convert.Int64ToStr(ptrx.Deref(req.PageSize, 0)),
	)

	result, err := h.statsSvc.ListContributors(
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
		common.Fail(ctx, c, err)
		return
	}

	common.Success(c, result)
}

func ListContributors(ctx context.Context, c *hertzApp.RequestContext) {
	handlerInst.listContributors(ctx, c)
}

func (h *StatsHandler) getContributor(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetContributorReq](c)
	if !ok {
		return
	}

	detail, err := h.statsSvc.GetContributor(
		ctx,
		req.Author,
		ptrx.Deref(req.Provider, ""),
		ptrx.Deref(req.Repo, ""),
		ptrx.Deref(req.From, ""),
		ptrx.Deref(req.To, ""),
	)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	if detail == nil {
		common.Fail(ctx, c, common.NotFound("contributor not found"))
		return
	}

	common.Success(c, detail)
}

func GetContributor(ctx context.Context, c *hertzApp.RequestContext) {
	handlerInst.getContributor(ctx, c)
}

func (h *StatsHandler) getContributorTrend(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetContributorTrendReq](c)
	if !ok {
		return
	}

	trend, err := h.statsSvc.GetContributorTrend(
		ctx,
		req.Author,
		ptrx.Deref(req.Provider, ""),
		ptrx.Deref(req.Repo, ""),
		ptrx.Deref(req.From, ""),
		ptrx.Deref(req.To, ""),
	)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}

	common.Success(c, map[string]any{"trend": trend})
}

func GetContributorTrend(ctx context.Context, c *hertzApp.RequestContext) {
	handlerInst.getContributorTrend(ctx, c)
}

func (h *StatsHandler) getContributorCategories(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetContributorCategoriesReq](c)
	if !ok {
		return
	}

	categories, err := h.statsSvc.GetContributorCategories(
		ctx,
		req.Author,
		ptrx.Deref(req.Provider, ""),
		ptrx.Deref(req.Repo, ""),
		ptrx.Deref(req.From, ""),
		ptrx.Deref(req.To, ""),
	)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}

	common.Success(c, map[string]any{"categories": categories})
}

func GetContributorCategories(ctx context.Context, c *hertzApp.RequestContext) {
	handlerInst.getContributorCategories(ctx, c)
}

func (h *StatsHandler) listProjects(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.ListProjectsReq](c)
	if !ok {
		return
	}

	page, pageSize := common.PageParams(
		convert.Int64ToStr(ptrx.Deref(req.Page, 0)),
		convert.Int64ToStr(ptrx.Deref(req.PageSize, 0)),
	)
	result, err := h.statsSvc.ListProjects(
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
		common.Fail(ctx, c, err)
		return
	}

	common.Success(c, result)
}

func ListProjects(ctx context.Context, c *hertzApp.RequestContext) { handlerInst.listProjects(ctx, c) }

func (h *StatsHandler) getProject(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectReq](c)
	if !ok {
		return
	}

	detail, err := h.statsSvc.GetProject(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""))
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	if detail == nil {
		common.Fail(ctx, c, common.NotFound("project not found"))
		return
	}

	common.Success(c, detail)
}

func GetProject(ctx context.Context, c *hertzApp.RequestContext) { handlerInst.getProject(ctx, c) }

func (h *StatsHandler) getProjectTrend(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectReq](c)
	if !ok {
		return
	}

	trend, err := h.statsSvc.GetProjectTrend(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""))
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}

	common.Success(c, map[string]any{"trend": trend})
}

func GetProjectTrend(ctx context.Context, c *hertzApp.RequestContext) {
	handlerInst.getProjectTrend(ctx, c)
}

func (h *StatsHandler) getProjectOCRMetrics(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectReq](c)
	if !ok {
		return
	}
	metrics, err := h.statsSvc.GetProjectOCRMetrics(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""))
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{"metrics": metrics})
}

func GetProjectOCRMetrics(ctx context.Context, c *hertzApp.RequestContext) {
	handlerInst.getProjectOCRMetrics(ctx, c)
}

func (h *StatsHandler) getProjectOCRTrend(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectReq](c)
	if !ok {
		return
	}
	trend, err := h.statsSvc.GetProjectOCRTrend(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""))
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{"trend": trend})
}

func GetProjectOCRTrend(ctx context.Context, c *hertzApp.RequestContext) {
	handlerInst.getProjectOCRTrend(ctx, c)
}

func (h *StatsHandler) getProjectCategories(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectReq](c)
	if !ok {
		return
	}

	categories, err := h.statsSvc.GetProjectCategories(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""))
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}

	common.Success(c, map[string]any{"categories": categories})
}

func GetProjectCategories(ctx context.Context, c *hertzApp.RequestContext) {
	handlerInst.getProjectCategories(ctx, c)
}

func (h *StatsHandler) getProjectFiles(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectReq](c)
	if !ok {
		return
	}

	files, err := h.statsSvc.GetProjectFiles(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""))
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}

	common.Success(c, map[string]any{"files": files})
}

func GetProjectFiles(ctx context.Context, c *hertzApp.RequestContext) {
	handlerInst.getProjectFiles(ctx, c)
}

func (h *StatsHandler) getProjectContributors(ctx context.Context, c *hertzApp.RequestContext) {
	req, ok := common.Bind[modelStats.GetProjectContributorsReq](c)
	if !ok {
		return
	}

	limit := int(ptrx.Deref(req.Limit, 10))
	if limit <= 0 {
		limit = 10
	}
	contributors, err := h.statsSvc.GetProjectContributors(ctx, req.Provider, req.Repo, ptrx.Deref(req.From, ""), ptrx.Deref(req.To, ""), limit)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}

	common.Success(c, map[string]any{"contributors": contributors})
}

func GetProjectContributors(ctx context.Context, c *hertzApp.RequestContext) {
	handlerInst.getProjectContributors(ctx, c)
}
