package syscmd

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	defaultRunTimeout = 45 * time.Second
	defaultPSTimeout  = 90 * time.Second
	quickRunTimeout   = 8 * time.Second
	quickPSTimeout    = 12 * time.Second
)

// Run runs a command hidden and returns combined stdout/stderr text.
func Run(name string, args ...string) (string, error) {
	return runWithTimeout(defaultRunTimeout, name, args...)
}

// RunQuick runs a command with a short timeout (best-effort / non-critical paths).
func RunQuick(name string, args ...string) (string, error) {
	return runWithTimeout(quickRunTimeout, name, args...)
}

// RunPS runs a PowerShell command hidden.
func RunPS(script string) (string, error) {
	return runWithTimeout(defaultPSTimeout, "powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
}

// RunPSQuick runs PowerShell with a short timeout.
func RunPSQuick(script string) (string, error) {
	return runWithTimeout(quickPSTimeout, "powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
}

func runWithTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := decodeACP(buf.Bytes())
	if ctx.Err() == context.DeadlineExceeded {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = fmt.Sprintf("命令超时（%v）", timeout)
		} else {
			msg = fmt.Sprintf("命令超时（%v）: %s", timeout, msg)
		}
		return out, fmt.Errorf("%s", msg)
	}
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("%s", msg)
	}
	return out, nil
}

func decodeACP(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	multiByteToWideChar := kernel32.NewProc("MultiByteToWideChar")
	const cpACP = 0
	r0, _, _ := multiByteToWideChar.Call(cpACP, 0, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0, 0)
	needed := int(r0)
	if needed <= 0 {
		return string(b)
	}
	wide := make([]uint16, needed)
	multiByteToWideChar.Call(
		cpACP, 0,
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)),
		uintptr(unsafe.Pointer(&wide[0])), uintptr(needed),
	)
	return windows.UTF16ToString(wide)
}
