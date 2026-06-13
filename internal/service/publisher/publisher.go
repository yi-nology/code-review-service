package publisher

import (
	"context"
	"fmt"
	"strings"

	"github.com/oekit/pkg"
	"review-service/internal/service/provider"
	"review-service/internal/service/review"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

const reportMarker = "<!-- review-service:summary -->"

type Config struct {
	Enabled     bool
	MaxComments int
	SummaryOnly bool
}

type Publisher struct {
	config Config
	gitSvc *provider.GitProviderService
}

func New(config Config, gitSvc *provider.GitProviderService) *Publisher {
	if config.MaxComments <= 0 {
		config.MaxComments = 20
	}
	return &Publisher{
		config: config,
		gitSvc: gitSvc,
	}
}

func (p *Publisher) Publish(ctx context.Context, req review.Request, result review.Result) error {
	if !p.config.Enabled {
		hlog.Info(
			"review comment publishing disabled",
			"provider", req.Provider,
			"repo", req.Repo,
			"findings", len(result.Findings),
		)
		return nil
	}

	if p.gitSvc == nil || !p.gitSvc.HasProvider(req.Provider) {
		return fmt.Errorf("unsupported comment provider %q: no git-platform-sdk provider configured", req.Provider)
	}

	body := FormatReport(result, p.config.MaxComments, p.config.SummaryOnly)
	return p.publishViaSDK(ctx, req, body)
}

func (p *Publisher) publishViaSDK(ctx context.Context, req review.Request, body string) error {
	owner, repo, err := provider.ParseRepoPath(req.Repo)
	if err != nil {
		return fmt.Errorf("parse repo path: %w", err)
	}

	var number int
	switch strings.ToLower(req.Provider) {
	case "github", "gitea", "gitcode", "forgejo", "gitee", "tencent_code":
		if req.PullRequestID != "" {
			_, err = fmt.Sscanf(strings.TrimSpace(req.PullRequestID), "%d", &number)
		} else {
			_, err = fmt.Sscanf(strings.TrimSpace(req.MergeRequestIID), "%d", &number)
		}
	case "gitlab":
		_, err = fmt.Sscanf(strings.TrimSpace(req.MergeRequestIID), "%d", &number)
		if number == 0 && req.PullRequestID != "" {
			_, err = fmt.Sscanf(strings.TrimSpace(req.PullRequestID), "%d", &number)
		}
	}
	if err != nil || number <= 0 {
		return fmt.Errorf("invalid PR/MR number: pull_request_id=%q merge_request_iid=%q", req.PullRequestID, req.MergeRequestIID)
	}

	_, err = p.gitSvc.CreateComment(ctx, req.Provider, owner, repo, number, body)
	if err != nil {
		hlog.Error("failed to publish comment via SDK",
			"provider", req.Provider,
			"repo", req.Repo,
			"error", err,
		)
		return err
	}

	hlog.Info("published comment via SDK",
		"provider", req.Provider,
		"repo", req.Repo,
		"number", number,
	)
	return nil
}

func FormatReport(result review.Result, maxComments int, summaryOnly bool) string {
	var out strings.Builder
	out.WriteString(reportMarker)
	out.WriteString("\n## AI Code Review\n\n")
	out.WriteString(result.Summary)
	out.WriteString("\n\n")

	counts := severityCounts(result.Findings)
	fmt.Fprintf(
		&out,
		"**Findings:** %d critical, %d major, %d minor, %d info\n",
		counts[review.SeverityCritical],
		counts[review.SeverityMajor],
		counts[review.SeverityMinor],
		counts[review.SeverityInfo],
	)

	if summaryOnly || len(result.Findings) == 0 {
		return out.String()
	}

	limit := len(result.Findings)
	if maxComments > 0 && limit > maxComments {
		limit = maxComments
	}
	out.WriteString("\n### Findings\n")
	for index, finding := range result.Findings[:limit] {
		fmt.Fprintf(&out, "\n%d. **[%s] %s**", index+1, strings.ToUpper(string(finding.Severity)), finding.Title)
		if finding.Path != "" {
			fmt.Fprintf(&out, " - `%s", finding.Path)
			if finding.Line > 0 {
				fmt.Fprintf(&out, ":%d", finding.Line)
			}
			out.WriteString("`")
		}
		if strings.TrimSpace(finding.Body) != "" && strings.TrimSpace(finding.Body) != strings.TrimSpace(finding.Title) {
			out.WriteString("\n\n   ")
			out.WriteString(indent(strings.TrimSpace(finding.Body), "   "))
		}
		if strings.TrimSpace(finding.ExistingCode) != "" {
			out.WriteString("\n\n   **Existing code**\n\n   ```text\n   ")
			out.WriteString(indent(strings.TrimSpace(finding.ExistingCode), "   "))
			out.WriteString("\n   ```")
		}
		if strings.TrimSpace(finding.SuggestionCode) != "" {
			out.WriteString("\n\n   **Suggested code**\n\n   ```text\n   ")
			out.WriteString(indent(strings.TrimSpace(finding.SuggestionCode), "   "))
			out.WriteString("\n   ```")
		}
		out.WriteString("\n")
	}
	if limit < len(result.Findings) {
		fmt.Fprintf(&out, "\n_%d additional finding(s) omitted by policy._\n", len(result.Findings)-limit)
	}
	return out.String()
}

func severityCounts(findings []review.Finding) map[review.Severity]int {
	grouped := slice.GroupBy(findings, func(f review.Finding) review.Severity { return f.Severity })
	counts := make(map[review.Severity]int, len(grouped))
	for sev, items := range grouped {
		counts[sev] = len(items)
	}
	return counts
}

func indent(value, prefix string) string {
	return strings.ReplaceAll(value, "\n", "\n"+prefix)
}
