struct PublishReviewReq {
    1: string id (api.path="id")
}

struct PublishReviewResp {
    1: string message (api.body="message")
    2: string review_id (api.body="review_id")
}

struct PreviewCommentReq {
    1: string id (api.path="id")
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

service publisherService {
    PublishReviewResp PublishReview(1: PublishReviewReq req) (api.post="/api/v1/reviews/:id/publish")
    PreviewCommentResp PreviewComment(1: PreviewCommentReq req) (api.get="/api/v1/reviews/:id/preview")
    UpdatePublishedCommentResp UpdatePublishedComment(1: UpdatePublishedCommentReq req) (api.put="/api/v1/reviews/:id/publish")
}
