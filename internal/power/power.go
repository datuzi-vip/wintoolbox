package power

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"wintoolbox/internal/win/syscmd"
)

var (
	modUser32                  = windows.NewLazySystemDLL("user32.dll")
	procLockWorkStation        = modUser32.NewProc("LockWorkStation")
	modAdvapi32                = windows.NewLazySystemDLL("advapi32.dll")
	procLookupPrivilege        = modAdvapi32.NewProc("LookupPrivilegeValueW")
	procInitiateSystemShutdown = modAdvapi32.NewProc("InitiateSystemShutdownExW")
	procAbortSystemShutdown    = modAdvapi32.NewProc("AbortSystemShutdownW")
)

// Lock locks the workstation.
func Lock() error {
	r, _, e := procLockWorkStation.Call()
	if r == 0 {
		if e != syscall.Errno(0) {
			return fmt.Errorf("锁定失败: %v", e)
		}
		return fmt.Errorf("锁定失败")
	}
	return nil
}

const (
	// MaxPowerDelaySeconds is the maximum accepted restart/shutdown delay.
	MaxPowerDelaySeconds = 7 * 24 * 60 * 60 // 7 days
)

// Restart reboots after delaySeconds (0 = now).
func Restart(delaySeconds int) error {
	if delaySeconds < 0 || delaySeconds > MaxPowerDelaySeconds {
		return fmt.Errorf("延迟秒数无效（须为 0–%d）", MaxPowerDelaySeconds)
	}
	if err := initiateShutdown(uint32(delaySeconds), true); err == nil {
		return nil
	}
	return shutdownExe("/r", "/t", fmt.Sprintf("%d", delaySeconds), "/f")
}

// Shutdown powers off after delaySeconds.
func Shutdown(delaySeconds int) error {
	if delaySeconds < 0 || delaySeconds > MaxPowerDelaySeconds {
		return fmt.Errorf("延迟秒数无效（须为 0–%d）", MaxPowerDelaySeconds)
	}
	if err := initiateShutdown(uint32(delaySeconds), false); err == nil {
		return nil
	}
	return shutdownExe("/s", "/t", fmt.Sprintf("%d", delaySeconds), "/f")
}

// Abort cancels a pending shutdown/restart.
func Abort() error {
	r, _, e := procAbortSystemShutdown.Call(0)
	if r != 0 {
		return nil
	}
	out, err := syscmd.Run(shutdownPath(), "/a")
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			if e != syscall.Errno(0) {
				msg = e.Error()
			} else {
				msg = err.Error()
			}
		}
		return fmt.Errorf("取消关机失败（可能没有进行中的关机）: %s", msg)
	}
	return nil
}

func initiateShutdown(timeoutSec uint32, reboot bool) error {
	if err := enableShutdownPrivilege(); err != nil {
		return err
	}
	msg, err := windows.UTF16PtrFromString("WinToolbox 计划的系统关机/重启")
	if err != nil {
		return err
	}
	var force uintptr = 1
	var rebootFlag uintptr
	if reboot {
		rebootFlag = 1
	}
	// SHTDN_REASON_MAJOR_OTHER | SHTDN_REASON_FLAG_PLANNED
	const reason = 0x80000000
	r, _, e := procInitiateSystemShutdown.Call(
		0, // local machine
		uintptr(unsafe.Pointer(msg)),
		uintptr(timeoutSec),
		force,
		rebootFlag,
		reason,
	)
	if r == 0 {
		if e != syscall.Errno(0) {
			return e
		}
		return fmt.Errorf("InitiateSystemShutdown 失败")
	}
	return nil
}

func shutdownExe(args ...string) error {
	_ = enableShutdownPrivilege()
	out, err := syscmd.Run(shutdownPath(), args...)
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func shutdownPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "shutdown.exe")
}

func enableShutdownPrivilege() error {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token)
	if err != nil {
		return err
	}
	defer token.Close()

	var luid windows.LUID
	name, _ := windows.UTF16PtrFromString("SeShutdownPrivilege")
	r, _, e := procLookupPrivilege.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&luid)))
	if r == 0 {
		if e != syscall.Errno(0) {
			return e
		}
		return fmt.Errorf("LookupPrivilegeValue 失败")
	}
	tp := windows.Tokenprivileges{
		PrivilegeCount: 1,
		Privileges: [1]windows.LUIDAndAttributes{
			{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED},
		},
	}
	if err := windows.AdjustTokenPrivileges(token, false, &tp, 0, nil, nil); err != nil {
		return err
	}
	// AdjustTokenPrivileges may return nil even when not all privileges were assigned.
	if errno := windows.GetLastError(); errno == windows.ERROR_NOT_ALL_ASSIGNED {
		return fmt.Errorf("未能启用关机权限")
	}
	return nil
}
