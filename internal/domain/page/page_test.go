package page

import (
	"encoding/json"
	"testing"
)

func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPageJSONContract(t *testing.T) {
	// 锁定 wire 契约：字段名必须与此前 gormx.PageResponse 完全一致，
	// 改动会破坏前端与 mcp 客户端的分页解析。
	want := `{"items":[],"total":3,"page":2,"page_size":10,"pages":1}`
	got := marshal(t, Page[any]{Items: []any{}, Total: 3, Page: 2, PageSize: 10, Pages: 1})
	if got != want {
		t.Fatalf("wire contract changed:\n got: %s\nwant: %s", got, want)
	}
}
