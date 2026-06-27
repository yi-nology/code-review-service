package ocr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"
)

// ---------------------------------------------------------------------------
// Fake settingsStore
// ---------------------------------------------------------------------------

type fakeSettingsStore struct {
	settings *models.OCRSettings
	saved    *models.OCRSettings
	saveErr  error
}

func (f *fakeSettingsStore) Get(_ context.Context) (*models.OCRSettings, error) {
	return f.settings, nil
}

func (f *fakeSettingsStore) Save(_ context.Context, s *models.OCRSettings) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = s
	return nil
}

// ---------------------------------------------------------------------------
// 纯函数测试
// ---------------------------------------------------------------------------

func TestAssetName(t *testing.T) {
	name, err := assetName()
	if err != nil {
		t.Fatalf("assetName() unexpected error: %v", err)
	}
	expectedOS := runtime.GOOS
	expectedArch := runtime.GOARCH
	want := fmt.Sprintf("opencodereview-%s-%s", expectedOS, expectedArch)
	if name != want {
		t.Fatalf("assetName() = %q, want %q", name, want)
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1.6.4", []int{1, 6, 4}},
		{"v1.6.4", []int{1, 6, 4}},
		{"1.3.13", []int{1, 3, 13}},
		{"2.0.0", []int{2, 0, 0}},
		{"1.6", nil},
		{"", nil},
		{"abc", nil},
		{"v1.6.4-beta", []int{1, 6, 4}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseSemver(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("parseSemver(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("parseSemver(%q) = nil, want %v", tt.input, tt.want)
			}
			for i := 0; i < 3; i++ {
				if got[i] != tt.want[i] {
					t.Fatalf("parseSemver(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"newer patch", "1.3.13", "1.6.4", true},
		{"newer minor", "1.6.4", "1.7.0", true},
		{"newer major", "1.6.4", "2.0.0", true},
		{"same version", "1.6.4", "1.6.4", false},
		{"older latest", "1.6.4", "1.3.0", false},
		{"empty current", "", "1.6.4", true},
		{"empty latest", "1.6.4", "", false},
		{"with v prefix", "v1.3.0", "v1.6.4", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNewer(tt.current, tt.latest); got != tt.want {
				t.Fatalf("isNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 文件系统测试
// ---------------------------------------------------------------------------

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()

	content := []byte("#!/bin/sh\necho fake ocr\n")
	binaryPath := filepath.Join(dir, "opencodereview-darwin-arm64")
	if err := os.WriteFile(binaryPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	hash := sha256.Sum256(content)
	wantHash := hex.EncodeToString(hash[:])

	assetName := "opencodereview-darwin-arm64"
	checksumContent := fmt.Sprintf("%s  %s\n", wantHash, assetName)
	checksumPath := filepath.Join(dir, "sha256sum.txt")
	if err := os.WriteFile(checksumPath, []byte(checksumContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := verifyChecksum(binaryPath, checksumPath, assetName); err != nil {
		t.Fatalf("verifyChecksum() unexpected error: %v", err)
	}
}

func TestVerifyChecksumMismatch(t *testing.T) {
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "binary")
	if err := os.WriteFile(binaryPath, []byte("actual content"), 0o644); err != nil {
		t.Fatal(err)
	}

	checksumContent := "0000000000000000000000000000000000000000000000000000000000000000  binary\n"
	checksumPath := filepath.Join(dir, "sha256sum.txt")
	if err := os.WriteFile(checksumPath, []byte(checksumContent), 0o644); err != nil {
		t.Fatal(err)
	}

	err := verifyChecksum(binaryPath, checksumPath, "binary")
	if err == nil {
		t.Fatal("verifyChecksum() expected error for mismatched checksum")
	}
	if !strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("verifyChecksum() error = %q, want contains '不匹配'", err.Error())
	}
}

func TestVerifyChecksumAssetNotFound(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "binary")
	if err := os.WriteFile(binaryPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	checksumContent := "abc123  some-other-asset\n"
	checksumPath := filepath.Join(dir, "sha256sum.txt")
	if err := os.WriteFile(checksumPath, []byte(checksumContent), 0o644); err != nil {
		t.Fatal(err)
	}

	err := verifyChecksum(binaryPath, checksumPath, "binary")
	if err == nil {
		t.Fatal("verifyChecksum() expected error for missing asset")
	}
}

func TestAtomicReplace(t *testing.T) {
	dir := t.TempDir()

	stagedContent := []byte("new binary content")
	stagedPath := filepath.Join(dir, "staged")
	if err := os.WriteFile(stagedPath, stagedContent, 0o644); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(dir, "ocr")
	if err := os.WriteFile(destPath, []byte("old content"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := atomicReplace(stagedPath, destPath)
	if err != nil {
		t.Fatalf("atomicReplace() error: %v", err)
	}
	if result != destPath {
		t.Fatalf("atomicReplace() result = %q, want %q", result, destPath)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(stagedContent) {
		t.Fatalf("after replace, content = %q, want %q", got, stagedContent)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("after replace, mode = %v, want executable", info.Mode())
	}
}

func TestAtomicReplaceNewFile(t *testing.T) {
	dir := t.TempDir()
	stagedPath := filepath.Join(dir, "staged")
	if err := os.WriteFile(stagedPath, []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(dir, "subdir", "ocr")
	result, err := atomicReplace(stagedPath, destPath)
	if err != nil {
		t.Fatalf("atomicReplace() to new dir error: %v", err)
	}
	if result != destPath {
		t.Fatalf("result = %q, want %q", result, destPath)
	}
}

// ---------------------------------------------------------------------------
// HTTP mock 测试
// ---------------------------------------------------------------------------

// fakeGitHubServer 创建模拟 GitHub Releases 的 httptest.Server。
func fakeGitHubServer(t *testing.T, binaryContent []byte, tag string) *httptest.Server {
	t.Helper()

	hash := sha256.Sum256(binaryContent)
	hashHex := hex.EncodeToString(hash[:])

	mux := http.NewServeMux()

	// GitHub API: releases/latest
	mux.HandleFunc("/repos/alibaba/open-code-review/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag,
			"name":     tag,
			"body":     "changelog for " + tag,
			"html_url": "https://github.com/alibaba/open-code-review/releases/" + tag,
		})
	})

	// GitHub API: releases/tags/:tag
	mux.HandleFunc("/repos/alibaba/open-code-review/releases/tags/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag,
			"name":     tag,
			"body":     "changelog for " + tag,
			"html_url": "https://github.com/alibaba/open-code-review/releases/" + tag,
		})
	})

	// Download: releases/download/:tag/:asset
	mux.HandleFunc("/alibaba/open-code-review/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		asset := filepath.Base(r.URL.Path)
		if asset == "sha256sum.txt" {
			fmt.Fprintf(w, "%s  %s\n", hashHex, "opencodereview-"+runtime.GOOS+"-"+runtime.GOARCH)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(binaryContent)))
		w.Write(binaryContent)
	})

	return httptest.NewServer(mux)
}

func newTestUpgrader(t *testing.T, srvURL string, installDir string) *Upgrader {
	t.Helper()
	u := &Upgrader{
		store: &fakeSettingsStore{
			settings: &models.OCRSettings{},
		},
		repo:       defaultGitHubRepo,
		apiBase:    srvURL,
		dlBase:     srvURL,
		installDir: installDir,
		state:      review.UpgradeIdle,
	}
	return u
}

func TestFetchLatestRelease(t *testing.T) {
	srv := fakeGitHubServer(t, []byte("binary"), "v1.6.4")
	defer srv.Close()

	u := newTestUpgrader(t, srv.URL, t.TempDir())
	rel, err := u.fetchLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("fetchLatestRelease() error: %v", err)
	}
	if rel.TagName != "v1.6.4" {
		t.Fatalf("TagName = %q, want %q", rel.TagName, "v1.6.4")
	}
	if rel.Body == "" {
		t.Fatal("Body should not be empty")
	}
}

func TestFetchLatestReleaseCache(t *testing.T) {
	srv := fakeGitHubServer(t, []byte("binary"), "v1.6.4")
	defer srv.Close()

	u := newTestUpgrader(t, srv.URL, t.TempDir())

	rel1, err := u.fetchLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("first fetch error: %v", err)
	}

	srv.Close()

	rel2, err := u.fetchLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("second fetch (cached) error: %v", err)
	}
	if rel1.TagName != rel2.TagName {
		t.Fatalf("cached TagName changed: %q vs %q", rel1.TagName, rel2.TagName)
	}
}

func TestDownloadAsset(t *testing.T) {
	content := []byte("fake binary content for download test")
	srv := fakeGitHubServer(t, content, "v1.6.4")
	defer srv.Close()

	u := newTestUpgrader(t, srv.URL, t.TempDir())
	dest := filepath.Join(t.TempDir(), "downloaded")
	asset, _ := assetName()

	if err := u.downloadAsset(context.Background(), "v1.6.4", asset, dest); err != nil {
		t.Fatalf("downloadAsset() error: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("downloaded content mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}
}

func TestCheckUpdate(t *testing.T) {
	srv := fakeGitHubServer(t, []byte("x"), "v1.6.4")
	defer srv.Close()

	u := newTestUpgrader(t, srv.URL, t.TempDir())
	info, err := u.CheckUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckUpdate() error: %v", err)
	}
	if info.LatestVersion != "1.6.4" {
		t.Fatalf("LatestVersion = %q, want %q", info.LatestVersion, "1.6.4")
	}
	if info.CurrentVersion != "" {
		t.Fatalf("CurrentVersion = %q, want empty (nil versioner)", info.CurrentVersion)
	}
	if !info.UpdateAvailable {
		t.Fatal("UpdateAvailable should be true when current is empty")
	}
}

// ---------------------------------------------------------------------------
// 端到端升级测试
// ---------------------------------------------------------------------------

func TestUpgradeEndToEnd(t *testing.T) {
	fakeBinary := []byte("#!/bin/sh\necho fake ocr v1.6.4\n")
	srv := fakeGitHubServer(t, fakeBinary, "v1.6.4")
	defer srv.Close()

	installDir := t.TempDir()
	u := newTestUpgrader(t, srv.URL, installDir)

	status, err := u.Upgrade(context.Background(), "")
	if err != nil {
		t.Fatalf("Upgrade() error: %v", err)
	}
	if status.State != review.UpgradeDownloading && status.State != review.UpgradeDone {
		t.Fatalf("initial state = %q, want downloading or done", status.State)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		s, _ := u.UpgradeStatus(context.Background())
		if s.State == review.UpgradeDone {
			break
		}
		if s.State == review.UpgradeFailed {
			t.Fatalf("upgrade failed: %s", s.Message)
		}
		time.Sleep(100 * time.Millisecond)
	}

	s, _ := u.UpgradeStatus(context.Background())
	if s.State != review.UpgradeDone {
		t.Fatalf("final state = %q, want done (msg: %s)", s.State, s.Message)
	}

	installedPath := filepath.Join(installDir, "ocr")
	got, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(got) != string(fakeBinary) {
		t.Fatalf("installed content mismatch")
	}

	info, err := os.Stat(installedPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("installed binary not executable: mode %v", info.Mode())
	}
}

func TestUpgradeConcurrentRejection(t *testing.T) {
	srv := fakeGitHubServer(t, []byte("x"), "v1.6.4")
	defer srv.Close()

	u := newTestUpgrader(t, srv.URL, t.TempDir())

	_, err := u.Upgrade(context.Background(), "")
	if err != nil {
		t.Fatalf("first Upgrade() error: %v", err)
	}

	_, err = u.Upgrade(context.Background(), "")
	if err == nil {
		t.Fatal("second concurrent Upgrade() should return error")
	}
}

func TestUpgradeStatusIdle(t *testing.T) {
	u := newTestUpgrader(t, "http://unused", t.TempDir())
	status, err := u.UpgradeStatus(context.Background())
	if err != nil {
		t.Fatalf("UpgradeStatus() error: %v", err)
	}
	if status.State != review.UpgradeIdle {
		t.Fatalf("state = %q, want idle", status.State)
	}
}

func TestUpgradeChecksumFailure(t *testing.T) {
	hash := sha256.Sum256([]byte("correct"))
	hashHex := hex.EncodeToString(hash[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/alibaba/open-code-review/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"tag_name": "v1.6.4"})
	})
	mux.HandleFunc("/alibaba/open-code-review/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		asset := filepath.Base(r.URL.Path)
		if asset == "sha256sum.txt" {
			fmt.Fprintf(w, "%s  opencodereview-%s-%s\n", hashHex, runtime.GOOS, runtime.GOARCH)
			return
		}
		w.Write([]byte("wrong content - does not match checksum"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	u := newTestUpgrader(t, srv.URL, t.TempDir())
	_, err := u.Upgrade(context.Background(), "")
	if err != nil {
		t.Fatalf("Upgrade() start error: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		s, _ := u.UpgradeStatus(context.Background())
		if s.State == review.UpgradeFailed {
			if !strings.Contains(s.Message, "校验") && !strings.Contains(s.Message, "不匹配") {
				t.Fatalf("failure message = %q, want contains '校验' or '不匹配'", s.Message)
			}
			return
		}
		if s.State == review.UpgradeDone {
			t.Fatal("upgrade should fail due to checksum mismatch")
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("upgrade did not fail within timeout")
}

func TestSnapshot(t *testing.T) {
	u := &Upgrader{
		state:   review.UpgradeDownloading,
		message: "downloading...",
		version: "1.6.4",
	}
	s := u.snapshot()
	if s.State != review.UpgradeDownloading {
		t.Fatalf("state = %q, want downloading", s.State)
	}
	if s.Message != "downloading..." {
		t.Fatalf("message = %q", s.Message)
	}
	if s.Version != "1.6.4" {
		t.Fatalf("version = %q", s.Version)
	}
}

func TestFallbackPath(t *testing.T) {
	tests := []struct {
		installDir string
		want       string
	}{
		{"/opt/ocr", "/opt/ocr/ocr"},
		{"", ""},
		{"  ", ""},
		{"/tmp/dir/", "/tmp/dir/ocr"},
	}
	for _, tt := range tests {
		u := &Upgrader{installDir: tt.installDir}
		got := u.fallbackPath()
		if got != tt.want {
			t.Fatalf("fallbackPath(%q) = %q, want %q", tt.installDir, got, tt.want)
		}
	}
}

// 确保未使用的导入在将来添加功能时不会报错
var _ = io.EOF
