include "stats.thrift"

service statsService {
    stats.ListContributorsResp ListContributors(1: stats.ListContributorsReq req) (api.get="/api/v1/stats/contributors")
    stats.GetContributorResp GetContributor(1: stats.GetContributorReq req) (api.get="/api/v1/stats/contributors/:author")
    stats.GetContributorTrendResp GetContributorTrend(1: stats.GetContributorTrendReq req) (api.get="/api/v1/stats/contributors/:author/trend")
    stats.GetContributorCategoriesResp GetContributorCategories(1: stats.GetContributorCategoriesReq req) (api.get="/api/v1/stats/contributors/:author/categories")
    stats.ListProjectsResp ListProjects(1: stats.ListProjectsReq req) (api.get="/api/v1/stats/projects")
    stats.GetProjectResp GetProject(1: stats.GetProjectReq req) (api.get="/api/v1/stats/projects/detail")
    stats.GetProjectTrendResp GetProjectTrend(1: stats.GetProjectReq req) (api.get="/api/v1/stats/projects/trend")
    stats.GetProjectCategoriesResp GetProjectCategories(1: stats.GetProjectReq req) (api.get="/api/v1/stats/projects/categories")
    stats.GetProjectFilesResp GetProjectFiles(1: stats.GetProjectReq req) (api.get="/api/v1/stats/projects/files")
    stats.GetProjectContributorsResp GetProjectContributors(1: stats.GetProjectContributorsReq req) (api.get="/api/v1/stats/projects/contributors")
}
