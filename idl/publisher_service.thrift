include "publisher.thrift"

service publisherService {
    publisher.PublishReviewResp PublishReview(1: publisher.PublishReviewReq req) (api.post="/api/v1/reviews/:id/publish")
    publisher.PreviewCommentResp PreviewComment(1: publisher.PreviewCommentReq req) (api.get="/api/v1/reviews/:id/preview")
    publisher.UpdatePublishedCommentResp UpdatePublishedComment(1: publisher.UpdatePublishedCommentReq req) (api.put="/api/v1/reviews/:id/publish")
}
