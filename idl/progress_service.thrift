include "progress.thrift"

service progressService {
    progress.HandleProgressWSResp HandleProgressWS(1: progress.HandleProgressWSReq req) (api.get="/api/v1/reviews/:id/progress")
    progress.GetProgressResp GetProgress(1: progress.GetProgressReq req) (api.get="/api/v1/reviews/:id/progress/status")
    progress.GetActiveReviewsResp GetActiveReviews(1: progress.GetActiveReviewsReq req) (api.get="/api/v1/progress/active")
}
