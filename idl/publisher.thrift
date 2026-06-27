struct PublishReviewReq {
    1: string id (api.path="id")
    2: optional string mode (api.body="mode")
    3: optional bool summary_only (api.body="summary_only")
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

struct PublishStatusItem {
    1: string finding_key (api.body="finding_key")
    2: string kind (api.body="kind")
    3: string status (api.body="status")
    4: i64 attempts (api.body="attempts")
    5: optional string external_id (api.body="external_id")
    6: optional string url (api.body="url")
    7: optional string last_error (api.body="last_error")
}

struct PublishStatus {
    1: string review_id (api.body="review_id")
    2: i64 total (api.body="total")
    3: i64 published (api.body="published")
    4: i64 failed (api.body="failed")
    5: i64 downgraded (api.body="downgraded")
    6: i64 pending (api.body="pending")
    7: list<PublishStatusItem> items (api.body="items")
}

struct PublishStatusReq {
    1: string id (api.path="id")
}

struct RetryPublishReq {
    1: string id (api.path="id")
}

struct RetryPublishResp {
    1: string message (api.body="message")
    2: string review_id (api.body="review_id")
}
