package webview2rt

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const bootstrapperURL = `https://go.microsoft.com/fwlink/p/?LinkId=2124703`

type progressFunc func(done, total int64)

func downloadBootstrapper(dest string, onProgress progressFunc) (err error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, bootstrapperURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "WinToolbox/1.0 (WebView2 bootstrapper)")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".partial"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		if out != nil {
			_ = out.Close()
		}
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()

	total := resp.ContentLength
	var done int64
	buf := make([]byte, 32*1024)
	lastReport := time.Time{}
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				err = werr
				return err
			}
			done += int64(n)
			if onProgress != nil {
				now := time.Now()
				if now.Sub(lastReport) >= 100*time.Millisecond || (total > 0 && done >= total) {
					onProgress(done, total)
					lastReport = now
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			err = fmt.Errorf("下载中断: %w", readErr)
			return err
		}
	}
	if cerr := out.Close(); cerr != nil {
		err = cerr
		return err
	}
	out = nil
	_ = os.Remove(dest)
	if rerr := os.Rename(tmp, dest); rerr != nil {
		err = rerr
		return err
	}
	if onProgress != nil {
		onProgress(done, total)
	}
	return nil
}

func installerPath() string {
	return filepath.Join(os.TempDir(), "WinToolbox_MicrosoftEdgeWebview2Setup.exe")
}
