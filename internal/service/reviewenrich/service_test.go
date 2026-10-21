package reviewenrich

import (
	"context"
	"errors"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/go-git-platform/provider"
)

type fakeCRSource struct {
	platforms map[string]bool
	cr        *provider.ChangeRequest
	getErr    error
	getCalled int
}

func (f *fakeCRSource) HasProvider(platform string) bool { return f.platforms[platform] }

func (f *fakeCRSource) GetCR(context.Context, string, string, string, string) (*provider.ChangeRequest, error) {
	f.getCalled++
	return f.cr, f.getErr
}

func TestFillAuthor(t *testing.T) {
	author := &provider.CRUser{Username: "octocat", Name: "Octo Cat", AvatarURL: "https://avatar/octocat"}

	tests := []struct {
		name    string
		svc     *Service
		req     domain.Request
		wantSet bool
		want    domain.Request
	}{
		{"已有作者短路", newService(&fakeCRSource{platforms: map[string]bool{"github": true}, cr: &provider.ChangeRequest{Author: author}}),
			domain.Request{Provider: "github", Repo: "o/r", Author: "webhook-author"}, false, domain.Request{}},
		{"nil service 不 panic", nil,
			domain.Request{Provider: "github", Repo: "o/r", PullRequestID: "7"}, false, domain.Request{}},
		{"无 PR 编号跳过", newService(&fakeCRSource{platforms: map[string]bool{"github": true}}),
			domain.Request{Provider: "github", Repo: "o/r"}, false, domain.Request{}},
		{"平台未配置跳过", newService(&fakeCRSource{platforms: map[string]bool{}}),
			domain.Request{Provider: "github", Repo: "o/r", PullRequestID: "7"}, false, domain.Request{}},
		{"非法 repo 路径跳过", newService(&fakeCRSource{platforms: map[string]bool{"github": true}}),
			domain.Request{Provider: "github", Repo: "", PullRequestID: "7"}, false, domain.Request{}},
		{"GetCR 失败跳过", newService(&fakeCRSource{platforms: map[string]bool{"github": true}, getErr: errors.New("boom")}),
			domain.Request{Provider: "github", Repo: "o/r", PullRequestID: "7"}, false, domain.Request{}},
		{"CR 无作者跳过", newService(&fakeCRSource{platforms: map[string]bool{"github": true}, cr: &provider.ChangeRequest{}}),
			domain.Request{Provider: "github", Repo: "o/r", PullRequestID: "7"}, false, domain.Request{}},
		{"命中补全", newService(&fakeCRSource{platforms: map[string]bool{"github": true}, cr: &provider.ChangeRequest{Author: author}}),
			domain.Request{Provider: "github", Repo: "o/r", PullRequestID: "7"}, true, domain.Request{
				Author: "octocat", AuthorName: "Octo Cat", AuthorAvatar: "https://avatar/octocat"}},
		{"Name 缺失回退 Username", newService(&fakeCRSource{platforms: map[string]bool{"github": true},
			cr: &provider.ChangeRequest{Author: &provider.CRUser{Username: "octocat"}}}),
			domain.Request{Provider: "github", Repo: "o/r", MergeRequestIID: "!9"}, true, domain.Request{
				Author: "octocat", AuthorName: "octocat"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.req
			original := tt.req.Author
			tt.svc.FillAuthor(context.Background(), &req)
			if tt.wantSet {
				if req.Author != tt.want.Author || req.AuthorName != tt.want.AuthorName || req.AuthorAvatar != tt.want.AuthorAvatar {
					t.Fatalf("补全不符: got %+v", req)
				}
			} else if req.Author != original {
				// 未命中时应保持原值（跳过分支原样返回；已有作者短路的 original 即输入值）。
				t.Fatalf("不应改写作者: got %q, want %q", req.Author, original)
			}
		})
	}
}

func TestFillAuthorSkipsQueryWhenShortCircuit(t *testing.T) {
	f := &fakeCRSource{platforms: map[string]bool{"github": true}}
	newService(f).FillAuthor(context.Background(), &domain.Request{Provider: "github", Repo: "o/r"})
	if f.getCalled != 0 {
		t.Fatalf("无 PR 编号不应触达平台: called=%d", f.getCalled)
	}
}
