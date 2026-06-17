package modules

import (
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/runner"

	"go.uber.org/fx"
)

var RunnerModule = fx.Options(
	fx.Provide(NewOpenCodeReviewRunner),
	fx.Provide(NewDynamicOpenCodeReviewRunner),
	fx.Provide(NewConfigFileWriter),
)

func NewOpenCodeReviewRunner(cfg *config.Config) *runner.OpenCodeReviewRunner {
	return runner.NewOpenCodeReviewRunner(runner.OpenCodeReviewConfig{
		Binary:          cfg.OCR.Binary,
		Audience:        "human",
		RulePath:        cfg.OCR.Rule,
		DefaultSeverity: "major",
	})
}

func NewDynamicOpenCodeReviewRunner(
	fallback *runner.OpenCodeReviewRunner,
	repos *repository.RepoContainer,
) *runner.DynamicOpenCodeReviewRunner {
	return runner.NewDynamicOpenCodeReviewRunner(fallback, repos.OCRSettings, repos.GlobalRules)
}

func NewConfigFileWriter(repos *repository.RepoContainer, cfg *config.Config) *runner.ConfigFileWriter {
	return runner.NewConfigFileWriter(repos.OCRSettings, repos.CustomProvider, cfg.OCR.ConfigFile)
}
