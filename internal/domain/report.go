// Package domain defines the core business entities and interfaces
// for the SysScope application (Clean Architecture — innermost layer).
package domain

import "time"

// ─── Report ──────────────────────────────────────────────────────────────────

// Report is the aggregate root that holds all collected system data.
type Report struct {
	ID            string         `json:"id"`
	GeneratedAt   time.Time      `json:"generated_at"`
	Hostname      string         `json:"hostname"`
	OS            *OSInfo        `json:"os"`
	CPU           *CPUInfo       `json:"cpu,omitempty"`
	Memory        *MemoryInfo    `json:"memory,omitempty"`
	Disks         []DiskInfo     `json:"disks,omitempty"`
	Network       []NetAdapter   `json:"network,omitempty"`
	BIOS          *BIOSInfo      `json:"bios,omitempty"`
	Motherboard   *MoboInfo      `json:"motherboard,omitempty"`
	GPU           []GPUInfo      `json:"gpu,omitempty"`
	Monitors      []MonitorInfo  `json:"monitors,omitempty"`
	Battery       *BatteryInfo   `json:"battery,omitempty"`
	Temperatures  *TempInfo      `json:"temperatures,omitempty"`
	SMART         []SMARTInfo    `json:"smart,omitempty"`
	Processes     []ProcessInfo  `json:"processes,omitempty"`
	Services      []ServiceInfo  `json:"services,omitempty"`
	Drivers       []DriverInfo   `json:"drivers,omitempty"`
	Startup       []StartupItem  `json:"startup,omitempty"`
	Programs      []ProgramInfo  `json:"installed_programs,omitempty"`
	EnvVars       map[string]string `json:"environment,omitempty"`
	USB           []USBDevice    `json:"usb_devices,omitempty"`
	Bluetooth     *BluetoothInfo `json:"bluetooth,omitempty"`
	PCI           []PCIDevice    `json:"pci_devices,omitempty"`
	WinFeatures   *WinFeatures   `json:"windows_features,omitempty"`
	Security      *SecurityInfo  `json:"security,omitempty"`
	HealthScore   Score          `json:"health_score"`
	SecurityScore Score          `json:"security_score"`
	Recommendations []string     `json:"recommendations,omitempty"`
	Errors        []CollectError `json:"errors,omitempty"`
}

// CollectError records a failure to collect specific data.
type CollectError struct {
	Module  string `json:"module"`
	Message string `json:"message"`
}

// ─── OS ──────────────────────────────────────────────────────────────────────

type OSInfo struct {
	Edition      string `json:"edition"`
	Version      string `json:"version"`
	Build        string `json:"build"`
	ProductName  string `json:"product_name"`
	Architecture string `json:"architecture"`
	InstallDate  string `json:"install_date,omitempty"`
	Uptime       string `json:"uptime"`
	Hostname     string `json:"hostname"`
	Kernel       string `json:"kernel,omitempty"`
}

// ─── CPU ─────────────────────────────────────────────────────────────────────

type CPUInfo struct {
	Vendor        string  `json:"vendor"`
	Model         string  `json:"model"`
	Family        string  `json:"family"`
	PhysicalCores int32   `json:"physical_cores"`
	LogicalCores  int32   `json:"logical_cores"`
	UsagePercent  float64 `json:"usage_percent"`
	CurrentMHz    float64 `json:"current_mhz"`
	MaxMHz        float64 `json:"max_mhz"`
	Architecture  string  `json:"architecture"`
}

// ─── Memory ──────────────────────────────────────────────────────────────────

type MemoryInfo struct {
	TotalBytes   uint64     `json:"total_bytes"`
	UsedBytes    uint64     `json:"used_bytes"`
	FreeBytes    uint64     `json:"free_bytes"`
	UsagePercent float64    `json:"usage_percent"`
	Sticks       []RAMStick `json:"sticks,omitempty"`
}

type RAMStick struct {
	BankLabel    string `json:"bank_label"`
	SizeBytes    uint64 `json:"size_bytes"`
	SpeedMHz     uint32 `json:"speed_mhz"`
	Manufacturer string `json:"manufacturer"`
	PartNumber   string `json:"part_number,omitempty"`
}

// ─── Disk ────────────────────────────────────────────────────────────────────

type DiskInfo struct {
	DeviceID     string      `json:"device_id"`
	Model        string      `json:"model"`
	SerialNumber string      `json:"serial_number,omitempty"`
	MediaType    string      `json:"media_type"` // SSD / HDD / Unspecified
	SizeBytes    uint64      `json:"size_bytes"`
	Partitions   []Partition `json:"partitions,omitempty"`
}

type Partition struct {
	Letter       string  `json:"letter,omitempty"`
	FileSystem   string  `json:"filesystem,omitempty"`
	TotalBytes   uint64  `json:"total_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// ─── Network ─────────────────────────────────────────────────────────────────

type NetAdapter struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	MAC         string   `json:"mac"`
	IPv4        []string `json:"ipv4,omitempty"`
	IPv6        []string `json:"ipv6,omitempty"`
	DNS         []string `json:"dns,omitempty"`
	Gateway     []string `json:"gateway,omitempty"`
	SpeedMbps   uint64   `json:"speed_mbps,omitempty"`
	IsUp        bool     `json:"is_up"`
	Type        string   `json:"type,omitempty"`    // Ethernet / Wi-Fi / VPN
	DHCP        bool     `json:"dhcp,omitempty"`
	Manufacturer string  `json:"manufacturer,omitempty"`
}

// ─── Temperatures ────────────────────────────────────────────────────────────

type TempInfo struct {
	CPU         float64 `json:"cpu_c,omitempty"`
	GPU         float64 `json:"gpu_c,omitempty"`
	SSD         float64 `json:"ssd_c,omitempty"`
	Motherboard float64 `json:"motherboard_c,omitempty"`
}

// ─── SMART ───────────────────────────────────────────────────────────────────

type SMARTInfo struct {
	DeviceID      string  `json:"device_id"`
	Model         string  `json:"model"`
	Health        string  `json:"health"` // OK / Warning / Failed
	TemperatureC  float64 `json:"temperature_c,omitempty"`
	PowerOnHours  int64   `json:"power_on_hours,omitempty"`
	PowerCycles   int64   `json:"power_cycles,omitempty"`
	RemainingLife float64 `json:"remaining_life_percent,omitempty"`
	TotalWritesGB float64 `json:"total_writes_gb,omitempty"`
	TotalReadsGB  float64 `json:"total_reads_gb,omitempty"`
}

// ─── BIOS ────────────────────────────────────────────────────────────────────

type BIOSInfo struct {
	Manufacturer string `json:"manufacturer"`
	Version      string `json:"version"`
	ReleaseDate  string `json:"release_date"`
}

// ─── Motherboard ─────────────────────────────────────────────────────────────

type MoboInfo struct {
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	SerialNumber string `json:"serial_number,omitempty"`
}

// ─── GPU ─────────────────────────────────────────────────────────────────────

type GPUInfo struct {
	Name          string `json:"name"`
	Vendor        string `json:"vendor"`
	VRAMBytes     uint64 `json:"vram_bytes,omitempty"`
	DriverVersion string `json:"driver_version"`
}

// ─── Monitor ─────────────────────────────────────────────────────────────────

type MonitorInfo struct {
	Name          string  `json:"name"`
	Manufacturer  string  `json:"manufacturer,omitempty"`
	SerialNumber  string  `json:"serial_number,omitempty"`
	ResolutionX   int     `json:"resolution_x"`
	ResolutionY   int     `json:"resolution_y"`
	RefreshRateHz int     `json:"refresh_rate_hz"`
	DiagonalInch  float64 `json:"diagonal_inch,omitempty"`
	HDR           bool    `json:"hdr,omitempty"`
	ScalePercent  int     `json:"scale_percent,omitempty"`
}

// ─── Battery ─────────────────────────────────────────────────────────────────

type BatteryInfo struct {
	IsPresent         bool    `json:"is_present"`
	ChargePercent     float64 `json:"charge_percent"`
	IsCharging        bool    `json:"is_charging"`
	HealthPercent     float64 `json:"health_percent,omitempty"`
	DesignCapacityWh  float64 `json:"design_capacity_wh,omitempty"`
	FullChargeCapWh   float64 `json:"full_charge_capacity_wh,omitempty"`
	WearLevelPercent  float64 `json:"wear_level_percent,omitempty"`
	CycleCount        int     `json:"cycle_count,omitempty"`
	RemainingTime     string  `json:"remaining_time,omitempty"`
}

// ─── Process ─────────────────────────────────────────────────────────────────

type ProcessInfo struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	Status     string  `json:"status,omitempty"`
}

// ─── Services ────────────────────────────────────────────────────────────────

type ServiceInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	StartType   string `json:"start_type"`
	ExePath     string `json:"exe_path,omitempty"`
}

// ─── Drivers ─────────────────────────────────────────────────────────────────

type DriverInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Provider string `json:"provider,omitempty"`
	Date     string `json:"date,omitempty"`
	Status   string `json:"status"`
}

// ─── Startup ─────────────────────────────────────────────────────────────────

type StartupItem struct {
	Name     string `json:"name"`
	Command  string `json:"command,omitempty"`
	Source   string `json:"source"` // Registry / StartupFolder / TaskScheduler
	Enabled  bool   `json:"enabled"`
}

// ─── Installed Programs ──────────────────────────────────────────────────────

type ProgramInfo struct {
	Name         string `json:"name"`
	Version      string `json:"version,omitempty"`
	Publisher    string `json:"publisher,omitempty"`
	InstallDate  string `json:"install_date,omitempty"`
	EstimatedMB  int    `json:"estimated_size_mb,omitempty"`
}

// ─── USB ─────────────────────────────────────────────────────────────────────

type USBDevice struct {
	Name        string `json:"name"`
	VendorID    string `json:"vendor_id,omitempty"`
	ProductID   string `json:"product_id,omitempty"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
}

// ─── Bluetooth ───────────────────────────────────────────────────────────────

type BluetoothInfo struct {
	Adapters  []BluetoothAdapter  `json:"adapters,omitempty"`
	Devices   []BluetoothDevice   `json:"devices,omitempty"`
}

type BluetoothAdapter struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	MAC    string `json:"mac,omitempty"`
}

type BluetoothDevice struct {
	Name       string `json:"name"`
	MAC        string `json:"mac,omitempty"`
	Connected  bool   `json:"connected"`
	Type       string `json:"type,omitempty"`
}

// ─── PCI ─────────────────────────────────────────────────────────────────────

type PCIDevice struct {
	Name        string `json:"name"`
	VendorID    string `json:"vendor_id,omitempty"`
	DeviceID    string `json:"device_id,omitempty"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
}

// ─── Windows Features ────────────────────────────────────────────────────────

type WinFeatures struct {
	HyperV              string `json:"hyper_v,omitempty"`
	WSL                 string `json:"wsl,omitempty"`
	Sandbox             string `json:"sandbox,omitempty"`
	VirtualMachinePlatform string `json:"virtual_machine_platform,omitempty"`
	NetFramework        string `json:"net_framework,omitempty"`
	PowerShellVersion   string `json:"powershell_version,omitempty"`
}

// ─── Security ────────────────────────────────────────────────────────────────

type SecurityInfo struct {
	DefenderEnabled          bool     `json:"defender_enabled"`
	DefenderRealtime         bool     `json:"defender_realtime_protection"`
	ControlledFolderAccess   bool     `json:"controlled_folder_access"`
	FirewallEnabled          bool     `json:"firewall_enabled"`
	SecureBootEnabled        bool     `json:"secure_boot_enabled"`
	TPMPresent               bool     `json:"tpm_present"`
	TPMVersion               string   `json:"tpm_version,omitempty"`
	BitLockerEnabled         bool     `json:"bitlocker_enabled"`
	OSUpdateCurrent          bool     `json:"os_update_current"`
	LastUpdateDate           string   `json:"last_update_date,omitempty"`
	CredentialGuard          bool     `json:"credential_guard"`
	CoreIsolation            bool     `json:"core_isolation"`
	MemoryIntegrity          bool     `json:"memory_integrity"`
	SmartScreen              bool     `json:"smartscreen"`
	ThreatsFound             []string `json:"threats_found,omitempty"`
}

// ─── Score ───────────────────────────────────────────────────────────────────

type Score struct {
	Value         float64  `json:"value"`
	MaxValue      float64  `json:"max_value"`
	Label         string   `json:"label"`
	Reasons       []string `json:"reasons"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// ─── Compare ─────────────────────────────────────────────────────────────────

type CompareResult struct {
	Field        string      `json:"field"`
	Report1Value interface{} `json:"report1_value"`
	Report2Value interface{} `json:"report2_value"`
	Diff         string      `json:"diff"`
	Category     string      `json:"category,omitempty"`
}
