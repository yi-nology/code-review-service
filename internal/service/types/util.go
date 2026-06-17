package types

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func GenerateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102150405") + "-" + hex.EncodeToString(b)
}

func ContainsAny(text string, keywords ...string) bool {
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func ResolvePRNumber(provider string, pullRequestID, mergeRequestIID string) int {
	var num int
	switch provider {
	case "gitlab":
		if mergeRequestIID != "" {
			_, _ = fmt.Sscanf(mergeRequestIID, "%d", &num)
		}
	default:
		if pullRequestID != "" {
			_, _ = fmt.Sscanf(pullRequestID, "%d", &num)
		}
	}
	return num
}
