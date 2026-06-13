include "review.thrift"
include "settings.thrift"

service reviewService {
    review.CreateReviewResp CreateReview(1: review.CreateReviewReq req) (api.post="/api/v1/reviews")
    review.GetReviewResp GetReview(1: review.GetReviewReq req) (api.get="/api/v1/reviews/:id")
    review.ListReviewsResp ListReviews(1: review.ListReviewsReq req) (api.get="/api/v1/reviews")
    review.PreviewResp PreviewReview(1: review.PreviewReq req) (api.post="/api/v1/reviews/preview")
    review.CreateReviewResp RerunReview(1: review.ReviewActionReq req) (api.post="/api/v1/reviews/:id/rerun")
    review.CreateReviewResp CancelReview(1: review.ReviewActionReq req) (api.post="/api/v1/reviews/:id/cancel")
}

service settingsService {
    settings.GetOCRSettingsResp GetOCRSettings() (api.get="/api/v1/settings/ocr")
    settings.UpdateOCRSettingsResp UpdateOCRSettings(1: settings.UpdateOCRSettingsReq req) (api.put="/api/v1/settings/ocr")
    settings.TestLLMResp TestLLM() (api.post="/api/v1/settings/ocr/test-llm")
    settings.ListProjectPoliciesResp ListProjectPolicies() (api.get="/api/v1/settings/projects")
    settings.PutProjectPolicyResp PutProjectPolicy(1: settings.PutProjectPolicyReq req) (api.put="/api/v1/settings/projects")
    settings.DeleteProjectPolicyResp DeleteProjectPolicy(1: settings.DeleteProjectPolicyReq req) (api.delete="/api/v1/settings/projects")
    settings.ListPlatformsResp ListPlatforms() (api.get="/api/v1/settings/platforms")
    settings.PutPlatformResp PutPlatform(1: settings.PutPlatformReq req) (api.put="/api/v1/settings/platforms")
    settings.DeletePlatformResp DeletePlatform(1: settings.DeletePlatformReq req) (api.delete="/api/v1/settings/platforms")
    settings.TestPlatformResp TestPlatform(1: settings.TestPlatformReq req) (api.post="/api/v1/settings/platforms/test")
    settings.GetGlobalRulesResp GetGlobalRules() (api.get="/api/v1/settings/rules/global")
    settings.UpdateGlobalRulesResp UpdateGlobalRules(1: settings.UpdateGlobalRulesReq req) (api.put="/api/v1/settings/rules/global")
}
