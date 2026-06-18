package settings_service

import (
	"encoding/json"
	"fmt"

	"github.com/oekit/pkg/convert"
)

func parseUintID(s string) (uint, error) {
	id := convert.Str2Int(s, -1)
	if id < 0 {
		return 0, fmt.Errorf("invalid id: %s", s)
	}
	return uint(id), nil
}

func mustMarshalJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func safeInt64(v uint) int64 {
	const maxInt64 = ^uint(0) >> 1
	if v > maxInt64 {
		return int64(maxInt64)
	}
	return int64(v)
}

func safeUint(v int64) uint {
	if v < 0 {
		return 0
	}
	return uint(v)
}
