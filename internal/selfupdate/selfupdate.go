package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"wintoolbox/internal/win/syscmd"
)

const (
	repoOwner = "datuzi-vip"
	repoName  = "wintoolbox"
	userAgent = "WinToolbox-SelfUpdate"
)

// Info describes the latest release relative to the running app.
type Info struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	AssetName      string `json:"assetName"`
	AssetURL       string `json:"assetURL"`
	AssetSize      int64  `json:"assetSize"`
	AssetSHA256    string `json:"assetSHA256"`
	Notes          string `json:"notes"`
	Downloaded     bool   `json:"downloaded"`
	DownloadPath   string `json:"downloadPath"`
	Verified       bool   `json:"verified"`
	Error          string `json:"error,omitempty"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Body    string    `json:"body"`
	Assets  []ghAsset `json:"assets"`
}

var (
	mu     sync.Mutex
	cached Info

	sha256BodyRe = regexp.MustCompile(`(?i)SHA256\s*[:：]?\s*` + "`?" + `([A-Fa-f0-9]{64})` + "`?")
)

// Check queries GitHub Releases for a newer version than current.
func Check(current string) (Info, error) {
	mu.Lock()
	defer mu.Unlock()

	info := Info{CurrentVersion: normalizeVersion(current)}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		info.Error = err.Error()
		cached = info
		return info, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		info.Error = "无法连接更新服务器: " + err.Error()
		cached = info
		return info, fmt.Errorf("%s", info.Error)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		info.Error = "读取更新信息失败: " + err.Error()
		cached = info
		return info, fmt.Errorf("%s", info.Error)
	}
	if resp.StatusCode != http.StatusOK {
		info.Error = fmt.Sprintf("更新接口返回 %d", resp.StatusCode)
		cached = info
		return info, fmt.Errorf("%s", info.Error)
	}

	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		info.Error = "解析更新信息失败"
		cached = info
		return info, fmt.Errorf("%s: %w", info.Error, err)
	}

	info.LatestVersion = normalizeVersion(rel.TagName)
	info.Notes = strings.TrimSpace(rel.Body)
	info.HasUpdate = compareVersions(info.LatestVersion, info.CurrentVersion) > 0

	asset := pickAsset(rel.Assets)
	if asset != nil {
		info.AssetName = asset.Name
		info.AssetURL = asset.BrowserDownloadURL
		info.AssetSize = asset.Size
		info.AssetSHA256 = digestSHA256(asset.Digest)
		if info.AssetSHA256 == "" {
			info.AssetSHA256 = parseSHA256FromNotes(rel.Body)
		}
	} else if info.HasUpdate {
		info.Error = "最新版本未找到 Windows amd64 安装包"
		cached = info
		return info, fmt.Errorf("%s", info.Error)
	}

	if info.HasUpdate && info.AssetName != "" {
		dest := downloadPath(info.LatestVersion, info.AssetName)
		if ok, verified := fileReadyVerified(dest, info.AssetSize, info.AssetSHA256); ok {
			info.Downloaded = true
			info.DownloadPath = dest
			info.Verified = verified
			// If the asset was previously downloaded, it might still have
			// Zone.Identifier (MOTW). Best-effort unblock to reduce SmartScreen
			// false positives when the user clicks "Install".
			_ = syscmd.UnblockFile(dest)
		}
	}

	cached = info
	return info, nil
}

// Download downloads the latest release asset when an update is available.
func Download(current string) (Info, error) {
	mu.Lock()
	info := cached
	mu.Unlock()

	if info.LatestVersion == "" || info.CurrentVersion == "" {
		var err error
		info, err = Check(current)
		if err != nil {
			return info, err
		}
	}
	if !info.HasUpdate {
		return info, nil
	}
	if info.AssetURL == "" {
		return info, fmt.Errorf("没有可下载的更新包")
	}

	dest := downloadPath(info.LatestVersion, info.AssetName)
	if ok, verified := fileReadyVerified(dest, info.AssetSize, info.AssetSHA256); ok {
		info.Downloaded = true
		info.DownloadPath = dest
		info.Verified = verified
		_ = syscmd.UnblockFile(dest)
		mu.Lock()
		cached = info
		mu.Unlock()
		return info, nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return info, fmt.Errorf("创建下载目录失败: %w", err)
	}

	tmp := dest + ".partial"
	_ = os.Remove(tmp)

	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, info.AssetURL, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return info, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(tmp)
	if err != nil {
		return info, fmt.Errorf("创建临时文件失败: %w", err)
	}
	h := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(f, h), resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return info, fmt.Errorf("写入更新包失败: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return info, closeErr
	}
	if info.AssetSize > 0 && written != info.AssetSize {
		_ = os.Remove(tmp)
		return info, fmt.Errorf("下载不完整: 期望 %d 字节，实际 %d", info.AssetSize, written)
	}

	got := strings.ToLower(hex.EncodeToString(h.Sum(nil)))
	if info.AssetSHA256 != "" {
		want := strings.ToLower(info.AssetSHA256)
		if got != want {
			_ = os.Remove(tmp)
			return info, fmt.Errorf("更新包校验失败: SHA256 不匹配（期望 %s，实际 %s）", want, got)
		}
		info.Verified = true
	} else {
		_ = os.Remove(tmp)
		return info, fmt.Errorf("发布页未提供 SHA256，已拒绝安装未校验的更新包")
	}

	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return info, fmt.Errorf("保存更新包失败: %w", err)
	}
	// Best-effort: remove Zone.Identifier ADS so Windows/Defender/SmartScreen
	// is less likely to block executing the downloaded update.
	_ = syscmd.UnblockFile(dest)

	info.Downloaded = true
	info.DownloadPath = dest
	mu.Lock()
	cached = info
	mu.Unlock()
	return info, nil
}

// Apply replaces the running executable with the downloaded update and restarts.
func Apply() error {
	mu.Lock()
	info := cached
	mu.Unlock()

	if !info.HasUpdate || !info.Downloaded || info.DownloadPath == "" {
		return fmt.Errorf("请先检查并下载更新")
	}
	if info.AssetSHA256 == "" {
		return fmt.Errorf("缺少 SHA256，无法安全安装更新")
	}
	ok, verified := fileReadyVerified(info.DownloadPath, info.AssetSize, info.AssetSHA256)
	if !ok || !verified {
		return fmt.Errorf("更新包校验失败或不存在，请重新下载")
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("定位当前程序失败: %w", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}

	pid := os.Getpid()
	script := filepath.Join(os.TempDir(), fmt.Sprintf("wintoolbox-update-%d.ps1", pid))
	ps := fmt.Sprintf(`$ErrorActionPreference='Stop'
$target = %s
$source = %s
$procId = %d
while (Get-Process -Id $procId -ErrorAction SilentlyContinue) { Start-Sleep -Seconds 1 }
Copy-Item -LiteralPath $source -Destination $target -Force
Unblock-File -LiteralPath $target -ErrorAction SilentlyContinue
Start-Process -FilePath $target
Remove-Item -LiteralPath $MyInvocation.MyCommand.Path -Force -ErrorAction SilentlyContinue
`, psSingleQuote(exe), psSingleQuote(info.DownloadPath), pid)

	if err := os.WriteFile(script, []byte(ps), 0o700); err != nil {
		return fmt.Errorf("写入更新脚本失败: %w", err)
	}

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动更新脚本失败: %w", err)
	}

	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

// Cached returns the last Check/Download result without network I/O.
func Cached() Info {
	mu.Lock()
	defer mu.Unlock()
	return cached
}

func pickAsset(assets []ghAsset) *ghAsset {
	_ = runtime.GOOS
	var fallback *ghAsset
	for i := range assets {
		a := &assets[i]
		lower := strings.ToLower(a.Name)
		if !strings.HasSuffix(lower, ".exe") {
			continue
		}
		if strings.Contains(lower, "windows") && strings.Contains(lower, "amd64") {
			return a
		}
		if fallback == nil {
			fallback = a
		}
	}
	return fallback
}

func digestSHA256(digest string) string {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return ""
	}
	lower := strings.ToLower(digest)
	if strings.HasPrefix(lower, "sha256:") {
		return strings.TrimPrefix(lower, "sha256:")
	}
	if len(digest) == 64 && isHex(digest) {
		return lower
	}
	return ""
}

func parseSHA256FromNotes(notes string) string {
	m := sha256BodyRe.FindStringSubmatch(notes)
	if len(m) >= 2 {
		return strings.ToLower(m[1])
	}
	return ""
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func downloadDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "WinToolbox", "updates")
}

func downloadPath(version, assetName string) string {
	safe := strings.ReplaceAll(normalizeVersion(version), "/", "_")
	return filepath.Join(downloadDir(), safe+"-"+filepath.Base(assetName))
}

func fileReadyVerified(path string, expectSize int64, expectSHA256 string) (ok bool, verified bool) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() || st.Size() == 0 {
		return false, false
	}
	if expectSize > 0 && st.Size() != expectSize {
		return false, false
	}
	if expectSHA256 == "" {
		return true, false
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return false, false
	}
	if !strings.EqualFold(sum, expectSHA256) {
		_ = os.Remove(path)
		return false, false
	}
	return true, true
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

func compareVersions(a, b string) int {
	as := versionParts(a)
	bs := versionParts(b)
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = normalizeVersion(v)
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
			for _, ch := range p {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				} else {
					break
				}
			}
		}
		out = append(out, n)
	}
	return out
}
