package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"

	"github.com/oekit/concurrency/async"
)

func newPolicyTestRepo(t *testing.T) *ProjectPolicyRepo {
	t.Helper()
	if err := truncateTables(t, "project_policies"); err != nil {
		t.Fatalf("truncate project_policies: %v", err)
	}
	return NewProjectPolicyRepo(testDB)
}

// TestProjectPolicyRepo_GetOrCreateDefault_NotFoundCreates 验证：
// 项目未配置策略时，GetOrCreateDefault 自动创建一条默认记录（FailOn=major）并返回。
func TestProjectPolicyRepo_GetOrCreateDefault_NotFoundCreates(t *testing.T) {
	ctx := context.Background()
	repo := newPolicyTestRepo(t)

	got, err := repo.GetOrCreateDefault(ctx, "github", "acme/web")
	if err != nil {
		t.Fatalf("GetOrCreateDefault: %v", err)
	}
	// 返回值应是默认策略：FailOn=major，其余零值。
	if got.Provider != "github" || got.Repo != "acme/web" {
		t.Fatalf("unexpected provider/repo: %+v", got)
	}
	if got.FailOn != projectpolicy.DefaultFailOn {
		t.Fatalf("expected FailOn=%q, got %q", projectpolicy.DefaultFailOn, got.FailOn)
	}

	// 落库验证：再读一次应命中刚创建的记录。
	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 policy persisted, got %d", len(all))
	}
	if all[0].Provider != "github" || all[0].Repo != "acme/web" || all[0].FailOn != projectpolicy.DefaultFailOn {
		t.Fatalf("persisted policy mismatch: %+v", all[0])
	}
}

// TestProjectPolicyRepo_GetOrCreateDefault_HitReturnsExisting 验证：
// 项目已配置策略时，GetOrCreateDefault 直接返回已有记录，不创建、不覆盖。
func TestProjectPolicyRepo_GetOrCreateDefault_HitReturnsExisting(t *testing.T) {
	ctx := context.Background()
	repo := newPolicyTestRepo(t)

	// 先手动建一条带自定义值的策略。
	existing := &projectpolicy.ProjectPolicy{
		Provider: "gitlab", Repo: "team/api", FailOn: "critical",
		OCRConcurrency: 4, OCRBackground: "custom bg",
	}
	if err := repo.Save(ctx, existing); err != nil {
		t.Fatalf("seed policy: %v", err)
	}

	got, err := repo.GetOrCreateDefault(ctx, "gitlab", "team/api")
	if err != nil {
		t.Fatalf("GetOrCreateDefault: %v", err)
	}
	// 必须返回已有记录的值，而非默认 major。
	if got.FailOn != "critical" {
		t.Fatalf("expected existing FailOn=critical, got %q", got.FailOn)
	}
	if got.OCRConcurrency != 4 || got.OCRBackground != "custom bg" {
		t.Fatalf("expected existing custom values preserved, got %+v", got)
	}

	// 不应多建记录。
	all, _ := repo.List(ctx)
	if len(all) != 1 {
		t.Fatalf("expected still 1 policy (no duplicate), got %d", len(all))
	}
}

// TestProjectPolicyRepo_GetOrCreateDefault_ConcurrentNoDup 验证并发安全：
// 多个 goroutine 同时为同一未配置项目调用 GetOrCreateDefault，
// 最终只创建一条记录（ON CONFLICT DO NOTHING），且所有调用都拿到有效策略。
func TestProjectPolicyRepo_GetOrCreateDefault_ConcurrentNoDup(t *testing.T) {
	ctx := context.Background()
	repo := newPolicyTestRepo(t)

	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)
	results := make([]projectpolicy.ProjectPolicy, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		async.GoSafe(func() {
			defer wg.Done()
			got, err := repo.GetOrCreateDefault(ctx, "gitea", "org/svc")
			errs[i] = err
			results[i] = got
		})
	}
	wg.Wait()

	// 所有调用都应成功，不因唯一约束冲突报错。
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d returned error: %v", i, err)
		}
		if results[i].FailOn != projectpolicy.DefaultFailOn {
			t.Fatalf("goroutine %d got unexpected FailOn=%q", i, results[i].FailOn)
		}
	}

	// 最终只应有一条记录。
	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected exactly 1 policy after concurrent creates, got %d", len(all))
	}
}

// TestProjectPolicyRepo_GetByProviderRepo_NotFound 兜底确认原读取方法语义不变：
// 不存在时返回 (nil, ErrNotFound)。
func TestProjectPolicyRepo_GetByProviderRepo_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := newPolicyTestRepo(t)

	got, err := repo.GetByProviderRepo(ctx, "github", "no/such-repo")
	if !errors.Is(err, repoerr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil policy on not-found, got %+v", got)
	}
}
