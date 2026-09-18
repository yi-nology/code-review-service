package argus_service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	modelReview "github.com/yi-nology/code-review-service/biz/model/review"
	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/domain/argusingest"
)

func sp(s string) *string { return &s }

func validIngestReq() *modelReview.ArgusIngestReq {
	return &modelReview.ArgusIngestReq{
		ArgusTaskID: sp("task-1"), Platform: "gitea", Owner: "zhangyi", Repo: "demo",
		Number: "7", Title: sp("fix login"), HeadSha: "abc123", BaseBranch: sp("main"),
		Verdict: argusingest.VerdictComment, OccurredAt: sp(time.Now().Format(time.RFC3339)),
		Plugins: []string{"correctness", "ocr"},
		Findings: []*modelReview.ArgusFindingItem{
			{Fp: sp("fp-1"), File: "a.go", Line: 3, Severity: "high", Comment: " 竞态 ", SuggestedFix: sp("加锁"), By: sp(" ocr ")},
			{Fp: sp("fp-2"), File: "b.go", Line: 9, Severity: "low", Comment: "命名"},
		},
	}
}

// 合法上报：字段裁剪、计数聚合、findings 逐条映射。
func TestBuildReviewValid(t *testing.T) {
	rev, err := buildReview(validIngestReq())
	if err != nil {
		t.Fatal(err)
	}
	if rev.FindingsTotal != 2 || rev.FindingsHigh != 1 || rev.FindingsMedium != 0 || rev.FindingsLow != 1 {
		t.Fatalf("计数聚合不符: %+v", rev)
	}
	if rev.Findings[0].Comment != "竞态" || rev.Findings[0].By != "ocr" {
		t.Fatalf("裁剪不符: %+v", rev.Findings[0])
	}
	if rev.OccurredAt.IsZero() {
		t.Fatal("occurred_at 应保留上游值")
	}
}

// 同 payload 内非空 fp 去重（last-wins）：重复 fp 保留最后一条、计数按去重后聚合；
// 空 fp 意见不去重（上游旧数据形态，允许多条共存）。
func TestBuildReviewDedupFP(t *testing.T) {
	req := validIngestReq()
	req.Findings = []*modelReview.ArgusFindingItem{
		{Fp: sp("fp-1"), File: "a.go", Line: 3, Severity: "high", Comment: "旧"},
		{Fp: sp("fp-1"), File: "a2.go", Line: 4, Severity: "low", Comment: "新（last-wins）"},
		{Fp: sp("fp-2"), File: "b.go", Line: 9, Severity: "medium", Comment: "x"},
		{File: "c.go", Line: 1, Severity: "low", Comment: "无指纹 1"},
		{File: "d.go", Line: 2, Severity: "low", Comment: "无指纹 2"},
	}
	rev, err := buildReview(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(rev.Findings) != 4 {
		t.Fatalf("去重后应 4 条（fp-1/fp-2/两条空 fp）: %+v", rev.Findings)
	}
	if rev.Findings[0].File != "a2.go" || rev.Findings[0].Comment != "新（last-wins）" {
		t.Fatalf("fp-1 应保留最后一条: %+v", rev.Findings[0])
	}
	if rev.FindingsTotal != 4 || rev.FindingsHigh != 0 || rev.FindingsMedium != 1 || rev.FindingsLow != 3 {
		t.Fatalf("计数应按去重后聚合: %+v", rev)
	}
}

// error_message（verdict=error 时的失败原因）裁剪透传。
func TestBuildReviewErrorMessage(t *testing.T) {
	req := validIngestReq()
	req.Verdict = argusingest.VerdictError
	req.ErrorMessage = sp("  llm timeout after 3 retries  ")
	rev, err := buildReview(req)
	if err != nil {
		t.Fatal(err)
	}
	if rev.ErrorMessage != "llm timeout after 3 retries" {
		t.Fatalf("error_message 应裁剪: %q", rev.ErrorMessage)
	}
}

// toReviewItem 空 error_message 出 nil（omitempty 语义）。
func TestToReviewItemErrorMessageNil(t *testing.T) {
	rev, err := buildReview(validIngestReq())
	if err != nil {
		t.Fatal(err)
	}
	item := toReviewItem(rev)
	if item.ErrorMessage != nil {
		t.Fatalf("空 error_message 应为 nil: %v", item.ErrorMessage)
	}
}

func TestBuildReviewValidation(t *testing.T) {
	t.Run("缺必填字段", func(t *testing.T) {
		req := validIngestReq()
		req.HeadSha = ""
		if _, err := buildReview(req); err == nil || !strings.Contains(err.Error(), "head_sha") {
			t.Fatalf("应报 head_sha 必填: %v", err)
		}
	})
	t.Run("verdict 越界", func(t *testing.T) {
		req := validIngestReq()
		req.Verdict = "lgtm"
		if _, err := buildReview(req); err == nil {
			t.Fatal("应报 verdict 越界")
		}
	})
	t.Run("severity 越界", func(t *testing.T) {
		req := validIngestReq()
		req.Findings[0].Severity = "critical" // 归一化是 argus 侧职责，承接侧钉死词表
		if _, err := buildReview(req); err == nil {
			t.Fatal("应报 severity 越界")
		}
	})
	t.Run("finding 缺 file/comment", func(t *testing.T) {
		req := validIngestReq()
		req.Findings[0].File = ""
		if _, err := buildReview(req); err == nil {
			t.Fatal("应报 file 必填")
		}
	})
	t.Run("意见超上限", func(t *testing.T) {
		req := validIngestReq()
		req.Findings = make([]*modelReview.ArgusFindingItem, maxFindingsPerIngest+1)
		for i := range req.Findings {
			req.Findings[i] = &modelReview.ArgusFindingItem{File: "a.go", Severity: "low", Comment: "x"}
		}
		if _, err := buildReview(req); err == nil || !strings.Contains(err.Error(), "上限") {
			t.Fatalf("应报超上限: %v", err)
		}
	})
}

// ---------- doSuppress（反馈回流代理，出站调用收口在 internal/argusclient） ----------

type fakeRepo struct {
	reviews map[uint]*argusingest.Review
	states  map[string]string
}

func (f *fakeRepo) UpsertReview(context.Context, *argusingest.Review) (uint, error) { return 0, nil }

func (f *fakeRepo) GetReview(_ context.Context, id uint) (*argusingest.Review, error) {
	r, ok := f.reviews[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return r, nil
}

func (f *fakeRepo) ListReviews(context.Context, argusingest.ListFilter) ([]argusingest.Review, int64, error) {
	return nil, 0, nil
}

func (f *fakeRepo) SetFindingSuppressState(_ context.Context, reviewID uint, fp, state string) error {
	f.states[strconv.FormatUint(uint64(reviewID), 10)+"|"+fp] = state
	return nil
}

func sampleReview() *argusingest.Review {
	rev, err := buildReview(validIngestReq())
	if err != nil {
		panic(err)
	}
	rev.ID = 1
	return rev
}

// 命中：代理经 argusclient 调 /findings/suppress，matched=true → 本地落 suppressed。
func TestDoSuppressMatched(t *testing.T) {
	argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"matched":true,"fingerprint":"fp-1-full"}}`))
	}))
	t.Cleanup(argus.Close)

	repo := &fakeRepo{reviews: map[uint]*argusingest.Review{1: sampleReview()}, states: map[string]string{}}
	h := &Handler{repo: repo, argus: argusclient.New(argus.URL, "tok")}

	matched, err := doSuppress(context.Background(), h, 1, "fp-1", "", "")
	if err != nil || !matched {
		t.Fatalf("matched=%v err=%v", matched, err)
	}
	if repo.states["1|fp-1"] != "suppressed" {
		t.Fatalf("应落 suppressed: %v", repo.states)
	}
}

// 未命中：不落状态，返回 matched=false。
func TestDoSuppressUnmatched(t *testing.T) {
	argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"matched":false,"fingerprint":""}}`))
	}))
	t.Cleanup(argus.Close)
	repo := &fakeRepo{reviews: map[uint]*argusingest.Review{1: sampleReview()}, states: map[string]string{}}
	h := &Handler{repo: repo, argus: argusclient.New(argus.URL, "tok")}

	matched, err := doSuppress(context.Background(), h, 1, "fp-2", "", "")
	if err != nil || matched {
		t.Fatalf("未命中应 matched=false，got %v/%v", matched, err)
	}
	if len(repo.states) != 0 {
		t.Fatalf("未命中不得落状态: %v", repo.states)
	}
}

// fp 不属于该任务 → 本地拒绝，不触达 argus。
func TestDoSuppressForeignFP(t *testing.T) {
	called := false
	argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(argus.Close)
	repo := &fakeRepo{reviews: map[uint]*argusingest.Review{1: sampleReview()}, states: map[string]string{}}
	h := &Handler{repo: repo, argus: argusclient.New(argus.URL, "tok")}

	if _, err := doSuppress(context.Background(), h, 1, "fp-unknown", "", ""); err == nil {
		t.Fatal("陌生 fp 应本地拒绝")
	}
	if called {
		t.Fatal("不得触达 argus")
	}
}

// comment 兜底模式：按 file+comment 原文匹配，命中后按该意见的本地 fp 落状态。
func TestDoSuppressByComment(t *testing.T) {
	var gotBody string
	argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"matched":true,"fingerprint":"fp-1-full"}}`))
	}))
	t.Cleanup(argus.Close)
	repo := &fakeRepo{reviews: map[uint]*argusingest.Review{1: sampleReview()}, states: map[string]string{}}
	h := &Handler{repo: repo, argus: argusclient.New(argus.URL, "tok")}

	rev := sampleReview()
	target := rev.Findings[0]
	matched, err := doSuppress(context.Background(), h, 1, "", target.File, target.Comment)
	if err != nil || !matched {
		t.Fatalf("matched=%v err=%v", matched, err)
	}
	if !strings.Contains(gotBody, `"file"`) || !strings.Contains(gotBody, `"comment"`) {
		t.Fatalf("应走 file+comment 模式: %s", gotBody)
	}
	if target.FP != "" && repo.states["1|"+target.FP] != "suppressed" {
		t.Fatalf("应按命中意见的 fp 落状态: %v", repo.states)
	}
}

// fp 未命中且带 file+comment → 自动降级兜底模式。
func TestDoSuppressFPFallbackToComment(t *testing.T) {
	mode := ""
	argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		body := string(buf)
		if strings.Contains(body, "fingerprint") {
			mode = "fp"
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"matched":false,"fingerprint":""}}`))
			return
		}
		mode = "comment"
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"matched":true,"fingerprint":"fp-x"}}`))
	}))
	t.Cleanup(argus.Close)
	repo := &fakeRepo{reviews: map[uint]*argusingest.Review{1: sampleReview()}, states: map[string]string{}}
	h := &Handler{repo: repo, argus: argusclient.New(argus.URL, "tok")}

	rev := sampleReview()
	target := rev.Findings[0]
	matched, err := doSuppress(context.Background(), h, 1, "fp-1", target.File, target.Comment)
	if err != nil || !matched {
		t.Fatalf("降级兜底应命中: matched=%v err=%v", matched, err)
	}
	if mode != "comment" {
		t.Fatalf("应先 fp 后 comment 两次调用, last=%s", mode)
	}
}

// argus 侧 500 → 错误上抛（前端提示重试）。
func TestDoSuppressArgusError(t *testing.T) {
	argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(argus.Close)
	repo := &fakeRepo{reviews: map[uint]*argusingest.Review{1: sampleReview()}, states: map[string]string{}}
	h := &Handler{repo: repo, argus: argusclient.New(argus.URL, "tok")}

	if _, err := doSuppress(context.Background(), h, 1, "fp-1", "", ""); err == nil {
		t.Fatal("argus 500 应上抛")
	}
	if len(repo.states) != 0 {
		t.Fatalf("失败不得落状态: %v", repo.states)
	}
}
