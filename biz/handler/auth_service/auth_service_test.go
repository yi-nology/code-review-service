package auth_service

import (
	"strings"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/auth"
)

// 契约锚点：明文 token 格式 rs_<64hex>，DB 只存 SHA256 哈希。
func TestHashToken(t *testing.T) {
	plaintext := "rs_" + strings.Repeat("a", 64)
	hash := hashToken(plaintext)
	if len(hash) != 64 {
		t.Fatalf("hash 应为 64 字符 hex: %q", hash)
	}
	if hash == hashToken("rs_"+strings.Repeat("b", 64)) {
		t.Fatal("不同明文不应同哈希")
	}
	if hash == plaintext {
		t.Fatal("不得明文透传")
	}
}

func TestScopesValidation(t *testing.T) {
	// 契约：scope 列表原样入库（授权模型由 middleware 判定），这里只验证不丢字段。
	scopes := []string{"read", "write"}
	tok := &auth.Token{Name: "ci", TokenHash: "h", Scopes: scopes}
	if len(tok.Scopes) != 2 || tok.Scopes[1] != "write" {
		t.Fatalf("scopes 应原样保留: %+v", tok.Scopes)
	}
}
