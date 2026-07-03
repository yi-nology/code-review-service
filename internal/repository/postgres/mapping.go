package postgres

import (
	"encoding/json"

	"github.com/yi-nology/code-review-service/internal/domain/feedback"
	"github.com/yi-nology/code-review-service/internal/domain/llmprovider"
	"github.com/yi-nology/code-review-service/internal/domain/ocrasset"
	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
	"github.com/yi-nology/code-review-service/internal/domain/platform"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/domain/reviewer"
	"github.com/yi-nology/code-review-service/internal/domain/revision"
	"github.com/yi-nology/code-review-service/internal/domain/rules"
	domainWebhook "github.com/yi-nology/code-review-service/internal/domain/webhook"
	"github.com/yi-nology/code-review-service/internal/domain/writeback"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

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

// dataTypeJSON 将 json.RawMessage 转为 datatypes.JSON。
func dataTypeJSON(r json.RawMessage) datatypes.JSON {
	if len(r) == 0 {
		return nil
	}
	return datatypes.JSON(r)
}

// ---- feedback ----

func feedbackToModel(f *feedback.Feedback) models.FindingFeedback {
	return models.FindingFeedback{
		ReviewID:     f.ReviewID,
		FindingIndex: f.FindingIndex,
		Status:       f.Status,
		Note:         f.Note,
		UpdatedAt:    f.UpdatedAt,
	}
}

func feedbackToDomain(m models.FindingFeedback) feedback.Feedback {
	return feedback.Feedback{
		ReviewID:     m.ReviewID,
		FindingIndex: m.FindingIndex,
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

func revisionToModel(c *revision.Change) models.ChangeRevision {
	return models.ChangeRevision{
		Provider:       c.Provider,
		InstallationID: c.InstallationID,
		RepositoryID:   c.RepositoryID,
		Repository:     c.Repository,
		ChangeNumber:   c.ChangeNumber,
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
		ChangeNumber:   m.ChangeNumber,
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
		Model:      gormModel(a.ID),
		Name:        a.Name,
		DisplayName: a.DisplayName,
		Description: a.Description,
		Background:  a.Background,
		RuleFile:    a.RuleFile,
		Redundancy:  a.Redundancy,
		Enabled:     a.Enabled,
		Priority:    a.Priority,
		IsDefault:   a.IsDefault,
		Tags:        dataTypeJSON(a.Tags),
	}
}

func reviewerToDomain(m models.ReviewerAgent) reviewer.Agent {
	return reviewer.Agent{
		ID:          m.ID,
		Name:        m.Name,
		DisplayName: m.DisplayName,
		Description: m.Description,
		Background:  m.Background,
		RuleFile:    m.RuleFile,
		Redundancy:  m.Redundancy,
		Enabled:     m.Enabled,
		Priority:    m.Priority,
		IsDefault:   m.IsDefault,
		Tags:        rawJSON(m.Tags),
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
		Model:     gormModel(c.ID),
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

func deliveryToModel(d *domainWebhook.Delivery) models.WebhookDelivery {	return models.WebhookDelivery{
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
		TelemetryEnabled:        s.TelemetryEnabled,
		TelemetryExporter:       s.TelemetryExporter,
		TelemetryOTLPEndpoint:   s.TelemetryOTLPEndpoint,
		TelemetryContentLogging: s.TelemetryContentLogging,
		ContextWindow:           s.ContextWindow,
		MaxOutputTokens:         s.MaxOutputTokens,
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
		TelemetryEnabled:        m.TelemetryEnabled,
		TelemetryExporter:       m.TelemetryExporter,
		TelemetryOTLPEndpoint:   m.TelemetryOTLPEndpoint,
		TelemetryContentLogging: m.TelemetryContentLogging,
		ContextWindow:           m.ContextWindow,
		MaxOutputTokens:         m.MaxOutputTokens,
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
		OCREnabled:                 p.OCREnabled,
		OCRConcurrency:             p.OCRConcurrency,
		OCRTimeoutMinutes:          p.OCRTimeoutMinutes,
		OCRMaxTools:                p.OCRMaxTools,
		OCRDefaultSeverity:         p.OCRDefaultSeverity,
		OCRBackground:              p.OCRBackground,
		OCRAudience:                p.OCRAudience,
		OCRProvider:                p.OCRProvider,
		OCRURL:                     p.OCRURL,
		OCRModel:                   p.OCRModel,
		OCRMaxGitProcs:             p.OCRMaxGitProcs,
		OCRUseAnthropic:            p.OCRUseAnthropic,
		OCRExtraBody:               p.OCRExtraBody,
		OCRLanguage:                p.OCRLanguage,
		OCRBinary:                  p.OCRBinary,
		OCRToolsPath:               p.OCRToolsPath,
		OCRTelemetryEnabled:        p.OCRTelemetryEnabled,
		OCRTelemetryExporter:       p.OCRTelemetryExporter,
		OCRTelemetryOTLPEndpoint:   p.OCRTelemetryOTLPEndpoint,
		OCRTelemetryContentLogging: p.OCRTelemetryContentLogging,
	}
}

func projectPolicyToDomain(m models.ProjectPolicy) projectpolicy.ProjectPolicy {
	return projectpolicy.ProjectPolicy{
		Provider:                   m.Provider,
		Repo:                       m.Repo,
		FailOn:                     m.FailOn,
		IgnorePaths:                rawJSON(m.IgnorePaths),
		Rules:                      rawJSON(m.Rules),
		OCREnabled:                 m.OCREnabled,
		OCRConcurrency:             m.OCRConcurrency,
		OCRTimeoutMinutes:          m.OCRTimeoutMinutes,
		OCRMaxTools:                m.OCRMaxTools,
		OCRDefaultSeverity:         m.OCRDefaultSeverity,
		OCRBackground:              m.OCRBackground,
		OCRAudience:                m.OCRAudience,
		OCRProvider:                m.OCRProvider,
		OCRURL:                     m.OCRURL,
		OCRModel:                   m.OCRModel,
		OCRMaxGitProcs:             m.OCRMaxGitProcs,
		OCRUseAnthropic:            m.OCRUseAnthropic,
		OCRExtraBody:               m.OCRExtraBody,
		OCRLanguage:                m.OCRLanguage,
		OCRBinary:                  m.OCRBinary,
		OCRToolsPath:               m.OCRToolsPath,
		OCRTelemetryEnabled:        m.OCRTelemetryEnabled,
		OCRTelemetryExporter:       m.OCRTelemetryExporter,
		OCRTelemetryOTLPEndpoint:   m.OCRTelemetryOTLPEndpoint,
		OCRTelemetryContentLogging: m.OCRTelemetryContentLogging,
	}
}
