package rdp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"wintoolbox/internal/win/syscmd"
)

const tscClientPath = `Software\Microsoft\Terminal Server Client`

// HistoryEntry is one local RDP client history item for UI display.
type HistoryEntry struct {
	Kind     string `json:"kind"` // mru | server | credential | file | cache
	Host     string `json:"host"`
	Username string `json:"username"`
	Source   string `json:"source"`
	Detail   string `json:"detail"`
	Sid      string `json:"sid"` // registry hive SID for mru/server; empty otherwise
}

// ListConnectionHistory returns mstsc-related history across loaded user hives,
// TERMSRV credentials, and common Default.rdp / recent .rdp / cache files.
// User hives are read only via HKEY_USERS (current SID labeled 当前用户) to avoid duplicates.
func ListConnectionHistory() ([]HistoryEntry, error) {
	var out []HistoryEntry
	var listErrs []string
	seen := map[string]bool{}
	add := func(e HistoryEntry) {
		e.Host = strings.TrimSpace(e.Host)
		e.Username = strings.TrimSpace(e.Username)
		e.Source = strings.TrimSpace(e.Source)
		e.Detail = strings.TrimSpace(e.Detail)
		e.Sid = strings.TrimSpace(e.Sid)
		if e.Kind == "" {
			return
		}
		key := strings.ToLower(e.Kind + "|" + e.Host + "|" + e.Username + "|" + e.Detail + "|" + e.Sid + "|" + e.Source)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, e)
	}

	curSID, _ := currentUserSID()
	entries, hiveErr := listTscAllUsers(curSID)
	if hiveErr != nil {
		listErrs = append(listErrs, hiveErr.Error())
	}
	for _, e := range entries {
		add(e)
	}

	targets, credErr := listTermsrvCredentialTargets()
	if credErr != nil {
		listErrs = append(listErrs, "凭据: "+credErr.Error())
	}
	for _, t := range targets {
		host := t
		if i := strings.Index(strings.ToUpper(t), "TERMSRV/"); i >= 0 {
			host = t[i+len("TERMSRV/"):]
		}
		add(HistoryEntry{
			Kind:   "credential",
			Host:   host,
			Source: "凭据管理器",
			Detail: t,
		})
	}
	for _, p := range listDefaultRdpFiles() {
		add(HistoryEntry{
			Kind:   "file",
			Host:   filepath.Base(p),
			Source: "用户文件",
			Detail: p,
		})
	}
	for _, p := range listRecentRdpShortcuts() {
		add(HistoryEntry{
			Kind:   "file",
			Host:   filepath.Base(p),
			Source: "最近访问",
			Detail: p,
		})
	}
	for _, p := range listBitmapCacheDirs() {
		add(HistoryEntry{
			Kind:   "cache",
			Host:   "位图缓存",
			Source: "本地缓存",
			Detail: p,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return kindOrder(out[i].Kind) < kindOrder(out[j].Kind)
		}
		if !strings.EqualFold(out[i].Host, out[j].Host) {
			return strings.ToLower(out[i].Host) < strings.ToLower(out[j].Host)
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Detail < out[j].Detail
	})

	if len(out) == 0 && len(listErrs) > 0 {
		return nil, fmt.Errorf("读取连接记录失败: %s", strings.Join(listErrs, "; "))
	}
	return out, nil
}

func kindOrder(k string) int {
	switch k {
	case "mru":
		return 0
	case "server":
		return 1
	case "credential":
		return 2
	case "file":
		return 3
	case "cache":
		return 4
	default:
		return 9
	}
}

// ClearConnectionHistory removes local RDP client history and verifies leftovers.
func ClearConnectionHistory() (string, error) {
	before, listErr := ListConnectionHistory()
	if listErr != nil && len(before) == 0 {
		return "", fmt.Errorf("无法枚举连接记录，未执行清理: %w", listErr)
	}
	if len(before) == 0 {
		return "未发现可清理的 RDP 连接记录", nil
	}

	var (
		mru, servers, creds, files, caches int
		failMsgs                           []string
	)

	nMru, nSrv, fails := clearTscAllUsers()
	mru += nMru
	servers += nSrv
	failMsgs = append(failMsgs, fails...)

	cOK, cFail, cErrs := clearTermsrvCredentials()
	creds += cOK
	failMsgs = append(failMsgs, cErrs...)
	_ = cFail

	fOK, fErrs := clearListedFiles(listDefaultRdpFiles())
	files += fOK
	failMsgs = append(failMsgs, fErrs...)
	rOK, rErrs := clearListedFiles(listRecentRdpShortcuts())
	files += rOK
	failMsgs = append(failMsgs, rErrs...)

	kOK, kErrs := clearBitmapCaches()
	caches += kOK
	failMsgs = append(failMsgs, kErrs...)

	detail := fmt.Sprintf("已尝试清理：MRU %d · 主机项 %d · 凭据 %d · 文件 %d · 缓存目录 %d",
		mru, servers, creds, files, caches)

	after, afterErr := ListConnectionHistory()
	if afterErr != nil && len(after) == 0 {
		msg := detail + "；清理后复查失败: " + afterErr.Error()
		return msg, fmt.Errorf("%s", msg)
	}
	if len(after) > 0 {
		msg := fmt.Sprintf("%s；仍残留 %d 条（请检查权限或组策略）", detail, len(after))
		if len(failMsgs) > 0 {
			msg += "；失败: " + strings.Join(failMsgs, "; ")
		}
		return msg, fmt.Errorf("%s", msg)
	}
	if len(failMsgs) > 0 {
		msg := detail + "；部分操作失败但复查已清空: " + strings.Join(failMsgs, "; ")
		return msg, nil
	}
	if mru+servers+creds+files+caches == 0 {
		return "", fmt.Errorf("清理失败：未能删除任何记录（共发现 %d 条）", len(before))
	}
	return detail, nil
}

// DeleteHistoryEntry removes a single listed history item.
// sid scopes mru/server deletes to one user hive (required for safe multi-user machines).
func DeleteHistoryEntry(kind, host, username, detail, sid string) error {
	kind = strings.TrimSpace(kind)
	host = strings.TrimSpace(host)
	username = strings.TrimSpace(username)
	detail = strings.TrimSpace(detail)
	sid = strings.TrimSpace(sid)
	if kind == "" {
		return fmt.Errorf("记录类型无效")
	}

	switch kind {
	case "credential":
		target := detail
		if target == "" {
			target = "TERMSRV/" + host
		}
		targets, err := listTermsrvCredentialTargets()
		if err != nil {
			return fmt.Errorf("枚举凭据失败: %w", err)
		}
		if !stringInFold(targets, target) {
			return fmt.Errorf("凭据不在当前列表中，已拒绝删除")
		}
		if _, err := syscmd.Run("cmdkey", "/delete:"+target); err != nil {
			return fmt.Errorf("删除凭据失败: %w", err)
		}
	case "file":
		if detail == "" {
			return fmt.Errorf("文件路径为空")
		}
		if !pathInAllowed(detail, append(listDefaultRdpFiles(), listRecentRdpShortcuts()...)) {
			return fmt.Errorf("路径不在允许的 RDP 历史文件范围内，已拒绝删除")
		}
		if err := os.Remove(detail); err != nil {
			return fmt.Errorf("删除文件失败: %w", err)
		}
	case "cache":
		if detail == "" {
			return fmt.Errorf("缓存路径为空")
		}
		if !pathInAllowed(detail, listBitmapCacheDirs()) {
			return fmt.Errorf("路径不在允许的 RDP 缓存目录范围内，已拒绝删除")
		}
		if err := os.RemoveAll(detail); err != nil {
			return fmt.Errorf("删除缓存失败: %w", err)
		}
	case "mru", "server":
		if sid == "" {
			// Fall back to current user only — never wipe all profiles.
			if cur, err := currentUserSID(); err == nil {
				sid = cur
			}
		}
		if sid == "" {
			return fmt.Errorf("缺少用户 SID，无法安全删除注册表项")
		}
		if err := deleteTscEntry(kind, host, username, detail, sid); err != nil {
			return err
		}
	default:
		return fmt.Errorf("不支持的记录类型: %s", kind)
	}

	left, _ := ListConnectionHistory()
	for _, e := range left {
		if e.Kind == kind && strings.EqualFold(e.Host, host) &&
			strings.EqualFold(e.Username, username) && e.Detail == detail &&
			(sid == "" || strings.EqualFold(e.Sid, sid)) {
			return fmt.Errorf("已提交删除，但复查仍可见该记录")
		}
	}
	return nil
}

func pathInAllowed(target string, allowed []string) bool {
	abs, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return false
	}
	for _, a := range allowed {
		aa, err := filepath.Abs(filepath.Clean(a))
		if err != nil {
			continue
		}
		if strings.EqualFold(abs, aa) {
			return true
		}
	}
	return false
}

func stringInFold(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

func deleteTscEntry(kind, host, username, detail, sid string) error {
	if !isUserProfileSID(sid) {
		return fmt.Errorf("无效的用户 SID")
	}
	clientPath := sid + `\` + tscClientPath
	switch kind {
	case "mru":
		defPath := clientPath + `\Default`
		def, err := registry.OpenKey(registry.USERS, defPath, registry.SET_VALUE|registry.QUERY_VALUE)
		if err != nil {
			return fmt.Errorf("打开注册表失败: %w", err)
		}
		defer def.Close()
		if detail != "" && strings.EqualFold(detail, "Username") {
			v, _, _ := def.GetStringValue("Username")
			if username != "" && !strings.EqualFold(v, username) {
				return fmt.Errorf("用户名不匹配")
			}
			if delErr := def.DeleteValue("Username"); delErr != nil {
				return fmt.Errorf("删除 Username 失败: %w", delErr)
			}
			return nil
		}
		vals, _ := def.ReadValueNames(-1)
		for _, name := range vals {
			if !strings.HasPrefix(strings.ToUpper(name), "MRU") {
				continue
			}
			if detail != "" && !strings.EqualFold(name, detail) {
				continue
			}
			v, _, err := def.GetStringValue(name)
			if err != nil || !strings.EqualFold(strings.TrimSpace(v), host) {
				continue
			}
			if delErr := def.DeleteValue(name); delErr != nil {
				return fmt.Errorf("删除 %s 失败: %w", name, delErr)
			}
			return nil
		}
		return fmt.Errorf("未找到对应 MRU 项")
	case "server":
		path := clientPath + `\Servers\` + host
		if delErr := registry.DeleteKey(registry.USERS, path); delErr != nil {
			if isNotExistReg(delErr) {
				return fmt.Errorf("未找到对应主机项")
			}
			return fmt.Errorf("删除主机项失败: %w", delErr)
		}
		return nil
	default:
		return fmt.Errorf("不支持的注册表记录类型: %s", kind)
	}
}

func currentUserSID() (string, error) {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return "", err
	}
	defer token.Close()
	tu, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return tu.User.Sid.String(), nil
}

func listTscAllUsers(curSID string) ([]HistoryEntry, error) {
	var out []HistoryEntry
	users, err := registry.OpenKey(registry.USERS, "", registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil, fmt.Errorf("枚举用户配置失败: %w", err)
	}
	defer users.Close()
	names, err := users.ReadSubKeyNames(-1)
	if err != nil {
		return nil, fmt.Errorf("读取用户 SID 失败: %w", err)
	}
	for _, sid := range names {
		if !isUserProfileSID(sid) {
			continue
		}
		label := "用户 " + shortSID(sid)
		if curSID != "" && strings.EqualFold(sid, curSID) {
			label = "当前用户"
		}
		out = append(out, listTscHive(registry.USERS, sid+`\`+tscClientPath, label, sid)...)
	}
	return out, nil
}

func shortSID(sid string) string {
	parts := strings.Split(sid, "-")
	if len(parts) == 0 {
		return sid
	}
	if len(parts) <= 3 {
		return sid
	}
	return "…" + parts[len(parts)-1]
}

func listTscHive(root registry.Key, clientPath, source, sid string) []HistoryEntry {
	var out []HistoryEntry

	defPath := clientPath + `\Default`
	if def, err := registry.OpenKey(root, defPath, registry.QUERY_VALUE); err == nil {
		vals, _ := def.ReadValueNames(-1)
		for _, name := range vals {
			upper := strings.ToUpper(name)
			if !strings.HasPrefix(upper, "MRU") {
				continue
			}
			host, _, err := def.GetStringValue(name)
			if err != nil || strings.TrimSpace(host) == "" {
				continue
			}
			out = append(out, HistoryEntry{
				Kind:   "mru",
				Host:   host,
				Source: source,
				Detail: name,
				Sid:    sid,
			})
		}
		if user, _, err := def.GetStringValue("Username"); err == nil && strings.TrimSpace(user) != "" {
			out = append(out, HistoryEntry{
				Kind:     "mru",
				Host:     "(默认)",
				Username: user,
				Source:   source,
				Detail:   "Username",
				Sid:      sid,
			})
		}
		def.Close()
	}

	srvPath := clientPath + `\Servers`
	srv, err := registry.OpenKey(root, srvPath, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return out
	}
	subnames, err := srv.ReadSubKeyNames(-1)
	srv.Close()
	if err != nil {
		return out
	}
	for _, host := range subnames {
		uname := ""
		hintPath := srvPath + `\` + host
		if sk, err := registry.OpenKey(root, hintPath, registry.QUERY_VALUE); err == nil {
			if u, _, err := sk.GetStringValue("UsernameHint"); err == nil {
				uname = u
			}
			sk.Close()
		}
		out = append(out, HistoryEntry{
			Kind:     "server",
			Host:     host,
			Username: uname,
			Source:   source,
			Detail:   "Servers",
			Sid:      sid,
		})
	}
	return out
}

func clearTscAllUsers() (mru, servers int, errs []string) {
	users, err := registry.OpenKey(registry.USERS, "", registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		errs = append(errs, "枚举用户配置失败: "+err.Error())
		return
	}
	defer users.Close()

	names, err := users.ReadSubKeyNames(-1)
	if err != nil {
		errs = append(errs, "读取用户 SID 失败: "+err.Error())
		return
	}
	for _, sid := range names {
		if !isUserProfileSID(sid) {
			continue
		}
		nMru, nSrv, failMsgs := clearTscHive(registry.USERS, sid+`\`+tscClientPath)
		mru += nMru
		servers += nSrv
		for _, m := range failMsgs {
			errs = append(errs, shortSID(sid)+": "+m)
		}
	}
	return
}

func isUserProfileSID(sid string) bool {
	if sid == "" || strings.Contains(sid, "_Classes") {
		return false
	}
	if sid == ".DEFAULT" || sid == "S-1-5-18" || sid == "S-1-5-19" || sid == "S-1-5-20" {
		return false
	}
	return strings.HasPrefix(sid, "S-1-5-21-")
}

func isNotExistReg(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "The system cannot find the file specified") ||
		strings.Contains(s, "找不到") ||
		err == registry.ErrNotExist
}

func clearTscHive(root registry.Key, clientPath string) (mruCount, serverCount int, failMsgs []string) {
	defPath := clientPath + `\Default`
	if def, openErr := registry.OpenKey(root, defPath, registry.SET_VALUE|registry.QUERY_VALUE); openErr == nil {
		vals, _ := def.ReadValueNames(-1)
		for _, name := range vals {
			upper := strings.ToUpper(name)
			if strings.HasPrefix(upper, "MRU") || upper == "USERNAME" {
				if delErr := def.DeleteValue(name); delErr == nil {
					mruCount++
				} else {
					failMsgs = append(failMsgs, "删除 "+name+": "+delErr.Error())
				}
			}
		}
		def.Close()
	} else if !isNotExistReg(openErr) {
		failMsgs = append(failMsgs, "打开 Default: "+openErr.Error())
	}

	srvPath := clientPath + `\Servers`
	srv, openErr := registry.OpenKey(root, srvPath, registry.ENUMERATE_SUB_KEYS)
	if openErr != nil {
		if !isNotExistReg(openErr) {
			failMsgs = append(failMsgs, "打开 Servers: "+openErr.Error())
		}
		return
	}
	subnames, readErr := srv.ReadSubKeyNames(-1)
	srv.Close()
	if readErr != nil {
		failMsgs = append(failMsgs, "枚举 Servers: "+readErr.Error())
		return
	}
	for _, host := range subnames {
		if delErr := registry.DeleteKey(root, srvPath+`\`+host); delErr == nil {
			serverCount++
		} else {
			failMsgs = append(failMsgs, "删除主机 "+host+": "+delErr.Error())
		}
	}
	return
}

func listTermsrvCredentialTargets() ([]string, error) {
	out, err := syscmd.Run("cmdkey", "/list")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var targets []string
	for _, line := range strings.Split(out, "\n") {
		u := strings.ToUpper(line)
		idx := strings.Index(u, "TERMSRV/")
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(line[idx:])
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		t := strings.Trim(fields[0], " ,\t\r\"")
		key := strings.ToUpper(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		targets = append(targets, t)
	}
	return targets, nil
}

func clearTermsrvCredentials() (ok, fail int, errs []string) {
	targets, err := listTermsrvCredentialTargets()
	if err != nil {
		errs = append(errs, "列举凭据失败: "+err.Error())
		return
	}
	for _, t := range targets {
		if _, delErr := syscmd.Run("cmdkey", "/delete:"+t); delErr == nil {
			ok++
		} else {
			fail++
			errs = append(errs, "凭据 "+t+": "+delErr.Error())
		}
	}
	return
}

func listDefaultRdpFiles() []string {
	var out []string
	for _, home := range userProfileDirs() {
		for _, rel := range []string{
			`Documents\Default.rdp`,
			`文档\Default.rdp`,
		} {
			p := filepath.Join(home, rel)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				out = append(out, p)
			}
		}
	}
	return out
}

func listRecentRdpShortcuts() []string {
	var out []string
	for _, home := range userProfileDirs() {
		recent := filepath.Join(home, `AppData\Roaming\Microsoft\Windows\Recent`)
		entries, err := os.ReadDir(recent)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".rdp") || strings.HasSuffix(lower, ".rdp.lnk") {
				out = append(out, filepath.Join(recent, name))
			}
		}
	}
	return out
}

func clearListedFiles(paths []string) (ok int, errs []string) {
	for _, p := range paths {
		if err := os.Remove(p); err == nil {
			ok++
		} else {
			errs = append(errs, p+": "+err.Error())
		}
	}
	return
}

func listBitmapCacheDirs() []string {
	var out []string
	for _, home := range userProfileDirs() {
		p := filepath.Join(home, `AppData\Local\Microsoft\Terminal Server Client\Cache`)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			// Only list if non-empty.
			ents, err := os.ReadDir(p)
			if err == nil && len(ents) > 0 {
				out = append(out, p)
			}
		}
	}
	return out
}

func clearBitmapCaches() (ok int, errs []string) {
	for _, p := range listBitmapCacheDirs() {
		if err := os.RemoveAll(p); err == nil {
			ok++
		} else {
			errs = append(errs, p+": "+err.Error())
		}
	}
	return
}

func userProfileDirs() []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if seen[strings.ToLower(p)] {
			return
		}
		seen[strings.ToLower(p)] = true
		out = append(out, p)
	}
	if p, err := os.UserHomeDir(); err == nil {
		add(p)
	}
	add(os.Getenv("USERPROFILE"))
	root := os.Getenv("SystemDrive") + `\Users`
	entries, err := os.ReadDir(root)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if name == "Public" || name == "Default" || name == "Default User" || name == "All Users" {
				continue
			}
			add(filepath.Join(root, name))
		}
	}
	return out
}
