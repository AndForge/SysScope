package usecase_test

import (
	"testing"

	"sysscope/internal/domain"
	"sysscope/internal/usecase"
)

func TestHealthScore_AllHealthy(t *testing.T) {
	r := &domain.Report{
		CPU: &domain.CPUInfo{UsagePercent: 30},
		Memory: &domain.MemoryInfo{
			TotalBytes:   16 << 30,
			FreeBytes:    8 << 30,
			UsagePercent: 50,
		},
		Disks: []domain.DiskInfo{{
			DeviceID: "/dev/sda",
			Partitions: []domain.Partition{
				{Letter: "/", UsagePercent: 50},
			},
		}},
	}
	scorer := usecase.NewScoreUseCase()
	score := scorer.ComputeHealth(r)

	if score.Value != 100 {
		t.Errorf("expected 100, got %.0f", score.Value)
	}
	if score.Label != "Excellent" {
		t.Errorf("expected Excellent, got %s", score.Label)
	}
	if len(score.Reasons) != 0 {
		t.Errorf("expected no reasons, got %d: %v", len(score.Reasons), score.Reasons)
	}
}

func TestHealthScore_HighCPU(t *testing.T) {
	r := &domain.Report{
		CPU: &domain.CPUInfo{UsagePercent: 96},
	}
	scorer := usecase.NewScoreUseCase()
	score := scorer.ComputeHealth(r)

	if score.Value >= 100 {
		t.Errorf("expected reduced score for high CPU, got %.0f", score.Value)
	}
	if len(score.Reasons) == 0 {
		t.Error("expected at least one reason")
	}
}

func TestHealthScore_LowFreeMemory(t *testing.T) {
	r := &domain.Report{
		Memory: &domain.MemoryInfo{
			TotalBytes:   8 << 30,
			FreeBytes:    1 << 30, // 1 GB
			UsagePercent: 87.5,
		},
	}
	scorer := usecase.NewScoreUseCase()
	score := scorer.ComputeHealth(r)

	if score.Value >= 95 {
		t.Errorf("expected reduced score for low free memory, got %.0f", score.Value)
	}
}

func TestHealthScore_FullDisk(t *testing.T) {
	r := &domain.Report{
		Disks: []domain.DiskInfo{{
			DeviceID: "/dev/sda",
			Partitions: []domain.Partition{
				{Letter: "/", UsagePercent: 97},
			},
		}},
	}
	scorer := usecase.NewScoreUseCase()
	score := scorer.ComputeHealth(r)

	if score.Value >= 90 {
		t.Errorf("expected reduced score for full disk, got %.0f", score.Value)
	}
}

func TestSecurityScore_AllSecure(t *testing.T) {
	r := &domain.Report{
		Security: &domain.SecurityInfo{
			DefenderEnabled:        true,
			DefenderRealtime:       true,
			FirewallEnabled:        true,
			SecureBootEnabled:      true,
			TPMPresent:             true,
			BitLockerEnabled:       true,
			OSUpdateCurrent:        true,
			ControlledFolderAccess: true,
			CoreIsolation:          true,
			MemoryIntegrity:        true,
			SmartScreen:            true,
			CredentialGuard:        true,
		},
	}
	scorer := usecase.NewScoreUseCase()
	score := scorer.ComputeSecurity(r)

	if score.Value != 100 {
		t.Errorf("expected 100, got %.0f", score.Value)
	}
}

func TestSecurityScore_NoSecurityInfo(t *testing.T) {
	r := &domain.Report{}
	scorer := usecase.NewScoreUseCase()
	score := scorer.ComputeSecurity(r)

	if score.Value != 0 {
		t.Errorf("expected 0, got %.0f", score.Value)
	}
	if score.Label != "Unknown" {
		t.Errorf("expected Unknown, got %s", score.Label)
	}
}

func TestSecurityScore_Insecure(t *testing.T) {
	r := &domain.Report{
		Security: &domain.SecurityInfo{
			DefenderEnabled:  false,
			FirewallEnabled:  false,
			BitLockerEnabled: false,
			SecureBootEnabled: false,
			TPMPresent:       false,
			OSUpdateCurrent:  false,
			ThreatsFound:     []string{"malware.exe"},
		},
	}
	scorer := usecase.NewScoreUseCase()
	score := scorer.ComputeSecurity(r)

	if score.Value > 15 {
		t.Errorf("expected very low score, got %.0f", score.Value)
	}
	if score.Label != "Critical" {
		t.Errorf("expected Critical, got %s", score.Label)
	}
}

func TestHealthScore_BatteryDegraded(t *testing.T) {
	r := &domain.Report{
		Battery: &domain.BatteryInfo{
			IsPresent:     true,
			HealthPercent: 60,
		},
	}
	scorer := usecase.NewScoreUseCase()
	score := scorer.ComputeHealth(r)

	if score.Value >= 95 {
		t.Errorf("expected reduced score for degraded battery, got %.0f", score.Value)
	}
}
