package exporter_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"sysscope/internal/adapter/exporter"
	"sysscope/internal/domain"
)

func TestJSONExport(t *testing.T) {
	report := &domain.Report{
		ID:          "test-1",
		GeneratedAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Hostname:    "test-host",
		OS: &domain.OSInfo{
			Edition:      "Windows 11",
			Version:      "23H2",
			Build:        "22631",
			ProductName:  "Microsoft Windows 11 Pro",
			Architecture: "amd64",
			Uptime:       "2d 5h 30m",
		},
		CPU: &domain.CPUInfo{
			Model:         "Intel Core i7-12700K",
			Vendor:        "GenuineIntel",
			PhysicalCores: 12,
			LogicalCores:  20,
			UsagePercent:  42.5,
			CurrentMHz:    3600,
			MaxMHz:        5000,
		},
		Memory: &domain.MemoryInfo{
			TotalBytes:   32 << 30,
			UsedBytes:    16 << 30,
			UsagePercent: 50,
			Sticks: []domain.RAMStick{
				{BankLabel: "DIMM1", SizeBytes: 16 << 30, SpeedMHz: 3200, Manufacturer: "Samsung"},
			},
		},
		HealthScore:   domain.Score{Value: 85, MaxValue: 100, Label: "Good"},
		SecurityScore: domain.Score{Value: 70, MaxValue: 100, Label: "Good"},
	}

	exp := exporter.NewJSON()
	if exp.Extension() != "json" {
		t.Errorf("expected json, got %s", exp.Extension())
	}

	data, err := exp.Export(report)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	var parsed domain.Report
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}

	if parsed.Hostname != "test-host" {
		t.Errorf("hostname mismatch: %s", parsed.Hostname)
	}
	if parsed.CPU.Model != "Intel Core i7-12700K" {
		t.Errorf("CPU model mismatch: %s", parsed.CPU.Model)
	}
	if parsed.Memory.Sticks[0].Manufacturer != "Samsung" {
		t.Errorf("RAM manufacturer mismatch: %s", parsed.Memory.Sticks[0].Manufacturer)
	}
	if parsed.HealthScore.Value != 85 {
		t.Errorf("health score mismatch: %.0f", parsed.HealthScore.Value)
	}
}

func TestHTMLExport(t *testing.T) {
	report := &domain.Report{
		ID:          "test-html",
		GeneratedAt: time.Now(),
		Hostname:    "html-host",
		OS:          &domain.OSInfo{ProductName: "Windows 11 Pro", Version: "23H2", Architecture: "amd64"},
		CPU:         &domain.CPUInfo{Model: "AMD Ryzen 9 7950X", PhysicalCores: 16, LogicalCores: 32},
		HealthScore: domain.Score{Value: 90, MaxValue: 100, Label: "Excellent"},
		SecurityScore: domain.Score{
			Value:   60,
			MaxValue: 100,
			Label:   "Good",
			Reasons: []string{"Firewall is disabled"},
		},
	}

	exp := exporter.NewHTML()
	if exp.Extension() != "html" {
		t.Errorf("expected html, got %s", exp.Extension())
	}

	data, err := exp.Export(report)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	html := string(data)

	checks := []string{"html-host", "SysScope", "AMD Ryzen", "Windows 11", "90", "Firewall is disabled"}
	for _, c := range checks {
		if !strings.Contains(html, c) {
			t.Errorf("HTML missing: %s", c)
		}
	}
}
