package syscmd

import (
	"fmt"
	"strings"
)

// UnblockFile removes Mark-of-the-Web (Zone.Identifier ADS) for the given file.
// This reduces chances of SmartScreen / Windows Defender blocking execution of
// a freshly-downloaded binary.
//
// Best-effort: returns an error if PowerShell execution fails, but callers may
// safely ignore it.
func UnblockFile(path string) error {
	escaped := strings.ReplaceAll(path, "'", "''")
	script := fmt.Sprintf(`
$ErrorActionPreference='SilentlyContinue'
$p='%s'
try {
  if (Test-Path -LiteralPath $p) {
    Unblock-File -LiteralPath $p -ErrorAction SilentlyContinue
  }
} catch {
  # ignore
}
`, escaped)
	_, err := RunPSQuick(script)
	return err
}
