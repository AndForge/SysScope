package collector

import (
	"context"

	"sysscope/internal/domain"
)

// ─── platform hooks ──────────────────────────────────────────────────────────

// OS
type osCollector struct{}

func NewOS() domain.Collector                                      { return &osCollector{} }
func (c *osCollector) Name() string                                { return "os" }
func (c *osCollector) Collect(ctx context.Context) (any, error)    { return osCollectFunc(ctx) }

var osCollectFunc = func(ctx context.Context) (*domain.OSInfo, error) {
	return nil, errNotImplemented("os")
}

func RegisterOS(f func(context.Context) (*domain.OSInfo, error)) { osCollectFunc = f }

// CPU
type cpuCollector struct{}

func NewCPU() domain.Collector                                      { return &cpuCollector{} }
func (c *cpuCollector) Name() string                                { return "cpu" }
func (c *cpuCollector) Collect(ctx context.Context) (any, error)    { return cpuCollectFunc(ctx) }

var cpuCollectFunc = func(ctx context.Context) (*domain.CPUInfo, error) {
	return nil, errNotImplemented("cpu")
}

func RegisterCPU(f func(context.Context) (*domain.CPUInfo, error)) { cpuCollectFunc = f }

// Memory
type memoryCollector struct{}

func NewMemory() domain.Collector                                      { return &memoryCollector{} }
func (c *memoryCollector) Name() string                                { return "memory" }
func (c *memoryCollector) Collect(ctx context.Context) (any, error)    { return memCollectFunc(ctx) }

var memCollectFunc = func(ctx context.Context) (*domain.MemoryInfo, error) {
	return nil, errNotImplemented("memory")
}

func RegisterMemory(f func(context.Context) (*domain.MemoryInfo, error)) { memCollectFunc = f }

// Disks
type diskCollector struct{}

func NewDisk() domain.Collector                                      { return &diskCollector{} }
func (c *diskCollector) Name() string                                { return "disk" }
func (c *diskCollector) Collect(ctx context.Context) (any, error)    { return diskCollectFunc(ctx) }

var diskCollectFunc = func(ctx context.Context) ([]domain.DiskInfo, error) {
	return nil, errNotImplemented("disk")
}

func RegisterDisk(f func(context.Context) ([]domain.DiskInfo, error)) { diskCollectFunc = f }

// Network
type networkCollector struct{}

func NewNetwork() domain.Collector                                      { return &networkCollector{} }
func (c *networkCollector) Name() string                                { return "network" }
func (c *networkCollector) Collect(ctx context.Context) (any, error)    { return netCollectFunc(ctx) }

var netCollectFunc = func(ctx context.Context) ([]domain.NetAdapter, error) {
	return nil, errNotImplemented("network")
}

func RegisterNetwork(f func(context.Context) ([]domain.NetAdapter, error)) { netCollectFunc = f }

// BIOS
type biosCollector struct{}

func NewBIOS() domain.Collector                                      { return &biosCollector{} }
func (c *biosCollector) Name() string                                { return "bios" }
func (c *biosCollector) Collect(ctx context.Context) (any, error)    { return biosCollectFunc(ctx) }

var biosCollectFunc = func(ctx context.Context) (*domain.BIOSInfo, error) {
	return nil, errNotImplemented("bios")
}

func RegisterBIOS(f func(context.Context) (*domain.BIOSInfo, error)) { biosCollectFunc = f }

// Motherboard
type moboCollector struct{}

func NewMotherboard() domain.Collector                                      { return &moboCollector{} }
func (c *moboCollector) Name() string                                       { return "motherboard" }
func (c *moboCollector) Collect(ctx context.Context) (any, error)           { return moboCollectFunc(ctx) }

var moboCollectFunc = func(ctx context.Context) (*domain.MoboInfo, error) {
	return nil, errNotImplemented("motherboard")
}

func RegisterMotherboard(f func(context.Context) (*domain.MoboInfo, error)) { moboCollectFunc = f }

// GPU
type gpuCollector struct{}

func NewGPU() domain.Collector                                      { return &gpuCollector{} }
func (c *gpuCollector) Name() string                                { return "gpu" }
func (c *gpuCollector) Collect(ctx context.Context) (any, error)    { return gpuCollectFunc(ctx) }

var gpuCollectFunc = func(ctx context.Context) ([]domain.GPUInfo, error) {
	return nil, errNotImplemented("gpu")
}

func RegisterGPU(f func(context.Context) ([]domain.GPUInfo, error)) { gpuCollectFunc = f }

// Monitors
type monitorCollector struct{}

func NewMonitor() domain.Collector                                      { return &monitorCollector{} }
func (c *monitorCollector) Name() string                                { return "monitor" }
func (c *monitorCollector) Collect(ctx context.Context) (any, error)    { return monCollectFunc(ctx) }

var monCollectFunc = func(ctx context.Context) ([]domain.MonitorInfo, error) {
	return nil, errNotImplemented("monitor")
}

func RegisterMonitor(f func(context.Context) ([]domain.MonitorInfo, error)) { monCollectFunc = f }

// Battery
type batteryCollector struct{}

func NewBattery() domain.Collector                                      { return &batteryCollector{} }
func (c *batteryCollector) Name() string                                { return "battery" }
func (c *batteryCollector) Collect(ctx context.Context) (any, error)    { return batCollectFunc(ctx) }

var batCollectFunc = func(ctx context.Context) (*domain.BatteryInfo, error) {
	return nil, errNotImplemented("battery")
}

func RegisterBattery(f func(context.Context) (*domain.BatteryInfo, error)) { batCollectFunc = f }

// Processes
type processesCollector struct{}

func NewProcesses() domain.Collector                                      { return &processesCollector{} }
func (c *processesCollector) Name() string                                { return "processes" }
func (c *processesCollector) Collect(ctx context.Context) (any, error)    { return procCollectFunc(ctx) }

var procCollectFunc = func(ctx context.Context) ([]domain.ProcessInfo, error) {
	return nil, errNotImplemented("processes")
}

func RegisterProcesses(f func(context.Context) ([]domain.ProcessInfo, error)) { procCollectFunc = f }

// Security
type securityCollector struct{}

func NewSecurity() domain.Collector                                      { return &securityCollector{} }
func (c *securityCollector) Name() string                                { return "security" }
func (c *securityCollector) Collect(ctx context.Context) (any, error)    { return secCollectFunc(ctx) }

var secCollectFunc = func(ctx context.Context) (*domain.SecurityInfo, error) {
	return nil, errNotImplemented("security")
}

func RegisterSecurity(f func(context.Context) (*domain.SecurityInfo, error)) { secCollectFunc = f }

// ─── NEW COLLECTORS ──────────────────────────────────────────────────────────

// Temperatures
type tempCollector struct{}

func NewTemperatures() domain.Collector                                      { return &tempCollector{} }
func (c *tempCollector) Name() string                                        { return "temperatures" }
func (c *tempCollector) Collect(ctx context.Context) (any, error)            { return tempCollectFunc(ctx) }

var tempCollectFunc = func(ctx context.Context) (*domain.TempInfo, error) {
	return nil, errNotImplemented("temperatures")
}

func RegisterTemperatures(f func(context.Context) (*domain.TempInfo, error)) { tempCollectFunc = f }

// SMART
type smartCollector struct{}

func NewSMART() domain.Collector                                      { return &smartCollector{} }
func (c *smartCollector) Name() string                                { return "smart" }
func (c *smartCollector) Collect(ctx context.Context) (any, error)    { return smartCollectFunc(ctx) }

var smartCollectFunc = func(ctx context.Context) ([]domain.SMARTInfo, error) {
	return nil, errNotImplemented("smart")
}

func RegisterSMART(f func(context.Context) ([]domain.SMARTInfo, error)) { smartCollectFunc = f }

// Services
type servicesCollector struct{}

func NewServices() domain.Collector                                      { return &servicesCollector{} }
func (c *servicesCollector) Name() string                                { return "services" }
func (c *servicesCollector) Collect(ctx context.Context) (any, error)    { return svcCollectFunc(ctx) }

var svcCollectFunc = func(ctx context.Context) ([]domain.ServiceInfo, error) {
	return nil, errNotImplemented("services")
}

func RegisterServices(f func(context.Context) ([]domain.ServiceInfo, error)) { svcCollectFunc = f }

// Drivers
type driversCollector struct{}

func NewDrivers() domain.Collector                                      { return &driversCollector{} }
func (c *driversCollector) Name() string                                { return "drivers" }
func (c *driversCollector) Collect(ctx context.Context) (any, error)    { return drvCollectFunc(ctx) }

var drvCollectFunc = func(ctx context.Context) ([]domain.DriverInfo, error) {
	return nil, errNotImplemented("drivers")
}

func RegisterDrivers(f func(context.Context) ([]domain.DriverInfo, error)) { drvCollectFunc = f }

// Startup
type startupCollector struct{}

func NewStartup() domain.Collector                                      { return &startupCollector{} }
func (c *startupCollector) Name() string                                { return "startup" }
func (c *startupCollector) Collect(ctx context.Context) (any, error)    { return startCollectFunc(ctx) }

var startCollectFunc = func(ctx context.Context) ([]domain.StartupItem, error) {
	return nil, errNotImplemented("startup")
}

func RegisterStartup(f func(context.Context) ([]domain.StartupItem, error)) { startCollectFunc = f }

// Programs
type programsCollector struct{}

func NewPrograms() domain.Collector                                      { return &programsCollector{} }
func (c *programsCollector) Name() string                                { return "programs" }
func (c *programsCollector) Collect(ctx context.Context) (any, error)    { return progCollectFunc(ctx) }

var progCollectFunc = func(ctx context.Context) ([]domain.ProgramInfo, error) {
	return nil, errNotImplemented("programs")
}

func RegisterPrograms(f func(context.Context) ([]domain.ProgramInfo, error)) { progCollectFunc = f }

// Environment
type envCollector struct{}

func NewEnvironment() domain.Collector                                      { return &envCollector{} }
func (c *envCollector) Name() string                                        { return "environment" }
func (c *envCollector) Collect(ctx context.Context) (any, error)            { return envCollectFunc(ctx) }

var envCollectFunc = func(ctx context.Context) (map[string]string, error) {
	return nil, errNotImplemented("environment")
}

func RegisterEnvironment(f func(context.Context) (map[string]string, error)) { envCollectFunc = f }

// USB
type usbCollector struct{}

func NewUSB() domain.Collector                                      { return &usbCollector{} }
func (c *usbCollector) Name() string                                { return "usb" }
func (c *usbCollector) Collect(ctx context.Context) (any, error)    { return usbCollectFunc(ctx) }

var usbCollectFunc = func(ctx context.Context) ([]domain.USBDevice, error) {
	return nil, errNotImplemented("usb")
}

func RegisterUSB(f func(context.Context) ([]domain.USBDevice, error)) { usbCollectFunc = f }

// Bluetooth
type bluetoothCollector struct{}

func NewBluetooth() domain.Collector                                      { return &bluetoothCollector{} }
func (c *bluetoothCollector) Name() string                                { return "bluetooth" }
func (c *bluetoothCollector) Collect(ctx context.Context) (any, error)    { return btCollectFunc(ctx) }

var btCollectFunc = func(ctx context.Context) (*domain.BluetoothInfo, error) {
	return nil, errNotImplemented("bluetooth")
}

func RegisterBluetooth(f func(context.Context) (*domain.BluetoothInfo, error)) { btCollectFunc = f }

// PCI
type pciCollector struct{}

func NewPCI() domain.Collector                                      { return &pciCollector{} }
func (c *pciCollector) Name() string                                { return "pci" }
func (c *pciCollector) Collect(ctx context.Context) (any, error)    { return pciCollectFunc(ctx) }

var pciCollectFunc = func(ctx context.Context) ([]domain.PCIDevice, error) {
	return nil, errNotImplemented("pci")
}

func RegisterPCI(f func(context.Context) ([]domain.PCIDevice, error)) { pciCollectFunc = f }

// Windows Features
type winfeatCollector struct{}

func NewWinFeatures() domain.Collector                                      { return &winfeatCollector{} }
func (c *winfeatCollector) Name() string                                    { return "winfeatures" }
func (c *winfeatCollector) Collect(ctx context.Context) (any, error)        { return wfCollectFunc(ctx) }

var wfCollectFunc = func(ctx context.Context) (*domain.WinFeatures, error) {
	return nil, errNotImplemented("winfeatures")
}

func RegisterWinFeatures(f func(context.Context) (*domain.WinFeatures, error)) { wfCollectFunc = f }
