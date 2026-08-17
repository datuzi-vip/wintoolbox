package webview2rt

import (
	"fmt"
	"path/filepath"
	"strings"

	"wintoolbox/internal/win/syscmd"
)

// verifyMicrosoftSigned checks Authenticode is Valid and signer is Microsoft Corporation.
func verifyMicrosoftSigned(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("解析安装包路径失败: %w", err)
	}
	escaped := strings.ReplaceAll(abs, "'", "''")
	script := fmt.Sprintf(`
$ErrorActionPreference='Stop'
$p='%s'
if (-not (Test-Path -LiteralPath $p)) { throw 'installer file missing' }
$s = Get-AuthenticodeSignature -FilePath $p
if ($s.Status -ne 'Valid') { throw ('Authenticode status: ' + $s.Status) }
$sub = [string]$s.SignerCertificate.Subject
if ($sub -notmatch '(?i)O=Microsoft Corporation') { throw ('unexpected signer: ' + $sub) }
Write-Output 'OK'
`, escaped)
	out, err := syscmd.RunPS(script)
	if err != nil || !strings.Contains(out, "OK") {
		msg := strings.TrimSpace(out)
		if msg == "" && err != nil {
			msg = err.Error()
		}
		if msg == "" {
			msg = "未知错误"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
