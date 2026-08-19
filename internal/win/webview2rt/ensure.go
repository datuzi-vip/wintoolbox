package webview2rt

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"wintoolbox/internal/win/syscmd"
)

// Ensure makes sure WebView2 Evergreen Runtime is installed.
// If missing, it downloads the official bootstrapper (with progress UI),
// installs silently, and blocks concurrent downloads via a named mutex.
// Callers should present errors to the user (Ensure itself does not MessageBox).
func Ensure() error {
	if IsInstalled() {
		return nil
	}

	return withSetupLock(func() error {
		if IsInstalled() {
			return nil
		}
		return downloadAndInstall()
	})
}

func downloadAndInstall() error {
	prog := openProgress("WinToolbox", "正在连接 Microsoft 下载 WebView2…")
	defer prog.Close()

	dest := installerPath()
	prog.SetStatus("正在下载 WebView2 运行时…")
	if err := downloadBootstrapper(dest, prog.SetProgress); err != nil {
		return fmt.Errorf("下载 WebView2 失败: %w", err)
	}
	// Best-effort: remove Zone.Identifier ADS so the silent installer is less
	// likely to be blocked by SmartScreen/Defender.
	_ = syscmd.UnblockFile(dest)

	prog.SetStatus("正在校验安装包数字签名…")
	if err := verifyMicrosoftSigned(dest); err != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("WebView2 安装包签名校验失败（已删除文件）: %w", err)
	}

	prog.SetMarquee("正在安装 WebView2 运行时，请稍候…\n（安装期间请勿关闭本窗口）")
	if err := runSilentInstall(dest); err != nil {
		return fmt.Errorf("安装 WebView2 失败: %w", err)
	}

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if IsInstalled() {
			prog.SetStatus("WebView2 安装完成")
			time.Sleep(400 * time.Millisecond)
			_ = os.Remove(dest)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	_ = os.Remove(dest)
	return fmt.Errorf("安装程序已结束，但仍未检测到 WebView2 运行时；请重启后再试或手动安装")
}

func runSilentInstall(installer string) error {
	cmd := exec.Command(installer, "/silent", "/install")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := string(out)
		if msg == "" {
			msg = err.Error()
		}
		cmd2 := exec.Command(installer, "/install")
		if err2 := cmd2.Run(); err2 != nil {
			return fmt.Errorf("%s", msg)
		}
	}
	return nil
}
