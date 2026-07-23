//go:build darwin

package platform

import (
	"bufio"
	"bytes"
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
	collector.RegisterTemperatures(collectDarwinTemps)
	collector.RegisterSMART(collectDarwinSMART)
	collector.RegisterServices(collectDarwinServices)
	collector.RegisterDrivers(collectDarwinDrivers)
	collector.RegisterStartup(collectDarwinStartup)
	collector.RegisterPrograms(collectDarwinPrograms)
	collector.RegisterEnvironment(collectDarwinEnvironment)
	collector.RegisterUSB(collectDarwinUSB)
	collector.RegisterBluetooth(collectDarwinBluetooth)
	collector.RegisterPCI(collectDarwinPCI)
	collector.RegisterWinFeatures(collectDarwinWinFeatures)
}

func sysctl(key string) string {
	out, err := exec.Command("sysctl", "-n", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func systemProfiler(dt string) string {
	out, err := exec.Command("system_profiler", dt).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// ─── OS ──────────────────────────────────────────────────────────────────────

func collectOS(_ context.Context) (*domain.OSInfo, error) {
	hi, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("host.Info: %w", err)
	}
	hostname, _ := os.Hostname()
	ver := sysctl("kern.osproductversion")
	build := sysctl("kern.osversion")

	uptimeDur := time.Duration(hi.Uptime) * time.Second
	days := int(uptimeDur.Hours()) / 24
	hours := int(uptimeDur.Hours()) % 24
	mins := int(uptimeDur.Minutes()) % 60

	return &domain.OSInfo{
		Edition:      "macOS",
		Version:      ver,
		Build:        build,
		ProductName:  "macOS " + ver,
		Architecture: runtime.GOARCH,
		Uptime:       fmt.Sprintf("%dd %dh %dm", days, hours, mins),
		Hostname:     hostname,
		Kernel:       hi.KernelVersion,
	}, nil
}

// ─── CPU ─────────────────────────────────────────────────────────────────────

func collectCPU(_ context.Context) (*domain.CPUInfo, error) {
	info := &domain.CPUInfo{
		Architecture: runtime.GOARCH,
		Model:        sysctl("machdep.cpu.brand_string"),
		Vendor:       sysctl("machdep.cpu.vendor"),
		Family:       sysctl("machdep.cpu.family"),
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

	// Current freq (may not be available on Apple Silicon)
	freq := sysctl("hw.cpufrequency")
	if freq != "" {
		hz, _ := strconv.ParseFloat(freq, 64)
		info.CurrentMHz = hz / 1e6
		info.MaxMHz = hz / 1e6
	}

	return info, nil
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
	// Use gopsutil partitions + diskutil info for model/serial
	parts, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	deviceMap := map[string]*domain.DiskInfo{}
	for _, p := range parts {
		if strings.HasPrefix(p.Device, "/dev/disk") && !strings.Contains(p.Device, "s") {
			continue // skip root disk entries
		}
		base := getBaseDisk(p.Device)
		if _, exists := deviceMap[base]; !exists {
			di := &domain.DiskInfo{
				DeviceID: base,
				Model:    base,
			}
			// Get info via diskutil
			info := getDiskUtilInfo(base)
			if info != nil {
				di.Model = info.model
				di.MediaType = info.mediaType
				di.SizeBytes = info.sizeBytes
			}
			deviceMap[base] = di
		}

		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}
		deviceMap[base].Partitions = append(deviceMap[base].Partitions, domain.Partition{
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

type diskUtilResult struct {
	model     string
	mediaType string
	sizeBytes uint64
}

func getDiskUtilInfo(dev string) *diskUtilResult {
	out, err := exec.Command("diskutil", "info", dev).Output()
	if err != nil {
		return nil
	}
	r := &diskUtilResult{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "Media Name") {
			r.model = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
		if strings.Contains(line, "Solid State") {
			val := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			if strings.ToLower(val) == "yes" {
				r.mediaType = "SSD"
			} else {
				r.mediaType = "HDD"
			}
		}
		if strings.Contains(line, "Total Size") {
			val := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			// Parse "500.3 GB (500277790720 Bytes)"
			if start := strings.Index(val, "("); start >= 0 {
				end := strings.Index(val, " Bytes")
				if end > start {
					b, _ := strconv.ParseUint(strings.TrimSpace(val[start+1:end]), 10, 64)
					r.sizeBytes = b
				}
			}
		}
	}
	return r
}

func getBaseDisk(dev string) string {
	// /dev/disk1s1 -> /dev/disk1
	name := strings.TrimPrefix(dev, "/dev/")
	for i, ch := range name {
		if ch == 's' && i > 0 {
			return "/dev/" + name[:i]
		}
	}
	return dev
}

// ─── Network ─────────────────────────────────────────────────────────────────

func collectNetwork(_ context.Context) ([]domain.NetAdapter, error) {
	out, err := exec.Command("networksetup", "-listallhardwareports").Output()
	if err != nil {
		return nil, fmt.Errorf("networksetup: %w", err)
	}

	var adapters []domain.NetAdapter
	current := domain.NetAdapter{}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "Hardware Port:") {
			if current.Name != "" {
				adapters = append(adapters, current)
			}
			current = domain.NetAdapter{
				Name: strings.TrimSpace(strings.TrimPrefix(line, "Hardware Port:")),
			}
		}
		if strings.HasPrefix(line, "Ethernet Address:") {
			current.MAC = strings.TrimSpace(strings.TrimPrefix(line, "Ethernet Address:"))
		}
	}
	if current.Name != "" {
		adapters = append(adapters, current)
	}

	// Get IPs via ifconfig
	ifOut, err := exec.Command("ifconfig").Output()
	if err == nil {
		current := ""
		for _, line := range strings.Split(string(ifOut), "\n") {
			if !strings.HasPrefix(line, "\t") && strings.Contains(line, ":") {
				current = strings.SplitN(line, ":", 2)[0]
			}
			if strings.Contains(line, "inet ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					for i := range adapters {
						// Match by rough name
						if strings.Contains(adapters[i].Name, current) || current == "en0" && i == 0 {
							adapters[i].IPv4 = append(adapters[i].IPv4, parts[1])
						}
					}
				}
			}
		}
	}

	// DNS
	dnsOut, err := exec.Command("scutil", "--dns").Output()
	if err == nil {
		var dnsServers []string
		for _, line := range strings.Split(string(dnsOut), "\n") {
			if strings.Contains(line, "nameserver[") || strings.Contains(line, "nameserver :") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					dnsServers = append(dnsServers, parts[2])
				}
			}
		}
		if len(adapters) > 0 {
			adapters[0].DNS = dnsServers
		}
	}

	// Route
	routeOut, err := exec.Command("route", "-n", "get", "default").Output()
	if err == nil {
		for _, line := range strings.Split(string(routeOut), "\n") {
			if strings.Contains(line, "gateway:") {
				gw := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				if len(adapters) > 0 {
					adapters[0].Gateway = []string{gw}
				}
			}
		}
	}

	for i := range adapters {
		adapters[i].IsUp = true
	}
	return adapters, nil
}

// ─── BIOS ────────────────────────────────────────────────────────────────────

func collectBIOS(_ context.Context) (*domain.BIOSInfo, error) {
	out := systemProfiler("SPHardwareDataType")
	info := &domain.BIOSInfo{Manufacturer: "Apple Inc."}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Boot ROM Version") {
			info.Version = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
	}
	if info.Version == "" {
		return nil, fmt.Errorf("BIOS info not available on this system")
	}
	return info, nil
}

// ─── Motherboard ─────────────────────────────────────────────────────────────

func collectMotherboard(_ context.Context) (*domain.MoboInfo, error) {
	out := systemProfiler("SPHardwareDataType")
	info := &domain.MoboInfo{Manufacturer: "Apple Inc."}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Model Identifier") {
			info.Model = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
		if strings.Contains(line, "Serial Number") {
			info.SerialNumber = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
	}
	if info.Model == "" {
		return nil, fmt.Errorf("motherboard info not available")
	}
	return info, nil
}

// ─── GPU ─────────────────────────────────────────────────────────────────────

func collectGPU(_ context.Context) ([]domain.GPUInfo, error) {
	out := systemProfiler("SPDisplaysDataType")
	var gpus []domain.GPUInfo
	current := domain.GPUInfo{}

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      ") && strings.Contains(line, ":") {
			if current.Name != "" {
				gpus = append(gpus, current)
			}
			current = domain.GPUInfo{
				Name: strings.TrimSpace(strings.SplitN(line, ":", 2)[0]),
			}
		}
		if strings.Contains(line, "Vendor") {
			current.Vendor = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
		if strings.Contains(line, "Metal") || strings.Contains(line, "Driver") {
			current.DriverVersion = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
	}
	if current.Name != "" {
		gpus = append(gpus, current)
	}
	if len(gpus) == 0 {
		return nil, fmt.Errorf("no GPU detected")
	}
	return gpus, nil
}

// ─── Monitor ─────────────────────────────────────────────────────────────────

func collectMonitor(_ context.Context) ([]domain.MonitorInfo, error) {
	out := systemProfiler("SPDisplaysDataType")
	var monitors []domain.MonitorInfo
	current := domain.MonitorInfo{}

	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Resolution") {
			res := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			parts := strings.Fields(res)
			if len(parts) >= 2 {
				current.ResolutionX, _ = strconv.Atoi(parts[0])
				current.ResolutionY, _ = strconv.Atoi(parts[1])
			}
		}
		if strings.Contains(line, "Refresh Rate") || strings.Contains(line, "Display Type") {
			val := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			if strings.Contains(val, "Hz") {
				hz := strings.Split(val, " ")[0]
				current.RefreshRateHz, _ = strconv.Atoi(hz)
			}
		}
		// Detection of display name
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") && current.Name == "" {
			// This might be a display header
		}
		if strings.Contains(line, "Connection Type") {
			if current.Name == "" {
				current.Name = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
			monitors = append(monitors, current)
			current = domain.MonitorInfo{}
		}
	}
	if current.Name != "" {
		monitors = append(monitors, current)
	}
	if len(monitors) == 0 {
		return nil, fmt.Errorf("no monitors detected")
	}
	return monitors, nil
}

// ─── Battery ─────────────────────────────────────────────────────────────────

func collectBattery(_ context.Context) (*domain.BatteryInfo, error) {
	out, err := exec.Command("pmset", "-g", "batt").Output()
	if err != nil {
		return nil, fmt.Errorf("no battery: %w", err)
	}
	output := string(out)
	if strings.Contains(output, "No Battery") {
		return nil, fmt.Errorf("no battery present")
	}

	info := &domain.BatteryInfo{IsPresent: true}
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "%") {
			// "  85%; charging;"
			fields := strings.Fields(line)
			for _, f := range fields {
				if strings.HasSuffix(f, "%;") || strings.HasSuffix(f, "%") {
					f = strings.TrimRight(f, "%;")
					info.ChargePercent, _ = strconv.ParseFloat(f, 64)
				}
			}
			info.IsCharging = strings.Contains(line, "charging")
		}
	}

	// Health
	healthOut, err := exec.Command("system_profiler", "SPPowerDataType").Output()
	if err == nil {
		for _, line := range strings.Split(string(healthOut), "\n") {
			if strings.Contains(line, "Condition") {
				cond := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				if strings.Contains(strings.ToLower(cond), "normal") {
					info.HealthPercent = 100
				}
			}
			if strings.Contains(line, "Maximum Capacity") {
				val := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				val = strings.TrimSuffix(val, "%")
				info.HealthPercent, _ = strconv.ParseFloat(val, 64)
			}
		}
	}

	return info, nil
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
			PID:        p.Pid,
			Name:       name,
			CPUPercent: cpuPct,
			MemPercent: float64(memPct),
			Status:     s,
		})
	}
	return result, nil
}

// ─── Security ────────────────────────────────────────────────────────────────

func collectSecurity(_ context.Context) (*domain.SecurityInfo, error) {
	info := &domain.SecurityInfo{}

	// FileVault
	out, err := exec.Command("fdesetup", "status").Output()
	if err == nil {
		info.BitLockerEnabled = strings.Contains(string(out), "FileVault is On")
	}

	// Firewall
	out, err = exec.Command("/usr/libexec/ApplicationFirewall/socketfilterfw", "--getglobalstate").Output()
	if err == nil {
		info.FirewallEnabled = strings.Contains(string(out), "enabled")
	}

	// SIP (like Secure Boot)
	out, err = exec.Command("csrutil", "status").Output()
	if err == nil {
		info.SecureBootEnabled = strings.Contains(string(out), "enabled")
	}

	// T2/Apple Silicon security
	out, err = exec.Command("system_profiler", "SPiBridgeDataType").Output()
	if err == nil {
		info.TPMPresent = strings.Contains(string(out), "Apple T2") || strings.Contains(string(out), "Apple Silicon")
	}

	// macOS doesn't have Windows Defender
	info.DefenderEnabled = false
	info.DefenderRealtime = false

	// Check XProtect (macOS built-in malware protection)
	out, err = exec.Command("system_profiler", "SPInstallDataType").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "Install Date") {
				info.LastUpdateDate = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				break
			}
		}
	}
	info.OSUpdateCurrent = true // Simplified — would need more complex check

	return info, nil
}

// ─── macOS: Temperatures ─────────────────────────────────────────────────────

func collectDarwinTemps(_ context.Context) (*domain.TempInfo, error) {
	info := &domain.TempInfo{}
	// Try powermetrics (requires root)
	out, err := exec.Command("powermetrics", "--samplers", "smc", "-n", "1", "-i", "100").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "CPU die temperature") {
				parts := strings.Fields(line)
				for _, p := range parts {
					if t, e := strconv.ParseFloat(strings.TrimRight(p, "C"), 64); e == nil && t > 20 && t < 120 {
						info.CPU = t
					}
				}
			}
			if strings.Contains(line, "GPU die temperature") {
				parts := strings.Fields(line)
				for _, p := range parts {
					if t, e := strconv.ParseFloat(strings.TrimRight(p, "C"), 64); e == nil && t > 20 && t < 120 {
						info.GPU = t
					}
				}
			}
		}
	}
	if info.CPU == 0 && info.GPU == 0 {
		return nil, fmt.Errorf("temperature sensors require root (powermetrics)")
	}
	return info, nil
}

// ─── macOS: SMART ────────────────────────────────────────────────────────────

func collectDarwinSMART(_ context.Context) ([]domain.SMARTInfo, error) {
	out, err := exec.Command("diskutil", "info", "disk0").Output()
	if err != nil {
		return nil, fmt.Errorf("diskutil not available: %w", err)
	}
	var results []domain.SMARTInfo
	si := domain.SMARTInfo{DeviceID: "/dev/disk0"}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Media Name") {
			si.Model = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
		if strings.Contains(line, "Volume Used") {
			// Approximate
		}
		if strings.Contains(line, "SMART Status") {
			si.Health = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
	}
	if si.Model != "" {
		results = append(results, si)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no SMART data available")
	}
	return results, nil
}

// ─── macOS: Services (LaunchDaemons) ─────────────────────────────────────────

func collectDarwinServices(_ context.Context) ([]domain.ServiceInfo, error) {
	out, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return nil, fmt.Errorf("launchctl: %w", err)
	}
	var services []domain.ServiceInfo
	for _, line := range strings.Split(string(out), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		services = append(services, domain.ServiceInfo{
			Name:   fields[2],
			Status: "running",
		})
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no services found")
	}
	return services, nil
}

func collectDarwinDrivers(_ context.Context) ([]domain.DriverInfo, error) {
	return nil, fmt.Errorf("macOS does not expose drivers in the same way as Windows/Linux")
}

func collectDarwinStartup(_ context.Context) ([]domain.StartupItem, error) {
	out, err := exec.Command("osascript", "-e", `tell application "System Events" to get the name of every login item`).Output()
	if err != nil {
		return nil, fmt.Errorf("login items: %w", err)
	}
	var items []domain.StartupItem
	for _, name := range strings.Split(strings.TrimSpace(string(out)), ", ") {
		if name != "" {
			items = append(items, domain.StartupItem{Name: name, Source: "LoginItems", Enabled: true})
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no startup items found")
	}
	return items, nil
}

func collectDarwinPrograms(_ context.Context) ([]domain.ProgramInfo, error) {
	entries, err := os.ReadDir("/Applications")
	if err != nil {
		return nil, fmt.Errorf("/Applications: %w", err)
	}
	var progs []domain.ProgramInfo
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".app") {
			progs = append(progs, domain.ProgramInfo{Name: strings.TrimSuffix(e.Name(), ".app")})
		}
	}
	if len(progs) == 0 {
		return nil, fmt.Errorf("no applications found")
	}
	return progs, nil
}

func collectDarwinEnvironment(_ context.Context) (map[string]string, error) {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env, nil
}

func collectDarwinUSB(_ context.Context) ([]domain.USBDevice, error) {
	out := systemProfiler("SPUSBDataType")
	var devices []domain.USBDevice
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Product ID") || strings.Contains(line, "Vendor ID") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") {
			// This could be a device name
		}
		if strings.Contains(line, "Manufacturer:") {
			name := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			if name != "" {
				devices = append(devices, domain.USBDevice{Name: name, Status: "connected"})
			}
		}
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("no USB devices found")
	}
	return devices, nil
}

func collectDarwinBluetooth(_ context.Context) (*domain.BluetoothInfo, error) {
	out := systemProfiler("SPBluetoothDataType")
	info := &domain.BluetoothInfo{}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Address:") {
			info.Adapters = append(info.Adapters, domain.BluetoothAdapter{
				Name: "Apple Bluetooth", Status: "on",
			})
			break
		}
	}
	if len(info.Adapters) == 0 {
		return nil, fmt.Errorf("no Bluetooth adapter found")
	}
	return info, nil
}

func collectDarwinPCI(_ context.Context) ([]domain.PCIDevice, error) {
	out := systemProfiler("SPPCIDataType")
	var devices []domain.PCIDevice
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ":") && len(trimmed) > 2 {
			name := strings.TrimSuffix(trimmed, ":")
			devices = append(devices, domain.PCIDevice{Name: name, Status: "present"})
		}
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("no PCI devices found")
	}
	return devices, nil
}

func collectDarwinWinFeatures(_ context.Context) (*domain.WinFeatures, error) {
	return nil, fmt.Errorf("Windows features not applicable on macOS")
}
