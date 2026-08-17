package elevate

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"

	"wintoolbox/internal/win/dialog"
)

var (
	modShell32       = windows.NewLazySystemDLL("shell32.dll")
	procShellExecute = modShell32.NewProc("ShellExecuteW")
)

// IsAdmin reports whether the current process is elevated.
func IsAdmin() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()

	var elevation uint32
	var returned uint32
	err = windows.GetTokenInformation(
		token,
		windows.TokenElevation,
		(*byte)(unsafe.Pointer(&elevation)),
		uint32(unsafe.Sizeof(elevation)),
		&returned,
	)
	if err != nil {
		return false
	}
	return elevation != 0
}

// RelaunchAsAdmin starts a new elevated instance via UAC and returns whether launch was attempted.
func RelaunchAsAdmin() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return false
	}
	exePtr, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return false
	}
	cwd, _ := os.Getwd()
	cwdPtr, _ := windows.UTF16PtrFromString(cwd)

	const swShowNormal = 1
	ret, _, _ := procShellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exePtr)),
		0,
		uintptr(unsafe.Pointer(cwdPtr)),
		swShowNormal,
	)
	// ShellExecute returns value > 32 on success.
	return ret > 32
}

// EnsureAdmin relaunches elevated via UAC, or shows a message and exits.
func EnsureAdmin() {
	if IsAdmin() {
		return
	}
	if RelaunchAsAdmin() {
		os.Exit(0)
	}
	dialog.MessageBox("WinToolbox", "本工具需要管理员权限才能运行。\n请右键以管理员身份重新打开。", true)
	os.Exit(1)
}
