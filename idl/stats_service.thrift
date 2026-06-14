include "stats.thrift"

service statsService {
    stats.ListContributorsResp ListContributors(1: stats.ListContributorsReq req) (api.get="/api/v1/stats/contributors")
    stats.GetContributorResp GetContributor(1: stats.GetContributorReq req) (api.get="/api/v1/stats/contributors/:author")
    stats.GetContributorTrendResp GetContributorTrend(1: stats.GetContributorTrendReq req) (api.get="/api/v1/stats/contributors/:author/trend")
    stats.GetContributorCategoriesResp GetContributorCategories(1: stats.GetContributorCategoriesReq req) (api.get="/api/v1/stats/contributors/:author/categories")
}
