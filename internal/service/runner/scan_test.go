package runner

import (
	"reflect"
	"testing"
)

func TestParsePreviewFilesHumanOutput(t *testing.T) {
	output := []byte("Preview: 3 file(s) changed | \x1b[32m+12\x1b[0m \x1b[31m-1\x1b[0m\n" +
		"\x1b[1mWill review (2):\x1b[0m\n" +
		"\x1b[34m[S]\x1b[0m internal/service.go \x1b[32m+5 \x1b[0m \x1b[31m-0 \x1b[0m\n" +
		"\x1b[34m[S]\x1b[0m frontend/src/App.vue \x1b[32m+7 \x1b[0m \x1b[31m-1 \x1b[0m\n" +
		"\x1b[1mExcluded from review (1):\x1b[0m\n" +
		"\x1b[34m[S]\x1b[0m README.md \x1b[2m(unsupported_ext)\x1b[0m\n")

	got := parsePreviewFiles(output)
	want := []string{"internal/service.go", "frontend/src/App.vue"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePreviewFiles() = %#v, want %#v", got, want)
	}
}

func TestParsePreviewFilesJSON(t *testing.T) {
	got := parsePreviewFiles([]byte(`["internal/service.go","frontend/src/App.vue"]`))
	want := []string{"internal/service.go", "frontend/src/App.vue"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePreviewFiles() = %#v, want %#v", got, want)
	}
}
