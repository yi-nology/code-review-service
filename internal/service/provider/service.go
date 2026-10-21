package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/oekit/logx"
	"github.com/oekit/pkg/cache"
	domainplatform "github.com/yi-nology/code-review-service/internal/domain/platform"
	"github.com/yi-nology/code-review-service/internal/vcs"

	"github.com/yi-nology/go-git-platform/provider"
)

type GitProviderService struct {
	mu        sync.RWMutex
	providers map[string]provider.Provider
	configs   map[string]provider.Config

	// repoCache 是首页仓库列表的缓存（Cache-Aside）。
	//
	// 此前是手搓的 map + 5 分钟 TTL：没有并发回源合并——缓存过期瞬间 N 个并发
	// 请求会各自跑一遍「翻完所有页」，等于对平台 API 打 N 倍请求。
	// 现在用 oekit/pkg/cache：LocalCache 提供 TTL/空值防穿透/回源失败回退陈旧值，
	// 外层 SingleFlightCache 装饰器提供并发回源合并（同一 key 只回源一次）。
	//
	// repoCacheInner 单独持有是为了拿到 Clear/Close（Cache 接口面上没有这两个）。
	repoCache      *cache.SingleFlightCache[[]*provider.PlatformRepo]
	repoCacheInner *cache.LocalCache[[]*provider.PlatformRepo]
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

	s := &GitProviderService{providers: providers, configs: configs}
	s.repoCacheInner = newRepoCache(s)
	s.repoCache = newSingleFlightRepoCache(s.repoCacheInner)
	return s, nil
}

// newRepoCache 构造首页仓库列表的本地缓存。
func newRepoCache(s *GitProviderService) *cache.LocalCache[[]*provider.PlatformRepo] {
	return cache.NewLocalCache(cache.LocalConfig[[]*provider.PlatformRepo]{
		TTL:     repoCacheTTL,
		MaxSize: repoCacheMaxSize,
		// 回源失败时回退已过期的历史值（fail-open）：平台 API 抖一下不该让
		// 首页变成「没有仓库」。
		StaleIfError: true,
		Source:       s.loadRepos,
	})
}

// newSingleFlightRepoCache 给本地缓存套上并发回源合并：
// 同一 key 的并发未命中只回源一次（防止缓存击穿时对平台 API 放大请求）。
func newSingleFlightRepoCache(inner *cache.LocalCache[[]*provider.PlatformRepo]) *cache.SingleFlightCache[[]*provider.PlatformRepo] {
	return cache.NewSingleFlightCache(inner)
}

// repoCacheKeySuffix 是仓库列表缓存 key 的固定后缀（首页只取全量这一种视图）。
const repoCacheKeySuffix = ":all"

// repoCacheTTL 是首页仓库列表的缓存时长。
const repoCacheTTL = 5 * time.Minute

// repoCacheMaxSize 是缓存条目上限（平台数量有限，留足余量即可）。
const repoCacheMaxSize = 16

// loadRepos 是缓存的回源函数：按 key 里的平台名收集全��仓库。
//
// key 形如 "github:all"——平台名 + 固定后缀（首页只取全量这一种视图）。
func (s *GitProviderService) loadRepos(ctx context.Context, key string) (*[]*provider.PlatformRepo, error) {
	platform := strings.TrimSuffix(key, repoCacheKeySuffix)
	p, err := s.GetProvider(platform)
	if err != nil {
		return nil, err
	}
	var all []*provider.PlatformRepo
	page := 1
	for {
		batch, err := p.ListRepos(ctx, provider.ListRepoOptions{Page: page, PerPage: 100})
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
		page++
	}
	logx.Infof("[provider] cached %d repos for platform %s", len(all), platform)
	return &all, nil
}

// Close 停止缓存的后台清扫协程（LocalCache 在 TTL>0 时会起一个）。
//
// 由 fx 的 OnStop 钩子调用（见 internal/modules/provider.go）。
func (s *GitProviderService) Close(ctx context.Context) error {
	if s.repoCacheInner == nil {
		return nil
	}
	return s.repoCacheInner.Close(ctx)
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
	s.invalidateRepoCache(name)

	return nil
}

func (s *GitProviderService) Remove(name string) {
	s.mu.Lock()
	delete(s.providers, name)
	delete(s.configs, name)
	s.mu.Unlock()

	s.invalidateRepoCache(name)
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
	s.repoCacheInner.Clear()

	return nil
}

// invalidateRepoCache 清除某平台的仓库列表缓存。
func (s *GitProviderService) invalidateRepoCache(platform string) {
	if s.repoCache == nil {
		return
	}
	_ = s.repoCache.Del(context.Background(), platform+repoCacheKeySuffix)
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

// ValidateCloneURL 校验拼出的 clone URL 可否安全出站。
//
// owner/repo 来自 webhook 载荷（外部可控），未校验就拼进 URL 会有两个后果：
// host 逃逸（owner 含 "@" 时 URL 的 host 变成攻击者的地址，服务带着 git token
// 去访问它）与路径穿越（owner 含 ".." 时指向平台上的另一个路径）。
//
// 判据在 vcs.CloneURLGuard（底座 netguard）：拼出的 host 必须与该平台
// 配置的 base URL 的 host 完全一致，且通过出站校验（拒回环）。
func (s *GitProviderService) ValidateCloneURL(platform, owner, repo, cloneURL string) error {
	if err := vcs.ValidateRepoPath(owner, repo); err != nil {
		return err
	}

	s.mu.RLock()
	cfg, ok := s.configs[platform]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("平台 %q 未配置", platform)
	}

	// 基地址取**未做平台路径改写**的原始值：host 比较只关心 host，
	// 而改写只动路径部分（/api/v4 之类），host 不变。
	return vcs.NewCloneURLGuard().Validate(cfg.BaseURL, cloneURL)
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

	// 首页缓存（page=1, perPage=100），减少前端首次加载延迟。
	//
	// GetOrLoad 是 Cache-Aside：未命中回源并回填；并发未命中由 singleflight
	// 合并成一次回源（此前 N 个并发请求会各翻一遍全部页）。
	if opts.Page == 1 && opts.PerPage >= 100 {
		repos, err := s.repoCache.GetOrLoad(ctx, platform+repoCacheKeySuffix)
		if err != nil {
			return nil, err
		}
		if repos != nil {
			return *repos, nil
		}
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
	// go-git-platform 的 total 为 *int（平台不支持时 nil），服务契约收窄为 int
	crs, total, err := p.ListCRs(ctx, opts)
	if err != nil {
		return nil, 0, err
	}
	if total != nil {
		return crs, *total, nil
	}
	return crs, 0, nil
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
