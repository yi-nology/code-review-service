package review_service

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	rsHandler "review-service/biz/handler/review_service"
)

func Register(r *server.Hertz, h *rsHandler.ReviewHandler, wh *rsHandler.WebhookHandler) {
	root := r.Group("/", rootMw()...)
	{
		_api := root.Group("/api", _apiMw()...)
		{
			_v1 := _api.Group("/v1", _v1Mw()...)
			_v1.GET("/reviews", append(_listreviewsMw(), h.ListReviews)...)
			_v1.POST("/reviews", append(_createreviewMw(), h.CreateReview)...)
			_v1.POST("/reviews/preview", append(_createreviewMw(), h.PreviewReview)...)
			_reviews := _v1.Group("/reviews", _reviewsMw()...)
			_reviews.GET("/:id", append(_getreviewMw(), h.GetReview)...)
			_v1.POST("/webhooks/:provider/:installation", append(_webhookMw(), wh.HandleWebhook)...)
			{
				_platforms := _v1.Group("/platforms", _platformsMw()...)
				_platforms.GET("/:provider/repos", h.ListProviderRepos)
				_platforms.GET("/:provider/repos/:owner/:repo/branches", h.ListProviderBranches)
				_platforms.GET("/:provider/repos/:owner/:repo/branch-sha", h.GetBranchSHA)
				_platforms.GET("/:provider/repos/:owner/:repo/pulls", h.ListProviderPulls)
			}
			{
				_settings := _v1.Group("/settings", _settingsMw()...)
				_settings.GET("/ocr", append(_getocrsettingsMw(), h.GetOCRSettings)...)
				_settings.PUT("/ocr", append(_updateocrsettingsMw(), h.UpdateOCRSettings)...)
				_settings.POST("/ocr/test-llm", append(_updateocrsettingsMw(), h.TestLLM)...)
				_settings.DELETE("/platforms", append(_deleteplatformMw(), h.DeletePlatform)...)
				_platforms := _settings.Group("/platforms", _platformsMw()...)
				_platforms.POST("/test", append(_testplatformMw(), h.TestPlatform)...)
				_settings.GET("/platforms", append(_listplatformsMw(), h.ListPlatforms)...)
				_settings.PUT("/platforms", append(_putplatformMw(), h.PutPlatform)...)
				_settings.DELETE("/projects", append(_deleteprojectpolicyMw(), h.DeleteProjectPolicy)...)
				_settings.GET("/projects", append(_listprojectpoliciesMw(), h.ListProjectPolicies)...)
				_settings.PUT("/projects", append(_putprojectpolicyMw(), h.PutProjectPolicy)...)
				_settings.GET("/rules/global", append(_getocrsettingsMw(), h.GetGlobalRules)...)
				_settings.PUT("/rules/global", append(_updateocrsettingsMw(), h.UpdateGlobalRules)...)
			}
		}
	}
}
