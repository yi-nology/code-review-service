package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	domainplatform "github.com/yi-nology/code-review-service/internal/domain/platform"
	"github.com/yi-nology/code-review-service/internal/vcs"
	"github.com/oekit/logx"

	"github.com/yi-nology/git-platform-sdk/provider"
)

type repoCacheEntry struct {
	repos     []*provider.PlatformRepo
	timestamp time.Time
}

type GitProviderService struct {
	mu        sync.RWMutex
	providers map[string]provider.Provider
	configs   map[string]provider.Config

	repoCacheMu sync.RWMutex
	repoCache   map[string]repoCacheEntry
}

func NewGitProviderService(configs map[string]provider.Config) (*GitProviderService, error) {
	providers := make(map[string]provider.Provider)
	if configs == nil {
		configs = make(map[string]provider.Config)
	}

	for name, cfg := range configs {
		p, err := provider.NewProvider(cfg)
		if err != nil {
			return nil, fmt.Errorf("init provider %s: %w", name, err)
		}
		providers[name] = p
	}

	return &GitProviderService{providers: providers, configs: configs, repoCache: make(map[string]repoCacheEntry)}, nil
}

func (s *GitProviderService) AddOrUpdate(name string, cfg provider.Config) error {
	p, err := provider.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("init provider %s: %w", name, err)
	}
	s.mu.Lock()
	s.providers[name] = p
	s.configs[name] = cfg
	s.mu.Unlock()

	// 平台配置变更时清除缓存
	s.repoCacheMu.Lock()
	delete(s.repoCache, name+":all")
	s.repoCacheMu.Unlock()

	return nil
}

func (s *GitProviderService) Remove(name string) {
	s.mu.Lock()
	delete(s.providers, name)
	delete(s.configs, name)
	s.mu.Unlock()

	s.repoCacheMu.Lock()
	delete(s.repoCache, name+":all")
	s.repoCacheMu.Unlock()
}

func (s *GitProviderService) Reload(configs map[string]provider.Config) error {
	newProviders := make(map[string]provider.Provider, len(configs))
	for name, cfg := range configs {
		p, err := provider.NewProvider(cfg)
		if err != nil {
			return fmt.Errorf("init provider %s: %w", name, err)
		}
		newProviders[name] = p
	}
	s.mu.Lock()
	s.providers = newProviders
	s.mu.Unlock()

	// 重载时清除所有缓存
	s.repoCacheMu.Lock()
	s.repoCache = make(map[string]repoCacheEntry)
	s.repoCacheMu.Unlock()

	return nil
}

func (s *GitProviderService) GetProvider(platform string) (provider.Provider, error) {
	s.mu.RLock()
	p, ok := s.providers[platform]
	s.mu.RUnlock()
	if !ok {
		s.mu.RLock()
		available := make([]string, 0, len(s.providers))
		for k := range s.providers {
			available = append(available, k)
		}
		s.mu.RUnlock()
		logx.Errorf("GetProvider failed: platform=%q available=%v", platform, available)
		return nil, fmt.Errorf("unsupported provider: %s (available: %v)", platform, available)
	}
	return p, nil
}

func (s *GitProviderService) DetectAndGetProvider(remoteURL string) (provider.Provider, *provider.DetectResult, error) {
	result, err := provider.DetectPlatform(remoteURL)
	if err != nil {
		return nil, nil, err
	}

	p, err := s.GetProvider(string(result.Platform))
	if err != nil {
		return nil, nil, err
	}

	return p, result, nil
}

func (s *GitProviderService) GetCRDiff(ctx context.Context, platform, owner, repo, number string) (*provider.MergeDiff, error) {
	p, err := s.GetProvider(platform)
	if err != nil {
		return nil, err
	}
	return p.GetCRDiff(ctx, owner, repo, number)
}

func (s *GitProviderService) GetCRFiles(ctx context.Context, platform, owner, repo, number string) ([]*provider.ChangedFile, error) {
	p, err := s.GetProvider(platform)
	if err != nil {
		return nil, err
	}
	return p.GetCRFiles(ctx, owner, repo, number)
}

func (s *GitProviderService) CreateCommitStatus(ctx context.Context, platform, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	p, err := s.GetProvider(platform)
	if err != nil {
		return err
	}
	// SDK v0.48 起 CommitStatusManager 是可选能力接口（如 Gitee 无 commit-status
	// 端点），不再挂在 Provider 上，需断言。
	csm, ok := p.(provider.CommitStatusManager)
	if !ok {
		return fmt.Errorf("platform %s does not support commit statuses", platform)
	}
	return csm.CreateCommitStatus(ctx, owner, repo, sha, opts)
}

func (s *GitProviderService) GetCR(ctx context.Context, platform, owner, repo, number string) (*provider.ChangeRequest, error) {
	p, err := s.GetProvider(platform)
	if err != nil {
		return nil, err
	}
	return p.GetCR(ctx, owner, repo, number)
}

func (s *GitProviderService) CreateCR(ctx context.Context, platform string, opts provider.CreateCROptions) (*provider.ChangeRequest, error) {
	p, err := s.GetProvider(platform)
	if err != nil {
		return nil, err
	}
	return p.CreateCR(ctx, opts)
}

func (s *GitProviderService) TestConnection(ctx context.Context, platform string) (*provider.TestConnectionResult, error) {
	p, err := s.GetProvider(platform)
	if err != nil {
		return nil, err
	}
	return p.TestConnection(ctx)
}

// ParseRepoPath 实现已下沉到 internal/vcs（中立包，供 writeback/statuscheck 等共用）；
// 保留导出包装以维持既有调用点兼容。
func ParseRepoPath(repoPath string) (string, string, error) { return vcs.ParseRepoPath(repoPath) }

func (s *GitProviderService) HasProvider(platform string) bool {
	s.mu.RLock()
	_, ok := s.providers[platform]
	s.mu.RUnlock()
	return ok
}

func (s *GitProviderService) GetCRDiffString(ctx context.Context, platform, owner, repo, number string) (string, error) {
	diff, err := s.GetCRDiff(ctx, platform, owner, repo, number)
	if err != nil {
		return "", err
	}
	return diff.RawDiff, nil
}

func (s *GitProviderService) BuildCloneURL(platform, owner, repo string) string {
	s.mu.RLock()
	cfg, ok := s.configs[platform]
	s.mu.RUnlock()
	if !ok {
		return ""
	}

	base := strings.TrimRight(cfg.BaseURL, "/")
	switch strings.ToLower(platform) {
	case "gitea":
		base = strings.TrimSuffix(base, "/api/v1")
	case "gitcode":
		base = strings.TrimSuffix(base, "/api/v5")
		base = strings.Replace(base, "://api.gitcode.com", "://gitcode.com", 1)
	case "gitlab":
		base = strings.TrimSuffix(base, "/api/v4")
	case "github":
		base = strings.TrimSuffix(base, "/api/v3")
		base = strings.Replace(base, "://api.github.com", "://github.com", 1)
	case "gerrit":
		// Gerrit 认证 clone 使用 /a/ 前缀。
		// 不 strip API path（Gerrit REST API 路径就是根路径）。
		// Gerrit 的 repo 参数是完整 project name（owner 为空），
		// 需要提前返回，避免走通用的 /%s/%s.git 格式产生双斜杠。
		if !strings.HasSuffix(base, "/a") && !strings.Contains(base, "/a/") {
			base = strings.TrimSuffix(base, "/") + "/a"
		}
		if base == "" {
			return ""
		}
		return base + "/" + repo + ".git"
	}

	if base == "" {
		return ""
	}

	return base + fmt.Sprintf("/%s/%s.git", owner, repo)
}

func (s *GitProviderService) GetToken(platform string) string {
	s.mu.RLock()
	cfg, ok := s.configs[platform]
	s.mu.RUnlock()
	if !ok {
		return ""
	}
	return cfg.Token
}

// GetSkipTLS 返回该平台是否需要跳过 TLS 校验（自签名证书场景）。
// 仅对该平台的 git 子进程生效，不污染全局环境。
func (s *GitProviderService) GetSkipTLS(platform string) bool {
	s.mu.RLock()
	cfg, ok := s.configs[platform]
	s.mu.RUnlock()
	return ok && cfg.SkipTLS
}

func (s *GitProviderService) ListRepos(ctx context.Context, platform string, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error) {
	p, err := s.GetProvider(platform)
	if err != nil {
		return nil, err
	}

	// 首页缓存（page=1, perPage=100），减少前端首次加载延迟
	if opts.Page == 1 && opts.PerPage >= 100 {
		cacheKey := platform + ":all"
		s.repoCacheMu.RLock()
		cached, ok := s.repoCache[cacheKey]
		s.repoCacheMu.RUnlock()
		if ok && time.Since(cached.timestamp) < 5*time.Minute {
			return cached.repos, nil
		}

		// 收集所有页
		var allRepos []*provider.PlatformRepo
		page := 1
		for {
			batch, err := p.ListRepos(ctx, provider.ListRepoOptions{Page: page, PerPage: 100})
			if err != nil {
				return nil, err
			}
			allRepos = append(allRepos, batch...)
			if len(batch) < 100 {
				break
			}
			page++
		}

		s.repoCacheMu.Lock()
		s.repoCache[cacheKey] = repoCacheEntry{repos: allRepos, timestamp: time.Now()}
		s.repoCacheMu.Unlock()

		logx.Infof("[provider] cached %d repos for platform %s", len(allRepos), platform)
		return allRepos, nil
	}

	return p.ListRepos(ctx, opts)
}

func (s *GitProviderService) ListBranches(ctx context.Context, platform, owner, repo string) ([]*provider.PlatformBranch, error) {
	p, err := s.GetProvider(platform)
	if err != nil {
		return nil, err
	}
	return p.ListBranches(ctx, owner, repo)
}

func (s *GitProviderService) ListCRs(ctx context.Context, platform string, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error) {
	p, err := s.GetProvider(platform)
	if err != nil {
		return nil, 0, err
	}
	return p.ListCRs(ctx, opts)
}

func (s *GitProviderService) GetBranchSHA(ctx context.Context, platform, owner, repo, branch string) (string, error) {
	p, err := s.GetProvider(platform)
	if err != nil {
		return "", err
	}
	commits, err := p.ListCommits(ctx, owner, repo, provider.ListCommitsOptions{
		Branch:  branch,
		Page:    1,
		PerPage: 1,
	})
	if err != nil {
		return "", err
	}
	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found for branch %s", branch)
	}
	return commits[0].SHA, nil
}

func (s *GitProviderService) GetSupportedPlatforms() []string {
	s.mu.RLock()
	platforms := make([]string, 0, len(s.providers))
	for name := range s.providers {
		platforms = append(platforms, name)
	}
	s.mu.RUnlock()
	return platforms
}

func (s *GitProviderService) AddOrUpdateFromDB(p *domainplatform.Platform) error {
	// SkipTLS 通过 gitbackend 的 InsecureSkipTLS 选项 per-request 传递给 git 子进程，
	// 不再污染全局 os.Environ（历史上曾用 os.Setenv("GIT_SSL_NO_VERIFY") 导致
	// 配过一个自签名平台后所有平台 clone 都跳过 TLS 校验）。
	cfg := provider.Config{
		Platform: provider.Platform(p.Provider),
		BaseURL:  p.BaseURL,
		Token:    p.Token,
		SkipTLS:  p.SkipTLS,
	}
	return s.AddOrUpdate(p.Provider, cfg)
}

func (s *GitProviderService) TestWithToken(ctx context.Context, platform, baseURL, token string, skipTLS bool) (*domainplatform.TestResult, error) {
	cfg := provider.Config{
		Platform: provider.Platform(platform),
		BaseURL:  baseURL,
		Token:    token,
		SkipTLS:  skipTLS,
	}
	p, err := provider.NewProvider(cfg)
	if err != nil {
		return &domainplatform.TestResult{Connected: false, Platform: platform, Message: fmt.Sprintf("初始化失败: %v", err)}, nil
	}
	result, connErr := p.TestConnection(ctx)
	if connErr != nil {
		//nolint:nilerr // Connection test failures are represented in the response payload.
		return &domainplatform.TestResult{Connected: false, Platform: platform, Message: connErr.Error()}, nil
	}
	return &domainplatform.TestResult{
		Connected: result.Connected,
		Platform:  platform,
		UserName:  result.UserName,
		Message:   result.Message,
	}, nil
}
