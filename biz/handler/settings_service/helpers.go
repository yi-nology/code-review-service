package settings_service

import "encoding/json"

func mustMarshalJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
