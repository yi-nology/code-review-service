package webhook

import (
	"strings"
	"testing"
)

// TestDeliveryIDDynamicFallback 锁定 P0-#5：gitea/gitcode/tencent 的 DeliveryID
// 在平台未回传 header 时必须生成动态值，而不能是静态 "xxx-unknown"
// （后者会导致第二条起所有 webhook 被幂等键判重静默丢弃）。
func TestDeliveryIDDynamicFallback(t *testing.T) {
	cases := []struct {
		name   string
		fn     func(string) string
		prefix string
	}{
		{"gitea", (&GiteaNormalizer{}).DeliveryID, "gitea-"},
		{"gitcode", (&GitCodeNormalizer{}).DeliveryID, "gitcode-"},
		{"tencent", (&TencentCodeNormalizer{}).DeliveryID, "tencent-"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fn("provided-id"); got != "provided-id" {
				t.Errorf("header value should pass through, got %q", got)
			}

			id1 := tc.fn("")
			if id1 == "" {
				t.Fatal("fallback delivery id should not be empty")
			}
			if !strings.HasPrefix(id1, tc.prefix) {
				t.Errorf("fallback id %q should have prefix %q", id1, tc.prefix)
			}
			if id1 == tc.prefix+"unknown" {
				t.Errorf("fallback id must not be the old static value %q", id1)
			}

			// 连续多次调用应产生不同值（UnixNano 推进）。不要求严格两两不同
			// （系统时钟精度边界），但多次调用必须出现多个不同值，否则幂等键会碰撞。
			seen := map[string]bool{id1: true}
			for i := 0; i < 10; i++ {
				seen[tc.fn("")] = true
			}
			if len(seen) < 2 {
				t.Errorf("fallback ids should vary across calls, got identical value: %s", id1)
			}
		})
	}
}

// TestVerifySignatureEmptySecretDefaultDeny 锁定 P0-#6：所有平台在未配置 secret 时
// 默认拒绝签名校验（安全优先），防止 webhook 被伪造。
func TestVerifySignatureEmptySecretDefaultDeny(t *testing.T) {
	t.Setenv("WEBHOOK_ALLOW_INSECURE", "")
	insecureWarned.Store(false)

	verifiers := []struct {
		name string
		fn   func([]byte, string, string) bool
	}{
		{"github", (&GitHubNormalizer{}).VerifySignature},
		{"gitlab", (&GitLabNormalizer{}).VerifySignature},
		{"gitea", (&GiteaNormalizer{}).VerifySignature},
		{"gitcode", (&GitCodeNormalizer{}).VerifySignature},
		{"tencent", (&TencentCodeNormalizer{}).VerifySignature},
	}
	for _, v := range verifiers {
		t.Run(v.name, func(t *testing.T) {
			if v.fn([]byte("body"), "any-signature", "") {
				t.Errorf("%s: expected verification to fail when secret is empty (default deny)", v.name)
			}
		})
	}
}

// TestVerifySignatureAllowInsecureEnv 锁定 P0-#6 的回退开关：
// WEBHOOK_ALLOW_INSECURE=true 时，gitea/gitcode/tencent 在 secret 为空时放行
// （供历史部署临时回退用，应优先配置 webhook secret）。
func TestVerifySignatureAllowInsecureEnv(t *testing.T) {
	t.Setenv("WEBHOOK_ALLOW_INSECURE", "true")

	for _, tc := range []struct {
		name string
		fn   func([]byte, string, string) bool
	}{
		{"gitea", (&GiteaNormalizer{}).VerifySignature},
		{"gitcode", (&GitCodeNormalizer{}).VerifySignature},
		{"tencent", (&TencentCodeNormalizer{}).VerifySignature},
	} {
		insecureWarned.Store(false)
		if !tc.fn([]byte("body"), "any-signature", "") {
			t.Errorf("%s: expected verification to pass when WEBHOOK_ALLOW_INSECURE=true", tc.name)
		}
	}
}
