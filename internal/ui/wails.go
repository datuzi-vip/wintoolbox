package ui

import (
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// Run starts the Wails desktop shell with embedded frontend assets.
func Run(assets embed.FS) error {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:     "WinToolbox",
		Width:     1120,
		Height:    720,
		MinWidth:  960,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 250, A: 255},
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			Theme:                windows.SystemDefault,
			Messages: &windows.Messages{
				InstallationRequired: "本程序需要 WebView2 运行时。正在尝试自动安装…",
				UpdateRequired:       "WebView2 运行时版本过低，需要更新。",
				MissingRequirements:  "缺少运行组件",
				Webview2NotInstalled: "未安装 WebView2 运行时",
				Error:                "错误",
				FailedToInstall:      "WebView2 安装失败",
				DownloadPage:         "请前往下载页面安装 WebView2：",
				PressOKToInstall:     "按“确定”开始安装。",
				ContactAdmin:         "请联系管理员安装 WebView2 运行时。",
				InvalidFixedWebview2: "指定的 WebView2 路径无效。",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("wails: %w", err)
	}
	return nil
}
