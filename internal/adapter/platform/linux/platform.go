//go:build linux

package platform

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"sysscope/internal/adapter/collector"
	"sysscope/internal/domain"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

func init() {
	collector.RegisterOS(collectOS)
	collector.RegisterCPU(collectCPU)
	collector.RegisterMemory(collectMemory)
	collector.RegisterDisk(collectDisk)
	collector.RegisterNetwork(collectNetwork)
	collector.RegisterBIOS(collectBIOS)
	collector.RegisterMotherboard(collectMotherboard)
	collector.RegisterGPU(collectGPU)
	collector.RegisterMonitor(collectMonitor)
	collector.RegisterBattery(collectBattery)
	collector.RegisterProcesses(collectProcesses)
	collector.RegisterSecurity(collectSecurity)
	collector.RegisterTemperatures(collectTemperatures)
	collector.RegisterSMART(collectSMART)
	collector.RegisterServices(collectServices)
	collector.RegisterDrivers(collectDrivers)
	collector.RegisterStartup(collectStartup)
	collector.RegisterPrograms(collectPrograms)
	collector.RegisterEnvironment(collectEnvironment)
	collector.RegisterUSB(collectUSB)
	collector.RegisterBluetooth(collectBluetooth)
	collector.RegisterPCI(collectPCI)
	collector.RegisterWinFeatures(collectWinFeatures)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func readFileStr(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// ─── OS ──────────────────────────────────────────────────────────────────────

func collectOS(_ context.Context) (*domain.OSInfo, error) {
	hi, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("host.Info: %w", err)
	}
	hostname, _ := os.Hostname()
	uptimeDur := time.Duration(hi.Uptime) * time.Second
	days := int(uptimeDur.Hours()) / 24
	hours := int(uptimeDur.Hours()) % 24
	mins := int(uptimeDur.Minutes()) % 60
	edition, version := parseOSRelease()
	return &domain.OSInfo{
		Edition:      edition,
		Version:      version,
		Build:        hi.KernelVersion,
		ProductName:  hi.Platform,
		Architecture: runtime.GOARCH,
		Uptime:       fmt.Sprintf("%dd %dh %dm", days, hours, mins),
		Hostname:     hostname,
		Kernel:       hi.KernelVersion,
	}, nil
}

func parseOSRelease() (edition, version string) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "Linux", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "PRETTY_NAME="):
			edition = trimQuotes(strings.TrimPrefix(line, "PRETTY_NAME="))
		case strings.HasPrefix(line, "VERSION="):
			version = trimQuotes(strings.TrimPrefix(line, "VERSION="))
		}
	}
	if edition == "" {
		edition = "Linux"
	}
	return
}

// ─── CPU ─────────────────────────────────────────────────────────────────────

func collectCPU(_ context.Context) (*domain.CPUInfo, error) {
	info := &domain.CPUInfo{Architecture: runtime.GOARCH}
	infos, err := cpu.Info()
	if err == nil && len(infos) > 0 {
		c := infos[0]
		info.Model = strings.TrimSpace(c.ModelName)
		info.Vendor = strings.TrimSpace(c.VendorID)
		info.Family = strings.TrimSpace(c.Family)
		if c.Mhz > 0 {
			info.CurrentMHz = c.Mhz
		}
	}
	phys, err := cpu.Counts(false)
	if err == nil {
		info.PhysicalCores = int32(phys)
	}
	logical, err := cpu.Counts(true)
	if err == nil {
		info.LogicalCores = int32(logical)
	}
	pcts, err := cpu.Percent(time.Second, false)
	if err == nil && len(pcts) > 0 {
		info.UsagePercent = pcts[0]
	}
	info.MaxMHz = readMaxFreq()
	if info.CurrentMHz == 0 {
		info.CurrentMHz = readCurrentFreq()
	}
	return info, nil
}

func readMaxFreq() float64 {
	if data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"); err == nil {
		khz, _ := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
		return khz / 1000
	}
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "cpu MHz") {
			parts := strings.SplitN(sc.Text(), ":", 2)
			if len(parts) == 2 {
				v, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				return v
			}
		}
	}
	return 0
}

func readCurrentFreq() float64 {
	data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
	if err != nil {
		return 0
	}
	khz, _ := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	return khz / 1000
}

// ─── Memory ──────────────────────────────────────────────────────────────────

func collectMemory(_ context.Context) (*domain.MemoryInfo, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("mem.VirtualMemory: %w", err)
	}
	return &domain.MemoryInfo{
		TotalBytes:   vm.Total,
		UsedBytes:    vm.Used,
		FreeBytes:    vm.Free,
		UsagePercent: vm.UsedPercent,
	}, nil
}

// ─── Disk ────────────────────────────────────────────────────────────────────

func collectDisk(_ context.Context) ([]domain.DiskInfo, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, fmt.Errorf("disk.Partitions: %w", err)
	}
	deviceMap := map[string]*domain.DiskInfo{}
	for _, p := range partitions {
		if strings.HasPrefix(p.Device, "/dev/loop") ||
			strings.HasPrefix(p.Device, "/dev/snap") ||
			p.Fstype == "squashfs" || p.Fstype == "tmpfs" ||
			p.Fstype == "devtmpfs" || p.Fstype == "overlay" {
			continue
		}
		baseDev := baseDevice(p.Device)
		di, exists := deviceMap[baseDev]
		if !exists {
			di = &domain.DiskInfo{
				DeviceID:  baseDev,
				Model:     baseDev,
				MediaType: detectMediaType(baseDev),
			}
			if m := readSysfsAttr(baseDev, "model"); m != "" {
				di.Model = m
			}
			if sn := readSysfsAttr(baseDev, "serial"); sn != "" {
				di.SerialNumber = sn
			}
			deviceMap[baseDev] = di
		}
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}
		di.Partitions = append(di.Partitions, domain.Partition{
			Letter:       p.Mountpoint,
			FileSystem:   p.Fstype,
			TotalBytes:   usage.Total,
			FreeBytes:    usage.Free,
			UsagePercent: usage.UsedPercent,
		})
	}
	var result []domain.DiskInfo
	for _, d := range deviceMap {
		result = append(result, *d)
	}
	return result, nil
}

func baseDevice(dev string) string {
	if strings.HasPrefix(dev, "/dev/nvme") {
		for i := len(dev) - 1; i >= 0; i-- {
			if dev[i] == 'p' && i > 0 && dev[i-1] >= '0' && dev[i-1] <= '9' {
				return dev[:i]
			}
		}
		return dev
	}
	name := strings.TrimPrefix(dev, "/dev/")
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] < '0' || name[i] > '9' {
			return "/dev/" + name[:i+1]
		}
	}
	return dev
}

func detectMediaType(dev string) string {
	name := strings.TrimPrefix(dev, "/dev/")
	data, err := os.ReadFile(fmt.Sprintf("/sys/block/%s/queue/rotational", name))
	if err != nil {
		return "Unspecified"
	}
	switch strings.TrimSpace(string(data)) {
	case "0":
		return "SSD"
	case "1":
		return "HDD"
	}
	return "Unspecified"
}

func readSysfsAttr(dev, attr string) string {
	name := strings.TrimPrefix(dev, "/dev/")
	return readFileStr(fmt.Sprintf("/sys/block/%s/device/%s", name, attr))
}

// ─── Network ─────────────────────────────────────────────────────────────────

func collectNetwork(_ context.Context) ([]domain.NetAdapter, error) {
	return collectNetworkSimple()
}

func collectNetworkSimple() ([]domain.NetAdapter, error) {
	out, err := exec.Command("ip", "link", "show").Output()
	if err != nil {
		return nil, fmt.Errorf("ip link show: %w", err)
	}
	var adapters []domain.NetAdapter
	var current *domain.NetAdapter
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > 0 && line[0] >= '0' && line[0] <= '9' && strings.Contains(line, ": <") {
			if current != nil && current.Name != "lo" {
				adapters = append(adapters, *current)
			}
			parts := strings.SplitN(line, ": ", 3)
			if len(parts) < 3 {
				current = nil
				continue
			}
			name := strings.TrimSuffix(parts[1], ":")
			if name == "lo" {
				current = nil
				continue
			}
			isUp := strings.Contains(parts[2], "UP")
			current = &domain.NetAdapter{Name: name, IsUp: isUp}
			if idx := strings.Index(line, "link/ether "); idx >= 0 {
				rest := line[idx+11:]
				fields := strings.Fields(rest)
				if len(fields) > 0 {
					current.MAC = fields[0]
				}
			}
			// Determine type
			if strings.HasPrefix(name, "wl") || strings.HasPrefix(name, "wlan") {
				current.Type = "Wi-Fi"
			} else if strings.HasPrefix(name, "en") || strings.HasPrefix(name, "eth") {
				current.Type = "Ethernet"
			} else if strings.HasPrefix(name, "tun") || strings.HasPrefix(name, "tap") || strings.HasPrefix(name, "vpn") {
				current.Type = "VPN"
			} else {
				current.Type = "Ethernet"
			}
		}
	}
	if current != nil && current.Name != "lo" {
		adapters = append(adapters, *current)
	}
	for i := range adapters {
		ips := getIPsForInterface(adapters[i].Name)
		adapters[i].IPv4 = ips.v4
		adapters[i].IPv6 = ips.v6
	}
	dns := readDNS()
	if len(adapters) > 0 {
		adapters[0].DNS = dns
	}
	gw := readDefaultGateway()
	if gw != "" && len(adapters) > 0 {
		adapters[0].Gateway = []string{gw}
	}
	for i := range adapters {
		if data, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/speed", adapters[i].Name)); err == nil {
			if s, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil && s > 0 {
				adapters[i].SpeedMbps = s
			}
		}
	}
	// Get DHCP status
	for i := range adapters {
		adapters[i].DHCP = isDHCPEnabled(adapters[i].Name)
	}
	return adapters, nil
}

func isDHCPEnabled(iface string) bool {
	data, err := os.ReadFile(fmt.Sprintf("/run/systemd/netif/leases/%s", iface))
	if err != nil {
		// Try NetworkManager
		out, _ := exec.Command("nmcli", "-t", "-f", "IP4.METHOD", "device", "show", iface).Output()
		return strings.Contains(string(out), "auto")
	}
	return len(data) > 0
}

type ipResult struct {
	v4 []string
	v6 []string
}

func getIPsForInterface(name string) ipResult {
	out, err := exec.Command("ip", "-o", "addr", "show", "dev", name).Output()
	if err != nil {
		return ipResult{}
	}
	var r ipResult
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if (f == "inet" || f == "inet6") && i+1 < len(fields) {
				addr := strings.Split(fields[i+1], "/")[0]
				if f == "inet" {
					r.v4 = append(r.v4, addr)
				} else {
					r.v6 = append(r.v6, addr)
				}
			}
		}
	}
	return r
}

func readDNS() []string {
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	defer f.Close()
	var servers []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				servers = append(servers, fields[1])
			}
		}
	}
	return servers
}

func readDefaultGateway() string {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "via" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

// ─── BIOS ────────────────────────────────────────────────────────────────────

func collectBIOS(_ context.Context) (*domain.BIOSInfo, error) {
	info := &domain.BIOSInfo{
		Manufacturer: readFileStr("/sys/class/dmi/id/bios_vendor"),
		Version:      readFileStr("/sys/class/dmi/id/bios_version"),
		ReleaseDate:  readFileStr("/sys/class/dmi/id/bios_date"),
	}
	if info.Manufacturer == "" && info.Version == "" {
		return nil, fmt.Errorf("DMI BIOS info not accessible (may require root)")
	}
	return info, nil
}

// ─── Motherboard ─────────────────────────────────────────────────────────────

func collectMotherboard(_ context.Context) (*domain.MoboInfo, error) {
	info := &domain.MoboInfo{
		Manufacturer: readFileStr("/sys/class/dmi/id/board_vendor"),
		Model:        readFileStr("/sys/class/dmi/id/board_name"),
		SerialNumber: readFileStr("/sys/class/dmi/id/board_serial"),
	}
	if info.Manufacturer == "" && info.Model == "" {
		return nil, fmt.Errorf("DMI motherboard info not accessible (may require root)")
	}
	return info, nil
}

// ─── GPU ─────────────────────────────────────────────────────────────────────

func collectGPU(_ context.Context) ([]domain.GPUInfo, error) {
	out, err := exec.Command("lspci", "-nn").Output()
	if err != nil {
		return collectGPUSysfs()
	}
	var gpus []domain.GPUInfo
	for _, line := range strings.Split(string(out), "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "vga") && !strings.Contains(lower, "3d controller") && !strings.Contains(lower, "display controller") {
			continue
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) < 2 {
			continue
		}
		desc := parts[1]
		for _, prefix := range []string{"VGA compatible controller: ", "3D controller: ", "Display controller: "} {
			desc = strings.TrimPrefix(desc, prefix)
		}
		gpu := domain.GPUInfo{Name: desc}
		switch {
		case strings.Contains(lower, "nvidia") || strings.Contains(lower, "[10de"):
			gpu.Vendor = "NVIDIA"
		case strings.Contains(lower, "amd") || strings.Contains(lower, "ati") || strings.Contains(lower, "[1002"):
			gpu.Vendor = "AMD"
		case strings.Contains(lower, "intel") || strings.Contains(lower, "[8086"):
			gpu.Vendor = "Intel"
		default:
			gpu.Vendor = "Unknown"
		}
		gpu.DriverVersion = getDriverVersion(gpu.Vendor)
		gpus = append(gpus, gpu)
	}
	if len(gpus) == 0 {
		return collectGPUSysfs()
	}
	return gpus, nil
}

func collectGPUSysfs() ([]domain.GPUInfo, error) {
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return nil, fmt.Errorf("GPU info not available: %w", err)
	}
	var gpus []domain.GPUInfo
	seen := map[string]bool{}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "card") || strings.Contains(e.Name(), "-") {
			continue
		}
		vendor := readFileStr(fmt.Sprintf("/sys/class/drm/%s/device/vendor", e.Name()))
		device := readFileStr(fmt.Sprintf("/sys/class/drm/%s/device/device", e.Name()))
		key := vendor + ":" + device
		if seen[key] || vendor == "" {
			continue
		}
		seen[key] = true
		vendorName := "Unknown"
		switch strings.ToLower(vendor) {
		case "0x8086":
			vendorName = "Intel"
		case "0x10de":
			vendorName = "NVIDIA"
		case "0x1002":
			vendorName = "AMD"
		}
		gpus = append(gpus, domain.GPUInfo{Name: fmt.Sprintf("%s (%s)", vendorName, device), Vendor: vendorName})
	}
	if len(gpus) == 0 {
		return nil, fmt.Errorf("no GPU detected")
	}
	return gpus, nil
}

func getDriverVersion(vendor string) string {
	switch vendor {
	case "NVIDIA":
		out, err := exec.Command("nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	case "Intel":
		out, err := exec.Command("modinfo", "i915").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "version:") {
					return strings.TrimSpace(strings.TrimPrefix(line, "version:"))
				}
			}
		}
	}
	return "N/A"
}

// ─── Monitor ─────────────────────────────────────────────────────────────────

func collectMonitor(_ context.Context) ([]domain.MonitorInfo, error) {
	out, err := exec.Command("xrandr", "--current").Output()
	if err != nil {
		return collectMonitorSysfs()
	}
	var monitors []domain.MonitorInfo
	var current domain.MonitorInfo
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, " connected ") {
			if current.Name != "" {
				monitors = append(monitors, current)
			}
			fields := strings.Fields(line)
			current = domain.MonitorInfo{Name: fields[0]}
			for _, f := range fields {
				if strings.Contains(f, "x") && strings.Contains(f, "+") {
					res := strings.Split(f, "+")[0]
					parts := strings.Split(res, "x")
					if len(parts) == 2 {
						current.ResolutionX, _ = strconv.Atoi(parts[0])
						current.ResolutionY, _ = strconv.Atoi(parts[1])
					}
				}
			}
		}
		if current.Name != "" && strings.Contains(line, "*") {
			fields := strings.Fields(line)
			for _, f := range fields {
				f = strings.TrimRight(f, "*+")
				if rate, err := strconv.ParseFloat(f, 64); err == nil && rate > 20 && rate < 500 {
					current.RefreshRateHz = int(rate)
					break
				}
			}
		}
	}
	if current.Name != "" {
		monitors = append(monitors, current)
	}
	if len(monitors) == 0 {
		return collectMonitorSysfs()
	}
	return monitors, nil
}

func collectMonitorSysfs() ([]domain.MonitorInfo, error) {
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return nil, fmt.Errorf("monitor info not available (no display server)")
	}
	var monitors []domain.MonitorInfo
	for _, e := range entries {
		name := e.Name()
		if !strings.Contains(name, "card") || strings.Contains(name, "-") {
			continue
		}
		status := readFileStr(fmt.Sprintf("/sys/class/drm/%s/status", name))
		if status != "connected" {
			continue
		}
		monitors = append(monitors, domain.MonitorInfo{Name: name})
	}
	if len(monitors) == 0 {
		return nil, fmt.Errorf("no connected monitors detected")
	}
	return monitors, nil
}

// ─── Battery ─────────────────────────────────────────────────────────────────

func collectBattery(_ context.Context) (*domain.BatteryInfo, error) {
	entries, err := os.ReadDir("/sys/class/power_supply")
	if err != nil {
		return nil, fmt.Errorf("no battery found")
	}
	for _, e := range entries {
		base := fmt.Sprintf("/sys/class/power_supply/%s", e.Name())
		if readFileStr(base+"/type") != "Battery" {
			continue
		}
		info := &domain.BatteryInfo{IsPresent: true}
		if cap, err := strconv.ParseFloat(readFileStr(base+"/capacity"), 64); err == nil {
			info.ChargePercent = cap
		}
		status := readFileStr(base + "/status")
		info.IsCharging = strings.EqualFold(status, "charging")

		// Health from charge_full / charge_full_design
		designCap := readFileStr(base + "/charge_full_design")
		currCap := readFileStr(base + "/charge_full")
		if designCap != "" && currCap != "" {
			dc, e1 := strconv.ParseFloat(designCap, 64)
			cc, e2 := strconv.ParseFloat(currCap, 64)
			if e1 == nil && e2 == nil && dc > 0 {
				info.HealthPercent = (cc / dc) * 100
				info.FullChargeCapWh = cc / 1e6  // μAh → Wh (approximate)
				info.DesignCapacityWh = dc / 1e6
				if dc > cc {
					info.WearLevelPercent = ((dc - cc) / dc) * 100
				}
			}
		}
		// Cycle count
		if cc := readFileStr(base + "/cycle_count"); cc != "" {
			info.CycleCount, _ = strconv.Atoi(cc)
		}
		return info, nil
	}
	return nil, fmt.Errorf("no battery found on this system")
}

// ─── Temperatures ────────────────────────────────────────────────────────────

func collectTemperatures(_ context.Context) (*domain.TempInfo, error) {
	info := &domain.TempInfo{}
	found := false

	// Read from /sys/class/thermal
	entries, err := os.ReadDir("/sys/class/thermal")
	if err == nil {
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), "thermal_zone") {
				continue
			}
			zoneType := readFileStr(fmt.Sprintf("/sys/class/thermal/%s/type", e.Name()))
			tempStr := readFileStr(fmt.Sprintf("/sys/class/thermal/%s/temp", e.Name()))
			temp, err := strconv.ParseFloat(tempStr, 64)
			if err != nil {
				continue
			}
			tempC := temp / 1000.0

			switch {
			case strings.Contains(zoneType, "cpu") || strings.Contains(zoneType, "x86_pkg") || strings.Contains(zoneType, "acpi"):
				if info.CPU == 0 {
					info.CPU = tempC
					found = true
				}
			case strings.Contains(zoneType, "gpu"):
				info.GPU = tempC
				found = true
			case strings.Contains(zoneType, "pch") || strings.Contains(zoneType, "board") || strings.Contains(zoneType, "motherboard"):
				info.Motherboard = tempC
				found = true
			}
		}
	}

	// Try hwmon as fallback for CPU temp
	if info.CPU == 0 {
		hwmonEntries, err := os.ReadDir("/sys/class/hwmon")
		if err == nil {
			for _, e := range hwmonEntries {
				name := readFileStr(fmt.Sprintf("/sys/class/hwmon/%s/name", e.Name()))
				if strings.Contains(strings.ToLower(name), "coretemp") || strings.Contains(strings.ToLower(name), "k10temp") {
					for i := 1; i <= 10; i++ {
						tempStr := readFileStr(fmt.Sprintf("/sys/class/hwmon/%s/temp%d_input", e.Name(), i))
						if tempStr != "" {
							temp, err := strconv.ParseFloat(tempStr, 64)
							if err == nil {
								info.CPU = temp / 1000.0
								found = true
								break
							}
						}
					}
					break
				}
			}
		}
	}

	// GPU temp via sensors or nvidia-smi
	if info.GPU == 0 {
		out, err := exec.Command("nvidia-smi", "--query-gpu=temperature.gpu", "--format=csv,noheader").Output()
		if err == nil {
			if temp, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
				info.GPU = temp
				found = true
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("temperature sensors not accessible (may require lm-sensors or root)")
	}
	return info, nil
}

// ─── SMART ───────────────────────────────────────────────────────────────────

func collectSMART(_ context.Context) ([]domain.SMARTInfo, error) {
	// Try smartctl
	out, err := exec.Command("smartctl", "--scan").Output()
	if err != nil {
		return nil, fmt.Errorf("smartctl not available (install smartmontools): %w", err)
	}

	var smartResults []domain.SMARTInfo
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 1 || strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		dev := fields[0]
		if !strings.HasPrefix(dev, "/dev/") {
			continue
		}
		si := domain.SMARTInfo{DeviceID: dev}

		// Get detailed SMART info
		detail, err := exec.Command("smartctl", "-A", "-H", "-i", dev).Output()
		if err != nil {
			continue
		}
		for _, dline := range strings.Split(string(detail), "\n") {
			dline = strings.TrimSpace(dline)
			if strings.Contains(dline, "Model Number") || strings.Contains(dline, "Device Model") {
				parts := strings.SplitN(dline, ":", 2)
				if len(parts) == 2 {
					si.Model = strings.TrimSpace(parts[1])
				}
			}
			if strings.Contains(dline, "SMART overall-health") || strings.Contains(dline, "SMART Health Status") {
				parts := strings.SplitN(dline, ":", 2)
				if len(parts) == 2 {
					si.Health = strings.TrimSpace(parts[1])
				}
			}
			if strings.Contains(dline, "Temperature") && !strings.Contains(dline, "Transport") {
				fields := strings.Fields(dline)
				for _, f := range fields {
					if t, err := strconv.ParseFloat(f, 64); err == nil && t > 10 && t < 100 {
						si.TemperatureC = t
						break
					}
				}
			}
			if strings.Contains(dline, "Power_On_Hours") {
				fields := strings.Fields(dline)
				for _, f := range fields {
					if v, err := strconv.ParseInt(f, 10, 64); err == nil && v > 0 {
						si.PowerOnHours = v
					}
				}
			}
			if strings.Contains(dline, "Power_Cycle_Count") {
				fields := strings.Fields(dline)
				for _, f := range fields {
					if v, err := strconv.ParseInt(f, 10, 64); err == nil && v > 0 {
						si.PowerCycles = v
					}
				}
			}
			if strings.Contains(dline, "Percentage Used") || strings.Contains(dline, "Media_Wearout") {
				fields := strings.Fields(dline)
				for _, f := range fields {
					if v, err := strconv.ParseFloat(f, 64); err == nil {
						if strings.Contains(dline, "Percentage Used") {
							si.RemainingLife = 100 - v
						} else {
							si.RemainingLife = v
						}
					}
				}
			}
			if strings.Contains(dline, "Data Units Written") {
				fields := strings.Fields(dline)
				for _, f := range fields {
					if v, err := strconv.ParseFloat(f, 64); err == nil && v > 100 {
						si.TotalWritesGB = v * 512 / 1e9 // 512-byte sectors → GB
					}
				}
			}
			if strings.Contains(dline, "Data Units Read") {
				fields := strings.Fields(dline)
				for _, f := range fields {
					if v, err := strconv.ParseFloat(f, 64); err == nil && v > 100 {
						si.TotalReadsGB = v * 512 / 1e9
					}
				}
			}
		}
		if si.Model != "" {
			smartResults = append(smartResults, si)
		}
	}

	if len(smartResults) == 0 {
		return nil, fmt.Errorf("no SMART data available (may require root or smartmontools)")
	}
	return smartResults, nil
}

// ─── Processes ───────────────────────────────────────────────────────────────

func collectProcesses(_ context.Context) ([]domain.ProcessInfo, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("process.Processes: %w", err)
	}
	var result []domain.ProcessInfo
	for _, p := range procs {
		name, _ := p.Name()
		if name == "" {
			continue
		}
		cpuPct, _ := p.CPUPercent()
		memPct, _ := p.MemoryPercent()
		status, _ := p.Status()
		s := ""
		if len(status) > 0 {
			s = status[0]
		}
		result = append(result, domain.ProcessInfo{
			PID: p.Pid, Name: name, CPUPercent: cpuPct,
			MemPercent: float64(memPct), Status: s,
		})
	}
	return result, nil
}

// ─── Security ────────────────────────────────────────────────────────────────

func collectSecurity(_ context.Context) (*domain.SecurityInfo, error) {
	info := &domain.SecurityInfo{}
	out, err := exec.Command("ufw", "status").Output()
	if err == nil {
		info.FirewallEnabled = strings.Contains(string(out), "active")
	}
	if !info.FirewallEnabled {
		out, err = exec.Command("iptables", "-L", "-n").Output()
		if err == nil {
			rules := 0
			for _, l := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(l, "ACCEPT") || strings.HasPrefix(l, "DROP") || strings.HasPrefix(l, "REJECT") {
					rules++
				}
			}
			info.FirewallEnabled = rules > 0
		}
	}
	data, err := os.ReadFile("/sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c")
	if err == nil && len(data) >= 5 {
		info.SecureBootEnabled = data[4] == 1
	}
	out, err = exec.Command("lsblk", "-o", "FSTYPE").Output()
	if err == nil {
		info.BitLockerEnabled = strings.Contains(string(out), "crypto_LUKS")
	}
	out, err = exec.Command("sh", "-c", "apt list --upgradable 2>/dev/null | grep -c upgradable || echo 0").Output()
	if err == nil {
		count := strings.TrimSpace(string(out))
		info.OSUpdateCurrent = count == "0"
	}
	if data, err := os.ReadFile("/var/log/apt/history.log"); err == nil {
		lines := strings.Split(string(data), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.HasPrefix(lines[i], "Start-Date:") {
				info.LastUpdateDate = strings.TrimSpace(strings.TrimPrefix(lines[i], "Start-Date:"))
				break
			}
		}
	}
	if _, err := os.Stat("/sys/class/tpm/tpm0"); err == nil {
		info.TPMPresent = true
		if desc := readFileStr("/sys/class/tpm/tpm0/device/description"); desc != "" {
			info.TPMVersion = desc
		} else {
			info.TPMVersion = "present"
		}
	}
	return info, nil
}

// ─── Services ────────────────────────────────────────────────────────────────

func collectServices(_ context.Context) ([]domain.ServiceInfo, error) {
	// systemd
	out, err := exec.Command("systemctl", "list-units", "--type=service", "--all", "--no-pager", "--no-legend").Output()
	if err != nil {
		return nil, fmt.Errorf("systemctl not available: %w", err)
	}
	var services []domain.ServiceInfo
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name := strings.TrimSuffix(fields[0], ".service")
		status := "unknown"
		if strings.Contains(fields[2], "running") || strings.Contains(fields[3], "running") {
			status = "running"
		} else if strings.Contains(fields[2], "dead") || strings.Contains(fields[3], "dead") {
			status = "stopped"
		} else if strings.Contains(fields[2], "failed") {
			status = "failed"
		}
		services = append(services, domain.ServiceInfo{
			Name:   name,
			Status: status,
		})
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no services found")
	}
	return services, nil
}

// ─── Drivers ─────────────────────────────────────────────────────────────────

func collectDrivers(_ context.Context) ([]domain.DriverInfo, error) {
	out, err := exec.Command("lsmod").Output()
	if err != nil {
		return nil, fmt.Errorf("lsmod not available: %w", err)
	}
	var drivers []domain.DriverInfo
	for _, line := range strings.Split(string(out), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		drivers = append(drivers, domain.DriverInfo{
			Name:   fields[0],
			Status: "loaded",
		})
	}
	if len(drivers) == 0 {
		return nil, fmt.Errorf("no drivers found")
	}
	return drivers, nil
}

// ─── Startup ─────────────────────────────────────────────────────────────────

func collectStartup(_ context.Context) ([]domain.StartupItem, error) {
	// systemd enabled services as startup items
	out, err := exec.Command("systemctl", "list-unit-files", "--type=service", "--state=enabled", "--no-pager", "--no-legend").Output()
	if err != nil {
		return nil, fmt.Errorf("systemctl not available: %w", err)
	}
	var items []domain.StartupItem
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		items = append(items, domain.StartupItem{
			Name:    fields[0],
			Source:  "systemd",
			Enabled: true,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no startup items found")
	}
	return items, nil
}

// ─── Programs ────────────────────────────────────────────────────────────────

func collectPrograms(_ context.Context) ([]domain.ProgramInfo, error) {
	var progs []domain.ProgramInfo

	// dpkg
	out, err := exec.Command("dpkg-query", "-W", "--showformat=${Package}\t${Version}\t${Status}\n").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Split(line, "\t")
			if len(fields) < 3 {
				continue
			}
			if !strings.Contains(fields[2], "installed") {
				continue
			}
			progs = append(progs, domain.ProgramInfo{
				Name:    fields[0],
				Version: fields[1],
			})
		}
	}

	// snap
	out, err = exec.Command("snap", "list").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n")[1:] {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			progs = append(progs, domain.ProgramInfo{
				Name:    fields[0],
				Version: fields[1],
			})
		}
	}

	if len(progs) == 0 {
		return nil, fmt.Errorf("no installed programs found")
	}
	return progs, nil
}

// ─── Environment ─────────────────────────────────────────────────────────────

func collectEnvironment(_ context.Context) (map[string]string, error) {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	if len(env) == 0 {
		return nil, fmt.Errorf("no environment variables found")
	}
	return env, nil
}

// ─── USB ─────────────────────────────────────────────────────────────────────

func collectUSB(_ context.Context) ([]domain.USBDevice, error) {
	out, err := exec.Command("lsusb").Output()
	if err != nil {
		return nil, fmt.Errorf("lsusb not available (install usbutils): %w", err)
	}
	var devices []domain.USBDevice
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		// Bus 001 Device 002: ID 1234:5678 Device Name
		fields := strings.SplitN(line, ": ", 2)
		if len(fields) < 2 {
			continue
		}
		name := fields[1]
		idPart := ""
		if idx := strings.Index(line, "ID "); idx >= 0 {
			idPart = line[idx+3:]
			if spaceIdx := strings.Index(idPart, " "); spaceIdx >= 0 {
				idPart = idPart[:spaceIdx]
			}
		}
		vendorID := ""
		productID := ""
		if parts := strings.Split(idPart, ":"); len(parts) == 2 {
			vendorID = parts[0]
			productID = parts[1]
		}
		devices = append(devices, domain.USBDevice{
			Name:      name,
			VendorID:  vendorID,
			ProductID: productID,
			Status:    "connected",
		})
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("no USB devices found")
	}
	return devices, nil
}

// ─── Bluetooth ───────────────────────────────────────────────────────────────

func collectBluetooth(_ context.Context) (*domain.BluetoothInfo, error) {
	out, err := exec.Command("bluetoothctl", "devices").Output()
	if err != nil {
		return nil, fmt.Errorf("bluetooth not available: %w", err)
	}
	info := &domain.BluetoothInfo{}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "Device ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		info.Devices = append(info.Devices, domain.BluetoothDevice{
			MAC:  fields[1],
			Name: strings.Join(fields[2:], " "),
		})
	}
	// Adapter info
	adapterOut, err := exec.Command("bluetoothctl", "show").Output()
	if err == nil {
		adapter := domain.BluetoothAdapter{Name: "default", Status: "unknown"}
		for _, line := range strings.Split(string(adapterOut), "\n") {
			if strings.Contains(line, "Powered:") && strings.Contains(line, "yes") {
				adapter.Status = "on"
			}
			if strings.HasPrefix(line, "Controller") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					adapter.MAC = fields[1]
				}
			}
		}
		info.Adapters = []domain.BluetoothAdapter{adapter}
	}
	if len(info.Devices) == 0 && len(info.Adapters) == 0 {
		return nil, fmt.Errorf("no Bluetooth devices or adapters found")
	}
	return info, nil
}

// ─── PCI ─────────────────────────────────────────────────────────────────────

func collectPCI(_ context.Context) ([]domain.PCIDevice, error) {
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return nil, fmt.Errorf("lspci not available (install pciutils): %w", err)
	}
	var devices []domain.PCIDevice
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		descParts := strings.SplitN(parts[1], ": ", 2)
		if len(descParts) < 2 {
			continue
		}
		devices = append(devices, domain.PCIDevice{
			Name:        descParts[1],
			Description: descParts[0],
			Status:      "present",
		})
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("no PCI devices found")
	}
	return devices, nil
}

// ─── Windows Features (N/A on Linux) ─────────────────────────────────────────

func collectWinFeatures(_ context.Context) (*domain.WinFeatures, error) {
	return nil, fmt.Errorf("Windows features not applicable on Linux")
}
