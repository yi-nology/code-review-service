package writeback

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

func FindingKey(category, path string, side Side, startLine int, body string) string {
	h := sha256.New()
	h.Write([]byte(category))
	h.Write([]byte{0})
	h.Write([]byte(normalizePath(path)))
	h.Write([]byte{0})
	h.Write([]byte(side))
	h.Write([]byte{0})
	h.Write([]byte(strings.TrimSpace(body)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func normalizePath(p string) string {
	p = filepath.ToSlash(p)
	p = filepath.Clean(p)
	if strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") {
		return ""
	}
	p = strings.TrimPrefix(p, "./")
	return p
}
