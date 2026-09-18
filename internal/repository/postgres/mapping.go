package postgres

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/yi-nology/code-review-service/internal/domain/feedback"
	"github.com/yi-nology/code-review-service/internal/domain/llmprovider"
	"github.com/yi-nology/code-review-service/internal/domain/ocrasset"
	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
	"github.com/yi-nology/code-review-service/internal/domain/platform"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/domain/reviewer"
	"github.com/yi-nology/code-review-service/internal/domain/revision"
	"github.com/yi-nology/code-review-service/internal/domain/rules"
	"github.com/yi-nology/code-review-service/internal/domain/session"
	domainWebhook "github.com/yi-nology/code-review-service/internal/domain/webhook"
	"github.com/yi-nology/code-review-service/internal/domain/writeback"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"gorm.io/datatypes"
	gormDB "gorm.io/gorm"
)

// mapping.go 是 domain.* ↔ models.* 的防腐层映射中枢（#6 迁移）。
// 仓储接口暴露 domain 类型，postgres 实现在边界处双向转换，
// 使上层不再 import postgres/models。

// ---- helpers ----

// gormModel 用领域 ID 重建 gorm.Model（保留主键供 Save/Update 使用）。
func gormModel(id uint) gormDB.Model {
	return gormDB.Model{ID: id}
}

// rawJSON 将 datatypes.JSON 转为 json.RawMessage（nil 安全）。
func rawJSON(j datatypes.JSON) json.RawMessage {
	if len(j) == 0 {
		return nil
	}
	return json.RawMessage(j)
}

// rawJSONArray 将 datatypes.JSON 字符串数组转为 []string（nil 安全）。
func rawJSONArray(j datatypes.JSON) []string {
	if len(j) == 0 {
		return nil
	}
	var arr []string
	if err := json.Unmarshal(j, &arr); err != nil {
		return nil
	}
	return arr
}

// dataTypeJSON 将 json.RawMessage 转为 datatypes.JSON。
func dataTypeJSON(r json.RawMessage) datatypes.JSON {
	if len(r) == 0 {
		return nil
	}
	return datatypes.JSON(r)
}

// stringSliceToJSON 将 []string 转为 datatypes.JSON。
func stringSliceToJSON(s []string) datatypes.JSON {
	if len(s) == 0 {
		return nil
	}
	b, _ := json.Marshal(s)
	return datatypes.JSON(b)
}

// ---- feedback ----

func feedbackToModel(f *feedback.Feedback) models.FindingFeedback {
	return models.FindingFeedback{
		ReviewID:     f.ReviewID,
		FindingIndex: f.FindingIndex,
		Fingerprint:  f.Fingerprint,
		Status:       f.Status,
		Note:         f.Note,
		UpdatedAt:    f.UpdatedAt,
	}
}

func feedbackToDomain(m models.FindingFeedback) feedback.Feedback {
	return feedback.Feedback{
		ReviewID:     m.ReviewID,
		FindingIndex: m.FindingIndex,
		Fingerprint:  m.Fingerprint,
		Status:       m.Status,
		Note:         m.Note,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

// ---- platform ----

func platformToModel(p *platform.Platform) models.GitPlatform {
	return models.GitPlatform{
		Provider:       p.Provider,
		InstallationID: p.InstallationID,
		BaseURL:        p.BaseURL,
		Token:          p.Token,
		WebhookSecret:  p.WebhookSecret,
		Enabled:        p.Enabled,
		SkipTLS:        p.SkipTLS,
	}
}

func platformToDomain(m models.GitPlatform) platform.Platform {
	return platform.Platform{
		Provider:       m.Provider,
		InstallationID: m.InstallationID,
		BaseURL:        m.BaseURL,
		Token:          m.Token,
		WebhookSecret:  m.WebhookSecret,
		Enabled:        m.Enabled,
		SkipTLS:        m.SkipTLS,
	}
}

// ---- revision ----

// changeNumberToInt 把 string 编号转为 DB int 列值（change_number 列保持 int）。
// 非数字编号按 0 处理，仅影响存量行匹配，不影响新写入。
func changeNumberToInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func revisionToModel(c *revision.Change) models.ChangeRevision {
	return models.ChangeRevision{
		Provider:       c.Provider,
		InstallationID: c.InstallationID,
		RepositoryID:   c.RepositoryID,
		Repository:     c.Repository,
		ChangeNumber:   changeNumberToInt(c.ChangeNumber),
		BaseSHA:        c.BaseSHA,
		StartSHA:       c.StartSHA,
		HeadSHA:        c.HeadSHA,
		LatestReviewID: c.LatestReviewID,
	}
}

func revisionToDomain(m models.ChangeRevision) revision.Change {
	return revision.Change{
		Provider:       m.Provider,
		InstallationID: m.InstallationID,
		RepositoryID:   m.RepositoryID,
		Repository:     m.Repository,
		ChangeNumber:   strconv.Itoa(m.ChangeNumber),
		BaseSHA:        m.BaseSHA,
		StartSHA:       m.StartSHA,
		HeadSHA:        m.HeadSHA,
		LatestReviewID: m.LatestReviewID,
		CreatedAt:      m.CreatedAt,
	}
}

// ---- writeback ----

func writebackToModel(i *writeback.Item) models.WritebackItem {
	return models.WritebackItem{
		ReviewID:           i.ReviewID,
		FindingKey:         i.FindingKey,
		Kind:               i.Kind,
		ProviderExternalID: i.ProviderExternalID,
		ProviderURL:        i.ProviderURL,
		Status:             i.Status,
		Attempts:           i.Attempts,
		LastError:          i.LastError,
		PublishedAt:        i.PublishedAt,
	}
}

func writebackToDomain(m models.WritebackItem) writeback.Item {
	return writeback.Item{
		ReviewID:           m.ReviewID,
		FindingKey:         m.FindingKey,
		Kind:               m.Kind,
		ProviderExternalID: m.ProviderExternalID,
		ProviderURL:        m.ProviderURL,
		Status:             m.Status,
		Attempts:           m.Attempts,
		LastError:          m.LastError,
		PublishedAt:        m.PublishedAt,
	}
}

// ---- reviewer ----

func reviewerToModel(a *reviewer.Agent) models.ReviewerAgent {
	return models.ReviewerAgent{
		Model:          gormModel(a.ID),
		Name:           a.Name,
		DisplayName:    a.DisplayName,
		Description:    a.Description,
		Background:     a.Background,
		RuleFile:       a.RuleFile,
		Redundancy:     a.Redundancy,
		Enabled:        a.Enabled,
		Priority:       a.Priority,
		IsDefault:      a.IsDefault,
		Tags:           dataTypeJSON(a.Tags),
		RulePackageIDs: dataTypeJSON(a.RulePackageIDs),
	}
}

func reviewerToDomain(m models.ReviewerAgent) reviewer.Agent {
	return reviewer.Agent{
		ID:             m.ID,
		Name:           m.Name,
		DisplayName:    m.DisplayName,
		Description:    m.Description,
		Background:     m.Background,
		RuleFile:       m.RuleFile,
		Redundancy:     m.Redundancy,
		Enabled:        m.Enabled,
		Priority:       m.Priority,
		IsDefault:      m.IsDefault,
		Tags:           rawJSON(m.Tags),
		RulePackageIDs: rawJSON(m.RulePackageIDs),
	}
}

func projectAgentToModel(p *reviewer.ProjectAssignment) models.ProjectAgent {
	return models.ProjectAgent{
		Model:    gormModel(p.ID),
		Provider: p.Provider,
		Repo:     p.Repo,
		AgentID:  p.AgentID,
		Enabled:  p.Enabled,
		Priority: p.Priority,
	}
}

func projectAgentToDomain(m models.ProjectAgent) reviewer.ProjectAssignment {
	return reviewer.ProjectAssignment{
		ID:          m.ID,
		Provider:    m.Provider,
		Repo:        m.Repo,
		AgentID:     m.AgentID,
		Enabled:     m.Enabled,
		Priority:    m.Priority,
		AgentName:   m.AgentName,
		DisplayName: m.DisplayName,
		Background:  m.Background,
	}
}

// ---- llmprovider ----

func customProviderToModel(c *llmprovider.Custom) models.CustomProvider {
	return models.CustomProvider{
		Model:      gormModel(c.ID),
		Name:       c.Name,
		Protocol:   c.Protocol,
		URL:        c.URL,
		APIKey:     c.APIKey,
		ModelName:  c.ModelName,
		AuthHeader: c.AuthHeader,
		ExtraBody:  dataTypeJSON(c.ExtraBody),
		Enabled:    c.Enabled,
	}
}

func customProviderToDomain(m models.CustomProvider) llmprovider.Custom {
	return llmprovider.Custom{
		ID:         m.ID,
		Name:       m.Name,
		Protocol:   m.Protocol,
		URL:        m.URL,
		APIKey:     m.APIKey,
		ModelName:  m.ModelName,
		AuthHeader: m.AuthHeader,
		ExtraBody:  rawJSON(m.ExtraBody),
		Enabled:    m.Enabled,
	}
}

// ---- webhook delivery ----

func deliveryToModel(d *domainWebhook.Delivery) models.WebhookDelivery {
	return models.WebhookDelivery{
		Provider:       d.Provider,
		InstallationID: d.InstallationID,
		DeliveryID:     d.DeliveryID,
		EventType:      d.EventType,
		Action:         d.Action,
		PayloadSHA256:  d.PayloadSHA256,
		Payload:        dataTypeJSON(d.Payload),
		Status:         d.Status,
		Error:          d.Error,
		ReceivedAt:     d.ReceivedAt,
	}
}

func deliveryToDomain(m models.WebhookDelivery) domainWebhook.Delivery {
	return domainWebhook.Delivery{
		Provider:       m.Provider,
		InstallationID: m.InstallationID,
		DeliveryID:     m.DeliveryID,
		EventType:      m.EventType,
		Action:         m.Action,
		PayloadSHA256:  m.PayloadSHA256,
		Payload:        rawJSON(m.Payload),
		Status:         m.Status,
		Error:          m.Error,
		ReceivedAt:     m.ReceivedAt,
		ProcessedAt:    m.ProcessedAt,
	}
}

// ---- global rules ----

func globalRulesToModel(g *rules.Global) models.GlobalRules {
	return models.GlobalRules{Rules: dataTypeJSON(g.Content)}
}

func globalRulesToDomain(m models.GlobalRules) rules.Global {
	return rules.Global{Content: rawJSON(m.Rules)}
}

// ---- ocr settings ----

func ocrSettingsToModel(s *ocrsettings.Settings) models.OCRSettings {
	return models.OCRSettings{
		Enabled:                 s.Enabled,
		Binary:                  s.Binary,
		Audience:                s.Audience,
		DefaultSeverity:         s.DefaultSeverity,
		Provider:                s.Provider,
		URL:                     s.URL,
		AuthToken:               s.AuthToken,
		AuthHeader:              s.AuthHeader,
		ExtraHeaders:            s.ExtraHeaders,
		ModelName:               s.ModelName,
		UseAnthropic:            s.UseAnthropic,
		Protocol:                s.Protocol,
		BackgroundFile:          s.BackgroundFile,
		ExtraBody:               dataTypeJSON(s.ExtraBody),
		ReasoningMode:           s.ReasoningMode,
		ReasoningBudget:         s.ReasoningBudget,
		Language:                s.Language,
		Concurrency:             s.Concurrency,
		MaxGitProcs:             s.MaxGitProcs,
		TimeoutMinutes:          s.TimeoutMinutes,
		MaxTools:                s.MaxTools,
		Background:              s.Background,
		ToolsPath:               s.ToolsPath,
		ToolsConfig:             dataTypeJSON(s.ToolsConfig),
		McpServers:              dataTypeJSON(s.McpServers),
		TelemetryEnabled:        s.TelemetryEnabled,
		TelemetryExporter:       s.TelemetryExporter,
		TelemetryOTLPEndpoint:   s.TelemetryOTLPEndpoint,
		TelemetryContentLogging: s.TelemetryContentLogging,
		ContextWindow:           s.ContextWindow,
		MaxOutputTokens:         s.MaxOutputTokens,
		PolicyFailOn:            s.PolicyFailOn,
		PolicyMaxComments:       s.PolicyMaxComments,
		PolicySummaryOnly:       s.PolicySummaryOnly,
		PolicyIgnorePaths:       stringSliceToJSON(s.PolicyIgnorePaths),
		MaxDiffBytes:            s.MaxDiffBytes,
		GitHubMirror:            s.GitHubMirror,
	}
}

func ocrSettingsToDomain(m models.OCRSettings) ocrsettings.Settings {
	return ocrsettings.Settings{
		Enabled:                 m.Enabled,
		Binary:                  m.Binary,
		Audience:                m.Audience,
		DefaultSeverity:         m.DefaultSeverity,
		Provider:                m.Provider,
		URL:                     m.URL,
		AuthToken:               m.AuthToken,
		AuthHeader:              m.AuthHeader,
		ExtraHeaders:            m.ExtraHeaders,
		ModelName:               m.ModelName,
		UseAnthropic:            m.UseAnthropic,
		Protocol:                m.Protocol,
		BackgroundFile:          m.BackgroundFile,
		ExtraBody:               rawJSON(m.ExtraBody),
		ReasoningMode:           m.ReasoningMode,
		ReasoningBudget:         m.ReasoningBudget,
		Language:                m.Language,
		Concurrency:             m.Concurrency,
		MaxGitProcs:             m.MaxGitProcs,
		TimeoutMinutes:          m.TimeoutMinutes,
		MaxTools:                m.MaxTools,
		Background:              m.Background,
		ToolsPath:               m.ToolsPath,
		ToolsConfig:             rawJSON(m.ToolsConfig),
		McpServers:              rawJSON(m.McpServers),
		TelemetryEnabled:        m.TelemetryEnabled,
		TelemetryExporter:       m.TelemetryExporter,
		TelemetryOTLPEndpoint:   m.TelemetryOTLPEndpoint,
		TelemetryContentLogging: m.TelemetryContentLogging,
		ContextWindow:           m.ContextWindow,
		MaxOutputTokens:         m.MaxOutputTokens,
		PolicyFailOn:            m.PolicyFailOn,
		PolicyMaxComments:       m.PolicyMaxComments,
		PolicySummaryOnly:       m.PolicySummaryOnly,
		PolicyIgnorePaths:       rawJSONArray(m.PolicyIgnorePaths),
		MaxDiffBytes:            m.MaxDiffBytes,
		GitHubMirror:            m.GitHubMirror,
	}
}

// ---- ocr rule package ----

func rulePackageToModel(p *ocrasset.RulePackage) models.OCRRulePackage {
	return models.OCRRulePackage{
		Model:       gormModel(p.ID),
		Name:        p.Name,
		Description: p.Description,
		Languages:   dataTypeJSON(p.Languages),
		Tags:        dataTypeJSON(p.Tags),
		Includes:    dataTypeJSON(p.Includes),
		Excludes:    dataTypeJSON(p.Excludes),
		Rules:       dataTypeJSON(p.Rules),
		Enabled:     p.Enabled,
		ProjectRefs: dataTypeJSON(p.ProjectRefs),
		BuiltIn:     p.BuiltIn,
		Scope:       p.Scope,
	}
}

func rulePackageToDomain(m models.OCRRulePackage) ocrasset.RulePackage {
	return ocrasset.RulePackage{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Languages:   rawJSON(m.Languages),
		Tags:        rawJSON(m.Tags),
		Includes:    rawJSON(m.Includes),
		Excludes:    rawJSON(m.Excludes),
		Rules:       rawJSON(m.Rules),
		Enabled:     m.Enabled,
		ProjectRefs: rawJSON(m.ProjectRefs),
		BuiltIn:     m.BuiltIn,
		Scope:       m.Scope,
	}
}

func rulePackageVersionToDomain(m models.OCRRulePackageVersion) ocrasset.RulePackageVersion {
	return ocrasset.RulePackageVersion{
		ID:        m.ID,
		PackageID: m.PackageID,
		Version:   m.Version,
		Snapshot:  rawJSON(m.Snapshot),
		CreatedAt: m.CreatedAt,
	}
}

// ---- ocr tool package ----

func toolPackageToModel(p *ocrasset.ToolPackage) models.OCRToolPackage {
	return models.OCRToolPackage{
		Model:       gormModel(p.ID),
		Name:        p.Name,
		Description: p.Description,
		ScanType:    p.ScanType,
		Config:      dataTypeJSON(p.Config),
		Enabled:     p.Enabled,
		ProjectRefs: dataTypeJSON(p.ProjectRefs),
		BuiltIn:     p.BuiltIn,
	}
}

func toolPackageToDomain(m models.OCRToolPackage) ocrasset.ToolPackage {
	return ocrasset.ToolPackage{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		ScanType:    m.ScanType,
		Config:      rawJSON(m.Config),
		Enabled:     m.Enabled,
		ProjectRefs: rawJSON(m.ProjectRefs),
		BuiltIn:     m.BuiltIn,
	}
}

func toolPackageVersionToDomain(m models.OCRToolPackageVersion) ocrasset.ToolPackageVersion {
	return ocrasset.ToolPackageVersion{
		ID:        m.ID,
		PackageID: m.PackageID,
		Version:   m.Version,
		Snapshot:  rawJSON(m.Snapshot),
		CreatedAt: m.CreatedAt,
	}
}

// ---- project policy ----

func projectPolicyToModel(p *projectpolicy.ProjectPolicy) models.ProjectPolicy {
	return models.ProjectPolicy{
		Provider:                   p.Provider,
		Repo:                       p.Repo,
		FailOn:                     p.FailOn,
		IgnorePaths:                dataTypeJSON(p.IgnorePaths),
		Rules:                      dataTypeJSON(p.Rules),
		ReviewEngine:               p.ReviewEngine,
		GateMode:                   p.GateMode,
		OCREnabled:                 p.OCREnabled,
		OCRConcurrency:             p.OCRConcurrency,
		OCRTimeoutMinutes:          p.OCRTimeoutMinutes,
		OCRMaxTools:                p.OCRMaxTools,
		OCRDefaultSeverity:         p.OCRDefaultSeverity,
		OCRBackground:              p.OCRBackground,
		OCRBackgroundFile:          p.OCRBackgroundFile,
		OCRAudience:                p.OCRAudience,
		OCRProvider:                p.OCRProvider,
		OCRURL:                     p.OCRURL,
		OCRModel:                   p.OCRModel,
		OCRMaxGitProcs:             p.OCRMaxGitProcs,
		OCRUseAnthropic:            p.OCRUseAnthropic,
		OCRProtocol:                p.OCRProtocol,
		OCRExtraBody:               p.OCRExtraBody,
		OCRLanguage:                p.OCRLanguage,
		OCRBinary:                  p.OCRBinary,
		OCRToolsPath:               p.OCRToolsPath,
		OCRTelemetryEnabled:        p.OCRTelemetryEnabled,
		OCRTelemetryExporter:       p.OCRTelemetryExporter,
		OCRTelemetryOTLPEndpoint:   p.OCRTelemetryOTLPEndpoint,
		OCRTelemetryContentLogging: p.OCRTelemetryContentLogging,
		RulePackageIDs:             dataTypeJSON(p.RulePackageIDs),
	}
}

func projectPolicyToDomain(m models.ProjectPolicy) projectpolicy.ProjectPolicy {
	return projectpolicy.ProjectPolicy{
		Provider:                   m.Provider,
		Repo:                       m.Repo,
		FailOn:                     m.FailOn,
		IgnorePaths:                rawJSON(m.IgnorePaths),
		Rules:                      rawJSON(m.Rules),
		ReviewEngine:               m.ReviewEngine,
		GateMode:                   m.GateMode,
		OCREnabled:                 m.OCREnabled,
		OCRConcurrency:             m.OCRConcurrency,
		OCRTimeoutMinutes:          m.OCRTimeoutMinutes,
		OCRMaxTools:                m.OCRMaxTools,
		OCRDefaultSeverity:         m.OCRDefaultSeverity,
		OCRBackground:              m.OCRBackground,
		OCRBackgroundFile:          m.OCRBackgroundFile,
		OCRAudience:                m.OCRAudience,
		OCRProvider:                m.OCRProvider,
		OCRURL:                     m.OCRURL,
		OCRModel:                   m.OCRModel,
		OCRMaxGitProcs:             m.OCRMaxGitProcs,
		OCRUseAnthropic:            m.OCRUseAnthropic,
		OCRProtocol:                m.OCRProtocol,
		OCRExtraBody:               m.OCRExtraBody,
		OCRLanguage:                m.OCRLanguage,
		OCRBinary:                  m.OCRBinary,
		OCRToolsPath:               m.OCRToolsPath,
		OCRTelemetryEnabled:        m.OCRTelemetryEnabled,
		OCRTelemetryExporter:       m.OCRTelemetryExporter,
		OCRTelemetryOTLPEndpoint:   m.OCRTelemetryOTLPEndpoint,
		OCRTelemetryContentLogging: m.OCRTelemetryContentLogging,
		RulePackageIDs:             rawJSON(m.RulePackageIDs),
	}
}

// ---- session ----

func sessionToModel(s *session.Session) models.ReviewSession {
	return models.ReviewSession{
		SessionID:    s.SessionID,
		ReviewID:     s.ReviewID,
		Round:        s.Round,
		Phase:        s.Phase,
		Status:       s.Status,
		Progress:     s.Progress,
		AgentResults: dataTypeJSON(s.AgentResults),
		Discussion:   dataTypeJSON(s.Discussion),
		FinalReport:  dataTypeJSON(s.FinalReport),
		Requirements: dataTypeJSON(s.Requirements),
		Metadata:     dataTypeJSON(s.Metadata),
		CompletedAt:  s.CompletedAt,
	}
}

func sessionToDomain(m models.ReviewSession) session.Session {
	return session.Session{
		SessionID:    m.SessionID,
		ReviewID:     m.ReviewID,
		Round:        m.Round,
		Phase:        m.Phase,
		Status:       m.Status,
		Progress:     m.Progress,
		AgentResults: rawJSON(m.AgentResults),
		Discussion:   rawJSON(m.Discussion),
		FinalReport:  rawJSON(m.FinalReport),
		Requirements: rawJSON(m.Requirements),
		Metadata:     rawJSON(m.Metadata),
		CompletedAt:  m.CompletedAt,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

// ---- review (keystone) ----

// reviewToModel 把 domain.Record 转为持久化 model。
// Request/Response 强类型 marshal 为 JSONB 列；RawOCR/RawOCRInput 直传。
func reviewToModel(r *domain.Record) (models.Review, error) {
	reqJSON, err := json.Marshal(r.Request)
	if err != nil {
		return models.Review{}, fmt.Errorf("marshal request: %w", err)
	}
	respJSON, err := json.Marshal(r.Response)
	if err != nil {
		return models.Review{}, fmt.Errorf("marshal response: %w", err)
	}
	return models.Review{
		ReviewID:           r.ID,
		IdempotencyKey:     strPtr(r.IdempotencyKey),
		RequestFingerprint: r.RequestFingerprint,
		Provider:           r.Provider,
		Repo:               r.Repo,
		Status:             r.Status,
		Author:             r.Author,
		AuthorName:         r.AuthorName,
		AuthorAvatar:       r.AuthorAvatar,
		Sender:             r.Sender,
		PullRequestID:      r.PullRequestID,
		MergeRequestIID:    r.MergeRequestIID,
		BaseSHA:            r.BaseSHA,
		HeadSHA:            r.HeadSHA,
		Score:              r.Score,
		Grade:              r.Grade,
		Summary:            r.Summary,
		Request:            datatypes.JSON(reqJSON),
		Response:           datatypes.JSON(respJSON),
		RawOCR:             dataTypeJSON(r.RawOCR),
		RawOCRInput:        dataTypeJSON(r.RawOCRInput),
	}, nil
}

// reviewToDomain 把持久化 model 转为 domain.Record。
// Request/Response JSONB unmarshal 为强类型；失败时回退零值（兼容历史脏数据）。
func reviewToDomain(m models.Review) domain.Record {
	r := domain.Record{
		ID:                 m.ReviewID,
		RequestFingerprint: m.RequestFingerprint,
		Provider:           m.Provider,
		Repo:               m.Repo,
		Status:             m.Status,
		Author:             m.Author,
		AuthorName:         m.AuthorName,
		AuthorAvatar:       m.AuthorAvatar,
		Sender:             m.Sender,
		PullRequestID:      m.PullRequestID,
		MergeRequestIID:    m.MergeRequestIID,
		BaseSHA:            m.BaseSHA,
		HeadSHA:            m.HeadSHA,
		Score:              m.Score,
		Grade:              m.Grade,
		Summary:            m.Summary,
		RetryCount:         m.RetryCount,
		RawOCR:             rawJSON(m.RawOCR),
		RawOCRInput:        rawJSON(m.RawOCRInput),
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
	if m.IdempotencyKey != nil {
		r.IdempotencyKey = *m.IdempotencyKey
	}
	_ = json.Unmarshal(m.Request, &r.Request)
	_ = json.Unmarshal(m.Response, &r.Response)
	return r
}

// strPtr 返回非空字符串的指针，空串返回 nil（供 nullable 列）。
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
