package modules

import (
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/ocr"
	"github.com/yi-nology/code-review-service/internal/service/runner"

	"go.uber.org/fx"
)

var OCRModule = fx.Options(
	fx.Provide(NewConfigResolver),
	fx.Provide(NewOCRTester),
	fx.Provide(NewOCRProviderLister),
	fx.Provide(NewOCRRulesChecker),
	fx.Provide(NewOCRVersioner),
	fx.Provide(NewOCRViewerManager),
)

func NewConfigResolver(repos *repository.RepoContainer, fallback *runner.OpenCodeReviewRunner) *ocr.ConfigResolver {
	return ocr.NewConfigResolver(repos.OCRSettings, fallback.Config())
}

func NewOCRTester(repos *repository.RepoContainer, resolver *ocr.ConfigResolver) *ocr.Tester {
	return ocr.NewTester(repos.OCRSettings, resolver)
}

func NewOCRProviderLister(repos *repository.RepoContainer, resolver *ocr.ConfigResolver) *ocr.ProviderLister {
	return ocr.NewProviderLister(repos.OCRSettings, resolver)
}

func NewOCRRulesChecker(repos *repository.RepoContainer, resolver *ocr.ConfigResolver) *ocr.RulesChecker {
	return ocr.NewRulesChecker(repos.OCRSettings, repos.GlobalRules, resolver)
}

func NewOCRVersioner(repos *repository.RepoContainer, resolver *ocr.ConfigResolver) *ocr.Versioner {
	return ocr.NewVersioner(repos.OCRSettings, resolver)
}

func NewOCRViewerManager(repos *repository.RepoContainer, resolver *ocr.ConfigResolver) *ocr.ViewerManager {
	return ocr.NewViewerManager(repos.OCRSettings, resolver)
}
