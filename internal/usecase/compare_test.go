package usecase_test

import (
	"testing"

	"sysscope/internal/domain"
	"sysscope/internal/usecase"
)

func TestCompare_DifferentReports(t *testing.T) {
	r1 := &domain.Report{
		Hostname: "pc1",
		CPU:      &domain.CPUInfo{Model: "Intel i7", PhysicalCores: 8},
		Memory:   &domain.MemoryInfo{TotalBytes: 16 << 30, UsagePercent: 50},
	}
	r2 := &domain.Report{
		Hostname: "pc2",
		CPU:      &domain.CPUInfo{Model: "AMD Ryzen 9", PhysicalCores: 16},
		Memory:   &domain.MemoryInfo{TotalBytes: 32 << 30, UsagePercent: 70},
	}

	uc := usecase.NewCompareUseCase()
	diffs := uc.Compare(r1, r2)

	if len(diffs) == 0 {
		t.Error("expected differences between reports")
	}

	found := false
	for _, d := range diffs {
		if d.Field == "hostname" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hostname difference to be detected")
	}
}

func TestCompare_IdenticalReports(t *testing.T) {
	r := &domain.Report{
		Hostname: "same-pc",
		CPU:      &domain.CPUInfo{Model: "Intel i5"},
	}

	uc := usecase.NewCompareUseCase()
	diffs := uc.Compare(r, r)

	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs for identical reports, got %d", len(diffs))
	}
}

func TestCompare_MissingField(t *testing.T) {
	r1 := &domain.Report{Hostname: "pc1"}
	r2 := &domain.Report{
		Hostname: "pc2",
		CPU:      &domain.CPUInfo{Model: "Intel i9"},
	}

	uc := usecase.NewCompareUseCase()
	diffs := uc.Compare(r1, r2)

	if len(diffs) < 1 {
		t.Error("expected at least 1 diff")
	}
}
