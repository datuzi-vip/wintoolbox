package webview2rt

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

const mutexName = `Global\WinToolbox_WebView2Setup`

// withSetupLock runs fn while holding a machine-wide mutex so only one
// download/install runs at a time. Other instances wait, then re-check.
func withSetupLock(fn func() error) error {
	namePtr, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return err
	}
	handle, err := windows.CreateMutex(nil, false, namePtr)
	if handle == 0 {
		if err != nil {
			return fmt.Errorf("创建安装锁失败: %w", err)
		}
		return fmt.Errorf("创建安装锁失败")
	}
	defer windows.CloseHandle(handle)

	alreadyRunning := err == windows.ERROR_ALREADY_EXISTS
	timeout := 30 * time.Second
	if alreadyRunning {
		timeout = 15 * time.Minute
	}

	waitResult, waitErr := windows.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
	if waitErr != nil {
		return fmt.Errorf("获取安装锁失败: %w", waitErr)
	}
	switch waitResult {
	case windows.WAIT_OBJECT_0, windows.WAIT_ABANDONED:
		defer windows.ReleaseMutex(handle)
		if IsInstalled() {
			return nil
		}
		return fn()
	case uint32(windows.WAIT_TIMEOUT):
		if IsInstalled() {
			return nil
		}
		if alreadyRunning {
			return fmt.Errorf("等待其他 WinToolbox 完成 WebView2 安装超时，请稍后重试")
		}
		return fmt.Errorf("无法获取 WebView2 安装锁，请稍后重试")
	default:
		return fmt.Errorf("等待 WebView2 安装锁失败（代码 %d）", waitResult)
	}
}
