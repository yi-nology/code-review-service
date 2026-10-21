package provider

import (
	"context"
	"sync"
	"testing"

	"github.com/yi-nology/go-git-platform/provider"
)

// fakePlatform 是测试用的假平台标识。
const fakePlatform provider.Platform = "faketest"

// fakePlatformProvider 是 provider.Provider 的替身。
//
// 手法：内嵌接口拿到全部方法，只实现本用例用到的那一个——其余方法若被调用
// 会因内嵌的 nil 接口而 panic（这正是「不该被调用」的信号）。
type fakePlatformProvider struct {
	provider.Provider

	mu    sync.Mutex
	calls int
	repos []*provider.PlatformRepo
}

func (f *fakePlatformProvider) ListRepos(_ context.Context, _ provider.ListRepoOptions) ([]*provider.PlatformRepo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.repos, nil
}

func (f *fakePlatformProvider) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fakeProviderInstance 是注册到 SDK 平台注册表的假平台实例。
//
// AddOrUpdate 走 provider.NewProvider（要求平台已注册），所以要注册一个。
var fakeProviderInstance *fakePlatformProvider

func init() {
	provider.Register(fakePlatform, func(provider.Config) (provider.Provider, error) {
		return fakeProviderInstance, nil
	})
}

// newCacheTestService 造一个只含假平台的服务（缓存构造与生产同一份）。
func newCacheTestService(fake *fakePlatformProvider) *GitProviderService {
	fakeProviderInstance = fake
	s := &GitProviderService{
		providers: map[string]provider.Provider{string(fakePlatform): fake},
		configs:   map[string]provider.Config{string(fakePlatform): {Platform: fakePlatform}},
	}
	s.repoCacheInner = newRepoCache(s)
	s.repoCache = newSingleFlightRepoCache(s.repoCacheInner)
	return s
}

// TestListRepos_CachesFirstPage 验证首页（全量视图）被缓存：第二次调用
// 不再打平台 API。
func TestListRepos_CachesFirstPage(t *testing.T) {
	fake := &fakePlatformProvider{repos: []*provider.PlatformRepo{{FullName: "o/r"}}}
	s := newCacheTestService(fake)
	defer func() { _ = s.Close(context.Background()) }()

	opts := provider.ListRepoOptions{Page: 1, PerPage: 100}
	for i := 0; i < 3; i++ {
		repos, err := s.ListRepos(context.Background(), string(fakePlatform), opts)
		if err != nil {
			t.Fatalf("ListRepos: %v", err)
		}
		if len(repos) != 1 {
			t.Fatalf("repos = %d, want 1", len(repos))
		}
	}
	if got := fake.callCount(); got != 1 {
		t.Errorf("平台 API 调用次数 = %d, want 1（首页应被缓存）", got)
	}
}

// TestListRepos_InvalidateOnAddOrUpdate 验证平台配置变更会清掉该平台的缓存。
func TestListRepos_InvalidateOnAddOrUpdate(t *testing.T) {
	fake := &fakePlatformProvider{repos: []*provider.PlatformRepo{{FullName: "o/r"}}}
	s := newCacheTestService(fake)
	defer func() { _ = s.Close(context.Background()) }()

	opts := provider.ListRepoOptions{Page: 1, PerPage: 100}
	if _, err := s.ListRepos(context.Background(), string(fakePlatform), opts); err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	// 重新登记平台（配置变更）→ 缓存失效，下次回源。
	if err := s.AddOrUpdate(string(fakePlatform), provider.Config{Platform: fakePlatform}); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}
	if _, err := s.ListRepos(context.Background(), string(fakePlatform), opts); err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if got := fake.callCount(); got != 2 {
		t.Errorf("平台 API 调用次数 = %d, want 2（配置变更后应回源）", got)
	}
}

// TestListRepos_NonHomePageNotCached 验证非首页视图不进缓存（分页视图每次都打 API）。
func TestListRepos_NonHomePageNotCached(t *testing.T) {
	fake := &fakePlatformProvider{repos: []*provider.PlatformRepo{{FullName: "o/r"}}}
	s := newCacheTestService(fake)
	defer func() { _ = s.Close(context.Background()) }()

	opts := provider.ListRepoOptions{Page: 2, PerPage: 50}
	for i := 0; i < 2; i++ {
		if _, err := s.ListRepos(context.Background(), string(fakePlatform), opts); err != nil {
			t.Fatalf("ListRepos: %v", err)
		}
	}
	if got := fake.callCount(); got != 2 {
		t.Errorf("平台 API 调用次数 = %d, want 2（非首页视图不该被缓存）", got)
	}
}
