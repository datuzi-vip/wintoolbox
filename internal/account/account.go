package account

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"wintoolbox/internal/win/syscmd"
)

var (
	modNetapi32          = windows.NewLazySystemDLL("netapi32.dll")
	procNetUserEnum      = modNetapi32.NewProc("NetUserEnum")
	procNetUserGetInfo   = modNetapi32.NewProc("NetUserGetInfo")
	procNetUserSetInfo   = modNetapi32.NewProc("NetUserSetInfo")
	procNetUserModalsGet = modNetapi32.NewProc("NetUserModalsGet")
	procNetUserModalsSet = modNetapi32.NewProc("NetUserModalsSet")
	procNetApiBufferFree = modNetapi32.NewProc("NetApiBufferFree")
)

type userInfo0 struct {
	Name *uint16
}

type userInfo1 struct {
	Name        *uint16
	Password    *uint16
	PasswordAge uint32
	Priv        uint32
	HomeDir     *uint16
	Comment     *uint16
	Flags       uint32
	ScriptPath  *uint16
}

type userInfo1003 struct {
	Password *uint16
}

type userInfo1008 struct {
	Flags uint32
}

const (
	filterNormalAccount = 0x0002
	maxPreferredLength  = 0xFFFFFFFF
	ufAccountDisable    = 0x0002
	ufScript            = 0x0001
)

// Info describes a local user account.
type Info struct {
	Name           string
	Enabled        bool
	EnabledUnknown bool
	Admin          bool
	AdminUnknown   bool
}

// CurrentUsername returns the current Windows logon user name.
func CurrentUsername() string {
	if u := strings.TrimSpace(os.Getenv("USERNAME")); u != "" {
		return u
	}
	if u, err := user.Current(); err == nil && u != nil {
		name := u.Username
		if i := strings.LastIndex(name, `\`); i >= 0 && i+1 < len(name) {
			return name[i+1:]
		}
		return name
	}
	return ""
}

// ListLocalUsers returns local normal user account names.
func ListLocalUsers() ([]string, error) {
	var buf *userInfo0
	var entriesRead, totalEntries, resumeHandle uint32

	r0, _, _ := procNetUserEnum.Call(
		0,
		0,
		filterNormalAccount,
		uintptr(unsafe.Pointer(&buf)),
		maxPreferredLength,
		uintptr(unsafe.Pointer(&entriesRead)),
		uintptr(unsafe.Pointer(&totalEntries)),
		uintptr(unsafe.Pointer(&resumeHandle)),
	)
	if r0 != 0 {
		return nil, fmt.Errorf("枚举本地用户失败，错误码 %d", r0)
	}
	defer procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buf)))

	if entriesRead == 0 || buf == nil {
		return []string{}, nil
	}

	users := make([]string, 0, entriesRead)
	for _, item := range unsafe.Slice(buf, entriesRead) {
		if item.Name == nil {
			continue
		}
		name := windows.UTF16PtrToString(item.Name)
		if name == "" {
			continue
		}
		users = append(users, name)
	}
	return users, nil
}

// ListAccounts returns local users with enabled/admin flags.
func ListAccounts() ([]Info, error) {
	names, err := ListLocalUsers()
	if err != nil {
		return nil, err
	}
	admins, adminOK := localAdminSet()
	out := make([]Info, len(names))
	var wg sync.WaitGroup
	wg.Add(len(names))
	for i, name := range names {
		i, name := i, name
		go func() {
			defer wg.Done()
			enabled := false
			enabledUnknown := false
			if en, err := isEnabled(name); err == nil {
				enabled = en
			} else {
				enabledUnknown = true
			}
			out[i] = Info{
				Name:           name,
				Enabled:        enabled,
				EnabledUnknown: enabledUnknown,
				Admin:          adminOK && admins[strings.ToLower(name)],
				AdminUnknown:   !adminOK,
			}
		}()
	}
	wg.Wait()
	return out, nil
}

// Get reads enabled state and Administrators membership for a user.
func Get(username string) (Info, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return Info{}, fmt.Errorf("用户名不能为空")
	}
	enabled, err := isEnabled(username)
	if err != nil {
		return Info{}, err
	}
	return Info{
		Name:    username,
		Enabled: enabled,
		Admin:   isLocalAdmin(username),
	}, nil
}

func localAdminSet() (map[string]bool, bool) {
	out, err := syscmd.Run("net", "localgroup", "Administrators")
	if err != nil {
		out, err = syscmd.Run("net", "localgroup", "管理员")
		if err != nil {
			return map[string]bool{}, false
		}
	}
	set := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || strings.HasPrefix(name, "-") || strings.Contains(name, "命令成功") {
			continue
		}
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "the command") || strings.HasPrefix(lower, "alias") ||
			strings.HasPrefix(lower, "comment") || strings.HasPrefix(lower, "members") ||
			strings.Contains(name, "别名") || strings.Contains(name, "注释") || strings.Contains(name, "成员") {
			continue
		}
		if i := strings.LastIndex(name, `\`); i >= 0 {
			name = name[i+1:]
		}
		set[strings.ToLower(name)] = true
	}
	return set, true
}

func isEnabled(username string) (bool, error) {
	userPtr, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return false, err
	}
	var buf *userInfo1
	r0, _, _ := procNetUserGetInfo.Call(
		0,
		uintptr(unsafe.Pointer(userPtr)),
		1,
		uintptr(unsafe.Pointer(&buf)),
	)
	if r0 != 0 {
		return false, fmt.Errorf("%s", mapNetError(r0))
	}
	defer procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buf)))
	return buf.Flags&ufAccountDisable == 0, nil
}

func isLocalAdmin(username string) bool {
	out, err := syscmd.Run("net", "localgroup", "Administrators")
	if err != nil {
		out, err = syscmd.Run("net", "localgroup", "管理员")
		if err != nil {
			return false
		}
	}
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || strings.HasPrefix(name, "-") || strings.Contains(name, "命令成功") {
			continue
		}
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "the command") || strings.HasPrefix(lower, "alias") ||
			strings.HasPrefix(lower, "comment") || strings.HasPrefix(lower, "members") ||
			strings.Contains(name, "别名") || strings.Contains(name, "注释") || strings.Contains(name, "成员") {
			continue
		}
		if strings.EqualFold(name, username) {
			return true
		}
		// DOMAIN\user form
		if i := strings.LastIndex(name, `\`); i >= 0 && strings.EqualFold(name[i+1:], username) {
			return true
		}
	}
	return false
}

// SetEnabled enables or disables a local user account.
func SetEnabled(username string, enabled bool) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if strings.EqualFold(username, CurrentUsername()) && !enabled {
		return fmt.Errorf("不能禁用当前登录用户")
	}

	userPtr, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	var buf *userInfo1
	r0, _, _ := procNetUserGetInfo.Call(
		0,
		uintptr(unsafe.Pointer(userPtr)),
		1,
		uintptr(unsafe.Pointer(&buf)),
	)
	if r0 != 0 {
		return fmt.Errorf("%s", mapNetError(r0))
	}
	defer procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buf)))

	flags := buf.Flags
	if enabled {
		flags &^= ufAccountDisable
	} else {
		flags |= ufAccountDisable
	}
	// Keep UF_SCRIPT bit commonly required for local accounts.
	flags |= ufScript

	payload := userInfo1008{Flags: flags}
	r0, _, _ = procNetUserSetInfo.Call(
		0,
		uintptr(unsafe.Pointer(userPtr)),
		1008,
		uintptr(unsafe.Pointer(&payload)),
		0,
	)
	if r0 != 0 {
		return fmt.Errorf("%s", mapNetError(r0))
	}
	return nil
}

// SetAdmin adds or removes the user from the local Administrators group.
func SetAdmin(username string, admin bool) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if strings.EqualFold(username, CurrentUsername()) && !admin {
		return fmt.Errorf("不能移除当前登录用户的管理员权限")
	}

	group := "Administrators"
	action := "/add"
	if !admin {
		action = "/delete"
	}
	_, err := syscmd.Run("net", "localgroup", group, username, action)
	if err != nil {
		groupCN := "管理员"
		_, err2 := syscmd.Run("net", "localgroup", groupCN, username, action)
		if err2 != nil {
			return fmt.Errorf("调整管理员组成员失败: %v", err)
		}
	}
	return nil
}

// SetPassword sets the password for a local user account.
func SetPassword(username, newPassword string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if newPassword == "" {
		return fmt.Errorf("密码不能为空")
	}

	userPtr, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	passPtr, err := syscall.UTF16PtrFromString(newPassword)
	if err != nil {
		return err
	}

	info := userInfo1003{Password: passPtr}
	r0, _, _ := procNetUserSetInfo.Call(
		0,
		uintptr(unsafe.Pointer(userPtr)),
		1003,
		uintptr(unsafe.Pointer(&info)),
		0,
	)
	if r0 != 0 {
		return fmt.Errorf("%s", mapNetError(r0))
	}
	return nil
}

func mapNetError(code uintptr) string {
	switch code {
	case 5:
		return "拒绝访问，请以管理员身份运行"
	case 86:
		return "当前密码不正确"
	case 1326:
		return "用户名或密码不正确"
	case 2221:
		return "指定的用户不存在"
	case 2245:
		return "密码不符合系统复杂度策略"
	case 2246:
		return "密码太短，不符合策略要求"
	default:
		return fmt.Sprintf("账户操作失败，错误码 %d", code)
	}
}
