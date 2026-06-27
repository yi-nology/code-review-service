package agent

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/oekit/pkg/slicex"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestProbeStacks(t *testing.T) {
	t.Run("empty or missing dir returns nil", func(t *testing.T) {
		if got := ProbeStacks(""); got != nil {
			t.Fatalf("empty dir: want nil, got %v", got)
		}
		if got := ProbeStacks("/no/such/path/xyz"); got != nil {
			t.Fatalf("missing dir: want nil, got %v", got)
		}
	})

	t.Run("file path not dir returns nil", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "afile")
		writeFile(t, dir, "afile", "x")
		if got := ProbeStacks(f); got != nil {
			t.Fatalf("file path: want nil, got %v", got)
		}
	})

	t.Run("go project", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "go.mod", "module x")
		assertStacks(t, ProbeStacks(dir), []string{"stack:go"})
	})

	t.Run("python project", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "pyproject.toml", "[tool]")
		assertStacks(t, ProbeStacks(dir), []string{"stack:python"})
	})

	t.Run("java project", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "pom.xml", "<project/>")
		assertStacks(t, ProbeStacks(dir), []string{"stack:java"})
	})

	t.Run("rust project", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "Cargo.toml", "[package]")
		assertStacks(t, ProbeStacks(dir), []string{"stack:rust"})
	})

	t.Run("node project without frontend sources", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "package.json", "{}")
		writeFile(t, dir, "server.js", "console.log(1)")
		assertStacks(t, ProbeStacks(dir), []string{"stack:node"})
	})

	t.Run("node project with vue sources adds frontend", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "package.json", "{}")
		writeFile(t, dir, "App.vue", "<template/>")
		got := ProbeStacks(dir)
		if !slicex.Contains(got, "stack:node") || !slicex.Contains(got, "stack:frontend") {
			t.Fatalf("want stack:node+frontend, got %v", got)
		}
	})

	t.Run("vue in src subdir adds frontend", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "package.json", "{}")
		if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "src"), "index.tsx", "export const X=1")
		got := ProbeStacks(dir)
		if !slicex.Contains(got, "stack:frontend") {
			t.Fatalf("want stack:frontend for src/*.tsx, got %v", got)
		}
	})

	t.Run("polyglot stacks dedup", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "go.mod", "module x")
		writeFile(t, dir, "package.json", "{}")
		writeFile(t, dir, "App.vue", "<template/>")
		got := ProbeStacks(dir)
		// 期望至少包含 go、node、frontend 三类
		for _, want := range []string{"stack:go", "stack:node", "stack:frontend"} {
			if !slicex.Contains(got, want) {
				t.Fatalf("missing %s in %v", want, got)
			}
		}
	})

	t.Run("unknown project returns nil", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "README.md", "hello")
		writeFile(t, dir, "Makefile", "all:")
		if got := ProbeStacks(dir); got != nil {
			t.Fatalf("unknown project: want nil, got %v", got)
		}
	})
}

func TestTagMatchesStacks(t *testing.T) {
	cases := []struct {
		name   string
		tags   []string
		stacks []string
		want   bool
	}{
		{"always matches any stack", []string{"always", "security"}, []string{"stack:go"}, true},
		{"always matches empty stacks", []string{"always"}, nil, true},
		{"stack intersection", []string{"stack:go", "backend"}, []string{"stack:go", "stack:python"}, true},
		{"no match", []string{"stack:node", "frontend"}, []string{"stack:go"}, false},
		{"empty tags no match", nil, []string{"stack:go"}, false},
		{"tag without prefix not matched against stack value", []string{"backend"}, []string{"stack:go"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := tagMatchesStacks(c.tags, c.stacks); got != c.want {
				t.Fatalf("tagMatchesStacks(%v,%v)=%v, want %v", c.tags, c.stacks, got, c.want)
			}
		})
	}
}

func assertStacks(t *testing.T, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("stacks = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("stacks = %v, want %v", got, want)
		}
	}
}
