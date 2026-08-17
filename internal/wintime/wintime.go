package wintime

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"

	"wintoolbox/internal/win/syscmd"
)

// Status holds current time zone and clock info.
type Status struct {
	TimeZone    string
	LocalTime   string
	UTCTime     string
	NTPServer   string
	ServiceHint string
}

// GetStatus reads timezone, local/UTC time and configured NTP peer.
func GetStatus() Status {
	st := Status{
		LocalTime: time.Now().Format("2006-01-02 15:04:05"),
		UTCTime:   time.Now().UTC().Format("2006-01-02 15:04:05"),
		TimeZone:  currentTimeZone(),
		NTPServer: ntpServer(),
	}
	out, err := syscmd.Run("sc", "qc", "W32Time")
	if err == nil {
		lower := strings.ToLower(out)
		switch {
		case strings.Contains(lower, "auto_start") || strings.Contains(out, "自动"):
			st.ServiceHint = "时间服务=自动"
		case strings.Contains(lower, "demand_start") || strings.Contains(out, "手动"):
			st.ServiceHint = "时间服务=手动"
		case strings.Contains(lower, "disabled") || strings.Contains(out, "已禁用"):
			st.ServiceHint = "时间服务=已禁用"
		default:
			st.ServiceHint = "时间服务=未知"
		}
	}
	return st
}

// CurrentTimeZone returns the active Windows timezone ID (tzutil /g).
func CurrentTimeZone() string {
	return currentTimeZone()
}

func currentTimeZone() string {
	out, err := syscmd.Run("tzutil", "/g")
	if err != nil {
		return "-"
	}
	return strings.TrimSpace(out)
}

func ntpServer() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\W32Time\Parameters`, registry.QUERY_VALUE)
	if err != nil {
		return "-"
	}
	defer k.Close()
	v, _, err := k.GetStringValue("NtpServer")
	if err != nil || strings.TrimSpace(v) == "" {
		return "-"
	}
	// value like "time.windows.com,0x9"
	return strings.TrimSpace(strings.Split(v, ",")[0])
}

// ZoneOption is a timezone entry for UI selection.
// ID is the Windows tzutil identifier; Label follows international display style.
type ZoneOption struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Offset string `json:"offset"`
}

// CommonZones returns frequently used zones with UTC-offset international labels.
func CommonZones() []ZoneOption {
	return []ZoneOption{
		{ID: "UTC", Offset: "UTC±00:00", Label: "(UTC±00:00) Coordinated Universal Time / UTC"},
		{ID: "GMT Standard Time", Offset: "UTC±00:00", Label: "(UTC±00:00) Dublin, Edinburgh, Lisbon, London"},
		{ID: "W. Europe Standard Time", Offset: "UTC+01:00", Label: "(UTC+01:00) Amsterdam, Berlin, Bern, Rome, Stockholm, Vienna"},
		{ID: "Central Europe Standard Time", Offset: "UTC+01:00", Label: "(UTC+01:00) Belgrade, Bratislava, Budapest, Ljubljana, Prague"},
		{ID: "Romance Standard Time", Offset: "UTC+01:00", Label: "(UTC+01:00) Brussels, Copenhagen, Madrid, Paris"},
		{ID: "GTB Standard Time", Offset: "UTC+02:00", Label: "(UTC+02:00) Athens, Bucharest"},
		{ID: "Russian Standard Time", Offset: "UTC+03:00", Label: "(UTC+03:00) Moscow, St. Petersburg"},
		{ID: "Arabian Standard Time", Offset: "UTC+04:00", Label: "(UTC+04:00) Abu Dhabi, Muscat"},
		{ID: "Pakistan Standard Time", Offset: "UTC+05:00", Label: "(UTC+05:00) Islamabad, Karachi"},
		{ID: "India Standard Time", Offset: "UTC+05:30", Label: "(UTC+05:30) Chennai, Kolkata, Mumbai, New Delhi"},
		{ID: "SE Asia Standard Time", Offset: "UTC+07:00", Label: "(UTC+07:00) Bangkok, Hanoi, Jakarta"},
		{ID: "China Standard Time", Offset: "UTC+08:00", Label: "(UTC+08:00) Beijing, Chongqing, Hong Kong, Urumqi"},
		{ID: "Singapore Standard Time", Offset: "UTC+08:00", Label: "(UTC+08:00) Kuala Lumpur, Singapore"},
		{ID: "Taipei Standard Time", Offset: "UTC+08:00", Label: "(UTC+08:00) Taipei"},
		{ID: "Tokyo Standard Time", Offset: "UTC+09:00", Label: "(UTC+09:00) Osaka, Sapporo, Tokyo"},
		{ID: "Korea Standard Time", Offset: "UTC+09:00", Label: "(UTC+09:00) Seoul"},
		{ID: "AUS Eastern Standard Time", Offset: "UTC+10:00", Label: "(UTC+10:00) Canberra, Melbourne, Sydney"},
		{ID: "New Zealand Standard Time", Offset: "UTC+12:00", Label: "(UTC+12:00) Auckland, Wellington"},
		{ID: "Aleutian Standard Time", Offset: "UTC-10:00", Label: "(UTC-10:00) Aleutian Islands"},
		{ID: "Hawaiian Standard Time", Offset: "UTC-10:00", Label: "(UTC-10:00) Hawaii"},
		{ID: "Alaskan Standard Time", Offset: "UTC-09:00", Label: "(UTC-09:00) Alaska"},
		{ID: "Pacific Standard Time", Offset: "UTC-08:00", Label: "(UTC-08:00) Pacific Time (US & Canada)"},
		{ID: "Mountain Standard Time", Offset: "UTC-07:00", Label: "(UTC-07:00) Mountain Time (US & Canada)"},
		{ID: "Central Standard Time", Offset: "UTC-06:00", Label: "(UTC-06:00) Central Time (US & Canada)"},
		{ID: "Eastern Standard Time", Offset: "UTC-05:00", Label: "(UTC-05:00) Eastern Time (US & Canada)"},
		{ID: "SA Western Standard Time", Offset: "UTC-04:00", Label: "(UTC-04:00) Georgetown, La Paz, Manaus, San Juan"},
		{ID: "E. South America Standard Time", Offset: "UTC-03:00", Label: "(UTC-03:00) Brasilia"},
		{ID: "Argentina Standard Time", Offset: "UTC-03:00", Label: "(UTC-03:00) City of Buenos Aires"},
	}
}

var (
	zoneListOnce  sync.Once
	zoneListCache []ZoneOption
)

// ListTimeZones returns timezone options parsed from tzutil /l when possible.
// Display labels keep the international "(UTC±HH:MM) City..." style from the OS.
// Result is cached for the process lifetime (tzutil /l is relatively slow).
func ListTimeZones() ([]ZoneOption, error) {
	zoneListOnce.Do(func() {
		zoneListCache = loadTimeZones()
	})
	out := make([]ZoneOption, len(zoneListCache))
	copy(out, zoneListCache)
	return out, nil
}

func loadTimeZones() []ZoneOption {
	out, err := syscmd.Run("tzutil", "/l")
	if err != nil {
		return CommonZones()
	}

	var options []ZoneOption
	lines := strings.Split(out, "\n")
	for i := 0; i < len(lines)-1; i++ {
		display := strings.TrimSpace(lines[i])
		id := strings.TrimSpace(lines[i+1])
		if display == "" || id == "" {
			continue
		}
		// tzutil /l pairs: display line then ID line.
		if !strings.HasPrefix(display, "(") && !strings.Contains(strings.ToUpper(display), "UTC") {
			continue
		}
		if strings.HasPrefix(id, "(") {
			continue
		}
		// Skip separator-like lines
		if strings.HasPrefix(id, "---") || strings.HasPrefix(display, "---") {
			continue
		}
		offset := ""
		if start := strings.Index(display, "("); start >= 0 {
			if end := strings.Index(display[start:], ")"); end > 0 {
				offset = display[start+1 : start+end]
			}
		}
		options = append(options, ZoneOption{ID: id, Label: display, Offset: offset})
		i++ // consume id line
	}
	if len(options) == 0 {
		return CommonZones()
	}
	return options
}

// EnsureZoneOption moves the current zone to the front (or prepends it when missing).
func EnsureZoneOption(list []ZoneOption, currentID string) []ZoneOption {
	currentID = strings.TrimSpace(currentID)
	if currentID == "" || currentID == "-" {
		return list
	}
	for i, z := range list {
		if strings.EqualFold(z.ID, currentID) {
			if i == 0 {
				return list
			}
			out := make([]ZoneOption, 0, len(list))
			out = append(out, z)
			out = append(out, list[:i]...)
			out = append(out, list[i+1:]...)
			return out
		}
	}
	label := currentID
	for _, z := range CommonZones() {
		if strings.EqualFold(z.ID, currentID) {
			label = z.Label
			break
		}
	}
	return append([]ZoneOption{{ID: currentID, Label: label}}, list...)
}

// SetTimeZone sets the system time zone by ID (e.g. "China Standard Time").
func SetTimeZone(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("时区不能为空")
	}
	_, err := syscmd.Run("tzutil", "/s", id)
	if err != nil {
		return fmt.Errorf("设置时区失败: %w", err)
	}
	return nil
}

// SyncNTP starts W32Time if needed and forces a resync.
func SyncNTP() error {
	_, _ = syscmd.Run("sc", "config", "W32Time", "start=", "auto")
	_, _ = syscmd.Run("sc", "start", "W32Time")
	out, err := syscmd.Run("w32tm", "/resync", "/force")
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		// fallback without /force
		out2, err2 := syscmd.Run("w32tm", "/resync")
		if err2 != nil {
			return fmt.Errorf("NTP 同步失败: %s", msg)
		}
		_ = out2
	}
	return nil
}

// SetNTPServer writes the primary NTP peer and updates w32tm config.
func SetNTPServer(server string) error {
	server = strings.TrimSpace(server)
	if server == "" {
		return fmt.Errorf("NTP 服务器不能为空")
	}
	_, err := syscmd.Run("w32tm", "/config", "/manualpeerlist:"+server+",0x9", "/syncfromflags:manual", "/reliable:NO", "/update")
	if err != nil {
		return fmt.Errorf("配置 NTP 失败: %w", err)
	}
	_, _ = syscmd.Run("sc", "stop", "W32Time")
	_, _ = syscmd.Run("sc", "start", "W32Time")
	return nil
}
