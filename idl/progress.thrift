struct HandleProgressWSReq {
    1: string id (api.path="id")
}

struct HandleProgressWSResp {}

struct GetProgressReq {
    1: string id (api.path="id")
}

struct GetProgressResp {
    1: string progress (api.body="progress")
}

struct GetActiveReviewsReq {}

struct GetActiveReviewsResp {
    1: string reviews (api.body="reviews")
    2: i64 total (api.body="total")
}
