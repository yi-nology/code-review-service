package ocr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/runner"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
)

const (
	defaultGitHubRepo    = "alibaba/open-code-review"
	defaultGitHubAPIBase = "https://api.github.com"
	defaultDownloadBase  = "https://github.com"
	defaultAssetPrefix   = "opencodereview"
	checksumFileName     = "sha256sum.txt"

	downloadTimeout = 10 * time.Minute
	apiTimeout      = 30 * time.Second
	cacheTTL        = 5 * time.Minute
)

// settingsStore 是 Upgrader 依赖的配置存储，支持读取与持久化。
// repository.OCRSettingsRepository 天然满足此接口。
type settingsStore interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
	Save(ctx context.Context, settings *models.OCRSettings) error
}

// Upgrader 管理外部 ocr 二进制的在线升级。
//
// 升级流程（移植自官方 install.sh）：
//  1. 查询 GitHub Releases 最新版本
//  2. 下载平台对应的二进制 asset
//  3. 校验 SHA256
//  4. 原子替换二进制（无写权限时回退到可配置目录）
type Upgrader struct {
	store     settingsStore
	resolver  *ConfigResolver
	versioner *Versioner

	repo       string
	apiBase    string
	dlBase     string
	installDir string // 无写权限时的回退安装目录

	mu       sync.Mutex
	state    domain.UpgradeState
	message  string
	version  string
	progress int

	// CheckUpdate 缓存，避免频繁请求 GitHub API 触发限流。
	cacheMu      sync.Mutex
	cacheRelease *githubRelease
	cacheExpiry  time.Time
}

// NewUpgrader 创建升级管理器。
func NewUpgrader(store settingsStore, resolver *ConfigResolver, versioner *Versioner, installDir, repo string) *Upgrader {
	if strings.TrimSpace(repo) == "" {
		repo = defaultGitHubRepo
	}
	return &Upgrader{
		store:      store,
		resolver:   resolver,
		versioner:  versioner,
		repo:       repo,
		apiBase:    defaultGitHubAPIBase,
		dlBase:     defaultDownloadBase,
		installDir: strings.TrimSpace(installDir),
		state:      domain.UpgradeIdle,
	}
}

var _ domain.OCRUpgrader = (*Upgrader)(nil)

// ---------------------------------------------------------------------------
// CheckUpdate
// ---------------------------------------------------------------------------

func (u *Upgrader) CheckUpdate(ctx context.Context) (*domain.OCRUpdateInfo, error) {
	current := ""
	if u.versioner != nil {
		if v, err := u.versioner.OCRVersion(ctx); err == nil && v != nil {
			current = v.Version
		}
	}

	rel, err := u.fetchLatestRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询最新版本失败: %w", err)
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	return &domain.OCRUpdateInfo{
		CurrentVersion:  current,
		LatestVersion:   latest,
		UpdateAvailable: isNewer(current, latest),
		ReleaseNotes:    rel.Body,
		ReleaseURL:      rel.HTMLURL,
	}, nil
}

// ---------------------------------------------------------------------------
// Upgrade（异步）
// ---------------------------------------------------------------------------

func (u *Upgrader) Upgrade(ctx context.Context, targetVersion string) (*domain.OCRUpgradeStatus, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	switch u.state {
	case domain.UpgradeDownloading, domain.UpgradeVerifying, domain.UpgradeInstalling, domain.UpgradeChecking:
		return u.snapshot(), errors.New("已有升级任务正在进行中")
	}

	u.state = domain.UpgradeDownloading
	u.message = "准备下载"
	u.version = targetVersion
	u.progress = 0

	async.GoSafe(func() {
		if err := u.runUpgrade(context.Background(), targetVersion); err != nil {
			logx.Errorf("ocr 升级失败: %v", err)
		}
	})

	return u.snapshot(), nil
}

func (u *Upgrader) UpgradeStatus(_ context.Context) (*domain.OCRUpgradeStatus, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.snapshot(), nil
}

func (u *Upgrader) snapshot() *domain.OCRUpgradeStatus {
	return &domain.OCRUpgradeStatus{
		State:    u.state,
		Message:  u.message,
		Version:  u.version,
		Progress: u.progress,
	}
}

func (u *Upgrader) setState(state domain.UpgradeState, msg string) {
	u.mu.Lock()
	u.state = state
	u.message = msg
	u.mu.Unlock()
}

func (u *Upgrader) setProgress(p int, msg string) {
	u.mu.Lock()
	u.progress = p
	u.message = msg
	u.mu.Unlock()
}

// ---------------------------------------------------------------------------
// runUpgrade — goroutine 内执行的实际升级逻辑
// ---------------------------------------------------------------------------

func (u *Upgrader) runUpgrade(ctx context.Context, targetVersion string) error {
	rel, err := u.resolveRelease(ctx, targetVersion)
	if err != nil {
		u.fail("解析 Release 失败: " + err.Error())
		return err
	}

	asset, err := assetName()
	if err != nil {
		u.fail(err.Error())
		return err
	}

	version := strings.TrimPrefix(rel.TagName, "v")

	// 1. 下载二进制
	u.setState(domain.UpgradeDownloading, "正在下载 "+version)
	tmpDir, err := os.MkdirTemp("", "ocr-upgrade-*")
	if err != nil {
		u.fail("创建临时目录失败: " + err.Error())
		return err
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, asset)
	if err := u.downloadAsset(ctx, rel.TagName, asset, binaryPath); err != nil {
		u.fail("下载失败: " + err.Error())
		return err
	}

	// 2. 校验 SHA256
	u.setState(domain.UpgradeVerifying, "正在校验完整性")
	checksumPath := filepath.Join(tmpDir, checksumFileName)
	if err := u.downloadAsset(ctx, rel.TagName, checksumFileName, checksumPath); err != nil {
		u.fail("下载校验文件失败: " + err.Error())
		return err
	}
	if err := verifyChecksum(binaryPath, checksumPath, asset); err != nil {
		u.fail("校验失败: " + err.Error())
		return err
	}

	// 3. 安装（原子替换或回退目录）
	u.setState(domain.UpgradeInstalling, "正在安装 "+version)
	installedPath, err := u.installBinary(binaryPath)
	if err != nil {
		u.fail("安装失败: " + err.Error())
		return err
	}

	u.mu.Lock()
	u.state = domain.UpgradeDone
	u.message = fmt.Sprintf("已升级到 %s（%s）", version, installedPath)
	u.version = version
	u.progress = 100
	u.mu.Unlock()

	return nil
}

func (u *Upgrader) fail(msg string) {
	u.mu.Lock()
	u.state = domain.UpgradeFailed
	u.message = msg
	u.mu.Unlock()
}

// ---------------------------------------------------------------------------
// resolveRelease — 获取最新 Release 或指定 tag 的 Release
// ---------------------------------------------------------------------------

func (u *Upgrader) resolveRelease(ctx context.Context, targetVersion string) (*githubRelease, error) {
	if strings.TrimSpace(targetVersion) == "" {
		return u.fetchLatestRelease(ctx)
	}
	tag := targetVersion
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	return u.fetchReleaseByTag(ctx, tag)
}

func (u *Upgrader) fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	u.cacheMu.Lock()
	defer u.cacheMu.Unlock()

	if u.cacheRelease != nil && time.Now().Before(u.cacheExpiry) {
		return u.cacheRelease, nil
	}

	rel, err := u.fetchRelease(ctx, "/repos/"+u.repo+"/releases/latest")
	if err != nil {
		return nil, err
	}
	u.cacheRelease = rel
	u.cacheExpiry = time.Now().Add(cacheTTL)
	return rel, nil
}

func (u *Upgrader) fetchReleaseByTag(ctx context.Context, tag string) (*githubRelease, error) {
	return u.fetchRelease(ctx, "/repos/"+u.repo+"/releases/tags/"+tag)
}

func (u *Upgrader) fetchRelease(ctx context.Context, path string) (*githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.apiBase+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: apiTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github api 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("解析 release 失败: %w", err)
	}
	return &rel, nil
}

// ---------------------------------------------------------------------------
// downloadAsset — 下载单个 asset 到指定路径，带进度回调
// ---------------------------------------------------------------------------

func (u *Upgrader) downloadAsset(ctx context.Context, tag, asset, dest string) error {
	url := fmt.Sprintf("%s/%s/releases/download/%s/%s", u.dlBase, u.repo, tag, asset)

	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 %s 返回 %d", asset, resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	written := int64(0)
	buf := make([]byte, 32*1024)
	lastReport := time.Now()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if total > 0 && time.Since(lastReport) > 500*time.Millisecond {
				pct := int(written * 100 / total)
				u.setProgress(pct, fmt.Sprintf("下载中 %d%%", pct))
				lastReport = time.Now()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// verifyChecksum — 比对 SHA256（与 install.sh 逻辑一致）
// ---------------------------------------------------------------------------

func verifyChecksum(binaryPath, checksumPath, asset string) error {
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("读取校验文件失败: %w", err)
	}

	want := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == asset {
			want = strings.ToLower(fields[0])
			break
		}
	}
	if want == "" {
		return fmt.Errorf("校验文件中未找到 asset %q", asset)
	}

	f, err := os.Open(binaryPath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))

	if got != want {
		return fmt.Errorf("校验和不匹配 (got %s, want %s)", got, want)
	}
	return nil
}

// ---------------------------------------------------------------------------
// installBinary — 原子替换二进制；无写权限时回退到可配置目录
// ---------------------------------------------------------------------------

func (u *Upgrader) installBinary(staged string) (string, error) {
	targetPath, err := u.resolveTargetPath()
	if err != nil {
		return "", err
	}

	// 优先尝试原地替换。
	if installed, err := atomicReplace(staged, targetPath); err == nil {
		return installed, nil
	}

	// 原地失败 → 回退到可配置目录。
	fallback := u.fallbackPath()
	if fallback == "" {
		return "", errors.New("无写权限且未配置回退目录（ocr.upgrade_install_dir）")
	}
	if err := os.MkdirAll(filepath.Dir(fallback), 0o755); err != nil {
		return "", fmt.Errorf("创建回退目录失败: %w", err)
	}
	if _, err := atomicReplace(staged, fallback); err != nil {
		return "", err
	}

	// 回退后持久化新 Binary 路径到配置。
	if u.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if s, err := u.store.Get(ctx); err == nil && s != nil {
			s.Binary = fallback
			if saveErr := u.store.Save(ctx, s); saveErr != nil {
				logx.Errorf("持久化回退 Binary 路径失败: %v", saveErr)
			}
		}
	}
	return fallback, nil
}

// resolveTargetPath 解析当前 ocr 二进制的绝对路径。
func (u *Upgrader) resolveTargetPath() (string, error) {
	var fallback runner.OpenCodeReviewConfig
	if u.resolver != nil {
		fallback = u.resolver.fallback
	}

	var binaryName string
	if u.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if s, err := u.store.Get(ctx); err == nil && s != nil {
			binaryName = resolveBinaryFrom(s, fallback)
		}
	}
	if binaryName == "" {
		binaryName = resolveBinaryFrom(nil, fallback)
	}

	abs, err := exec.LookPath(binaryName)
	if err != nil {
		// LookPath 失败（如自定义路径不存在），直接当作字面路径用。
		if filepath.IsAbs(binaryName) {
			return binaryName, nil
		}
		return "", fmt.Errorf("无法定位 ocr 二进制 %q: %w", binaryName, err)
	}
	return abs, nil
}

// fallbackPath 计算回退安装路径：<installDir>/ocr。
func (u *Upgrader) fallbackPath() string {
	dir := strings.TrimSpace(u.installDir)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "ocr")
}

// atomicReplace 将 staged 复制/重命名到 dest（同目录用 Rename，跨目录用 Copy+Rename）。
// 成功后设置 0755 权限。
func atomicReplace(staged, dest string) (string, error) {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 在目标同目录创建临时文件，确保 Rename 原子且跨设备安全。
	tmp, err := os.CreateTemp(dir, ".ocr-tmp-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()

	cleanup := func() { _ = os.Remove(tmpPath) }

	srcFile, err := os.Open(staged)
	if err != nil {
		cleanup()
		return "", err
	}
	defer srcFile.Close()

	if _, err := srcFile.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return "", err
	}
	if _, err := io.Copy(tmp, srcFile); err != nil {
		cleanup()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		cleanup()
		return "", err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		cleanup()
		return "", err
	}
	return dest, nil
}

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

// assetName 根据 runtime.GOOS/GOARCH 返回对应 asset 文件名（与 install.sh 映射一致）。
func assetName() (string, error) {
	var osName string
	switch runtime.GOOS {
	case "darwin", "linux":
		osName = runtime.GOOS
	default:
		return "", fmt.Errorf("不支持的平台 %q（请手动从 GitHub Releases 下载）", runtime.GOOS)
	}

	var arch string
	switch runtime.GOARCH {
	case "amd64", "arm64":
		arch = runtime.GOARCH
	default:
		return "", fmt.Errorf("不支持的架构 %q", runtime.GOARCH)
	}

	return fmt.Sprintf("%s-%s-%s", defaultAssetPrefix, osName, arch), nil
}

// isNewer 判断 latest 是否比 current 新（语义版本 major.minor.patch 比较）。
func isNewer(current, latest string) bool {
	cv := parseSemver(current)
	lv := parseSemver(latest)
	if cv == nil {
		return true
	}
	if lv == nil {
		return false
	}
	if lv[0] != cv[0] {
		return lv[0] > cv[0]
	}
	if lv[1] != cv[1] {
		return lv[1] > cv[1]
	}
	return lv[2] > cv[2]
}

func parseSemver(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 3 {
		return nil
	}
	out := make([]int, 3)
	for i, p := range parts {
		n, ok := atoiSafe(strings.TrimFunc(p, func(r rune) bool {
			return r < '0' || r > '9'
		}))
		if !ok {
			return nil
		}
		out[i] = n
	}
	return out
}

func atoiSafe(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// ---------------------------------------------------------------------------
// GitHub API 类型
// ---------------------------------------------------------------------------

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Name    string        `json:"name"`
	Body    string        `json:"body"`
	HTMLURL string        `json:"html_url"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}
