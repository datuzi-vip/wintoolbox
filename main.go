package main

import (
	"embed"
	"log"

	"wintoolbox/internal/ui"
	"wintoolbox/internal/win/dialog"
	"wintoolbox/internal/win/elevate"
	"wintoolbox/internal/win/webview2rt"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	elevate.EnsureAdmin()
	if err := webview2rt.Ensure(); err != nil {
		dialog.MessageBox("WinToolbox", "无法准备 WebView2 运行时：\n"+err.Error(), true)
		log.Fatal(err)
	}
	if err := ui.Run(assets); err != nil {
		log.Fatal(err)
	}
}
