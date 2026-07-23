//go:build windows

package platform

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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

// ─── Helpers ─────────────────────────────────────────────────────────────────

func ps(script string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"[Console]::OutputEncoding = [Text.Encoding]::UTF8; "+script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func wmiQuery(class string, props []string) (string, error) {
	return ps(fmt.Sprintf("Get-CimInstance -ClassName %s | Select-Object %s | ConvertTo-Csv -NoTypeInformation", class, strings.Join(props, ", ")))
}

func parseCSV(line string) []string {
	var f []string
	var c strings.Builder
	q := false
	for _, ch := range line {
		if ch == '"' {
			q = !q
			continue
		}
		if ch == ',' && !q {
			f = append(f, c.String())
			c.Reset()
			continue
		}
		c.WriteRune(ch)
	}
	f = append(f, c.String())
	return f
}

// parseWMIDate robustly handles multiple WMI date formats.
func parseWMIDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Standard: 20260627000000.000000+180
	re := regexp.MustCompile(`(\d{4})(\d{2})(\d{2})`)
	if m := re.FindStringSubmatch(raw); m != nil {
		y, _ := strconv.Atoi(m[1])
		if y >= 1990 && y <= 2099 {
			return fmt.Sprintf("%s-%s-%s", m[1], m[2], m[3])
		}
	}
	// DD.MM.YYYY or DD/MM/YYYY
	if re2 := regexp.MustCompile(`(\d{2})[./](\d{2})[./](\d{4})`); re2.MatchString(raw) {
		m := re2.FindStringSubmatch(raw)
		return fmt.Sprintf("%s-%s-%s", m[3], m[2], m[1])
	}
	return raw
}

func normVendor(v string) string {
	vl := strings.ToLower(v)
	switch {
	case strings.Contains(vl, "nvidia"):
		return "NVIDIA"
	case strings.Contains(vl, "intel"):
		return "Intel"
	case strings.Contains(vl, "amd") || strings.Contains(vl, "ati") || strings.Contains(vl, "advanced micro"):
		return "AMD"
	case strings.Contains(vl, "qualcomm"):
		return "Qualcomm"
	default:
		if v == "" {
			return "Unknown"
		}
		return v
	}
}

// ─── OS ──────────────────────────────────────────────────────────────────────

func collectOS(_ context.Context) (*domain.OSInfo, error) {
	hi, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("host.Info: %w", err)
	}
	hostname, _ := os.Hostname()
	d := time.Duration(hi.Uptime) * time.Second
	ed, ver := "", ""
	if out, e := ps("(Get-ItemProperty 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion').EditionID"); e == nil {
		ed = strings.TrimSpace(out)
	}
	if out, e := ps("(Get-ItemProperty 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion').VersionId"); e == nil {
		ver = strings.TrimSpace(out)
	}
	if out, e := ps("(Get-CimInstance Win32_OperatingSystem).Caption"); e == nil {
		hi.Platform = strings.TrimSpace(out)
	}
	inst := ""
	if out, e := ps("(Get-ItemProperty 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion').InstallDate"); e == nil {
		if epoch, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64); err == nil {
			inst = time.Unix(epoch, 0).Format("2006-01-02")
		}
	}
	return &domain.OSInfo{
		Edition: ed, Version: ver, Build: hi.KernelVersion,
		ProductName: hi.Platform, Architecture: runtime.GOARCH,
		InstallDate: inst, Uptime: fmt.Sprintf("%dd %dh %dm", int(d.Hours())/24, int(d.Hours())%24, int(d.Minutes())%60),
		Hostname: hostname, Kernel: hi.KernelVersion,
	}, nil
}

// ─── CPU ─────────────────────────────────────────────────────────────────────

// cpuTurboTable maps known Intel/AMD model numbers to turbo MHz.
// Used as fallback when WMI only returns base clock.
var cpuTurboTable = map[string]float64{
	"12450h": 4400, "12500h": 4500, "12600k": 4900, "12700k": 5000, "12900k": 5200,
	"13400f": 4600, "13600k": 5100, "13700k": 5400, "13900k": 5800,
	"14400f": 4700, "14600k": 5300, "14700k": 5600, "14900k": 6000,
	"i5-12450h": 4400, "i7-12700h": 4700, "i9-12900h": 5000,
}

func collectCPU(_ context.Context) (*domain.CPUInfo, error) {
	info := &domain.CPUInfo{Architecture: runtime.GOARCH}
	infos, err := cpu.Info()
	if err == nil && len(infos) > 0 {
		c := infos[0]
		info.Model = strings.TrimSpace(c.ModelName)
		info.Vendor = strings.TrimSpace(c.VendorID)
		info.Family = cpuFamilyName(info.Vendor, strings.TrimSpace(c.Family))
		if c.Mhz > 0 {
			info.CurrentMHz = c.Mhz
		}
	}
	if phys, err := cpu.Counts(false); err == nil {
		info.PhysicalCores = int32(phys)
	}
	if logical, err := cpu.Counts(true); err == nil {
		info.LogicalCores = int32(logical)
	}
	if pcts, err := cpu.Percent(time.Second, false); err == nil && len(pcts) > 0 {
		info.UsagePercent = pcts[0]
	}

	// WMI for clock speeds
	if out, err := wmiQuery("Win32_Processor", []string{"MaxClockSpeed", "CurrentClockSpeed", "Name"}); err == nil {
		for _, line := range strings.Split(out, "\n")[1:] {
			f := parseCSV(line)
			if len(f) >= 2 {
				if v, e := strconv.ParseFloat(strings.TrimSpace(f[0]), 64); e == nil && v > 0 {
					info.MaxMHz = v
				}
				if v, e := strconv.ParseFloat(strings.TrimSpace(f[1]), 64); e == nil && v > 0 {
					info.CurrentMHz = v
				}
				// Try to extract turbo from CPU name
				if len(f) >= 3 {
					name := strings.ToLower(strings.TrimSpace(f[2]))
					// Check turbo table
					for key, turbo := range cpuTurboTable {
						if strings.Contains(name, key) {
							info.MaxMHz = turbo
							break
						}
					}
					// Fallback: if name contains "@ X.XXGHz" and max is base, boost by 1.5x
					if info.MaxMHz > 0 && info.MaxMHz <= 3000 {
						if turbo := extractBaseGHz(name); turbo > 0 && info.MaxMHz == turbo*1000 {
							// Base = max means WMI only reported base.
							// Estimate turbo as 1.5-2x base for mobile CPUs
							if strings.Contains(name, "h") || strings.Contains(name, "u") || strings.Contains(name, "p") {
								info.MaxMHz = math.Round(turbo * 1000 * 1.8)
							}
						}
					}
				}
				break
			}
		}
	}

	// If current still 0, use MaxMHz as reference (better than 0)
	if info.CurrentMHz == 0 && info.MaxMHz > 0 {
		info.CurrentMHz = info.MaxMHz
	}

	return info, nil
}

func extractBaseGHz(name string) float64 {
	re := regexp.MustCompile(`@\s*([\d.]+)\s*GHz`)
	if m := re.FindStringSubmatch(name); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v
	}
	return 0
}

func cpuFamilyName(vendor, family string) string {
	if strings.Contains(strings.ToLower(vendor), "intel") || vendor == "GenuineIntel" {
		switch family {
		case "6":
			return "Intel Core/Xeon"
		case "15":
			return "Intel NetBurst"
		case "205":
			return "Intel Alder Lake"
		}
	}
	if strings.Contains(strings.ToLower(vendor), "amd") || vendor == "AuthenticAMD" {
		switch family {
		case "23":
			return "AMD Zen 1/2"
		case "25":
			return "AMD Zen 3/4"
		}
	}
	return family
}

// ─── Memory ──────────────────────────────────────────────────────────────────

func collectMemory(_ context.Context) (*domain.MemoryInfo, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("mem.VirtualMemory: %w", err)
	}
	info := &domain.MemoryInfo{
		TotalBytes: vm.Total, UsedBytes: vm.Used,
		FreeBytes: vm.Free, UsagePercent: vm.UsedPercent,
	}
	if out, err := wmiQuery("Win32_PhysicalMemory", []string{"BankLabel", "Capacity", "Speed", "Manufacturer", "PartNumber"}); err == nil {
		for _, line := range strings.Split(out, "\n")[1:] {
			f := parseCSV(line)
			if len(f) < 5 {
				continue
			}
			cap, _ := strconv.ParseUint(strings.TrimSpace(f[1]), 10, 64)
			if cap == 0 {
				continue
			}
			sp, _ := strconv.ParseUint(strings.TrimSpace(f[2]), 10, 64)
			info.Sticks = append(info.Sticks, domain.RAMStick{
				BankLabel: strings.TrimSpace(f[0]), SizeBytes: cap, SpeedMHz: uint32(sp),
				Manufacturer: strings.TrimSpace(f[3]), PartNumber: strings.TrimSpace(f[4]),
			})
		}
	}
	return info, nil
}

// ─── Disk ────────────────────────────────────────────────────────────────────

func collectDisk(_ context.Context) ([]domain.DiskInfo, error) {
	// Use Get-PhysicalDisk for BusType + Win32_DiskDrive for serial
	script := `
$disks = Get-PhysicalDisk | Select-Object FriendlyName, SerialNumber, MediaType, Size, BusType, HealthStatus
$disks | ForEach-Object {
    $mt = "Unspecified"
    if ($_.MediaType -eq 'SSD' -or $_.MediaType -eq 'Unspecified' -or $_.MediaType -eq '') {
        if ($_.BusType -eq 'NVMe' -or $_.BusType -eq 'SD' -or $_.BusType -eq 'SATA') { $mt = "SSD" }
        elseif ($_.MediaType -eq 'SCM') { $mt = "SSD" }
        elseif ($_.MediaType -eq 'HDD') { $mt = "HDD" }
        elseif ($_.FriendlyName -match 'SSD|NVMe|Solid State|SAMSUNG|WDC.*SA|CRUCIAL|KINGSTON|SANDISK|970|980|990|CL6|Fanxiang|Netac') { $mt = "SSD" }
        elseif ($_.BusType -eq 'RAID') { $mt = "SSD" }
        else { $mt = "Unspecified" }
    } elseif ($_.MediaType -eq 'HDD') { $mt = "HDD" }
    elseif ($_.MediaType -eq 'SCM') { $mt = "SSD" }
    $sn = $_.SerialNumber
    if ($sn -match '_') { $sn = $sn -replace '_','' }
    "$($_.FriendlyName)|$sn|$mt|$($_.Size)|$($_.BusType)|$($_.HealthStatus)"
}
`
	out, err := ps(script)
	if err != nil {
		return nil, fmt.Errorf("disk: %w", err)
	}
	var disks []domain.DiskInfo
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(strings.TrimSpace(line), "|")
		if len(f) < 4 {
			continue
		}
		sz, _ := strconv.ParseUint(strings.TrimSpace(f[3]), 10, 64)
		if sz == 0 {
			continue
		}
		di := domain.DiskInfo{
			Model: strings.TrimSpace(f[0]), SerialNumber: strings.TrimSpace(f[1]),
			MediaType: strings.TrimSpace(f[2]), SizeBytes: sz,
		}
		parts, _ := getPartitions()
		di.Partitions = parts
		disks = append(disks, di)
	}
	return disks, nil
}

func getPartitions() ([]domain.Partition, error) {
	ps, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}
	var r []domain.Partition
	for _, p := range ps {
		u, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}
		r = append(r, domain.Partition{
			Letter: p.Device, FileSystem: p.Fstype,
			TotalBytes: u.Total, FreeBytes: u.Free, UsagePercent: u.UsedPercent,
		})
	}
	return r, nil
}

// ─── Network ─────────────────────────────────────────────────────────────────

func collectNetwork(_ context.Context) ([]domain.NetAdapter, error) {
	script := `
Get-NetAdapter | ForEach-Object {
    $a = $_
    $ips=@();$ip6=@();$dns=@();$gw=@()
    try {
        $c = Get-NetIPConfiguration -InterfaceIndex $a.InterfaceIndex -ErrorAction SilentlyContinue
        if ($c) {
            $c.IPv4Address | ForEach-Object { $ips += $_.IPAddress }
            $c.IPv6Address | Where-Object { -not $_.IsIPv6LinkLocal } | ForEach-Object { $ip6 += $_.IPAddress }
            $c.DNSServer | ForEach-Object { $dns += $_.ServerAddresses }
            $c.IPv4DefaultGateway | ForEach-Object { $gw += $_.NextHop }
        }
    } catch {}
    $dhcp = $false
    try { $dhcp = (Get-NetIPInterface -InterfaceIndex $a.InterfaceIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue).Dhcp -eq 'Enabled' } catch {}
    $t = 'Ethernet'
    if ($a.InterfaceDescription -match 'Wi-Fi|Wireless|WLAN') { $t = 'Wi-Fi' }
    elseif ($a.InterfaceDescription -match 'VPN|Virtual|Tunnel') { $t = 'VPN' }
    [PSCustomObject]@{
        N=$a.Name; D=$a.InterfaceDescription; M=$a.MacAddress; Sp="$($a.LinkSpeed)"
        St=$a.Status; V4=($ips -join ';'); V6=($ip6 -join ';')
        DNS=($dns -join ';'); GW=($gw -join ';'); DHCP="$dhcp"; T=$t
    }
} | ConvertTo-Csv -NoTypeInformation`
	out, err := ps(script)
	if err != nil {
		return nil, fmt.Errorf("network: %w", err)
	}
	var adapters []domain.NetAdapter
	for _, line := range strings.Split(out, "\n")[1:] {
		f := parseCSV(line)
		if len(f) < 11 {
			continue
		}
		a := domain.NetAdapter{
			Name: strings.TrimSpace(f[0]), Description: strings.TrimSpace(f[1]),
			MAC: strings.TrimSpace(f[2]), IsUp: strings.EqualFold(strings.TrimSpace(f[4]), "up"),
			Type: strings.TrimSpace(f[10]),
		}
		if v := strings.TrimSpace(f[5]); v != "" {
			a.IPv4 = strings.Split(v, ";")
		}
		if v := strings.TrimSpace(f[6]); v != "" {
			a.IPv6 = strings.Split(v, ";")
		}
		if v := strings.TrimSpace(f[7]); v != "" {
			a.DNS = strings.Split(v, ";")
		}
		if v := strings.TrimSpace(f[8]); v != "" {
			a.Gateway = strings.Split(v, ";")
		}
		a.DHCP = strings.TrimSpace(f[9]) == "True"
		a.SpeedMbps = parseSpeed(strings.TrimSpace(f[3]))
		adapters = append(adapters, a)
	}
	return adapters, nil
}

func parseSpeed(s string) uint64 {
	s = strings.ToLower(strings.ReplaceAll(s, " ", ""))
	v := 0.0
	fmt.Sscanf(s, "%f", &v)
	if strings.Contains(s, "gbps") {
		return uint64(v * 1000)
	}
	if strings.Contains(s, "mbps") {
		return uint64(v)
	}
	return 0
}

// ─── BIOS ────────────────────────────────────────────────────────────────────

func collectBIOS(_ context.Context) (*domain.BIOSInfo, error) {
	out, err := wmiQuery("Win32_BIOS", []string{"Manufacturer", "SMBIOSBIOSVersion", "ReleaseDate"})
	if err != nil {
		return nil, fmt.Errorf("Win32_BIOS: %w", err)
	}
	for _, line := range strings.Split(out, "\n")[1:] {
		f := parseCSV(line)
		if len(f) < 3 {
			continue
		}
		return &domain.BIOSInfo{
			Manufacturer: strings.TrimSpace(f[0]),
			Version:      strings.TrimSpace(f[1]),
			ReleaseDate:  parseWMIDate(strings.TrimSpace(f[2])),
		}, nil
	}
	return nil, fmt.Errorf("BIOS info not found")
}

// ─── Motherboard ─────────────────────────────────────────────────────────────

func collectMotherboard(_ context.Context) (*domain.MoboInfo, error) {
	out, err := wmiQuery("Win32_BaseBoard", []string{"Manufacturer", "Product", "SerialNumber"})
	if err != nil {
		return nil, fmt.Errorf("Win32_BaseBoard: %w", err)
	}
	for _, line := range strings.Split(out, "\n")[1:] {
		f := parseCSV(line)
		if len(f) < 3 {
			continue
		}
		return &domain.MoboInfo{
			Manufacturer: strings.TrimSpace(f[0]), Model: strings.TrimSpace(f[1]),
			SerialNumber: strings.TrimSpace(f[2]),
		}, nil
	}
	return nil, fmt.Errorf("motherboard info not found")
}

// ─── GPU ─────────────────────────────────────────────────────────────────────

func collectGPU(_ context.Context) ([]domain.GPUInfo, error) {
	out, err := wmiQuery("Win32_VideoController", []string{"Name", "AdapterRAM", "DriverVersion", "AdapterCompatibility"})
	if err != nil {
		return nil, fmt.Errorf("Win32_VideoController: %w", err)
	}
	var gpus []domain.GPUInfo
	for _, line := range strings.Split(out, "\n")[1:] {
		f := parseCSV(line)
		if len(f) < 4 {
			continue
		}
		vram, _ := strconv.ParseUint(strings.TrimSpace(f[1]), 10, 64)
		gpus = append(gpus, domain.GPUInfo{
			Name: strings.TrimSpace(f[0]), Vendor: normVendor(strings.TrimSpace(f[3])),
			VRAMBytes: vram, DriverVersion: strings.TrimSpace(f[2]),
		})
	}
	if len(gpus) == 0 {
		return nil, fmt.Errorf("no GPU found")
	}
	return gpus, nil
}

// ─── Monitor ─────────────────────────────────────────────────────────────────

func collectMonitor(_ context.Context) ([]domain.MonitorInfo, error) {
	script := `
# Get all monitors from WmiMonitorID
$mons = Get-CimInstance -Namespace root\wmi -ClassName WmiMonitorID -ErrorAction SilentlyContinue
$dp = Get-CimInstance -Namespace root\wmi -ClassName WmiMonitorBasicDisplayParams -ErrorAction SilentlyContinue
$vc = Get-CimInstance Win32_VideoController -ErrorAction SilentlyContinue

Add-Type -AssemblyName System.Windows.Forms
$screens = [System.Windows.Forms.Screen]::AllScreens

# Get system DPI for scale
$dpi = 96
try {
    $tmpFile = "$env:TEMP\sysscope_dpi.cs"
    @"
using System;
using System.Runtime.InteropServices;
public class SysScopeDPI {
    [DllImport("user32.dll")] public static extern int GetDpiForSystem();
}
"@ | Out-File $tmpFile -Encoding UTF8
    Add-Type -Path $tmpFile -ErrorAction SilentlyContinue
    $dpi = [SysScopeDPI]::GetDpiForSystem()
    Remove-Item $tmpFile -Force -ErrorAction SilentlyContinue
} catch {}
$scale = [math]::Round($dpi / 96 * 100)

# Get refresh rate from VideoController
$refresh = 60
if ($vc) {
    $primaryVC = $vc | Where-Object { $_.CurrentHorizontalResolution -gt 0 } | Select-Object -First 1
    if ($primaryVC -and $primaryVC.CurrentRefreshRate -gt 0) { $refresh = $primaryVC.CurrentRefreshRate }
}

# HDR detection
$hdr = $false
try {
    $modes = Get-CimInstance -Namespace root\wmi -ClassName WmiMonitorListedSupportedSourceModes -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($modes) { $hdr = $true }
} catch {}

if ($mons -and $mons.Count -gt 0) {
    for ($i = 0; $i -lt $mons.Count; $i++) {
        $m = $mons[$i]
        # Decode manufacturer
        $mfg = ""
        if ($m.ManufacturerName) {
            $b = $m.ManufacturerName | Where-Object { $_ -ne 0 }
            if ($b) { $mfg = -join ($b | ForEach-Object { [char]$_ }) }
        }
        # Decode user friendly name
        $name = ""
        if ($m.UserFriendlyName) {
            $b = $m.UserFriendlyName | Where-Object { $_ -ne 0 }
            if ($b) { $name = -join ($b | ForEach-Object { [char]$_ }) }
        }
        if (-not $name) { $name = "Monitor $($i+1)" }
        # Serial
        $serial = ""
        if ($m.SerialNumberID) {
            $b = $m.SerialNumberID | Where-Object { $_ -ne 0 }
            if ($b) {
                $serial = -join ($b | ForEach-Object { [char]$_ })
                if ($serial -match '^\d+$') { $serial = "" }
            }
        }
        # Diagonal from display params
        $diag = 0
        if ($i -lt $dp.Count) {
            $h = $dp[$i].MaxHorizontalImageSize
            $v = $dp[$i].MaxVerticalImageSize
            if ($h -gt 0 -and $v -gt 0) {
                $diag = [math]::Round([math]::Sqrt($h*$h + $v*$v) / 2.54, 1)
            }
        }
        # Resolution from screens
        $rx = 0; $ry = 0
        if ($i -lt $screens.Count) {
            $rx = $screens[$i].Bounds.Width
            $ry = $screens[$i].Bounds.Height
        }
        Write-Output "$name|$mfg|$serial|$rx|$ry|$refresh|$diag|$hdr|$scale"
    }
} else {
    # Fallback to .NET screens
    for ($i = 0; $i -lt $screens.Count; $i++) {
        $s = $screens[$i]
        Write-Output "Display $($i+1)|||$($s.Bounds.Width)|$($s.Bounds.Height)|$refresh|0|$hdr|$scale"
    }
}
`
	out, err := ps(script)
	if err != nil {
		return nil, fmt.Errorf("monitor: %w", err)
	}
	var monitors []domain.MonitorInfo
	for _, line := range strings.Split(out, "\n") {
		p := strings.Split(strings.TrimSpace(line), "|")
		if len(p) < 6 {
			continue
		}
		rx, _ := strconv.Atoi(p[3])
		ry, _ := strconv.Atoi(p[4])
		rr, _ := strconv.Atoi(p[5])
		diag, _ := strconv.ParseFloat(p[6], 64)
		hdr := strings.EqualFold(p[7], "true")
		sc, _ := strconv.Atoi(p[8])
		if rx == 0 && ry == 0 {
			continue
		}
		monitors = append(monitors, domain.MonitorInfo{
			Name: p[0], Manufacturer: p[1], SerialNumber: p[2],
			ResolutionX: rx, ResolutionY: ry, RefreshRateHz: rr,
			DiagonalInch: diag, HDR: hdr, ScalePercent: sc,
		})
	}
	if len(monitors) == 0 {
		return nil, fmt.Errorf("no monitors detected")
	}
	return monitors, nil
}

// ─── Battery ─────────────────────────────────────────────────────────────────

func collectBattery(_ context.Context) (*domain.BatteryInfo, error) {
	// Step 1: Basic battery info
	out, err := ps(`
$b = Get-CimInstance Win32_Battery -ErrorAction SilentlyContinue
if (-not $b) { Write-Output "NONE"; return }
Write-Output "OK|$($b.EstimatedChargeRemaining)|$($b.BatteryStatus -eq 2)"
`)
	if err != nil || strings.Contains(out, "NONE") {
		return nil, fmt.Errorf("no battery present on this system")
	}

	info := &domain.BatteryInfo{IsPresent: true}
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "OK|") {
			continue
		}
		p := strings.Split(line, "|")
		if len(p) >= 3 {
			info.ChargePercent, _ = strconv.ParseFloat(strings.TrimSpace(p[1]), 64)
			info.IsCharging = strings.TrimSpace(p[2]) == "True"
		}
	}

	// Step 2: Battery health from powercfg report
	healthOut, _ := ps(`
try {
    $f = "$env:TEMP\sysscope_batt.xml"
    powercfg /batteryreport /output $f 2>$null
    if (Test-Path $f) {
        $xml = [xml](Get-Content $f -Raw -ErrorAction SilentlyContinue)
        if ($xml -and $xml.BatteryReport -and $xml.BatteryReport.Battery) {
            $bat = $xml.BatteryReport.Battery
            $dc = [double]("$($bat.'design-capacity')" -replace '[^\d]','')
            $fc = [double]("$($bat.'full-charge-capacity')" -replace '[^\d]','')
            $cc = 0; [int]::TryParse(("$($bat.'cycle-count')" -replace '[^\d]',''), [ref]$cc)
            Write-Output "H|$dc|$fc|$cc"
        }
    }
} catch {}
`)
	for _, line := range strings.Split(healthOut, "\n") {
		if !strings.HasPrefix(line, "H|") {
			continue
		}
		p := strings.Split(line, "|")
		if len(p) >= 4 {
			dc, _ := strconv.ParseFloat(p[1], 64)
			fc, _ := strconv.ParseFloat(p[2], 64)
			cc, _ := strconv.ParseFloat(p[3], 64)
			if dc > 0 {
				info.DesignCapacityWh = dc / 1000
			}
			if fc > 0 {
				info.FullChargeCapWh = fc / 1000
			}
			if dc > 0 && fc > 0 {
				info.HealthPercent = math.Round((fc/dc)*1000) / 10
				info.WearLevelPercent = math.Round((100-info.HealthPercent)*10) / 10
			}
			info.CycleCount = int(cc)
		}
	}

	// Step 3: Remaining time
	timeOut, _ := ps(`
$b = Get-CimInstance Win32_Battery -ErrorAction SilentlyContinue
if ($b) {
    $rt = $b.EstimatedRunTime
    if ($rt -gt 0 -and $rt -ne 71582788) { Write-Output "$rt min" }
    elseif ($b.BatteryStatus -eq 2) { Write-Output "Charging" }
    else { Write-Output "On battery" }
}
`)
	info.RemainingTime = strings.TrimSpace(timeOut)

	return info, nil
}

// ─── Temperatures ────────────────────────────────────────────────────────────

func collectTemperatures(_ context.Context) (*domain.TempInfo, error) {
	// Try multiple independent methods — each failure is caught separately
	info := &domain.TempInfo{}
	found := false

	// 1) CPU via MSAcpi_ThermalZoneTemperature (requires admin but we catch errors)
	if out, err := ps(`
try {
    $tz = Get-CimInstance -Namespace root/wmi -ClassName MSAcpi_ThermalZoneTemperature -ErrorAction Stop
    if ($tz -and $tz.Count -gt 0) {
        $t = ($tz[0].CurrentTemperature - 2732) / 10.0
        if ($t -gt 20 -and $t -lt 120) { Write-Output $t }
    }
} catch {}
`); err == nil {
		if v, e := strconv.ParseFloat(strings.TrimSpace(out), 64); e == nil && v > 20 && v < 120 {
			info.CPU = v
			found = true
		}
	}

	// 2) GPU via nvidia-smi
	if out, err := ps(`nvidia-smi --query-gpu=temperature.gpu --format=csv,noheader 2>$null`); err == nil {
		if v, e := strconv.ParseFloat(strings.TrimSpace(out), 64); e == nil && v > 0 && v < 120 {
			info.GPU = v
			found = true
		}
	}

	// 3) SSD via Storage Reliability Counter
	if out, err := ps(`
try {
    $d = Get-PhysicalDisk -ErrorAction Stop | Get-StorageReliabilityCounter -ErrorAction Stop | Select-Object -First 1
    if ($d -and $d.Temperature -gt 0) { Write-Output $d.Temperature }
} catch {}
`); err == nil {
		if v, e := strconv.ParseFloat(strings.TrimSpace(out), 64); e == nil && v > 0 && v < 100 {
			info.SSD = v
			found = true
		}
	}

	// 4) Fallback: Intel DTT thermal zone via PerfMon counters
	if info.CPU == 0 {
		if out, err := ps(`
try {
    $t = Get-CimInstance -Namespace root/cimv2 -ClassName Win32_TemperatureProbe -ErrorAction Stop | Select-Object -First 1
    if ($t -and $t.CurrentReading -gt 0) { Write-Output ($t.CurrentReading / 10.0 - 273.15) }
} catch {}
`); err == nil {
			if v, e := strconv.ParseFloat(strings.TrimSpace(out), 64); e == nil && v > 20 && v < 120 {
				info.CPU = v
				found = true
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("no temperature data available (thermal sensors require admin rights — try running as Administrator)")
	}
	return info, nil
}

// ─── SMART ───────────────────────────────────────────────────────────────────

func collectSMART(_ context.Context) ([]domain.SMARTInfo, error) {
	script := `
Get-PhysicalDisk | ForEach-Object {
    $d = $_
    $temp=0;$h=0;$c=0;$life=-1.0
    try {
        $r = $d | Get-StorageReliabilityCounter -EA SilentlyContinue
        if ($r) {
            if ($r.Temperature -gt 0) { $temp = $r.Temperature }
            $h = $r.PowerOnHours
            $c = $r.StartStopCycleCount
            if ($null -ne $r.Wear) { $life = 100 - [double]$r.Wear; if ($life -lt 0) { $life = 0 } }
        }
    } catch {}
    Write-Output "$($d.FriendlyName)|$($d.SerialNumber)|$($d.HealthStatus)|$temp|$h|$c|$life|$($d.BusType)"
}
`
	out, err := ps(script)
	if err != nil {
		return nil, fmt.Errorf("SMART: %w", err)
	}
	var results []domain.SMARTInfo
	for _, line := range strings.Split(out, "\n") {
		p := strings.SplitN(strings.TrimSpace(line), "|", 8)
		if len(p) < 7 || p[0] == "" {
			continue
		}
		t, _ := strconv.ParseFloat(p[3], 64)
		h, _ := strconv.ParseInt(p[4], 10, 64)
		c, _ := strconv.ParseInt(p[5], 10, 64)
		l, _ := strconv.ParseFloat(p[6], 64)
		if l < 0 {
			l = 0
		}
		results = append(results, domain.SMARTInfo{
			Model: p[0], DeviceID: p[1], Health: p[2],
			TemperatureC: t, PowerOnHours: h, PowerCycles: c, RemainingLife: l,
		})
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no SMART data (requires Windows 10+)")
	}
	return results, nil
}

// ─── Processes ───────────────────────────────────────────────────────────────

func collectProcesses(_ context.Context) ([]domain.ProcessInfo, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("process: %w", err)
	}
	var r []domain.ProcessInfo
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
		r = append(r, domain.ProcessInfo{PID: p.Pid, Name: name, CPUPercent: cpuPct, MemPercent: float64(memPct), Status: s})
	}
	return r, nil
}

// ─── Security ────────────────────────────────────────────────────────────────

func collectSecurity(_ context.Context) (*domain.SecurityInfo, error) {
	info := &domain.SecurityInfo{}

	// Defender
	if out, err := ps("Get-MpComputerStatus -EA SilentlyContinue | Select-Object RealTimeProtectionEnabled,AntivirusEnabled,ControlledFolderAccessEnabled | ConvertTo-Csv -NoTypeInformation"); err == nil {
		for _, l := range strings.Split(out, "\n")[1:] {
			f := parseCSV(l)
			if len(f) >= 3 {
				info.DefenderRealtime = strings.TrimSpace(f[0]) == "True"
				info.DefenderEnabled = strings.TrimSpace(f[1]) == "True"
				info.ControlledFolderAccess = strings.TrimSpace(f[2]) == "True"
			}
			break
		}
	}

	// Firewall
	if out, err := ps("Get-NetFirewallProfile -EA SilentlyContinue | Select-Object Enabled | ConvertTo-Csv -NoTypeInformation"); err == nil {
		info.FirewallEnabled = strings.Contains(out, "True")
	}

	// Secure Boot
	if out, err := ps("Confirm-SecureBootUEFI"); err == nil {
		info.SecureBootEnabled = strings.TrimSpace(out) == "True"
	}

	// TPM — try Get-Tpm, then WMI, then PnP
	tpmFound := false
	if out, err := ps("Get-Tpm -EA SilentlyContinue | Select-Object TpmPresent,SpecVersion | ConvertTo-Csv -NoTypeInformation"); err == nil {
		for _, l := range strings.Split(out, "\n")[1:] {
			f := parseCSV(l)
			if len(f) >= 2 && strings.TrimSpace(f[0]) == "True" {
				info.TPMPresent = true
				info.TPMVersion = strings.TrimSpace(f[1])
				tpmFound = true
			}
			break
		}
	}
	if !tpmFound {
		if out, err := ps("Get-CimInstance -Namespace root/cimv2/Security/MicrosoftTpm -ClassName Win32_Tpm -EA SilentlyContinue | Select-Object IsActivated_InitialValue"); err == nil {
			if strings.Contains(out, "True") {
				info.TPMPresent = true
				info.TPMVersion = "2.0"
				tpmFound = true
			}
		}
	}
	if !tpmFound {
		if out, err := ps("Get-PnpDevice -EA SilentlyContinue | Where-Object { $_.FriendlyName -match 'TPM' -and $_.Status -eq 'OK' } | Measure-Object | Select-Object -ExpandProperty Count"); err == nil {
			cnt, _ := strconv.Atoi(strings.TrimSpace(out))
			if cnt > 0 {
				info.TPMPresent = true
				info.TPMVersion = "detected"
			}
		}
	}

	// BitLocker
	if out, err := ps("Get-BitLockerVolume -EA SilentlyContinue | Select-Object ProtectionStatus | ConvertTo-Csv -NoTypeInformation"); err == nil {
		info.BitLockerEnabled = strings.Contains(out, "On")
	}

	// Windows Update
	if out, err := ps("(Get-HotFix -EA SilentlyContinue | Sort-Object InstalledOn -Desc | Select-Object -First 1).InstalledOn.ToString('yyyy-MM-dd')"); err == nil {
		d := strings.TrimSpace(out)
		if d != "" {
			info.LastUpdateDate = d
			if t, e := time.Parse("2006-01-02", d); e == nil {
				info.OSUpdateCurrent = time.Since(t) < 30*24*time.Hour
			}
		}
	}

	// Credential Guard
	if out, err := ps("try { (Get-CimInstance Win32_DeviceGuard -Namespace root\\Microsoft\\Windows\\DeviceGuard -EA Stop).SecurityServicesRunning -contains 1 } catch { 'False' }"); err == nil {
		info.CredentialGuard = strings.TrimSpace(out) == "True"
	}

	// Memory Integrity / Core Isolation
	if out, err := ps("(Get-ItemProperty 'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\DeviceGuard\\Scenarios\\HypervisorEnforcedCodeIntegrity' -Name Enabled -EA SilentlyContinue).Enabled"); err == nil {
		info.MemoryIntegrity = strings.TrimSpace(out) == "1"
		info.CoreIsolation = info.MemoryIntegrity
	}

	// SmartScreen — multiple locations
	if out, err := ps(`
$v = Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer' -Name SmartScreenEnabled -EA SilentlyContinue
if ($v.SmartScreenEnabled) { Write-Output $v.SmartScreenEnabled }
else {
    $v2 = Get-ItemProperty 'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppHost' -Name EnableWebContentEvaluation -EA SilentlyContinue
    if ($null -ne $v2.EnableWebContentEvaluation) { if ($v2.EnableWebContentEvaluation -eq 1) { Write-Output "On" } else { Write-Output "Off" } }
    else { Write-Output "On" }
}`); err == nil {
		v := strings.TrimSpace(out)
		info.SmartScreen = v == "On" || v == "RequireAdmin"
	}

	// Threats
	if out, err := ps("Get-MpThreatDetection -EA SilentlyContinue | Select-Object -First 5 ThreatID | ConvertTo-Csv -NoTypeInformation"); err == nil {
		for _, l := range strings.Split(out, "\n")[1:] {
			t := strings.TrimSpace(l)
			if t != "" {
				info.ThreatsFound = append(info.ThreatsFound, t)
			}
		}
	}

	return info, nil
}

// ─── Services ────────────────────────────────────────────────────────────────

func collectServices(_ context.Context) ([]domain.ServiceInfo, error) {
	out, err := ps(`
Get-Service | ForEach-Object {
    $p=""
    try { $p=(Get-CimInstance Win32_Service -Filter "Name='$($_.Name)'" -EA SilentlyContinue).PathName } catch {}
    "$($_.Name)|$($_.DisplayName)|$($_.Status)|$($_.StartType)|$p"
}`)
	if err != nil {
		return nil, fmt.Errorf("services: %w", err)
	}
	var r []domain.ServiceInfo
	for _, l := range strings.Split(out, "\n") {
		p := strings.SplitN(strings.TrimSpace(l), "|", 5)
		if len(p) < 4 || p[0] == "" {
			continue
		}
		s := domain.ServiceInfo{Name: p[0], DisplayName: p[1], Status: p[2], StartType: p[3]}
		if len(p) > 4 {
			s.ExePath = p[4]
		}
		r = append(r, s)
	}
	if len(r) == 0 {
		return nil, fmt.Errorf("no services")
	}
	return r, nil
}

// ─── Drivers ─────────────────────────────────────────────────────────────────

func collectDrivers(_ context.Context) ([]domain.DriverInfo, error) {
	out, err := ps(`
Get-CimInstance Win32_PnPSignedDriver | Where-Object { $_.DeviceName -and $_.DriverVersion } | Select-Object -First 200 DeviceName,DriverVersion,DriverProviderName,DriverDate,IsSigned | ForEach-Object {
    $d=""
    if ($_.DriverDate) { try { $d=$_.DriverDate.ToString('yyyy-MM-dd') } catch {} }
    $st = "OK"
    if ($_.IsSigned -eq $false) { $st = "Unsigned" }
    "$($_.DeviceName)|$($_.DriverVersion)|$($_.DriverProviderName)|$d|$st"
}`)
	if err != nil {
		return nil, fmt.Errorf("drivers: %w", err)
	}
	var r []domain.DriverInfo
	for _, l := range strings.Split(out, "\n") {
		p := strings.SplitN(strings.TrimSpace(l), "|", 5)
		if len(p) < 4 || p[0] == "" {
			continue
		}
		st := "OK"
		if len(p) > 4 && p[4] != "" {
			st = p[4]
		}
		r = append(r, domain.DriverInfo{Name: p[0], Version: p[1], Provider: p[2], Date: p[3], Status: st})
	}
	if len(r) == 0 {
		return nil, fmt.Errorf("no drivers")
	}
	return r, nil
}

// ─── Startup ─────────────────────────────────────────────────────────────────

func collectStartup(_ context.Context) ([]domain.StartupItem, error) {
	out, err := ps(`
$items = @()
@('HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run','HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run','HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce','HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce') | ForEach-Object {
    if (Test-Path $_) {
        (Get-ItemProperty $_ -EA SilentlyContinue).PSObject.Properties | Where-Object { $_.Name -notlike 'PS*' } | ForEach-Object {
            $items += "REG|$($_.Name)|$($_.Value)|True"
        }
    }
}
$sp = [IO.Path]::Combine($env:APPDATA,'Microsoft\Windows\Start Menu\Programs\Startup')
if (Test-Path $sp) { Get-ChildItem $sp | ForEach-Object { $items += "STARTUP|$($_.Name)|$($_.FullName)|True" } }
try { Get-ScheduledTask -EA SilentlyContinue | Where-Object { $_.State -eq 'Ready' -and $_.Triggers } | Select-Object -First 30 TaskName | ForEach-Object { $items += "TASK|$($_.TaskName)||True" } } catch {}
$items`)
	if err != nil {
		return nil, fmt.Errorf("startup: %w", err)
	}
	var r []domain.StartupItem
	for _, l := range strings.Split(out, "\n") {
		p := strings.SplitN(strings.TrimSpace(l), "|", 4)
		if len(p) < 4 {
			continue
		}
		src := ""
		switch p[0] {
		case "REG":
			src = "Registry"
		case "STARTUP":
			src = "StartupFolder"
		case "TASK":
			src = "TaskScheduler"
		}
		r = append(r, domain.StartupItem{Name: p[1], Command: p[2], Source: src, Enabled: p[3] == "True"})
	}
	if len(r) == 0 {
		return nil, fmt.Errorf("no startup items")
	}
	return r, nil
}

// ─── Programs ────────────────────────────────────────────────────────────────

func collectPrograms(_ context.Context) ([]domain.ProgramInfo, error) {
	out, err := ps(`
@('HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*','HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*','HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*') | ForEach-Object {
    Get-ItemProperty $_ -EA SilentlyContinue
} | Where-Object { $_.DisplayName -and $_.DisplayName -notmatch '^\$\{' } | ForEach-Object {
    "$($_.DisplayName)|$($_.DisplayVersion)|$($_.Publisher)|$($_.InstallDate)|$($_.EstimatedSize)"
}`)
	if err != nil {
		return nil, fmt.Errorf("programs: %w", err)
	}
	var r []domain.ProgramInfo
	for _, l := range strings.Split(out, "\n") {
		p := strings.SplitN(strings.TrimSpace(l), "|", 5)
		if len(p) < 1 || p[0] == "" {
			continue
		}
		pr := domain.ProgramInfo{Name: p[0]}
		if len(p) > 1 {
			pr.Version = p[1]
		}
		if len(p) > 2 {
			pr.Publisher = p[2]
		}
		if len(p) > 3 {
			pr.InstallDate = p[3]
		}
		if len(p) > 4 {
			s, _ := strconv.Atoi(p[4])
			pr.EstimatedMB = s / 1024
		}
		r = append(r, pr)
	}
	if len(r) == 0 {
		return nil, fmt.Errorf("no programs")
	}
	return r, nil
}

// ─── Environment ─────────────────────────────────────────────────────────────

func collectEnvironment(_ context.Context) (map[string]string, error) {
	out, err := ps("Get-ChildItem Env: | ForEach-Object { \"$($_.Name)|$($_.Value)\" }")
	if err != nil {
		return nil, fmt.Errorf("env: %w", err)
	}
	env := map[string]string{}
	for _, l := range strings.Split(out, "\n") {
		p := strings.SplitN(strings.TrimSpace(l), "|", 2)
		if len(p) == 2 && p[0] != "" {
			env[p[0]] = p[1]
		}
	}
	if len(env) == 0 {
		return nil, fmt.Errorf("no env vars")
	}
	return env, nil
}

// ─── USB ─────────────────────────────────────────────────────────────────────

func collectUSB(_ context.Context) ([]domain.USBDevice, error) {
	out, err := ps(`
$usb = Get-PnpDevice -EA SilentlyContinue | Where-Object { $_.InstanceId -match '^USB\\' -and $_.Status -eq 'OK' }
if (-not $usb) { $usb = Get-PnpDevice -Class USB -EA SilentlyContinue }
$usb | ForEach-Object {
    $vid="";$pid=""
    if ($_.InstanceId -match 'VID_([0-9A-F]+)&PID_([0-9A-F]+)') { $vid=$Matches[1];$pid=$Matches[2] }
    $n=$_.FriendlyName; if(-not $n){$n=$_.InstanceId}
    "$n|$vid|$pid|$($_.Status)"
}`)
	if err != nil {
		return nil, fmt.Errorf("USB: %w", err)
	}
	var r []domain.USBDevice
	for _, l := range strings.Split(out, "\n") {
		p := strings.SplitN(strings.TrimSpace(l), "|", 4)
		if len(p) < 4 || p[0] == "" {
			continue
		}
		r = append(r, domain.USBDevice{Name: p[0], VendorID: p[1], ProductID: p[2], Status: p[3]})
	}
	if len(r) == 0 {
		return nil, fmt.Errorf("no USB devices")
	}
	return r, nil
}

// ─── Bluetooth ───────────────────────────────────────────────────────────────

func collectBluetooth(_ context.Context) (*domain.BluetoothInfo, error) {
	out, err := ps(`
$a = Get-PnpDevice -Class Bluetooth -EA SilentlyContinue | Select-Object FriendlyName,Status
$d = Get-PnpDevice -EA SilentlyContinue | Where-Object { $_.InstanceId -match '^BTHENUM' } | Select-Object FriendlyName,Status
"ADAPTERS:"
$a | ForEach-Object { "$($_.FriendlyName)|$($_.Status)|" }
"DEVICES:"
$d | ForEach-Object { "$($_.FriendlyName)|$($_.Status)|" }
`)
	if err != nil {
		return nil, fmt.Errorf("Bluetooth: %w", err)
	}
	info := &domain.BluetoothInfo{}
	sec := ""
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l == "ADAPTERS:" {
			sec = "a"
			continue
		}
		if l == "DEVICES:" {
			sec = "d"
			continue
		}
		if l == "" {
			continue
		}
		p := strings.SplitN(l, "|", 3)
		if len(p) < 2 {
			continue
		}
		if sec == "a" {
			info.Adapters = append(info.Adapters, domain.BluetoothAdapter{Name: p[0], Status: p[1]})
		} else if sec == "d" {
			info.Devices = append(info.Devices, domain.BluetoothDevice{Name: p[0], Connected: strings.EqualFold(p[1], "OK")})
		}
	}
	if len(info.Adapters) == 0 && len(info.Devices) == 0 {
		return nil, fmt.Errorf("no Bluetooth found")
	}
	return info, nil
}

// ─── PCI ─────────────────────────────────────────────────────────────────────

func collectPCI(_ context.Context) ([]domain.PCIDevice, error) {
	out, err := ps(`Get-PnpDevice | Where-Object { $_.InstanceId -match '^PCI' } | Select-Object FriendlyName,Status,InstanceId,Class | ForEach-Object { "$($_.FriendlyName)|$($_.Status)|$($_.InstanceId)|$($_.Class)" }`)
	if err != nil {
		return nil, fmt.Errorf("PCI: %w", err)
	}
	var r []domain.PCIDevice
	for _, l := range strings.Split(out, "\n") {
		p := strings.SplitN(strings.TrimSpace(l), "|", 4)
		if len(p) < 3 || p[0] == "" {
			continue
		}
		vid, pid := "", ""
		if len(p) > 2 {
			vid = matchHexID(p[2], "VEN_")
			pid = matchHexID(p[2], "DEV_")
		}
		cl := ""
		if len(p) > 3 {
			cl = p[3]
		}
		r = append(r, domain.PCIDevice{Name: p[0], VendorID: vid, DeviceID: pid, Status: p[1], Description: cl})
	}
	if len(r) == 0 {
		return nil, fmt.Errorf("no PCI devices")
	}
	return r, nil
}

func matchHexID(s, prefix string) string {
	i := strings.Index(s, prefix)
	if i < 0 {
		return ""
	}
	r := s[i+len(prefix):]
	if e := strings.IndexAny(r, "&\\"); e > 0 {
		return r[:e]
	}
	if len(r) >= 4 {
		return r[:4]
	}
	return r
}

// ─── Windows Features ────────────────────────────────────────────────────────

func collectWinFeatures(_ context.Context) (*domain.WinFeatures, error) {
	info := &domain.WinFeatures{}

	out, err := ps(`
@('Hyper-V','Microsoft-Windows-Subsystem-Linux','Containers-DisposableClientVM','VirtualMachinePlatform','NetFx4-AdvSrvs') | ForEach-Object {
    try { $s=(Get-WindowsOptionalFeature -Online -FeatureName $_ -EA Stop).State; "$_|$s" }
    catch {
        # Fallback: check registry / files
        $det = "NotAvailable"
        switch ($_) {
            'Microsoft-Windows-Subsystem-Linux' { if (Test-Path "$env:SystemRoot\System32\lxss") { $det = "Enabled" } else { $det = "Disabled" } }
            'VirtualMachinePlatform' { if (Test-Path "$env:SystemRoot\System32\vmcompute.exe") { $det = "Enabled" } else { $det = "Disabled" } }
        }
        "$_|$det"
    }
}
"PSVersion|$($PSVersionTable.PSVersion)"
`)
	if err != nil {
		// Complete fallback via file checks
		info.HyperV = checkFeature("vmwp.exe")
		info.WSL = checkFeature("lxss")
		info.Sandbox = checkFeature("SandboxHost")
		info.VirtualMachinePlatform = checkFeature("vmcompute.exe")
		info.NetFramework = checkFeature("v4.0.30319")
		info.PowerShellVersion = psVersion()
		return info, nil
	}
	for _, l := range strings.Split(out, "\n") {
		p := strings.SplitN(strings.TrimSpace(l), "|", 2)
		if len(p) < 2 {
			continue
		}
		switch p[0] {
		case "Hyper-V":
			info.HyperV = p[1]
		case "Microsoft-Windows-Subsystem-Linux":
			info.WSL = p[1]
		case "Containers-DisposableClientVM":
			info.Sandbox = p[1]
		case "VirtualMachinePlatform":
			info.VirtualMachinePlatform = p[1]
		case "NetFx4-AdvSrvs":
			info.NetFramework = p[1]
		case "PSVersion":
			info.PowerShellVersion = p[1]
		}
	}
	return info, nil
}

func checkFeature(file string) string {
	paths := []string{
		`C:\Windows\System32\` + file,
		`C:\Windows\System32\lxss`,
	}
	for _, p := range paths {
		if strings.Contains(p, file) {
			if _, err := os.Stat(p); err == nil {
				return "Enabled"
			}
		}
	}
	return "Disabled"
}

func psVersion() string {
	out, err := ps("$PSVersionTable.PSVersion.ToString()")
	if err == nil {
		return strings.TrimSpace(out)
	}
	return "Unknown"
}
