package provider

import (
	"context"
	"testing"

	"github.com/yi-nology/go-git-platform/provider"
)

// fakeCommitStatusManager 记录写调用，并按预设返回当前状态列表。
type fakeCommitStatusManager struct {
	written []provider.CommitStatusOptions
	current []provider.CommitStatus
	listErr error
}

func (f *fakeCommitStatusManager) CreateCommitStatus(_ context.Context, _, _, _ string, opts provider.CommitStatusOptions) error {
	f.written = append(f.written, opts)
	return nil
}

func (f *fakeCommitStatusManager) ListCommitStatuses(_ context.Context, _, _, _ string) ([]provider.CommitStatus, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.current, nil
}

// fakeProvider 实现 provider.Provider 的最小集：只声明 commit status 能力。
type fakeProvider struct {
	provider.Provider
	csm *fakeCommitStatusManager
}

func (f *fakeProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	return f.csm.CreateCommitStatus(ctx, owner, repo, sha, opts)
}

func (f *fakeProvider) ListCommitStatuses(ctx context.Context, owner, repo, sha string) ([]provider.CommitStatus, error) {
	return f.csm.ListCommitStatuses(ctx, owner, repo, sha)
}

// newServiceWithFake 构造一个只带假 provider 的服务。
func newServiceWithFake(t *testing.T, platform string, csm *fakeCommitStatusManager) *GitProviderService {
	t.Helper()
	s := &GitProviderService{
		providers: map[string]provider.Provider{platform: &fakeProvider{csm: csm}},
		configs:   map[string]provider.Config{},
	}
	return s
}

// TestPlatformCommitState 钉住各平台的词表对齐。
//
// 这是本次修复的核心：SDK 各后端的兜底分支会把「不认识的词」静默改写
// （GitLab: error→pending；Gitee: failed→neutral），门禁信号就此丢掉。
// GitLab 上「error」必须变成「failed」，否则评审失败写出去的是「进行中」。
func TestPlatformCommitState(t *testing.T) {
	cases := []struct {
		platform string
		in       string
		want     string
	}{
		{platform: "gitlab", in: "error", want: "failed"},
		{platform: "gitlab", in: "failed", want: "failed"},
		{platform: "gitlab", in: "success", want: "success"},
		{platform: "gitlab", in: "pending", want: "pending"},
		{platform: "github", in: "error", want: "error"},
		{platform: "github", in: "failed", want: "failure"},
		{platform: "github", in: "success", want: "success"},
		{platform: "gitee", in: "error", want: "failure"},
		{platform: "gitee", in: "failed", want: "failure"},
		{platform: "gitcode", in: "error", want: "error"},
		{platform: "gitcode", in: "failed", want: "failure"},
		// 未知平台原样透传：让平台自己拒，比猜一个可能不存在的词好。
		{platform: "unknown", in: "error", want: "error"},
		{platform: "unknown", in: "failed", want: "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.platform+"/"+tc.in, func(t *testing.T) {
			if got := platformCommitState(tc.platform, tc.in); got != tc.want {
				t.Errorf("platformCommitState(%q, %q) = %q, want %q", tc.platform, tc.in, got, tc.want)
			}
		})
	}
}

// TestCreateCommitStatusTranslatesState 验证写出去的状态是**平台词**。
func TestCreateCommitStatusTranslatesState(t *testing.T) {
	csm := &fakeCommitStatusManager{}
	s := newServiceWithFake(t, "gitlab", csm)

	err := s.CreateCommitStatus(context.Background(), "gitlab", "acme", "app", "sha1",
		provider.CommitStatusOptions{State: "error", Context: "review-service/gate"})
	if err != nil {
		t.Fatalf("CreateCommitStatus: %v", err)
	}
	if len(csm.written) != 1 {
		t.Fatalf("写入次数 = %d, want 1", len(csm.written))
	}
	if got := csm.written[0].State; got != "failed" {
		t.Errorf("写出状态 = %q, want %q（error 必须落成 failed，否则门禁停在「进行中」）", got, "failed")
	}
}

// TestCreateCommitStatusSkipsDuplicateNonTerminal 验证「重复写同一非终态」被跳过。
//
// GitLab 的 pending→pending 是非法转移（400 "Cannot transition status via
// :enqueue from :pending"）。这里必须先读后写。
func TestCreateCommitStatusSkipsDuplicateNonTerminal(t *testing.T) {
	csm := &fakeCommitStatusManager{
		current: []provider.CommitStatus{
			{State: provider.CommitStatusPending, Context: "review-service/gate"},
		},
	}
	s := newServiceWithFake(t, "gitlab", csm)

	err := s.CreateCommitStatus(context.Background(), "gitlab", "acme", "app", "sha1",
		provider.CommitStatusOptions{State: "pending", Context: "review-service/gate"})
	if err != nil {
		t.Fatalf("CreateCommitStatus: %v", err)
	}
	if len(csm.written) != 0 {
		t.Errorf("已是 pending 时不应重复写，实际写了 %d 次", len(csm.written))
	}
}

// TestCreateCommitStatusWritesWhenStateDiffers 验证状态不同就照写。
func TestCreateCommitStatusWritesWhenStateDiffers(t *testing.T) {
	cases := []struct {
		name    string
		current provider.CommitStatusState
		write   string
		want    int
	}{
		{name: "pending → pending（重复，非法）", current: provider.CommitStatusPending, write: "pending", want: 0},
		{name: "pending → success", current: provider.CommitStatusPending, write: "success", want: 1},
		{name: "success → pending（终态回非终态，合法）", current: provider.CommitStatusSuccess, write: "pending", want: 1},
		{name: "failed → pending", current: provider.CommitStatusFailure, write: "pending", want: 1},
		{name: "无既有状态 → pending", current: "", write: "pending", want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			csm := &fakeCommitStatusManager{}
			if tc.current != "" {
				csm.current = []provider.CommitStatus{
					{State: tc.current, Context: "review-service/gate"},
				}
			}
			s := newServiceWithFake(t, "gitlab", csm)

			if err := s.CreateCommitStatus(context.Background(), "gitlab", "acme", "app", "sha1",
				provider.CommitStatusOptions{State: tc.write, Context: "review-service/gate"}); err != nil {
				t.Fatalf("CreateCommitStatus: %v", err)
			}
			if len(csm.written) != tc.want {
				t.Errorf("写入次数 = %d, want %d", len(csm.written), tc.want)
			}
		})
	}
}

// TestCreateCommitStatusIgnoresOtherContexts 验证只认**同名 context** 的状态。
//
// 不同 context 是独立的 check，别的 check 是 pending 不代表我们这个也是。
func TestCreateCommitStatusIgnoresOtherContexts(t *testing.T) {
	csm := &fakeCommitStatusManager{
		current: []provider.CommitStatus{
			{State: provider.CommitStatusPending, Context: "check_site"},
		},
	}
	s := newServiceWithFake(t, "gitlab", csm)

	if err := s.CreateCommitStatus(context.Background(), "gitlab", "acme", "app", "sha1",
		provider.CommitStatusOptions{State: "pending", Context: "review-service/gate"}); err != nil {
		t.Fatalf("CreateCommitStatus: %v", err)
	}
	if len(csm.written) != 1 {
		t.Errorf("别的 context 是 pending 不该影响我们，写入次数 = %d, want 1", len(csm.written))
	}
}

// TestCreateCommitStatusWritesWhenListFails 验证读失败不阻断写入。
func TestCreateCommitStatusWritesWhenListFails(t *testing.T) {
	csm := &fakeCommitStatusManager{listErr: context.DeadlineExceeded}
	s := newServiceWithFake(t, "gitlab", csm)

	if err := s.CreateCommitStatus(context.Background(), "gitlab", "acme", "app", "sha1",
		provider.CommitStatusOptions{State: "pending", Context: "review-service/gate"}); err != nil {
		t.Fatalf("CreateCommitStatus: %v", err)
	}
	if len(csm.written) != 1 {
		t.Errorf("读失败时仍应尝试写，写入次数 = %d, want 1", len(csm.written))
	}
}
