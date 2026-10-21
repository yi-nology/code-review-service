package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	sdk "github.com/yi-nology/go-git-platform/provider"

	// 平台注册是各后端包的 init 副作用（主程序用 backends/all 统一引入），
	// 测试里按需引入 GitLab 后端即可。
	_ "github.com/yi-nology/go-git-platform/backends/gitlab"
)

// fakeGitLab 复刻 GitLab 的 commit status 端点与**状态机**。
//
// 规则来自 2026-10-21 在生产 GitLab 实例上的实测：
//
//   - state 词表：pending / running / success / failed / canceled ——**没有 error**
//     （POST state=error 被拒为 "state does not have a valid value"）；
//   - 非终态（pending / running）**不能重复写**：第二次被当作转入内部的
//     enqueue / run，400 "Cannot transition status via :enqueue from :pending"；
//   - 终态（success / failed / canceled）可以任意重复写，也可以被覆盖成别的状态。
//
// 这三条合起来就是本服务踩过的坑：评审开始写 pending、评审失败写 error，
// 而 error 被 SDK 的 GitLab 后端兜底成 pending —— 于是「pending→pending」是
// 非法转移，**终态永远写不进去**，门禁一直停在黄色「进行中」。
type fakeGitLab struct {
	mu     sync.Mutex
	posted []fakeGitLabStatus
	seq    int
}

type fakeGitLabStatus struct {
	ID          int    `json:"id"`
	SHA         string `json:"sha"`
	Ref         string `json:"ref"`
	Status      string `json:"status"`
	Name        string `json:"name"`
	TargetURL   string `json:"target_url"`
	Description string `json:"description"`
}

func (f *fakeGitLab) statesFor(name string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, s := range f.posted {
		if s.Name == name {
			out = append(out, s.Status)
		}
	}
	return out
}

func (f *fakeGitLab) postCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.posted)
}

// lastPostedState 返回最后一次 POST 的 state。
func (f *fakeGitLab) lastPostedState() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.posted) == 0 {
		return ""
	}
	return f.posted[len(f.posted)-1].Status
}

// isTerminalState 复刻 GitLab 的终态判定。
func isTerminalState(state string) bool {
	switch state {
	case "success", "failed", "canceled":
		return true
	default:
		return false
	}
}

// start 起一个假 GitLab 服务。
func (f *fakeGitLab) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/statuses/"+lastSegment(r.URL.Path)):
			f.handlePost(t, w, r)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/statuses"):
			f.handleGet(t, w, r)
		default:
			t.Errorf("未预期的请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func lastSegment(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return parts[len(parts)-1]
}

func (f *fakeGitLab) handlePost(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var body struct {
		State       string `json:"state"`
		Name        string `json:"name"`
		Context     string `json:"context"`
		Description string `json:"description"`
		TargetURL   string `json:"target_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Errorf("解析请求体失败: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// GitLab 的 status 名字参数历史上叫 context、后来改名 name，两个都收；
	// SDK 走的是 client-go 的 Context 字段（线上实测 name 也是对的）。
	name := body.Name
	if name == "" {
		name = body.Context
	}

	// GitLab 的 state 词表校验。
	switch body.State {
	case "pending", "running", "success", "failed", "canceled":
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "state does not have a valid value"})
		return
	}

	// 转移校验：同名 context 的当前状态若是非终态，且与新状态相同 → 非法转移。
	f.mu.Lock()
	for i := len(f.posted) - 1; i >= 0; i-- {
		if f.posted[i].Name != name {
			continue
		}
		cur := f.posted[i].Status
		if cur == body.State && !isTerminalState(cur) {
			f.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"message": "Cannot transition status via :enqueue from :" + cur,
			})
			return
		}
		break
	}
	f.seq++
	st := fakeGitLabStatus{
		ID:          f.seq,
		SHA:         lastSegment(r.URL.Path),
		Ref:         "refs/merge-requests/1/head",
		Status:      body.State,
		Name:        name,
		TargetURL:   body.TargetURL,
		Description: body.Description,
	}
	f.posted = append(f.posted, st)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(st)
}

func (f *fakeGitLab) handleGet(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	f.mu.Lock()
	// GitLab 返回最新在前。
	all := make([]fakeGitLabStatus, 0, len(f.posted))
	for i := len(f.posted) - 1; i >= 0; i-- {
		all = append(all, f.posted[i])
	}
	f.mu.Unlock()

	// 真实分页：SDK 的分页助手以「空页」为终止条件，故超出范围必须返回空数组。
	page, perPage := 1, 100
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			perPage = n
		}
	}
	out := []fakeGitLabStatus{}
	start := (page - 1) * perPage
	if start < len(all) {
		end := start + perPage
		if end > len(all) {
			end = len(all)
		}
		out = all[start:end]
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// newGitLabService 用假 GitLab 构造一个 GitProviderService。
func newGitLabService(t *testing.T, srv *httptest.Server) *GitProviderService {
	t.Helper()
	p, err := sdk.NewProvider(sdk.Config{
		Platform: sdk.PlatformGitLab,
		BaseURL:  srv.URL + "/api/v4",
		Token:    "test-token",
	})
	if err != nil {
		t.Fatalf("构造 GitLab provider 失败: %v", err)
	}
	return &GitProviderService{
		providers: map[string]sdk.Provider{"gitlab": p},
		configs:   map[string]sdk.Config{},
	}
}

// TestGitLabErrorStatusLandsAsFailed 是本次修复的**决定性回归测试**。
//
// 场景就是生产上真实发生过的那条链：
//
//	评审开始 → 写 pending
//	评审失败 → 写 error
//
// 修复前：error 被 SDK 的 GitLab 后端兜底成 pending → pending→pending 是 GitLab
// 的非法转移 → 400，终态写不进去，门禁永远停在黄色「进行中」。
// 修复后：error 在平台适配层被翻译成 failed → 终态落得下去。
func TestGitLabErrorStatusLandsAsFailed(t *testing.T) {
	gl := &fakeGitLab{}
	srv := gl.start(t)
	svc := newGitLabService(t, srv)
	ctx := t.Context()

	// 评审开始
	if err := svc.CreateCommitStatus(ctx, "gitlab", "acme", "app", "sha1", sdk.CommitStatusOptions{
		State: "pending", Context: "review-service/gate", Description: "AI 代码审查进行中",
	}); err != nil {
		t.Fatalf("写 pending 失败: %v", err)
	}

	// 评审失败（内部词表的 error）
	if err := svc.CreateCommitStatus(ctx, "gitlab", "acme", "app", "sha1", sdk.CommitStatusOptions{
		State: "error", Context: "review-service/gate", Description: "评审执行失败",
	}); err != nil {
		t.Fatalf("写 error 失败（应被翻译成 failed 落下去）: %v", err)
	}

	if got := gl.lastPostedState(); got != "failed" {
		t.Errorf("最后一次写入的状态 = %q, want %q（error 必须落成 failed，否则门禁停在「进行中」）", got, "failed")
	}
	if got := gl.postCount(); got != 2 {
		t.Errorf("POST 次数 = %d, want 2", got)
	}
	// 门禁最终状态是 failed（红），不是 pending（黄）。
	states := gl.statesFor("review-service/gate")
	if len(states) != 2 || states[0] != "pending" || states[1] != "failed" {
		t.Errorf("状态序列 = %v, want [pending failed]", states)
	}
}

// TestGitLabDuplicatePendingIsSkipped 验证重复写同一非终态被跳过。
//
// 重复「评审开始」通知（webhook 重投、同 SHA 复审）会写第二次 pending，
// 而 pending→pending 是 GitLab 的非法转移。跳过它，状态本来就对。
func TestGitLabDuplicatePendingIsSkipped(t *testing.T) {
	gl := &fakeGitLab{}
	srv := gl.start(t)
	svc := newGitLabService(t, srv)
	ctx := t.Context()

	for i := 0; i < 3; i++ {
		if err := svc.CreateCommitStatus(ctx, "gitlab", "acme", "app", "sha1", sdk.CommitStatusOptions{
			State: "pending", Context: "review-service/gate", Description: "AI 代码审查进行中",
		}); err != nil {
			t.Fatalf("第 %d 次写 pending 失败: %v", i+1, err)
		}
	}
	if got := gl.postCount(); got != 1 {
		t.Errorf("POST 次数 = %d, want 1（重复的 pending 应被跳过）", got)
	}
}

// TestGitLabTerminalStatusIsOverwritten 验证终态可以被覆盖。
//
// 复审：先 success，后来判 failed —— 终态之间必须能改写（门禁要反映最新结论）。
func TestGitLabTerminalStatusIsOverwritten(t *testing.T) {
	gl := &fakeGitLab{}
	srv := gl.start(t)
	svc := newGitLabService(t, srv)
	ctx := t.Context()

	steps := []struct{ state, desc string }{
		{"pending", "评审中"},
		{"success", "评审通过"},
		{"failed", "门禁未通过"},
		{"success", "复审通过"},
	}
	for _, st := range steps {
		if err := svc.CreateCommitStatus(ctx, "gitlab", "acme", "app", "sha1", sdk.CommitStatusOptions{
			State: st.state, Context: "review-service/gate", Description: st.desc,
		}); err != nil {
			t.Fatalf("写 %s 失败: %v", st.state, err)
		}
	}
	if got := gl.postCount(); got != len(steps) {
		t.Errorf("POST 次数 = %d, want %d", got, len(steps))
	}
	if got := gl.lastPostedState(); got != "success" {
		t.Errorf("最终状态 = %q, want success", got)
	}
}

// TestGitLabOtherContextNotAffected 验证别的 context 不影响我们的判断。
func TestGitLabOtherContextNotAffected(t *testing.T) {
	gl := &fakeGitLab{}
	srv := gl.start(t)
	svc := newGitLabService(t, srv)
	ctx := t.Context()

	// 别的 check 已经是 pending
	if err := svc.CreateCommitStatus(ctx, "gitlab", "acme", "app", "sha1", sdk.CommitStatusOptions{
		State: "pending", Context: "check_site", Description: "CI",
	}); err != nil {
		t.Fatalf("写别的 context 失败: %v", err)
	}
	// 我们的 context 写 pending —— 别的 context 是 pending 不该让我们被跳过
	if err := svc.CreateCommitStatus(ctx, "gitlab", "acme", "app", "sha1", sdk.CommitStatusOptions{
		State: "pending", Context: "review-service/gate", Description: "评审中",
	}); err != nil {
		t.Fatalf("写 pending 失败: %v", err)
	}
	if got := gl.postCount(); got != 2 {
		t.Errorf("POST 次数 = %d, want 2", got)
	}
}
