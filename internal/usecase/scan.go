package usecase

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"sysscope/internal/domain"
)

// ScanUseCase orchestrates running all collectors and building a report.
type ScanUseCase struct {
	registry domain.CollectorRegistry
	scorer   domain.Scorer
}

func NewScanUseCase(registry domain.CollectorRegistry, scorer domain.Scorer) *ScanUseCase {
	return &ScanUseCase{registry: registry, scorer: scorer}
}

// Execute runs all collectors concurrently and assembles the report.
func (uc *ScanUseCase) Execute(ctx context.Context) (*domain.Report, error) {
	hostname, _ := os.Hostname()

	report := &domain.Report{
		ID:          fmt.Sprintf("sr_%d", time.Now().UnixNano()),
		GeneratedAt: time.Now(),
		Hostname:    hostname,
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, c := range uc.registry.Collectors() {
		wg.Add(1)
		go func(col domain.Collector) {
			defer wg.Done()

			result, err := col.Collect(ctx)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				report.Errors = append(report.Errors, domain.CollectError{
					Module:  col.Name(),
					Message: err.Error(),
				})
				return
			}
			uc.assignToReport(report, col.Name(), result)
		}(c)
	}
	wg.Wait()

	// Compute scores
	report.HealthScore = uc.scorer.ComputeHealth(report)
	report.SecurityScore = uc.scorer.ComputeSecurity(report)

	// Generate recommendations
	report.Recommendations = uc.generateRecommendations(report)

	return report, nil
}

func (uc *ScanUseCase) assignToReport(r *domain.Report, name string, data any) {
	switch name {
	case "os":
		if v, ok := data.(*domain.OSInfo); ok {
			r.OS = v
		}
	case "cpu":
		if v, ok := data.(*domain.CPUInfo); ok {
			r.CPU = v
		}
	case "memory":
		if v, ok := data.(*domain.MemoryInfo); ok {
			r.Memory = v
		}
	case "disk":
		if v, ok := data.([]domain.DiskInfo); ok {
			r.Disks = v
		}
	case "network":
		if v, ok := data.([]domain.NetAdapter); ok {
			r.Network = v
		}
	case "bios":
		if v, ok := data.(*domain.BIOSInfo); ok {
			r.BIOS = v
		}
	case "motherboard":
		if v, ok := data.(*domain.MoboInfo); ok {
			r.Motherboard = v
		}
	case "gpu":
		if v, ok := data.([]domain.GPUInfo); ok {
			r.GPU = v
		}
	case "monitor":
		if v, ok := data.([]domain.MonitorInfo); ok {
			r.Monitors = v
		}
	case "battery":
		if v, ok := data.(*domain.BatteryInfo); ok {
			r.Battery = v
		}
	case "temperatures":
		if v, ok := data.(*domain.TempInfo); ok {
			r.Temperatures = v
		}
	case "smart":
		if v, ok := data.([]domain.SMARTInfo); ok {
			r.SMART = v
		}
	case "processes":
		if v, ok := data.([]domain.ProcessInfo); ok {
			r.Processes = v
		}
	case "services":
		if v, ok := data.([]domain.ServiceInfo); ok {
			r.Services = v
		}
	case "drivers":
		if v, ok := data.([]domain.DriverInfo); ok {
			r.Drivers = v
		}
	case "startup":
		if v, ok := data.([]domain.StartupItem); ok {
			r.Startup = v
		}
	case "programs":
		if v, ok := data.([]domain.ProgramInfo); ok {
			r.Programs = v
		}
	case "environment":
		if v, ok := data.(map[string]string); ok {
			r.EnvVars = v
		}
	case "usb":
		if v, ok := data.([]domain.USBDevice); ok {
			r.USB = v
		}
	case "bluetooth":
		if v, ok := data.(*domain.BluetoothInfo); ok {
			r.Bluetooth = v
		}
	case "pci":
		if v, ok := data.([]domain.PCIDevice); ok {
			r.PCI = v
		}
	case "winfeatures":
		if v, ok := data.(*domain.WinFeatures); ok {
			r.WinFeatures = v
		}
	case "security":
		if v, ok := data.(*domain.SecurityInfo); ok {
			r.Security = v
		}
	}
}

// generateRecommendations produces contextual advice based on collected data.
func (uc *ScanUseCase) generateRecommendations(r *domain.Report) []string {
	var recs []string

	// Memory
	if r.Memory != nil {
		if r.Memory.UsagePercent > 85 {
			recs = append(recs, "🔧 Свободно менее 15% оперативной памяти. Рекомендуется закрыть неиспользуемые приложения или увеличить объём RAM.")
		}
		if len(r.Memory.Sticks) >= 2 {
			recs = append(recs, fmt.Sprintf("✅ Обнаружено %d планки памяти — возможен двухканальный режим работы.", len(r.Memory.Sticks)))
		} else if len(r.Memory.Sticks) == 1 {
			recs = append(recs, "💡 Установлена одна планка памяти. Добавление второй планки позволит использовать двухканальный режим и повысит производительность.")
		}
	}

	// Disks
	for _, d := range r.Disks {
		for _, p := range d.Partitions {
			if p.UsagePercent > 90 {
				recs = append(recs, fmt.Sprintf("⚠️  На диске %s свободно менее %.0f%% места. При заполнении выше 90%% производительность SSD может снижаться.", p.Letter, 100-p.UsagePercent))
			}
		}
	}

	// SMART
	for _, s := range r.SMART {
		if s.RemainingLife > 0 && s.RemainingLife < 20 {
			recs = append(recs, fmt.Sprintf("🔴 Ресурс SSD %s исчерпан на %.0f%%. Рекомендуется заменить накопитель в ближайшее время.", s.Model, 100-s.RemainingLife))
		}
		if s.PowerOnHours > 30000 {
			recs = append(recs, fmt.Sprintf("⚠️  Диск %s отработал %d часов. Рекомендуется проверить SMART и создать резервную копию.", s.Model, s.PowerOnHours))
		}
	}

	// Temperatures
	if r.Temperatures != nil {
		if r.Temperatures.CPU > 85 {
			recs = append(recs, fmt.Sprintf("🔴 Температура CPU %.0f°C — критическая. Проверьте систему охлаждения.", r.Temperatures.CPU))
		} else if r.Temperatures.CPU > 70 {
			recs = append(recs, fmt.Sprintf("⚠️  Температура CPU %.0f°C — высокая. Рекомендуется проверить вентиляцию.", r.Temperatures.CPU))
		}
		if r.Temperatures.GPU > 85 {
			recs = append(recs, fmt.Sprintf("🔴 Температура GPU %.0f°C — критическая.", r.Temperatures.GPU))
		}
	}

	// Battery
	if r.Battery != nil && r.Battery.IsPresent {
		if r.Battery.WearLevelPercent > 30 {
			recs = append(recs, fmt.Sprintf("⚠️  Износ батареи: %.0f%%. Рекомендуется проверить состояние.", r.Battery.WearLevelPercent))
		}
	}

	// Security
	if r.Security != nil {
		if !r.Security.FirewallEnabled {
			recs = append(recs, "🔒 Рекомендуется включить брандмауэр для защиты от сетевых угроз.")
		}
		if !r.Security.SecureBootEnabled {
			recs = append(recs, "🔒 Рекомендуется включить Secure Boot для защиты от несанкционированной загрузки.")
		}
		if !r.Security.BitLockerEnabled {
			recs = append(recs, "🔒 Рекомендуется включить шифрование диска для защиты данных.")
		}
		if !r.Security.OSUpdateCurrent {
			recs = append(recs, "🔒 Система не обновлена. Установите последние обновления безопасности.")
		}
	}

	// Programs — check for old drivers via driver dates
	for _, d := range r.Drivers {
		if d.Date != "" && d.Provider != "" {
			// Check if driver is old (simplified)
			t, err := time.Parse("2006-01-02", d.Date)
			if err == nil && time.Since(t) > 2*365*24*time.Hour {
				recs = append(recs, fmt.Sprintf("💡 Драйвер %s от %s устарел (%s). Рекомендуется обновить.", d.Name, d.Provider, d.Date))
				break // Only mention once
			}
		}
	}

	if len(recs) == 0 {
		recs = append(recs, "✅ Система в хорошем состоянии. Серьёзных проблем не обнаружено.")
	}

	sort.Strings(recs)
	return recs
}
