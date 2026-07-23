package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sysscope/internal/adapter/collector"
	"sysscope/internal/adapter/exporter"
	"sysscope/internal/domain"
	"sysscope/internal/usecase"

	"github.com/spf13/cobra"
)

var (
	version   = "0.4.0"
	buildDate = "dev"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "sysscope",
		Short: "SysScope — system diagnostics & reporting utility",
		Long: `SysScope collects detailed hardware, software, and security information
about your system and generates reports in JSON or HTML format.

Supports Windows (primary), Linux, and macOS.`,
	}
	root.AddCommand(newScanCmd())
	root.AddCommand(newSummaryCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newCompareCmd())
	root.AddCommand(newLiveCmd())
	root.AddCommand(newHistoryCmd())
	root.AddCommand(newVersionCmd())
	return root
}

// ─── scan ────────────────────────────────────────────────────────────────────

func newScanCmd() *cobra.Command {
	var (
		format string
		output string
		noSave bool
	)
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan system and generate a full report",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()

			registry := domain.NewRegistry()
			registerAllCollectors(registry)
			scorer := usecase.NewScoreUseCase()
			scanUC := usecase.NewScanUseCase(registry, scorer)

			fmt.Println("🔍 Scanning system...")
			report, err := scanUC.Execute(ctx)
			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}

			var exp domain.Exporter
			switch strings.ToLower(format) {
			case "json":
				exp = exporter.NewJSON()
			case "html":
				exp = exporter.NewHTML()
			default:
				return fmt.Errorf("unsupported format: %s (use json or html)", format)
			}

			data, err := exp.Export(report)
			if err != nil {
				return fmt.Errorf("export failed: %w", err)
			}

			if output == "" {
				ts := time.Now().Format("20060102_150405")
				output = fmt.Sprintf("sysscope_report_%s.%s", ts, exp.Extension())
			}

			if !noSave {
				if err := os.WriteFile(output, data, 0644); err != nil {
					return fmt.Errorf("write file: %w", err)
				}
				// Auto-save to history
				saveToHistory(data, exp.Extension())
			}

			fmt.Printf("✅ Report saved: %s\n", output)
			fmt.Printf("📊 Health Score:   %.0f/%.0f (%s)\n",
				report.HealthScore.Value, report.HealthScore.MaxValue, report.HealthScore.Label)
			fmt.Printf("🔒 Security Score: %.0f/%.0f (%s)\n",
				report.SecurityScore.Value, report.SecurityScore.MaxValue, report.SecurityScore.Label)

			if len(report.Errors) > 0 {
				fmt.Printf("\n⚠️  %d module(s) had errors:\n", len(report.Errors))
				for _, e := range report.Errors {
					fmt.Printf("   • %s: %s\n", e.Module, e.Message)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "json", "Export format: json, html")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (auto-generated if empty)")
	cmd.Flags().BoolVar(&noSave, "no-save", false, "Don't save to file")
	return cmd
}

// ─── summary ─────────────────────────────────────────────────────────────────

func newSummaryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Quick system summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			registry := domain.NewRegistry()
			registerAllCollectors(registry)
			scorer := usecase.NewScoreUseCase()
			scanUC := usecase.NewScanUseCase(registry, scorer)

			report, err := scanUC.Execute(ctx)
			if err != nil {
				return err
			}

			fmt.Println("╔══════════════════════════════════════════════════╗")
			fmt.Println("║          SysScope — System Summary              ║")
			fmt.Println("╚══════════════════════════════════════════════════╝")
			fmt.Println()

			if report.OS != nil {
				fmt.Printf("  💻 OS:           %s %s\n", report.OS.ProductName, report.OS.Version)
				fmt.Printf("  🏗️  Architecture: %s\n", report.OS.Architecture)
				fmt.Printf("  ⏱️  Uptime:       %s\n", report.OS.Uptime)
			}
			fmt.Println()

			if report.CPU != nil {
				fmt.Printf("  ⚡ CPU:    %s\n", report.CPU.Model)
				fmt.Printf("           %d cores / %d threads @ %.0f MHz\n",
					report.CPU.PhysicalCores, report.CPU.LogicalCores, report.CPU.MaxMHz)
				fmt.Printf("           Usage: %.1f%%\n", report.CPU.UsagePercent)
			}
			fmt.Println()

			if report.Memory != nil {
				fmt.Printf("  🧠 RAM:    %s / %s (%.1f%% used)\n",
					fmtBytes(report.Memory.UsedBytes), fmtBytes(report.Memory.TotalBytes), report.Memory.UsagePercent)
				if len(report.Memory.Sticks) > 0 {
					fmt.Printf("           %d stick(s):", len(report.Memory.Sticks))
					for _, s := range report.Memory.Sticks {
						fmt.Printf(" %s %dGB", s.Manufacturer, s.SizeBytes/(1<<30))
					}
					fmt.Println()
				}
			}
			fmt.Println()

			if len(report.GPU) > 0 {
				for _, g := range report.GPU {
					fmt.Printf("  🎮 GPU:    %s (%s)\n", g.Name, g.DriverVersion)
				}
			}
			fmt.Println()

			if len(report.Disks) > 0 {
				for _, d := range report.Disks {
					fmt.Printf("  💾 Disk:   %s [%s] %s\n", d.Model, d.MediaType, fmtBytes(d.SizeBytes))
					for _, p := range d.Partitions {
						fmt.Printf("           %s %s — %.1f%% used\n", p.Letter, p.FileSystem, p.UsagePercent)
					}
				}
			}
			fmt.Println()

			if len(report.Network) > 0 {
				for _, n := range report.Network {
					fmt.Printf("  🌐 Net:    %s [%s]", n.Name, n.Type)
					if len(n.IPv4) > 0 {
						fmt.Printf(" %s", n.IPv4[0])
					}
					if n.SpeedMbps > 0 {
						fmt.Printf(" (%d Mbps)", n.SpeedMbps)
					}
					fmt.Println()
				}
			}
			fmt.Println()

			healthEmoji := "✅"
			if report.HealthScore.Value < 50 {
				healthEmoji = "🔴"
			} else if report.HealthScore.Value < 70 {
				healthEmoji = "🟡"
			}
			secEmoji := "✅"
			if report.SecurityScore.Value < 50 {
				secEmoji = "🔴"
			} else if report.SecurityScore.Value < 70 {
				secEmoji = "🟡"
			}
			fmt.Printf("  %s Health:   %.0f/%.0f (%s)\n", healthEmoji, report.HealthScore.Value, report.HealthScore.MaxValue, report.HealthScore.Label)
			fmt.Printf("  %s Security: %.0f/%.0f (%s)\n", secEmoji, report.SecurityScore.Value, report.SecurityScore.MaxValue, report.SecurityScore.Label)

			if len(report.Recommendations) > 0 {
				fmt.Println("\n  💡 Recommendations:")
				for _, r := range report.Recommendations {
					fmt.Printf("     %s\n", r)
				}
			}
			fmt.Println()
			return nil
		},
	}
}

// ─── doctor ──────────────────────────────────────────────────────────────────

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run system diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			registry := domain.NewRegistry()
			registerAllCollectors(registry)
			scorer := usecase.NewScoreUseCase()
			scanUC := usecase.NewScanUseCase(registry, scorer)

			fmt.Println("🩺 Running system diagnostics...")
			fmt.Println()
			report, err := scanUC.Execute(ctx)
			if err != nil {
				return err
			}

			checks := runDiagnostics(report)
			passed, warnings, failures := 0, 0, 0

			for _, c := range checks {
				switch c.Severity {
				case "ok":
					fmt.Printf("  ✔ %s\n", c.Message)
					passed++
				case "warn":
					fmt.Printf("  ⚠ %s\n", c.Message)
					warnings++
				case "fail":
					fmt.Printf("  ✘ %s\n", c.Message)
					failures++
				}
			}

			fmt.Printf("\n  Summary: %d passed, %d warnings, %d failures\n", passed, warnings, failures)
			fmt.Printf("  Health:   %.0f/%.0f (%s)\n", report.HealthScore.Value, report.HealthScore.MaxValue, report.HealthScore.Label)
			fmt.Printf("  Security: %.0f/%.0f (%s)\n", report.SecurityScore.Value, report.SecurityScore.MaxValue, report.SecurityScore.Label)

			if len(report.Recommendations) > 0 {
				fmt.Println("\n  💡 Recommendations:")
				for _, r := range report.Recommendations {
					fmt.Printf("     %s\n", r)
				}
			}
			fmt.Println()
			return nil
		},
	}
}

type diagnostic struct {
	Severity string
	Message  string
}

func runDiagnostics(r *domain.Report) []diagnostic {
	var checks []diagnostic

	if r.CPU != nil {
		if r.CPU.UsagePercent > 90 {
			checks = append(checks, diagnostic{"warn", fmt.Sprintf("CPU usage high: %.1f%%", r.CPU.UsagePercent)})
		} else {
			checks = append(checks, diagnostic{"ok", fmt.Sprintf("CPU: %s (%d cores)", r.CPU.Model, r.CPU.PhysicalCores)})
		}
	}

	if r.Memory != nil {
		freeGB := float64(r.Memory.FreeBytes) / (1 << 30)
		if r.Memory.UsagePercent > 90 {
			checks = append(checks, diagnostic{"fail", fmt.Sprintf("RAM usage critical: %.1f%% (%.1f GB free)", r.Memory.UsagePercent, freeGB)})
		} else if r.Memory.UsagePercent > 80 {
			checks = append(checks, diagnostic{"warn", fmt.Sprintf("RAM usage high: %.1f%% (%.1f GB free)", r.Memory.UsagePercent, freeGB)})
		} else {
			checks = append(checks, diagnostic{"ok", fmt.Sprintf("RAM: %.1f GB free of %s", freeGB, fmtBytes(r.Memory.TotalBytes))})
		}
	}

	if len(r.Disks) > 0 {
		for _, d := range r.Disks {
			for _, p := range d.Partitions {
				if p.UsagePercent > 95 {
					checks = append(checks, diagnostic{"fail", fmt.Sprintf("Disk %s nearly full: %.1f%% used", p.Letter, p.UsagePercent)})
				} else if p.UsagePercent > 85 {
					checks = append(checks, diagnostic{"warn", fmt.Sprintf("Disk %s usage high: %.1f%%", p.Letter, p.UsagePercent)})
				} else {
					checks = append(checks, diagnostic{"ok", fmt.Sprintf("Disk %s: %.1f%% used", p.Letter, p.UsagePercent)})
				}
			}
		}
	}

	if r.Temperatures != nil {
		if r.Temperatures.CPU > 85 {
			checks = append(checks, diagnostic{"fail", fmt.Sprintf("CPU temperature critical: %.0f°C", r.Temperatures.CPU)})
		} else if r.Temperatures.CPU > 70 {
			checks = append(checks, diagnostic{"warn", fmt.Sprintf("CPU temperature high: %.0f°C", r.Temperatures.CPU)})
		} else if r.Temperatures.CPU > 0 {
			checks = append(checks, diagnostic{"ok", fmt.Sprintf("CPU temp: %.0f°C (normal)", r.Temperatures.CPU)})
		}
	}

	for _, s := range r.SMART {
		if s.Health == "Warning" || s.Health == "Failed" {
			checks = append(checks, diagnostic{"fail", fmt.Sprintf("SMART: %s — %s", s.Model, s.Health)})
		} else {
			checks = append(checks, diagnostic{"ok", fmt.Sprintf("SMART: %s — OK", s.Model)})
		}
	}

	if r.Security != nil {
		if r.Security.DefenderEnabled {
			checks = append(checks, diagnostic{"ok", "Defender / Antivirus is active"})
		} else {
			checks = append(checks, diagnostic{"fail", "Defender / Antivirus is NOT active"})
		}
		if r.Security.FirewallEnabled {
			checks = append(checks, diagnostic{"ok", "Firewall is enabled"})
		} else {
			checks = append(checks, diagnostic{"fail", "Firewall is DISABLED"})
		}
		if r.Security.SecureBootEnabled {
			checks = append(checks, diagnostic{"ok", "Secure Boot is enabled"})
		} else {
			checks = append(checks, diagnostic{"warn", "Secure Boot is disabled"})
		}
		if r.Security.BitLockerEnabled {
			checks = append(checks, diagnostic{"ok", "Disk encryption is enabled"})
		} else {
			checks = append(checks, diagnostic{"warn", "Disk encryption is not enabled"})
		}
		if r.Security.OSUpdateCurrent {
			checks = append(checks, diagnostic{"ok", fmt.Sprintf("OS updates current (last: %s)", r.Security.LastUpdateDate)})
		} else {
			checks = append(checks, diagnostic{"fail", fmt.Sprintf("OS updates OUTDATED (last: %s)", r.Security.LastUpdateDate)})
		}
	} else {
		checks = append(checks, diagnostic{"warn", "Security information not available"})
	}

	if r.Battery != nil && r.Battery.IsPresent {
		if r.Battery.HealthPercent > 0 {
			if r.Battery.HealthPercent < 50 {
				checks = append(checks, diagnostic{"fail", fmt.Sprintf("Battery health poor: %.0f%%", r.Battery.HealthPercent)})
			} else if r.Battery.HealthPercent < 80 {
				checks = append(checks, diagnostic{"warn", fmt.Sprintf("Battery health degraded: %.0f%%", r.Battery.HealthPercent)})
			} else {
				checks = append(checks, diagnostic{"ok", fmt.Sprintf("Battery health: %.0f%%", r.Battery.HealthPercent)})
			}
		} else {
			checks = append(checks, diagnostic{"ok", "Battery present (health data not available)"})
		}
	}

	return checks
}

// ─── compare ─────────────────────────────────────────────────────────────────

func newCompareCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compare <old.json> <new.json>",
		Short: "Compare two SysScope reports with grouped differences",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r1, err := loadReport(args[0])
			if err != nil {
				return fmt.Errorf("load report 1: %w", err)
			}
			r2, err := loadReport(args[1])
			if err != nil {
				return fmt.Errorf("load report 2: %w", err)
			}

			groups := compareGrouped(r1, r2)
			if len(groups) == 0 {
				fmt.Println("✅ Reports are identical — no differences found.")
				return nil
			}

			fmt.Println()
			fmt.Printf("📋  Comparison: %s vs %s\n", filepath.Base(args[0]), filepath.Base(args[1]))
			fmt.Println("─────────────────────────────────────────────────────")

			totalDiffs := 0
			for _, g := range groups {
				fmt.Printf("\n  %s %s\n", g.Icon, g.Name)
				for _, d := range g.Items {
					arrow := "→"
					if d.OldVal < d.NewVal {
						arrow = "↑"
					} else if d.OldVal > d.NewVal {
						arrow = "↓"
					}
					fmt.Printf("     %-20s %s  %s  %s\n", d.Label, d.OldStr, arrow, d.NewStr)
					totalDiffs++
				}
			}

			fmt.Println()
			fmt.Printf("  Total differences: %d\n", totalDiffs)

			// Score comparison summary
			if r1.HealthScore.Value != r2.HealthScore.Value || r1.SecurityScore.Value != r2.SecurityScore.Value {
				fmt.Println("\n  Score Changes:")
				healthArrow := "→"
				if r2.HealthScore.Value > r1.HealthScore.Value {
					healthArrow = "↑"
				} else if r2.HealthScore.Value < r1.HealthScore.Value {
					healthArrow = "↓"
				}
				secArrow := "→"
				if r2.SecurityScore.Value > r1.SecurityScore.Value {
					secArrow = "↑"
				} else if r2.SecurityScore.Value < r1.SecurityScore.Value {
					secArrow = "↓"
				}
				fmt.Printf("     Health:   %.0f %s %.0f\n", r1.HealthScore.Value, healthArrow, r2.HealthScore.Value)
				fmt.Printf("     Security: %.0f %s %.0f\n", r1.SecurityScore.Value, secArrow, r2.SecurityScore.Value)
			}
			fmt.Println()
			return nil
		},
	}
}

// compareGrouped returns differences grouped by section with numeric values for ordering.
type compareGroup struct {
	Name  string
	Icon  string
	Items []compareItem
}

type compareItem struct {
	Label  string
	OldStr string
	NewStr string
	OldVal float64
	NewVal float64
}

func compareGrouped(r1, r2 *domain.Report) []compareGroup {
	var groups []compareGroup

	// OS
	var osItems []compareItem
	if r1.OS != nil && r2.OS != nil {
		if r1.OS.Version != r2.OS.Version {
			osItems = append(osItems, compareItem{"Version", r1.OS.Version, r2.OS.Version, 0, 0})
		}
		if r1.OS.Build != r2.OS.Build {
			osItems = append(osItems, compareItem{"Build", r1.OS.Build, r2.OS.Build, 0, 0})
		}
		if r1.OS.Uptime != r2.OS.Uptime {
			osItems = append(osItems, compareItem{"Uptime", r1.OS.Uptime, r2.OS.Uptime, 0, 0})
		}
	} else if r1.OS != nil || r2.OS != nil {
		osItems = append(osItems, compareItem{"OS", safeStr(r1.OS), safeStr(r2.OS), 0, 0})
	}
	if len(osItems) > 0 {
		groups = append(groups, compareGroup{"Operating System", "💻", osItems})
	}

	// CPU
	var cpuItems []compareItem
	if r1.CPU != nil && r2.CPU != nil {
		if r1.CPU.UsagePercent != r2.CPU.UsagePercent {
			cpuItems = append(cpuItems, compareItem{"Usage",
				fmt.Sprintf("%.1f%%", r1.CPU.UsagePercent), fmt.Sprintf("%.1f%%", r2.CPU.UsagePercent),
				r1.CPU.UsagePercent, r2.CPU.UsagePercent})
		}
		if r1.CPU.CurrentMHz != r2.CPU.CurrentMHz {
			cpuItems = append(cpuItems, compareItem{"Current Freq",
				fmt.Sprintf("%.0f MHz", r1.CPU.CurrentMHz), fmt.Sprintf("%.0f MHz", r2.CPU.CurrentMHz),
				r1.CPU.CurrentMHz, r2.CPU.CurrentMHz})
		}
		if r1.CPU.Model != r2.CPU.Model {
			cpuItems = append(cpuItems, compareItem{"Model", r1.CPU.Model, r2.CPU.Model, 0, 0})
		}
	}
	if len(cpuItems) > 0 {
		groups = append(groups, compareGroup{"CPU", "⚡", cpuItems})
	}

	// Memory
	var memItems []compareItem
	if r1.Memory != nil && r2.Memory != nil {
		if r1.Memory.TotalBytes != r2.Memory.TotalBytes {
			memItems = append(memItems, compareItem{"Total",
				fmtBytes(r1.Memory.TotalBytes), fmtBytes(r2.Memory.TotalBytes),
				float64(r1.Memory.TotalBytes), float64(r2.Memory.TotalBytes)})
		}
		if r1.Memory.UsagePercent != r2.Memory.UsagePercent {
			memItems = append(memItems, compareItem{"Usage",
				fmt.Sprintf("%.1f%%", r1.Memory.UsagePercent), fmt.Sprintf("%.1f%%", r2.Memory.UsagePercent),
				r1.Memory.UsagePercent, r2.Memory.UsagePercent})
		}
		if r1.Memory.FreeBytes != r2.Memory.FreeBytes {
			memItems = append(memItems, compareItem{"Free",
				fmtBytes(r1.Memory.FreeBytes), fmtBytes(r2.Memory.FreeBytes),
				float64(r1.Memory.FreeBytes), float64(r2.Memory.FreeBytes)})
		}
	}
	if len(memItems) > 0 {
		groups = append(groups, compareGroup{"Memory", "🧠", memItems})
	}

	// Disks
	var diskItems []compareItem
	if len(r1.Disks) > 0 && len(r2.Disks) > 0 {
		for i := 0; i < len(r1.Disks) && i < len(r2.Disks); i++ {
			for j := 0; j < len(r1.Disks[i].Partitions) && j < len(r2.Disks[i].Partitions); j++ {
				p1, p2 := r1.Disks[i].Partitions[j], r2.Disks[i].Partitions[j]
				if p1.UsagePercent != p2.UsagePercent {
					diskItems = append(diskItems, compareItem{p1.Letter + " Usage",
						fmt.Sprintf("%.1f%%", p1.UsagePercent), fmt.Sprintf("%.1f%%", p2.UsagePercent),
						p1.UsagePercent, p2.UsagePercent})
				}
			}
		}
	}
	if len(diskItems) > 0 {
		groups = append(groups, compareGroup{"Disks", "💾", diskItems})
	}

	// Security
	var secItems []compareItem
	if r1.Security != nil && r2.Security != nil {
		secBool := func(label string, a, b bool) {
			if a != b {
				secItems = append(secItems, compareItem{label,
					boolStr(a), boolStr(b), boolF(a), boolF(b)})
			}
		}
		secBool("Defender", r1.Security.DefenderEnabled, r2.Security.DefenderEnabled)
		secBool("Firewall", r1.Security.FirewallEnabled, r2.Security.FirewallEnabled)
		secBool("Secure Boot", r1.Security.SecureBootEnabled, r2.Security.SecureBootEnabled)
		secBool("BitLocker", r1.Security.BitLockerEnabled, r2.Security.BitLockerEnabled)
		secBool("TPM", r1.Security.TPMPresent, r2.Security.TPMPresent)
		secBool("OS Updated", r1.Security.OSUpdateCurrent, r2.Security.OSUpdateCurrent)
		secBool("SmartScreen", r1.Security.SmartScreen, r2.Security.SmartScreen)
		secBool("Core Isolation", r1.Security.CoreIsolation, r2.Security.CoreIsolation)
	}
	if len(secItems) > 0 {
		groups = append(groups, compareGroup{"Security", "🔒", secItems})
	}

	// Scores
	var scoreItems []compareItem
	if r1.HealthScore.Value != r2.HealthScore.Value {
		scoreItems = append(scoreItems, compareItem{"Health",
			fmt.Sprintf("%.0f (%s)", r1.HealthScore.Value, r1.HealthScore.Label),
			fmt.Sprintf("%.0f (%s)", r2.HealthScore.Value, r2.HealthScore.Label),
			r1.HealthScore.Value, r2.HealthScore.Value})
	}
	if r1.SecurityScore.Value != r2.SecurityScore.Value {
		scoreItems = append(scoreItems, compareItem{"Security",
			fmt.Sprintf("%.0f (%s)", r1.SecurityScore.Value, r1.SecurityScore.Label),
			fmt.Sprintf("%.0f (%s)", r2.SecurityScore.Value, r2.SecurityScore.Label),
			r1.SecurityScore.Value, r2.SecurityScore.Value})
	}
	if len(scoreItems) > 0 {
		groups = append(groups, compareGroup{"Scores", "📊", scoreItems})
	}

	return groups
}

func boolStr(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}

func boolF(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func safeStr(os *domain.OSInfo) string {
	if os == nil {
		return "N/A"
	}
	return os.ProductName + " " + os.Version
}

// ─── live ────────────────────────────────────────────────────────────────────

func newLiveCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "live",
		Short: "Start real-time monitoring dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return startLiveServer(port)
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "HTTP server port")
	return cmd
}

// ─── history ─────────────────────────────────────────────────────────────────

func newHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "List saved scan history",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listHistory()
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("SysScope v%s (built %s)\n", version, buildDate)
		},
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func registerAllCollectors(reg *domain.DefaultRegistry) {
	reg.Register(collector.NewOS())
	reg.Register(collector.NewCPU())
	reg.Register(collector.NewMemory())
	reg.Register(collector.NewDisk())
	reg.Register(collector.NewNetwork())
	reg.Register(collector.NewBIOS())
	reg.Register(collector.NewMotherboard())
	reg.Register(collector.NewGPU())
	reg.Register(collector.NewMonitor())
	reg.Register(collector.NewBattery())
	reg.Register(collector.NewProcesses())
	reg.Register(collector.NewSecurity())
	reg.Register(collector.NewTemperatures())
	reg.Register(collector.NewSMART())
	reg.Register(collector.NewServices())
	reg.Register(collector.NewDrivers())
	reg.Register(collector.NewStartup())
	reg.Register(collector.NewPrograms())
	reg.Register(collector.NewEnvironment())
	reg.Register(collector.NewUSB())
	reg.Register(collector.NewBluetooth())
	reg.Register(collector.NewPCI())
	reg.Register(collector.NewWinFeatures())
}

func loadReport(path string) (*domain.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".json" && ext != "" {
		return nil, fmt.Errorf("unsupported format: %s", ext)
	}
	var r domain.Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}



func fmtBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func historyDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	dir := filepath.Join(home, ".sysscope", "history")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func saveToHistory(data []byte, ext string) {
	dir := historyDir()
	ts := time.Now().Format("20060102_150405")
	path := filepath.Join(dir, fmt.Sprintf("scan_%s.%s", ts, ext))
	_ = os.WriteFile(path, data, 0644)
}

func listHistory() error {
	dir := historyDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("No history found.")
		return nil
	}

	if len(entries) == 0 {
		fmt.Println("No history found. Run 'sysscope scan' to create snapshots.")
		return nil
	}

	fmt.Println()
	fmt.Printf("  📂 History (%d snapshots)\n", len(entries))
	fmt.Println("  " + strings.Repeat("─", 60))
	fmt.Printf("  %-8s %-22s %s\n", "#", "Date", "File")
	fmt.Println("  " + strings.Repeat("─", 60))

	i := 1
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		info, _ := e.Info()
		date := ""
		if info != nil {
			date = info.ModTime().Format("2006-01-02 15:04:05")
		}
		fmt.Printf("  %-8d %-22s %s\n", i, date, e.Name())
		i++
	}
	fmt.Println()
	return nil
}
