package webview2rt

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32               = windows.NewLazySystemDLL("user32.dll")
	modKernel32             = windows.NewLazySystemDLL("kernel32.dll")
	modComctl32             = windows.NewLazySystemDLL("comctl32.dll")
	procCreateWindowExW     = modUser32.NewProc("CreateWindowExW")
	procDestroyWindow       = modUser32.NewProc("DestroyWindow")
	procShowWindow          = modUser32.NewProc("ShowWindow")
	procUpdateWindow        = modUser32.NewProc("UpdateWindow")
	procGetMessageW         = modUser32.NewProc("GetMessageW")
	procTranslateMessage    = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW    = modUser32.NewProc("DispatchMessageW")
	procPostMessageW        = modUser32.NewProc("PostMessageW")
	procSendMessageW        = modUser32.NewProc("SendMessageW")
	procPostQuitMessage     = modUser32.NewProc("PostQuitMessage")
	procDefWindowProcW      = modUser32.NewProc("DefWindowProcW")
	procRegisterClassExW    = modUser32.NewProc("RegisterClassExW")
	procGetModuleHandleW    = modKernel32.NewProc("GetModuleHandleW")
	procInitCommonControlsE = modComctl32.NewProc("InitCommonControlsEx")
)

const (
	wsVisible      = 0x10000000
	wsChild        = 0x40000000
	wsCaption      = 0x00C00000
	wsSysMenu      = 0x00080000
	wsMinimizeBox  = 0x00020000
	wsClipSiblings = 0x04000000

	swShow = 5

	wmDestroy = 0x0002
	wmClose   = 0x0010
	wmSetText = 0x000C
	wmUser    = 0x0400

	wmAppClose = wmUser + 4

	pbmSetPos     = wmUser + 2
	pbmSetRange32 = wmUser + 6
	pbmSetMarquee = wmUser + 10
	pbsSmooth     = 0x01
	pbsMarquee    = 0x08

	iccProgressClass = 0x00000020
	colorWindow      = 5
)

type progressUI struct {
	hwnd   windows.Handle
	text   windows.Handle
	bar    windows.Handle
	ready  chan struct{}
	done   chan struct{}
	mu     sync.Mutex
	closed bool
	last   *uint16 // keeps last status string alive across SendMessage
}

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

type msg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type initCommonControlsEx struct {
	size uint32
	icc  uint32
}

var (
	progressClassOnce sync.Once
	progressClassName = windows.StringToUTF16Ptr("WinToolboxWV2Progress")
	progressWndProc   = syscall.NewCallback(progressDlgProc)
	progressOwner     *progressUI
	progressOwnerMu   sync.Mutex
)

func initCommonControls() {
	icc := initCommonControlsEx{size: uint32(unsafe.Sizeof(initCommonControlsEx{})), icc: iccProgressClass}
	procInitCommonControlsE.Call(uintptr(unsafe.Pointer(&icc)))
}

func registerProgressClass() {
	progressClassOnce.Do(func() {
		initCommonControls()
		hInst, _, _ := procGetModuleHandleW.Call(0)
		var wc wndClassEx
		wc.size = uint32(unsafe.Sizeof(wc))
		wc.wndProc = progressWndProc
		wc.instance = windows.Handle(hInst)
		wc.background = windows.Handle(colorWindow + 1)
		wc.className = progressClassName
		procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	})
}

func openProgress(title, status string) *progressUI {
	registerProgressClass()
	ui := &progressUI{
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(ui.done)

		hInst, _, _ := procGetModuleHandleW.Call(0)
		titlePtr, _ := windows.UTF16PtrFromString(title)
		cwDefault := uintptr(0x80000000) // CW_USEDEFAULT
		hwnd, _, _ := procCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(progressClassName)),
			uintptr(unsafe.Pointer(titlePtr)),
			uintptr(wsCaption|wsSysMenu|wsMinimizeBox|wsClipSiblings|wsVisible),
			cwDefault,
			cwDefault,
			420,
			160,
			0, 0, hInst, 0,
		)
		ui.hwnd = windows.Handle(hwnd)
		if hwnd == 0 {
			close(ui.ready)
			return
		}

		progressOwnerMu.Lock()
		progressOwner = ui
		progressOwnerMu.Unlock()
		defer func() {
			progressOwnerMu.Lock()
			if progressOwner == ui {
				progressOwner = nil
			}
			progressOwnerMu.Unlock()
		}()

		statusPtr, _ := windows.UTF16PtrFromString(status)
		staticClass, _ := windows.UTF16PtrFromString("STATIC")
		textHwnd, _, _ := procCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(staticClass)),
			uintptr(unsafe.Pointer(statusPtr)),
			uintptr(wsChild|wsVisible),
			16, 18, 372, 40,
			hwnd, 0, hInst, 0,
		)
		ui.text = windows.Handle(textHwnd)

		progClass, _ := windows.UTF16PtrFromString("msctls_progress32")
		barHwnd, _, _ := procCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(progClass)),
			0,
			uintptr(wsChild|wsVisible|pbsSmooth|pbsMarquee),
			16, 70, 372, 22,
			hwnd, 0, hInst, 0,
		)
		ui.bar = windows.Handle(barHwnd)
		procSendMessageW.Call(uintptr(ui.bar), pbmSetRange32, 0, 100)

		procShowWindow.Call(hwnd, swShow)
		procUpdateWindow.Call(hwnd)
		close(ui.ready)

		var m msg
		for {
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(ret) <= 0 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
	<-ui.ready
	return ui
}

func progressDlgProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmAppClose, wmClose:
		procDestroyWindow.Call(uintptr(hwnd))
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func (p *progressUI) setText(text string) {
	if p == nil || p.text == 0 {
		return
	}
	ptr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	p.mu.Lock()
	p.last = ptr
	p.mu.Unlock()
	procSendMessageW.Call(uintptr(p.text), wmSetText, 0, uintptr(unsafe.Pointer(ptr)))
}

func (p *progressUI) SetStatus(text string) {
	if p == nil || p.hwnd == 0 {
		return
	}
	p.setText(text)
}

func (p *progressUI) SetProgress(done, total int64) {
	if p == nil || p.hwnd == 0 || p.bar == 0 {
		return
	}
	if total <= 0 {
		p.setText(fmt.Sprintf("正在下载 WebView2 运行时… 已下载 %s", formatBytes(done)))
		procSendMessageW.Call(uintptr(p.bar), pbmSetMarquee, 1, 30)
		return
	}
	pct := int(done * 100 / total)
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	p.setText(fmt.Sprintf("正在下载 WebView2 运行时… %d%%（%s / %s）", pct, formatBytes(done), formatBytes(total)))
	procSendMessageW.Call(uintptr(p.bar), pbmSetMarquee, 0, 0)
	procSendMessageW.Call(uintptr(p.bar), pbmSetPos, uintptr(pct), 0)
}

func (p *progressUI) SetMarquee(text string) {
	if p == nil || p.hwnd == 0 || p.bar == 0 {
		return
	}
	p.setText(text)
	procSendMessageW.Call(uintptr(p.bar), pbmSetMarquee, 1, 30)
}

func (p *progressUI) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	if p.hwnd != 0 {
		procPostMessageW.Call(uintptr(p.hwnd), wmAppClose, 0, 0)
	}
	<-p.done
}

func formatBytes(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
	)
	switch {
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.0f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
