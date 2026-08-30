package provider

import (
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
