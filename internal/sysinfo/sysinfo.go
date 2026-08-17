package sysinfo

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"wintoolbox/internal/win/syscmd"
)

const overviewCacheTTL = 20 * time.Second

var (
	overviewCacheMu      sync.Mutex
	overviewCached       Overview
	overviewCachedAt     time.Time
	overviewCacheValid   bool
	overviewDetailReady  bool
)

// Overview holds high-level machine information.
type Overview struct {
	Hostname         string
	OSName           string
	OSBuild          string
	Arch             string
	Manufacturer     string
	Model            string
	Board            string
	BIOS             string
	CPU              string
	CPUCores         int
	MemoryTotalGB    float64
	MemoryAvailGB    float64
	MemoryModules    string
	Resolution       string
	IPs              []string
	Disks            []Disk
	PhysicalDisks    []PhysicalDisk
	GPUs             []string
	Activated        bool
	ActivationStatus string
}

// OverviewDetail is the slower hardware/licensing slice loaded after first paint.
type OverviewDetail struct {
	MemoryModules    string
	PhysicalDisks    []PhysicalDisk
	GPUs             []string
	Activated        bool
	ActivationStatus string
}

// Disk describes free/total space for a fixed drive.
type Disk struct {
	Root      string
	TotalGB   float64
	FreeGB    float64
	FreeRatio float64
}

// PhysicalDisk describes a physical storage device.
type PhysicalDisk struct {
	Model  string
	Media  string
	SizeGB float64
}

// GetOverviewFast collects registry/API fields for first paint (skips slow PS/slmgr).
func GetOverviewFast() Overview {
	overviewCacheMu.Lock()
	if overviewCacheValid && time.Since(overviewCachedAt) < overviewCacheTTL {
		ov := overviewCached
		overviewCacheMu.Unlock()
		return ov
	}
	overviewCacheMu.Unlock()

	var (
		hostname, arch, osName, osBuild string
		manufacturer, model, board    string
		bios, cpu, res                string
		cpuCores                      int
		memTotal, memAvail            float64
		ips                           []string
		disks                         []Disk
		wg                            sync.WaitGroup
	)
	wg.Add(11)
	go func() { defer wg.Done(); hostname = hostnameFn() }()
	go func() {
		defer wg.Done()
		arch = systemArch()
		cpuCores = runtime.NumCPU()
	}()
	go func() { defer wg.Done(); ips = ipv4Addresses() }()
	go func() { defer wg.Done(); disks = fixedDisks() }()
	go func() { defer wg.Done(); osName, osBuild = osVersion() }()
	go func() { defer wg.Done(); manufacturer, model = machineIdentity() }()
	go func() { defer wg.Done(); board = baseBoard() }()
	go func() { defer wg.Done(); bios = biosInfo() }()
	go func() { defer wg.Done(); cpu = cpuName() }()
	go func() { defer wg.Done(); memTotal, memAvail = memoryGB() }()
	go func() { defer wg.Done(); res = screenResolution() }()
	wg.Wait()

	// GPU: registry only on first paint (no CIM/PowerShell fallback).
	gpus := gpusViaRegistry()

	ov := Overview{
		Hostname:         hostname,
		OSName:           osName,
		OSBuild:          osBuild,
		Arch:             arch,
		Manufacturer:     manufacturer,
		Model:            model,
		Board:            board,
		BIOS:             bios,
		CPU:              cpu,
		CPUCores:         cpuCores,
		MemoryTotalGB:    memTotal,
		MemoryAvailGB:    memAvail,
		MemoryModules:    "检测中…",
		Resolution:       res,
		IPs:              ips,
		Disks:            disks,
		PhysicalDisks:    nil,
		GPUs:             gpus,
		Activated:        false,
		ActivationStatus: "检测中…",
	}

	overviewCacheMu.Lock()
	overviewCached = ov
	overviewCachedAt = time.Now()
	overviewCacheValid = true
	overviewDetailReady = false
	overviewCacheMu.Unlock()
	return ov
}

// GetOverviewDetail fills slow fields (activation / physical disks / memory modules / GPU CIM fallback).
func GetOverviewDetail() OverviewDetail {
	overviewCacheMu.Lock()
	if overviewCacheValid && overviewDetailReady && time.Since(overviewCachedAt) < overviewCacheTTL {
		ov := overviewCached
		overviewCacheMu.Unlock()
		return OverviewDetail{
			MemoryModules:    ov.MemoryModules,
			PhysicalDisks:    ov.PhysicalDisks,
			GPUs:             ov.GPUs,
			Activated:        ov.Activated,
			ActivationStatus: ov.ActivationStatus,
		}
	}
	overviewCacheMu.Unlock()

	var (
		memModules string
		phys       []PhysicalDisk
		gpus       []string
		activated  bool
		actStatus  string
		wg         sync.WaitGroup
	)
	wg.Add(4)
	go func() { defer wg.Done(); memModules = memoryModules() }()
	go func() { defer wg.Done(); phys = physicalDisks() }()
	go func() { defer wg.Done(); gpus = gpuNames() }()
	go func() { defer wg.Done(); activated, actStatus = activationStatus() }()
	wg.Wait()

	d := OverviewDetail{
		MemoryModules:    memModules,
		PhysicalDisks:    phys,
		GPUs:             gpus,
		Activated:        activated,
		ActivationStatus: actStatus,
	}

	overviewCacheMu.Lock()
	if overviewCacheValid {
		overviewCached.MemoryModules = d.MemoryModules
		overviewCached.PhysicalDisks = d.PhysicalDisks
		if len(d.GPUs) > 0 {
			overviewCached.GPUs = d.GPUs
		}
		overviewCached.Activated = d.Activated
		overviewCached.ActivationStatus = d.ActivationStatus
		overviewDetailReady = true
		overviewCachedAt = time.Now()
	}
	overviewCacheMu.Unlock()
	return d
}

// InvalidateOverviewCache forces the next GetOverviewFast to recollect.
func InvalidateOverviewCache() {
	overviewCacheMu.Lock()
	overviewCacheValid = false
	overviewDetailReady = false
	overviewCacheMu.Unlock()
}

func hostnameFn() string {
	h, err := os.Hostname()
	if err != nil || strings.TrimSpace(h) == "" {
		return "-"
	}
	return h
}

func osVersion() (name, build string) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Windows", "-"
	}
	defer k.Close()

	product, _, _ := k.GetStringValue("ProductName")
	display, _, _ := k.GetStringValue("DisplayVersion")
	currentBuild, _, _ := k.GetStringValue("CurrentBuild")
	ubr, _, ubrErr := k.GetIntegerValue("UBR")

	name = strings.TrimSpace(product)
	if name == "" {
		name = "Windows"
	}
	// Win11 still ships ProductName as "Windows 10 …"; build >= 22000 is the reliable signal.
	if buildNum, err := strconv.Atoi(strings.TrimSpace(currentBuild)); err == nil && buildNum >= 22000 {
		name = strings.Replace(name, "Windows 10", "Windows 11", 1)
	}
	if strings.TrimSpace(display) != "" {
		name = name + " " + display
	}
	build = strings.TrimSpace(currentBuild)
	if ubrErr == nil {
		build = fmt.Sprintf("%s.%d", build, ubr)
	}
	if build == "" {
		build = "-"
	}
	return name, build
}

func systemArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "x86"
	case "arm64":
		return "ARM64"
	case "arm":
		return "ARM"
	default:
		return runtime.GOARCH
	}
}

func machineIdentity() (manufacturer, model string) {
	paths := []string{
		`HARDWARE\DESCRIPTION\System\BIOS`,
		`SYSTEM\CurrentControlSet\Control\SystemInformation`,
	}
	for _, path := range paths {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		if manufacturer == "" {
			if v, _, err := k.GetStringValue("SystemManufacturer"); err == nil {
				manufacturer = strings.TrimSpace(v)
			}
		}
		if model == "" {
			if v, _, err := k.GetStringValue("SystemProductName"); err == nil {
				model = strings.TrimSpace(v)
			}
		}
		k.Close()
		if manufacturer != "" && model != "" {
			break
		}
	}
	if manufacturer == "" {
		manufacturer = "-"
	}
	if model == "" {
		model = "-"
	}
	return manufacturer, model
}

func baseBoard() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, registry.QUERY_VALUE)
	if err != nil {
		return "-"
	}
	defer k.Close()
	vendor, _, _ := k.GetStringValue("BaseBoardManufacturer")
	product, _, _ := k.GetStringValue("BaseBoardProduct")
	vendor = strings.TrimSpace(vendor)
	product = strings.TrimSpace(product)
	switch {
	case vendor != "" && product != "" && !strings.EqualFold(vendor, product):
		return vendor + " " + product
	case product != "":
		return product
	case vendor != "":
		return vendor
	default:
		return "-"
	}
}

func biosInfo() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, registry.QUERY_VALUE)
	if err != nil {
		return "-"
	}
	defer k.Close()
	vendor, _, _ := k.GetStringValue("BIOSVendor")
	version, _, _ := k.GetStringValue("BIOSVersion")
	date, _, _ := k.GetStringValue("BIOSReleaseDate")
	vendor = strings.TrimSpace(vendor)
	version = strings.Join(strings.Fields(strings.TrimSpace(version)), " ")
	date = strings.TrimSpace(date)
	parts := make([]string, 0, 3)
	if vendor != "" {
		parts = append(parts, vendor)
	}
	if version != "" {
		parts = append(parts, version)
	}
	if date != "" {
		parts = append(parts, date)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " · ")
}

func memoryModules() string {
	out, err := syscmd.RunPS(`Get-CimInstance Win32_PhysicalMemory | Select-Object Capacity,Speed,Manufacturer,PartNumber | ConvertTo-Csv -NoTypeInformation`)
	if err != nil || strings.TrimSpace(out) == "" {
		return "-"
	}
	type mod struct {
		capGB float64
		speed string
		mfr   string
	}
	var mods []mod
	for i, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || i == 0 && strings.Contains(strings.ToLower(line), "capacity") {
			continue
		}
		parts := splitCSVLine(line)
		if len(parts) < 2 {
			continue
		}
		cap := parseUint(parts[0])
		if cap == 0 {
			continue
		}
		m := mod{
			capGB: float64(cap) / (1024 * 1024 * 1024),
			speed: strings.TrimSpace(parts[1]),
		}
		if len(parts) > 2 {
			m.mfr = strings.TrimSpace(parts[2])
		}
		mods = append(mods, m)
	}
	if len(mods) == 0 {
		return "-"
	}
	// Group identical modules: "2×8 GB 3200 MHz"
	type key struct {
		cap   int
		speed string
	}
	counts := map[key]int{}
	order := make([]key, 0, len(mods))
	for _, m := range mods {
		k := key{cap: int(m.capGB + 0.5), speed: m.speed}
		if counts[k] == 0 {
			order = append(order, k)
		}
		counts[k]++
	}
	parts := make([]string, 0, len(order))
	for _, k := range order {
		n := counts[k]
		s := fmt.Sprintf("%d×%d GB", n, k.cap)
		if k.speed != "" && k.speed != "0" {
			s += " " + k.speed + " MHz"
		}
		parts = append(parts, s)
	}
	return fmt.Sprintf("%s（共 %d 条）", strings.Join(parts, " + "), len(mods))
}

var (
	modUser32          = windows.NewLazySystemDLL("user32.dll")
	procGetSystemMetrics = modUser32.NewProc("GetSystemMetrics")
)

func screenResolution() string {
	const (
		smCXScreen = 0
		smCYScreen = 1
	)
	w, _, _ := procGetSystemMetrics.Call(smCXScreen)
	h, _, _ := procGetSystemMetrics.Call(smCYScreen)
	if w == 0 || h == 0 {
		return "-"
	}
	return fmt.Sprintf("%d×%d", w, h)
}

func cpuName() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE)
	if err != nil {
		return "-"
	}
	defer k.Close()
	name, _, err := k.GetStringValue("ProcessorNameString")
	if err != nil {
		return "-"
	}
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "-"
	}
	return name
}

var (
	modKernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = modKernel32.NewProc("GlobalMemoryStatusEx")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func memoryGB() (total, avail float64) {
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if r1 == 0 || ms.TotalPhys == 0 {
		return 0, 0
	}
	const gib = 1024 * 1024 * 1024
	return float64(ms.TotalPhys) / gib, float64(ms.AvailPhys) / gib
}

func ipv4Addresses() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]string, 0, 4)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.To4() == nil || ip.IsLoopback() {
				continue
			}
			out = append(out, fmt.Sprintf("%s (%s)", ip.String(), iface.Name))
		}
	}
	return out
}

func fixedDisks() []Disk {
	// Prefer WinAPI: much faster than spawning wmic on startup.
	if disks := disksViaAPI(); len(disks) > 0 {
		return disks
	}
	out, err := syscmd.Run("wmic", "logicaldisk", "where", "DriveType=3", "get", "DeviceID,FreeSpace,Size", "/format:csv")
	if err != nil {
		return nil
	}
	disks := make([]Disk, 0, 4)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "node,") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			continue
		}
		// CSV: Node,DeviceID,FreeSpace,Size
		root := strings.TrimSpace(parts[1])
		free := parseUint(parts[2])
		total := parseUint(parts[3])
		if root == "" || total == 0 {
			continue
		}
		d := Disk{
			Root:    root,
			TotalGB: float64(total) / (1024 * 1024 * 1024),
			FreeGB:  float64(free) / (1024 * 1024 * 1024),
		}
		if total > 0 {
			d.FreeRatio = float64(free) / float64(total)
		}
		disks = append(disks, d)
	}
	if len(disks) == 0 {
		return disksViaAPI()
	}
	return disks
}

func disksViaAPI() []Disk {
	var disks []Disk
	for _, letter := range "CDEFGHIJKLMNOPQRSTUVWXYZ" {
		root := string(letter) + `:\`
		rootPtr, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		// DRIVE_FIXED = 3; skip removable/network/CD-ROM.
		if windows.GetDriveType(rootPtr) != windows.DRIVE_FIXED {
			continue
		}
		var freeBytesAvailable, totalBytes, totalFreeBytes uint64
		err = windows.GetDiskFreeSpaceEx(rootPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes)
		if err != nil || totalBytes == 0 {
			continue
		}
		d := Disk{
			Root:    string(letter) + ":",
			TotalGB: float64(totalBytes) / (1024 * 1024 * 1024),
			FreeGB:  float64(totalFreeBytes) / (1024 * 1024 * 1024),
		}
		d.FreeRatio = float64(totalFreeBytes) / float64(totalBytes)
		disks = append(disks, d)
	}
	return disks
}

func parseUint(s string) uint64 {
	s = strings.TrimSpace(s)
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + uint64(c-'0')
	}
	return n
}

func physicalDisks() []PhysicalDisk {
	out, err := syscmd.RunPS(`Get-PhysicalDisk | Select-Object FriendlyName,MediaType,Size | ConvertTo-Csv -NoTypeInformation`)
	if err != nil || strings.TrimSpace(out) == "" {
		return physicalDisksViaWMI()
	}
	disks := parsePhysicalDiskCSV(out)
	if len(disks) == 0 {
		return physicalDisksViaWMI()
	}
	return disks
}

func physicalDisksViaWMI() []PhysicalDisk {
	out, err := syscmd.RunPS(`Get-CimInstance Win32_DiskDrive | Select-Object Model,Size | ConvertTo-Csv -NoTypeInformation`)
	if err != nil {
		return nil
	}
	var disks []PhysicalDisk
	for i, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || i == 0 && strings.HasPrefix(strings.ToLower(line), `"model"`) {
			continue
		}
		parts := splitCSVLine(line)
		if len(parts) < 2 {
			continue
		}
		model := strings.TrimSpace(parts[0])
		size := parseUint(parts[1])
		if model == "" || size == 0 {
			continue
		}
		disks = append(disks, PhysicalDisk{
			Model:  model,
			SizeGB: float64(size) / (1024 * 1024 * 1024),
		})
	}
	return disks
}

func parsePhysicalDiskCSV(out string) []PhysicalDisk {
	var disks []PhysicalDisk
	for i, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || i == 0 && strings.Contains(strings.ToLower(line), "friendlyname") {
			continue
		}
		parts := splitCSVLine(line)
		if len(parts) < 3 {
			continue
		}
		model := strings.TrimSpace(parts[0])
		media := strings.TrimSpace(parts[1])
		size := parseUint(parts[2])
		if model == "" || size == 0 {
			continue
		}
		disks = append(disks, PhysicalDisk{
			Model:  model,
			Media:  normalizeMedia(media),
			SizeGB: float64(size) / (1024 * 1024 * 1024),
		})
	}
	return disks
}

func normalizeMedia(media string) string {
	switch strings.ToLower(strings.TrimSpace(media)) {
	case "ssd", "4":
		return "SSD"
	case "hdd", "3":
		return "HDD"
	case "scm", "5":
		return "SCM"
	case "unspecified", "0", "":
		return ""
	default:
		return media
	}
}

func gpuNames() []string {
	if names := gpusViaRegistry(); len(names) > 0 {
		return names
	}
	out, err := syscmd.RunPS(`Get-CimInstance Win32_VideoController | Select-Object -ExpandProperty Name`)
	if err != nil {
		return nil
	}
	var names []string
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		name := strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		names = append(names, name)
	}
	return names
}

func gpusViaRegistry() []string {
	const classPath = `SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, classPath, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()

	subs, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}
	var names []string
	seen := map[string]bool{}
	for _, sub := range subs {
		if len(sub) != 4 {
			continue
		}
		if _, err := strconv.Atoi(sub); err != nil {
			continue
		}
		sk, err := registry.OpenKey(registry.LOCAL_MACHINE, classPath+`\`+sub, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		name, _, err := sk.GetStringValue("DriverDesc")
		sk.Close()
		name = strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
		if err != nil || name == "" || seen[strings.ToLower(name)] {
			continue
		}
		// Skip basic/remote display adapters that aren't real GPUs.
		low := strings.ToLower(name)
		if strings.Contains(low, "microsoft basic") || strings.Contains(low, "remote desktop") {
			continue
		}
		seen[strings.ToLower(name)] = true
		names = append(names, name)
	}
	return names
}

func activationStatus() (activated bool, status string) {
	// slmgr is more reliable/faster than enumerating SoftwareLicensingProduct via CIM.
	slmgr := strings.TrimRight(os.Getenv("SystemRoot"), `\`) + `\System32\slmgr.vbs`
	if os.Getenv("SystemRoot") == "" {
		slmgr = `C:\Windows\System32\slmgr.vbs`
	}
	out, err := syscmd.Run("cscript", "//Nologo", slmgr, "/xpr")
	if err == nil {
		if a, s, ok := parseSlmgrXpr(out); ok {
			return a, s
		}
	}

	// Fallback: single-product WMI query (still may be slow on some hosts).
	out, err = syscmd.RunPS(`$p = Get-CimInstance -ClassName SoftwareLicensingProduct -Filter "ApplicationID='55c92734-d718-4ba1-b96c-3e9d026da924' AND PartialProductKey IS NOT NULL" -ErrorAction SilentlyContinue | Select-Object -First 1; if ($null -eq $p) { 'NONE' } else { [string]$p.LicenseStatus }`)
	if err != nil {
		return false, "未知"
	}
	code := strings.TrimSpace(out)
	switch code {
	case "1":
		return true, "已激活"
	case "0":
		return false, "未授权"
	case "2":
		return false, "OOB 宽限期"
	case "3":
		return false, "OOT 宽限期"
	case "4":
		return false, "非正版宽限期"
	case "5":
		return false, "通知模式"
	case "6":
		return false, "扩展宽限期"
	case "NONE", "":
		return false, "未知"
	default:
		return false, "状态 " + code
	}
}

func parseSlmgrXpr(out string) (activated bool, status string, ok bool) {
	text := strings.TrimSpace(out)
	if text == "" {
		return false, "", false
	}
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(text, "永久激活") || strings.Contains(lower, "permanently activated"):
		return true, "已激活", true
	case strings.Contains(text, "已激活") || strings.Contains(lower, "is activated"):
		return true, "已激活", true
	case strings.Contains(text, "宽限期") || strings.Contains(lower, "grace") || strings.Contains(lower, "expire"):
		return false, "宽限期", true
	case strings.Contains(text, "通知") || strings.Contains(lower, "notification"):
		return false, "通知模式", true
	case strings.Contains(text, "未授权") || strings.Contains(lower, "unlicensed") || strings.Contains(lower, "not activated"):
		return false, "未授权", true
	default:
		// slmgr often prints a multi-line banner; keep a short last meaningful line.
		lines := strings.Split(text, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" || strings.HasPrefix(strings.ToLower(line), "microsoft") {
				continue
			}
			if len(line) > 40 {
				line = line[:40] + "…"
			}
			return false, line, true
		}
		return false, "", false
	}
}

func splitCSVLine(line string) []string {
	var parts []string
	var cur strings.Builder
	inQuotes := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"':
			if inQuotes && i+1 < len(line) && line[i+1] == '"' {
				cur.WriteByte('"')
				i++
			} else {
				inQuotes = !inQuotes
			}
		case c == ',' && !inQuotes:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	parts = append(parts, cur.String())
	return parts
}
