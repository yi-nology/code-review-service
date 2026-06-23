include "review.thrift"
include "settings.thrift"
include "stats.thrift"
include "session.thrift"
include "progress.thrift"
include "review_map.thrift"
include "publisher.thrift"
include "project_agent.thrift"
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

service statsService {
    stats.ListContributorsResp ListContributors(1: stats.ListContributorsReq req) (api.get="/api/v1/stats/contributors")
    stats.GetContributorResp GetContributor(1: stats.GetContributorReq req) (api.get="/api/v1/stats/contributors/:author")
    stats.GetContributorTrendResp GetContributorTrend(1: stats.GetContributorTrendReq req) (api.get="/api/v1/stats/contributors/:author/trend")
    stats.GetContributorCategoriesResp GetContributorCategories(1: stats.GetContributorCategoriesReq req) (api.get="/api/v1/stats/contributors/:author/categories")
    stats.ListProjectsResp ListProjects(1: stats.ListProjectsReq req) (api.get="/api/v1/stats/projects")
    stats.GetProjectResp GetProject(1: stats.GetProjectReq req) (api.get="/api/v1/stats/projects/detail")
    stats.GetProjectTrendResp GetProjectTrend(1: stats.GetProjectReq req) (api.get="/api/v1/stats/projects/trend")
    stats.GetProjectCategoriesResp GetProjectCategories(1: stats.GetProjectReq req) (api.get="/api/v1/stats/projects/categories")
    stats.GetProjectFilesResp GetProjectFiles(1: stats.GetProjectReq req) (api.get="/api/v1/stats/projects/files")
    stats.GetProjectContributorsResp GetProjectContributors(1: stats.GetProjectContributorsReq req) (api.get="/api/v1/stats/projects/contributors")
}

service sessionService {
    session.ListSessionsResp ListSessions(1: session.ListSessionsReq req) (api.get="/api/v1/sessions")
    session.GetSessionsResp GetSessions(1: session.GetSessionsReq req) (api.get="/api/v1/reviews/:id/sessions")
    session.CreateSessionResp CreateSession(1: session.CreateSessionReq req) (api.post="/api/v1/reviews/:id/sessions")
    session.GetActiveSessionResp GetActiveSession(1: session.GetActiveSessionReq req) (api.get="/api/v1/reviews/:id/sessions/active")
    session.GetSessionResp GetSession(1: session.GetSessionReq req) (api.get="/api/v1/reviews/:id/sessions/:sessionId")
    session.UpdateSessionPhaseResp UpdateSessionPhase(1: session.UpdateSessionPhaseReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/phase")
    session.UpdateSessionStatusResp UpdateSessionStatus(1: session.UpdateSessionStatusReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/status")
    session.UpdateSessionAgentResultsResp UpdateSessionAgentResults(1: session.UpdateSessionAgentResultsReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/agent-results")
    session.UpdateSessionDiscussionResp UpdateSessionDiscussion(1: session.UpdateSessionDiscussionReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/discussion")
    session.UpdateSessionFinalReportResp UpdateSessionFinalReport(1: session.UpdateSessionFinalReportReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/final-report")
    session.CompleteSessionResp CompleteSession(1: session.CompleteSessionReq req) (api.post="/api/v1/reviews/:id/sessions/:sessionId/complete")
    session.FailSessionResp FailSession(1: session.FailSessionReq req) (api.post="/api/v1/reviews/:id/sessions/:sessionId/fail")
    session.CancelSessionResp CancelSession(1: session.CancelSessionReq req) (api.post="/api/v1/reviews/:id/sessions/:sessionId/cancel")
}

service progressService {
    progress.HandleProgressWSResp HandleProgressWS(1: progress.HandleProgressWSReq req) (api.get="/api/v1/reviews/:id/progress")
    progress.GetProgressResp GetProgress(1: progress.GetProgressReq req) (api.get="/api/v1/reviews/:id/progress/status")
    progress.GetActiveReviewsResp GetActiveReviews(1: progress.GetActiveReviewsReq req) (api.get="/api/v1/progress/active")
}

service reviewMapService {
    review_map.GenerateMapResp GenerateMap(1: review_map.GenerateMapReq req) (api.post="/api/v1/reviews/:id/maps")
    review_map.GetMapsResp GetMaps(1: review_map.GetMapsReq req) (api.get="/api/v1/reviews/:id/maps")
    review_map.GetMapResp GetMap(1: review_map.GetMapReq req) (api.get="/api/v1/reviews/:id/maps/:mapId")
    review_map.PreviewMapResp PreviewMap(1: review_map.PreviewMapReq req) (api.post="/api/v1/reviews/:id/maps/preview")
    review_map.GetSessionMapResp GetSessionMap(1: review_map.GetSessionMapReq req) (api.get="/api/v1/reviews/:id/sessions/:sessionId/map")
}

service publisherService {
    publisher.PublishReviewResp PublishReview(1: publisher.PublishReviewReq req) (api.post="/api/v1/reviews/:id/publish")
    publisher.PreviewCommentResp PreviewComment(1: publisher.PreviewCommentReq req) (api.get="/api/v1/reviews/:id/preview")
    publisher.UpdatePublishedCommentResp UpdatePublishedComment(1: publisher.UpdatePublishedCommentReq req) (api.put="/api/v1/reviews/:id/publish")
}

service projectAgentService {
    project_agent.GetProjectAgentsResp GetProjectAgents(1: project_agent.GetProjectAgentsReq req) (api.get="/api/v1/settings/project-agents")
    project_agent.UpdateProjectAgentResp UpdateProjectAgent(1: project_agent.UpdateProjectAgentReq req) (api.put="/api/v1/settings/project-agents")
    project_agent.BatchUpdateProjectAgentsResp BatchUpdateProjectAgents(1: project_agent.BatchUpdateProjectAgentsReq req) (api.post="/api/v1/settings/project-agents/batch")
    project_agent.DeleteProjectAgentResp DeleteProjectAgent(1: project_agent.DeleteProjectAgentReq req) (api.delete="/api/v1/settings/project-agents")
}

service settingsService {
    settings.GetOCRSettingsResp GetOCRSettings() (api.get="/api/v1/settings/ocr")
    settings.UpdateOCRSettingsResp UpdateOCRSettings(1: settings.UpdateOCRSettingsReq req) (api.put="/api/v1/settings/ocr")
    settings.TestLLMResp TestLLM() (api.post="/api/v1/settings/ocr/test-llm")
    settings.ListLLMProvidersResp ListLLMProviders() (api.get="/api/v1/settings/ocr/providers")
    settings.ListCustomProvidersResp ListCustomProviders() (api.get="/api/v1/settings/ocr/custom-providers")
    settings.CreateCustomProviderResp CreateCustomProvider(1: settings.CreateCustomProviderReq req) (api.post="/api/v1/settings/ocr/custom-providers")
    settings.UpdateCustomProviderResp UpdateCustomProvider(1: settings.UpdateCustomProviderReq req) (api.put="/api/v1/settings/ocr/custom-providers/:id")
    settings.DeleteCustomProviderResp DeleteCustomProvider(1: settings.DeleteCustomProviderReq req) (api.delete="/api/v1/settings/ocr/custom-providers/:id")
    settings.RulesCheckResp RulesCheck(1: settings.RulesCheckReq req) (api.post="/api/v1/settings/ocr/rules-check")
    settings.OCRVersionResp OCRVersion() (api.get="/api/v1/settings/ocr/version")
    settings.ViewerStatusResp ViewerStatus() (api.get="/api/v1/settings/ocr/viewer")
    settings.StartViewerResp StartViewer(1: settings.StartViewerReq req) (api.post="/api/v1/settings/ocr/viewer/start")
    settings.ViewerStatusResp StopViewer() (api.post="/api/v1/settings/ocr/viewer/stop")

    settings.ListProjectPoliciesResp ListProjectPolicies() (api.get="/api/v1/settings/projects")
    settings.PutProjectPolicyResp PutProjectPolicy(1: settings.PutProjectPolicyReq req) (api.put="/api/v1/settings/projects")
    settings.DeleteProjectPolicyResp DeleteProjectPolicy(1: settings.DeleteProjectPolicyReq req) (api.delete="/api/v1/settings/projects")
    settings.ListPlatformsResp ListPlatforms() (api.get="/api/v1/settings/platforms")
    settings.PutPlatformResp PutPlatform(1: settings.PutPlatformReq req) (api.put="/api/v1/settings/platforms")
    settings.DeletePlatformResp DeletePlatform(1: settings.DeletePlatformReq req) (api.delete="/api/v1/settings/platforms")
    settings.TestPlatformResp TestPlatform(1: settings.TestPlatformReq req) (api.post="/api/v1/settings/platforms/test")
    settings.GetGlobalRulesResp GetGlobalRules() (api.get="/api/v1/settings/rules/global")
    settings.UpdateGlobalRulesResp UpdateGlobalRules(1: settings.UpdateGlobalRulesReq req) (api.put="/api/v1/settings/rules/global")

    settings.ListReviewerAgentsResp ListReviewerAgents() (api.get="/api/v1/settings/agents")
    settings.CreateReviewerAgentResp CreateReviewerAgent(1: settings.CreateReviewerAgentReq req) (api.post="/api/v1/settings/agents")
    settings.UpdateReviewerAgentResp UpdateReviewerAgent(1: settings.UpdateReviewerAgentReq req) (api.put="/api/v1/settings/agents/:id")
    settings.DeleteReviewerAgentResp DeleteReviewerAgent(1: settings.DeleteReviewerAgentReq req) (api.delete="/api/v1/settings/agents/:id")
}
