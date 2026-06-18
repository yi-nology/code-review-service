package consts

const (
	ProviderGitHub      = "github"
	ProviderGitLab      = "gitlab"
	ProviderGitea       = "gitea"
	ProviderGitCode     = "gitcode"
	ProviderTencentCode = "tencent_code"
)

var SupportedProviders = []string{
	ProviderGitHub,
	ProviderGitLab,
	ProviderGitea,
	ProviderGitCode,
	ProviderTencentCode,
}
