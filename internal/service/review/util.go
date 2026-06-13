package review

import (
	"encoding/json"
	"fmt"

	"review-service/internal/pkg/ptr"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/datatypes"
)

func Ptr[T any](v T) *T          { return ptr.Of(v) }
func Deref[T any](p *T, fallback T) T { return ptr.Deref(p, fallback) }

func mustMarshal(v interface{}) datatypes.JSON {
	data, err := json.Marshal(v)
	if err != nil {
		hlog.Error("mustMarshal failed", "error", err, "type", fmt.Sprintf("%T", v))
		return datatypes.JSON("{}")
	}
	return datatypes.JSON(data)
}
