package webview2rt

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

const edgeClientGUID = `{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`

// IsInstalled reports whether a usable WebView2 Evergreen Runtime is present.
func IsInstalled() bool {
	return Version() != ""
}

// Version returns the installed WebView2 runtime version, or empty if missing.
func Version() string {
	roots := []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER}
	paths := []string{
		`SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\` + edgeClientGUID,
		`SOFTWARE\Microsoft\EdgeUpdate\Clients\` + edgeClientGUID,
	}
	for _, root := range roots {
		for _, path := range paths {
			if v := readPV(root, path); v != "" {
				return v
			}
		}
	}
	return ""
}

func readPV(root registry.Key, path string) string {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue("pv")
	if err != nil {
		return ""
	}
	v = strings.TrimSpace(v)
	if v == "" || v == "0.0.0.0" {
		return ""
	}
	return v
}
