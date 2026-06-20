package settings_service

import (
	"encoding/json"

	"github.com/yi-nology/code-review-service/biz/handler/common"
)

func parseUintID(s string) (uint, error) {
	return common.ParseUintID(s)
}

func mustMarshalJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func safeInt64(v uint) int64 {
	return common.SafeInt64(v)
}

func safeUint(v int64) uint {
	return common.SafeUint(v)
}
