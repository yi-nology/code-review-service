package review

import "context"

type Providers interface {
	Get(name string) (Provider, bool)
}

type Provider interface {
	Prepare(context.Context, Request) (Workspace, error)
}

type GitProvider interface {
	GetCRDiff(ctx context.Context, platform, owner, repo string, number int) (string, error)
	CreateComment(ctx context.Context, platform, owner, repo string, number int, body string) (string, error)
	HasProvider(platform string) bool
}

type Runner interface {
	Run(context.Context, Workspace) (Result, error)
}

type WritebackPublisher interface {
	PublishWithDiff(ctx context.Context, req Request, result Result, diff string) error
}

type PreviewRunner interface {
	Preview(ctx context.Context, workspace Workspace) (*PreviewResult, error)
}

type LLMTester interface {
	TestLLM(ctx context.Context) (*LLMTestResult, error)
}

type LLMProviderLister interface {
	ListLLMProviders(ctx context.Context) ([]LLMProvider, error)
}

type RulesChecker interface {
	RulesCheck(ctx context.Context, path string) (*RulesCheckResult, error)
}

type OCRVersioner interface {
	OCRVersion(ctx context.Context) (*OCRVersion, error)
}

type ViewerManager interface {
	ViewerStatus(ctx context.Context) (*ViewerStatus, error)
	StartViewer(ctx context.Context, addr string) (*ViewerStatus, error)
	StopViewer(ctx context.Context) (*ViewerStatus, error)
}
