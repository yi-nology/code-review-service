include "review.thrift"
include "webhook.thrift"
include "platform.thrift"

service reviewService {
    review.CreateReviewResp CreateReview(1: review.CreateReviewReq req) (api.post="/api/v1/reviews")
    review.GetReviewResp GetReview(1: review.GetReviewReq req) (api.get="/api/v1/reviews/:id")
    review.ListReviewsResp ListReviews(1: review.ListReviewsReq req) (api.get="/api/v1/reviews")
    review.PreviewResp PreviewReview(1: review.PreviewReq req) (api.post="/api/v1/reviews/preview")
    review.CreateReviewResp RerunReview(1: review.ReviewActionReq req) (api.post="/api/v1/reviews/:id/rerun")
    review.CreateReviewResp CancelReview(1: review.ReviewActionReq req) (api.post="/api/v1/reviews/:id/cancel")

    webhook.HandleWebhookResp HandleWebhook(1: webhook.HandleWebhookReq req) (api.post="/api/v1/webhooks/:provider/:installation")

    platform.ListProviderReposResp ListProviderRepos(1: platform.ListProviderReposReq req) (api.get="/api/v1/platforms/:provider/repos")
    platform.ListProviderBranchesResp ListProviderBranches(1: platform.ListProviderBranchesReq req) (api.get="/api/v1/platforms/:provider/repos/:owner/:repo/branches")
    platform.GetBranchSHAResp GetBranchSHA(1: platform.GetBranchSHAReq req) (api.get="/api/v1/platforms/:provider/repos/:owner/:repo/branch-sha")
    platform.ListProviderPullsResp ListProviderPulls(1: platform.ListProviderPullsReq req) (api.get="/api/v1/platforms/:provider/repos/:owner/:repo/pulls")
}
