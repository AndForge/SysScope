package usecase

import (
	"fmt"

	"sysscope/internal/domain"
)

const maxScore = 100.0

// ScoreUseCase computes Health and Security scores for a report.
type ScoreUseCase struct{}

func NewScoreUseCase() *ScoreUseCase { return &ScoreUseCase{} }

// ComputeHealth evaluates the overall system health based on real data.
func (uc *ScoreUseCase) ComputeHealth(r *domain.Report) domain.Score {
	score := maxScore
	var reasons []string
	var recommendations []string

	// CPU usage
	if r.CPU != nil {
		if r.CPU.UsagePercent > 95 {
			score -= 15
			reasons = append(reasons, fmt.Sprintf("🔴 Загрузка CPU %.1f%% — система работает на пределе. Возможно замедление отклика.", r.CPU.UsagePercent))
			recommendations = append(recommendations, "Закройте ресурсоёмкие приложения или проверьте, не потребляет ли что-то чрезмерно много CPU.")
		} else if r.CPU.UsagePercent > 80 {
			score -= 8
			reasons = append(reasons, fmt.Sprintf("🟡 Загрузка CPU %.1f%% — высокая нагрузка.", r.CPU.UsagePercent))
		}
	}

	// Memory
	if r.Memory != nil {
		freeMB := float64(r.Memory.FreeBytes) / (1024 * 1024)
		if r.Memory.UsagePercent > 95 {
			score -= 25
			reasons = append(reasons, fmt.Sprintf("🔴 Использовано %.1f%% RAM (свободно %.0f МБ). Возможны зависания и ошибки.", r.Memory.UsagePercent, freeMB))
			recommendations = append(recommendations, "Закройте неиспользуемые программы или установите больше оперативной памяти.")
		} else if r.Memory.UsagePercent > 85 {
			score -= 12
			reasons = append(reasons, fmt.Sprintf("🟡 Использовано %.1f%% RAM (свободно %.0f МБ).", r.Memory.UsagePercent, freeMB))
		} else if freeMB < 2048 && r.Memory.TotalBytes > 0 {
			score -= 15
			reasons = append(reasons, fmt.Sprintf("🔴 Свободно менее 2 ГБ памяти (%.0f МБ). При нехватке RAM система использует медленный файл подкачки.", freeMB))
			recommendations = append(recommendations, "Рекомендуется увеличить объём оперативной памяти минимум до 8 ГБ.")
		}
	}

	// Disks
	for _, d := range r.Disks {
		for _, p := range d.Partitions {
			if p.UsagePercent > 95 {
				score -= 20
				reasons = append(reasons, fmt.Sprintf("🔴 Диск %s заполнен на %.1f%%. При заполнении выше 90%% производительность SSD может снижаться.", p.Letter, p.UsagePercent))
				recommendations = append(recommendations, fmt.Sprintf("Освободите место на диске %s. Удалите временные файлы, старые загрузки.", p.Letter))
			} else if p.UsagePercent > 85 {
				score -= 8
				reasons = append(reasons, fmt.Sprintf("🟡 Диск %s заполнен на %.1f%%.", p.Letter, p.UsagePercent))
			}
		}
	}

	// Temperatures
	if r.Temperatures != nil {
		if r.Temperatures.CPU > 90 {
			score -= 20
			reasons = append(reasons, fmt.Sprintf("🔴 Температура CPU %.0f°C — критическая! Возможен троттлинг и повреждение.", r.Temperatures.CPU))
			recommendations = append(recommendations, "Немедленно проверьте систему охлаждения. Очистите пыль, замените термопасту.")
		} else if r.Temperatures.CPU > 75 {
			score -= 10
			reasons = append(reasons, fmt.Sprintf("🟡 Температура CPU %.0f°C — выше нормы.", r.Temperatures.CPU))
			recommendations = append(recommendations, "Проверьте вентиляцию корпуса и работу кулеров.")
		}
		if r.Temperatures.GPU > 90 {
			score -= 15
			reasons = append(reasons, fmt.Sprintf("🔴 Температура GPU %.0f°C — критическая!", r.Temperatures.GPU))
			recommendations = append(recommendations, "Проверьте охлаждение видеокарты.")
		}
	}

	// SMART
	for _, s := range r.SMART {
		if s.Health == "Warning" || s.Health == "Failed" {
			score -= 25
			reasons = append(reasons, fmt.Sprintf("🔴 SMART диск %s: состояние %s. Высокий риск отказа.", s.Model, s.Health))
			recommendations = append(recommendations, fmt.Sprintf("Срочно создайте резервную копию данных с %s и замените диск.", s.Model))
		}
		if s.RemainingLife > 0 && s.RemainingLife < 10 {
			score -= 20
			reasons = append(reasons, fmt.Sprintf("🔴 Ресурс SSD %s: осталось %.0f%%. Диск скоро выйдет из строя.", s.Model, s.RemainingLife))
			recommendations = append(recommendations, "Запланируйте замену SSD в ближайшее время.")
		}
	}

	// Battery
	if r.Battery != nil && r.Battery.IsPresent && r.Battery.HealthPercent > 0 {
		if r.Battery.HealthPercent < 50 {
			score -= 20
			reasons = append(reasons, fmt.Sprintf("🔴 Состояние батареи: %.0f%% — требует замены.", r.Battery.HealthPercent))
			recommendations = append(recommendations, "Рекомендуется заменить батарею.")
		} else if r.Battery.HealthPercent < 80 {
			score -= 10
			reasons = append(reasons, fmt.Sprintf("🟡 Состояние батареи: %.0f%% — деградация.", r.Battery.HealthPercent))
		}
	}

	if score < 0 {
		score = 0
	}

	return domain.Score{
		Value:         score,
		MaxValue:      maxScore,
		Label:         scoreLabel(score),
		Reasons:       reasons,
		Recommendations: recommendations,
	}
}

// ComputeSecurity evaluates security posture based on real data.
func (uc *ScoreUseCase) ComputeSecurity(r *domain.Report) domain.Score {
	score := maxScore
	var reasons []string
	var recommendations []string

	if r.Security == nil {
		return domain.Score{
			Value:         0,
			MaxValue:      maxScore,
			Label:         "Unknown",
			Reasons:       []string{"⚠️  Информация о безопасности недоступна — не удалось собрать данные"},
			Recommendations: []string{"Запустите SysScope с правами администратора для полной диагностики безопасности."},
		}
	}

	s := r.Security

	if !s.DefenderEnabled {
		score -= 20
		reasons = append(reasons, "🔴 Антивирус / Windows Defender не включён. Система беззащитна перед вредоносным ПО.")
		recommendations = append(recommendations, "Включите Windows Defender или установите сторонний антивирус.")
	}
	if !s.DefenderRealtime {
		score -= 5
		reasons = append(reasons, "🟡 Защита в реальном времени не активна. Угрозы могут быть обнаружены с задержкой.")
		recommendations = append(recommendations, "Включите защиту в реальном времени в настройках Windows Defender.")
	}
	if !s.FirewallEnabled {
		score -= 20
		reasons = append(reasons, "🔴 Брандмауэр отключён. Система уязвима для сетевых атак.")
		recommendations = append(recommendations, "Включите брандмауэр Windows: Параметры → Конфиденциальность и безопасность → Безопасность Windows → Брандмауэр.")
	}
	if !s.SecureBootEnabled {
		score -= 10
		reasons = append(reasons, "🟡 Secure Boot отключён. Возможно запуск несанкционированного кода при загрузке.")
		recommendations = append(recommendations, "Включите Secure Boot в настройках BIOS/UEFI.")
	}
	if !s.TPMPresent {
		score -= 10
		reasons = append(reasons, "🟡 TPM не обнаружен. Невозможно использовать BitLocker и некоторые функции безопасности.")
		recommendations = append(recommendations, "Включите TPM в настройках BIOS/UEFI, если поддерживается.")
	}
	if !s.BitLockerEnabled {
		score -= 15
		reasons = append(reasons, "🔴 Шифрование диска не включено. Данные на диске не защищены при физическом доступе.")
		recommendations = append(recommendations, "Включите BitLocker для шифрования системного диска.")
	}
	if !s.OSUpdateCurrent {
		score -= 15
		reasons = append(reasons, fmt.Sprintf("🔴 Обновления безопасности не установлены (последнее: %s). Система уязвима.", s.LastUpdateDate))
		recommendations = append(recommendations, "Запустите Центр обновления Windows и установите все доступные обновления.")
	}
	if !s.ControlledFolderAccess {
		score -= 5
		reasons = append(reasons, "🟡 Контролируемый доступ к папкам отключён. ransomware может зашифровать файлы.")
		recommendations = append(recommendations, "Включите контролируемый доступ к папкам в Windows Defender.")
	}
	if !s.CoreIsolation {
		score -= 5
		reasons = append(reasons, "🟡 Core Isolation отключён. Снижена защита от эксплойтов ядра.")
		recommendations = append(recommendations, "Включите Core Isolation в Безопасность Windows → Изоляция ядра.")
	}
	if !s.MemoryIntegrity {
		score -= 5
		reasons = append(reasons, "🟡 Целостность памяти не включена. Возможны атаки на код ядра.")
		recommendations = append(recommendations, "Включите целостность памяти в Изоляция ядра.")
	}
	if !s.SmartScreen {
		score -= 5
		reasons = append(reasons, "🟡 SmartScreen отключён. Нет защиты от фишинговых сайтов и приложений.")
		recommendations = append(recommendations, "Включите SmartScreen в Параметры → Конфиденциальность и безопасность.")
	}
	if !s.CredentialGuard {
		score -= 5
		reasons = append(reasons, "🟡 Credential Guard не активен. Учётные данные могут быть похищены.")
		recommendations = append(recommendations, "Включите Credential Guard через групповые политики.")
	}
	if len(s.ThreatsFound) > 0 {
		penalty := 5 * float64(len(s.ThreatsFound))
		if penalty > 30 {
			penalty = 30
		}
		score -= penalty
		reasons = append(reasons, fmt.Sprintf("🔴 Обнаружено угроз: %d. Требуется немедленная проверка.", len(s.ThreatsFound)))
		recommendations = append(recommendations, "Запустите полное сканирование Windows Defender.")
	}

	if score < 0 {
		score = 0
	}

	return domain.Score{
		Value:         score,
		MaxValue:      maxScore,
		Label:         scoreLabel(score),
		Reasons:       reasons,
		Recommendations: recommendations,
	}
}

func scoreLabel(v float64) string {
	switch {
	case v >= 90:
		return "Excellent"
	case v >= 70:
		return "Good"
	case v >= 50:
		return "Fair"
	case v >= 30:
		return "Poor"
	default:
		return "Critical"
	}
}
