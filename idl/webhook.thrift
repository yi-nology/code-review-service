struct HandleWebhookReq {
    1: string provider (api.path="provider")
    2: string installation (api.path="installation")
    3: binary body (api.raw_body="body")
}

struct HandleWebhookResp {
    1: string status (api.body="status")
    2: optional string review_id (api.body="review_id")
}
