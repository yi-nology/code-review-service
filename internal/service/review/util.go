package review

import (
	"encoding/json"
	"fmt"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/datatypes"
)

func mustMarshal(v interface{}) datatypes.JSON {
	return MustMarshal(v)
}

func MustMarshal(v interface{}) datatypes.JSON {
	data, err := json.Marshal(v)
	if err != nil {
		hlog.Error("mustMarshal failed", "error", err, "type", fmt.Sprintf("%T", v))
		return datatypes.JSON("{}")
	}
	return datatypes.JSON(data)
}
