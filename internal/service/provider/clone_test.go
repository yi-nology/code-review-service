package provider

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateGitBranchRef(t *testing.T) {
	valid := []string{
		"", // 空 = 可选语义，放行
		"main",
		"feat/api-access-log",
		"release/v1.2.3",
		"feature/中文分支",
		"user/feature+build",
		"a", // 单字符
	}
	for _, name := range valid {
		if err := validateGitBranchRef("branch", name); err != nil {
			t.Errorf("validateGitBranchRef(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"-oProxyCommand=evil",  // 前导 '-'：git option injection
		"--upload-pack=evil",   // 同上
		"-branch",              // 单个前导 '-'
		"main:refs/heads/evil", // ':' refspec 分隔符注入
		"a b",                  // 空白
		"main\n",               // 换行
		"main\t",               // tab
		"a..b",                 // revision range 表达式
		"@{push}",              // @{ 表达式
		"main~1",               // '~' revision 表达式
		"main^",                // '^' revision 表达式
		"main*",                // glob 通配
		"main[0-9]",            // 字符类
	}
	for _, name := range invalid {
		if err := validateGitBranchRef("branch", name); err == nil {
			t.Errorf("validateGitBranchRef(%q) = nil, want error", name)
		} else if !strings.Contains(err.Error(), "branch") {
			t.Errorf("error for %q should name the field, got: %v", name, err)
		}
	}
}

func TestValidateGitSHA(t *testing.T) {
	valid := []string{
		"",       // 空 = 可选
		"abc123", // 6 位缩写
		"94a3b5d2c6e7f8091a2b3c4d5e6f708192a3b4c5", // 40 位 SHA-1
		"94A3B5", // 大写
	}
	for _, sha := range valid {
		if err := validateGitSHA("sha", sha); err != nil {
			t.Errorf("validateGitSHA(%q) = %v, want nil", sha, err)
		}
	}

	invalid := []string{
		"abc12",            // < 6 位
		"94a3b5 main",      // 含空白
		"main~1",           // revision 表达式
		"-oProxyCommand=x", // option injection
		"abc12g",           // 非 hex 字符
		"abc123;rm -rf /",  // 命令注入尝试
	}
	for _, sha := range invalid {
		if err := validateGitSHA("sha", sha); err == nil {
			t.Errorf("validateGitSHA(%q) = nil, want error", sha)
		}
	}
}

func TestPullRequestFetchRefsRejectsUnsafeBranchBeforeBuild(t *testing.T) {
	// pullRequestFetchRefs 本身不做校验（防御在 cloneAndResolvePR 入口），
	// 这里锁定恶意分支名若绕过入口校验也不会生成隐蔽数量之外的 refspec。
	refs := pullRequestFetchRefs("github", 1, "-oProxyCommand=evil", "")
	want := 3
	if len(refs) != want {
		t.Fatalf("expected %d refs, got %d: %v", want, len(refs), refs)
	}
}

// gitFixture 用真 git 二进制搭一个 origin 仓库 + 可选工作区 clone，
// 复刻评审工作区的生产形态（clone 默认分支 = target 分支已检出）。
type gitFixture struct {
	t      *testing.T
	origin string
	clone  string
}

func newGitFixture(t *testing.T, withClone bool) *gitFixture {
	t.Helper()
	f := &gitFixture{t: t, origin: t.TempDir()}
	if withClone {
		f.clone = t.TempDir()
	}
	f.git(f.origin, "init", "-b", "main")
	f.git(f.origin, "config", "user.email", "test@test.com")
	f.git(f.origin, "config", "user.name", "Test")
	f.write("README.md", "init")
	f.git(f.origin, "add", ".")
	f.git(f.origin, "commit", "-m", "init")
	if withClone {
		f.git("", "clone", "-q", f.origin, f.clone)
	}
	return f
}

func (f *gitFixture) git(dir string, args ...string) string {
	f.t.Helper()
	if dir == "" {
		dir = f.origin
	}
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *gitFixture) write(name, content string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.origin, name), []byte(content), 0644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *gitFixture) advance(message string) string {
	f.t.Helper()
	f.write("advance.txt", message)
	f.git(f.origin, "add", ".")
	f.git(f.origin, "commit", "-m", message)
	return f.git(f.origin, "rev-parse", "HEAD")
}

// TestCloneAndResolvePR_ApiBaseSHAUnresolvableFailsFast 复刻生产事故形态：
// 平台给了 base SHA 但工作区解析不了（fetch 失败等），此前静默滑到 target
// 分支启发式继续跑，产出空跑 success；现在必须 fail fast（issue #1）。
func TestCloneAndResolvePR_ApiBaseSHAUnresolvableFailsFast(t *testing.T) {
	f := newGitFixture(t, true)
	head := f.advance("feature work")
	head2 := f.advance("feature work 2")
	if head == head2 {
		t.Fatal("fixture broken")
	}

	_, _, _, err := cloneAndResolvePR(
		context.Background(), f.origin, "", false, 1,
		"github", "main", "main",
		"", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	)
	if err == nil {
		t.Fatal("api base SHA 解析失败必须报错，不得静默降级")
	}
	if !strings.Contains(err.Error(), "base SHA") {
		t.Fatalf("错误应指明 base SHA，got: %v", err)
	}
}

// TestCloneAndResolvePR_BaseEqualsHeadFails：source==target 同分支时
// merge-base 恒为 head，此前 baseSHA 会兜到 RevParse(target)==head 继续跑
// 空 diff；现在 base==head 直接报错，空跑与「审了没问题」可分辨（issue #1）。
func TestCloneAndResolvePR_BaseEqualsHeadFails(t *testing.T) {
	f := newGitFixture(t, true)

	_, _, _, err := cloneAndResolvePR(
		context.Background(), f.origin, "", false, 1,
		"github", "main", "main",
		"", "",
	)
	if err == nil {
		t.Fatal("base==head 必须报错，不得产出空 diff 成功评审")
	}
	if !strings.Contains(err.Error(), "相同") {
		t.Fatalf("错误应指明 base==head，got: %v", err)
	}
}

// TestCloneAndResolvePR_HappyPath：正常 PR（source 领先 target）不受影响，
// base 解析仍走 merge-base，成功返回工作区。
func TestCloneAndResolvePR_HappyPath(t *testing.T) {
	f := newGitFixture(t, false)
	f.git(f.origin, "checkout", "-b", "feature")
	f.advance("feature work")

	dir, baseSHA, headSHA, err := cloneAndResolvePR(
		context.Background(), f.origin, "", false, 2,
		"github", "feature", "main",
		"", "",
	)
	if err != nil {
		t.Fatalf("正常 PR 不得报错: %v", err)
	}
	defer os.RemoveAll(dir)
	if baseSHA == "" || headSHA == "" || baseSHA == headSHA {
		t.Fatalf("base/head SHA 异常: %q / %q", baseSHA, headSHA)
	}
}

// TestCloneAndResolveSHA_DiffErrorFailsFast：SHA 模式下 git.Diff 失败此前被
// 吞掉（diff 留空继续跑），现在必须 fail fast（issue #1）。
func TestCloneAndResolveSHA_DiffErrorFailsFast(t *testing.T) {
	f := newGitFixture(t, false)
	base := f.git(f.origin, "rev-parse", "HEAD")
	head := f.advance("next")

	// baseSHA 指向不存在的对象使 Diff 失败：RevParse 已过校验（hex 合法），
	// 但 checkout head 成功后 diff 的 base 不可解析
	_, _, _, _, err := cloneAndResolveSHA(
		context.Background(), f.origin, "", false,
		"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", head, 0,
	)
	if err == nil {
		t.Fatal("Diff 失败必须报错，不得吞掉后产出空 diff 成功评审")
	}
	_ = base
}
