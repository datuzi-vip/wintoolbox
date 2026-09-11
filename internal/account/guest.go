package account

import (
	"fmt"
	"syscall"
	"unsafe"
)

const guestRID = 501

// GuestStatus describes the built-in Guest account.
type GuestStatus struct {
	Exists  bool
	Name    string
	Enabled bool
	Unknown bool
	Detail  string
}

// GetGuestStatus locates the built-in Guest account (RID 501) and reports enable state.
func GetGuestStatus() GuestStatus {
	name, err := findGuestAccount()
	if err != nil || name == "" {
		return GuestStatus{Unknown: true, Detail: "未找到来宾账户"}
	}
	en, err := isEnabled(name)
	if err != nil {
		return GuestStatus{Exists: true, Name: name, Unknown: true, Detail: err.Error()}
	}
	detail := name + " · 已禁用"
	if en {
		detail = name + " · 已启用"
	}
	return GuestStatus{Exists: true, Name: name, Enabled: en, Detail: detail}
}

// DisableGuest disables the built-in Guest account.
func DisableGuest() error {
	name, err := findGuestAccount()
	if err != nil || name == "" {
		return fmt.Errorf("未找到来宾账户")
	}
	if err := SetEnabled(name, false); err != nil {
		return fmt.Errorf("禁用来宾账户失败: %w", err)
	}
	st := GetGuestStatus()
	if st.Exists && !st.Unknown && st.Enabled {
		return fmt.Errorf("已提交禁用，但复查仍显示来宾账户已启用（可能被组策略覆盖）")
	}
	return nil
}

func findGuestAccount() (string, error) {
	for _, name := range []string{"Guest", "来宾"} {
		if rid, err := userRID(name); err == nil && rid == guestRID {
			return name, nil
		}
	}
	names, err := ListLocalUsers()
	if err != nil {
		return "", err
	}
	for _, name := range names {
		if rid, err := userRID(name); err == nil && rid == guestRID {
			return name, nil
		}
	}
	return "", fmt.Errorf("未找到来宾账户")
}

type userInfo3 struct {
	Name            *uint16
	Password        *uint16
	PasswordAge     uint32
	Priv            uint32
	HomeDir         *uint16
	Comment         *uint16
	Flags           uint32
	ScriptPath      *uint16
	AuthFlags       uint32
	FullName        *uint16
	UsrComment      *uint16
	Parms           *uint16
	Workstations    *uint16
	LastLogon       uint32
	LastLogoff      uint32
	AcctExpires     uint32
	MaxStorage      uint32
	UnitsPerWeek    uint32
	LogonHours      *byte
	BadPwCount      uint32
	NumLogons       uint32
	LogonServer     *uint16
	CountryCode     uint32
	CodePage        uint32
	UserId          uint32
	PrimaryGroupId  uint32
	Profile         *uint16
	HomeDirDrive    *uint16
	PasswordExpired uint32
}

func userRID(username string) (uint32, error) {
	userPtr, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return 0, err
	}
	var buf *userInfo3
	r0, _, _ := procNetUserGetInfo.Call(
		0,
		uintptr(unsafe.Pointer(userPtr)),
		3,
		uintptr(unsafe.Pointer(&buf)),
	)
	if r0 != 0 {
		return 0, fmt.Errorf("%s", mapNetError(r0))
	}
	defer procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buf)))
	return buf.UserId, nil
}
