struct PublishReviewReq {
    1: string id (api.path="id")
    2: optional string mode (api.body="mode")
}

struct PublishReviewResp {
    1: string message (api.body="message")
    2: string review_id (api.body="review_id")
}

struct PreviewCommentReq {
    1: string id (api.path="id")
    2: optional string mode (api.body="mode")
}

struct PreviewCommentResp {
    1: string review_id (api.body="review_id")
    2: string summary (api.body="summary")
    3: string findings (api.body="findings")
    4: i64 total (api.body="total")
}

struct UpdatePublishedCommentReq {
    1: string id (api.path="id")
}

struct UpdatePublishedCommentResp {
    1: string message (api.body="message")
    2: string review_id (api.body="review_id")
}
