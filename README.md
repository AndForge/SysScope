# 🔍 SysScope v0.2

**Кроссплатформенная утилита полной диагностики компьютера с реальными данными.**

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![Platforms](https://img.shields.io/badge/platforms-Windows%20|%20Linux%20|%20macOS-green)
![Tests](https://img.shields.io/badge/tests-13%20passing-brightgreen)

## ✨ Что собирает (реальные данные)

| Модуль | Источники Windows | Источники Linux | Источники macOS |
|--------|------------------|-----------------|-----------------|
| **ОС** | Registry, WMI | gopsutil, /etc/os-release | gopsutil, sysctl |
| **CPU** | gopsutil, WMI | gopsutil, /sys, /proc | gopsutil, sysctl |
| **RAM** | gopsutil, Win32_PhysicalMemory | gopsutil | gopsutil |
| **Диски** | WMI (Win32_DiskDrive) | gopsutil, /sys/block | gopsutil, diskutil |
| **Сеть** | Get-NetAdapter, NetIPConfiguration | ip link/addr, resolv.conf | networksetup, ifconfig, scutil |
| **BIOS** | WMI (Win32_BIOS) | /sys/class/dmi/id | system_profiler |
| **Мат. плата** | WMI (Win32_BaseBoard) | /sys/class/dmi/id | system_profiler |
| **GPU** | WMI (Win32_VideoController) | lspci, /sys/class/drm | system_profiler |
| **Мониторы** | .NET Screen class | xrandr, /sys/class/drm | system_profiler |
| **Батарея** | WMI (Win32_Battery) | /sys/class/power_supply | pmset, system_profiler |
| **Процессы** | gopsutil | gopsutil | gopsutil |
| **Безопасность** | Defender, Firewall, TPM, SecureBoot, BitLocker, Updates | ufw/iptables, SecureBoot, TPM, LUKS, apt | FileVault, Firewall, SIP, csrutil |

## 📐 Архитектура (Clean Architecture)

```
┌───────────────────────────────────────────────────────────┐
│  CLI (Cobra) — cmd/root.go                                │
├───────────────────────────────────────────────────────────┤
│  Use Cases — scan.go, compare.go, score.go                │
├───────────────────────────────────────────────────────────┤
│  Domain — report.go, interfaces.go, registry.go           │
├───────────────────────────────────────────────────────────┤
│  Adapters                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────┐  │
│  │ Collectors   │  │  Exporters   │  │  Platform      │  │
│  │ 12 модулей   │  │  JSON, HTML  │  │  linux|win|mac │  │
│  │ + platform   │  │              │  │  (build tags)  │  │
│  │  hooks       │  │              │  │                │  │
│  └──────────────┘  └──────────────┘  └────────────────┘  │
└───────────────────────────────────────────────────────────┘
```

### Структура каталогов

```
sysscope/
├── main.go
├── platform_linux.go          # blank-import для Linux
├── platform_windows.go        # blank-import для Windows
├── platform_darwin.go         # blank-import для macOS
├── cmd/root.go                # CLI: scan, compare, version
├── internal/
│   ├── domain/                # Ядро: модели + интерфейсы
│   │   ├── report.go          # 20+ типов данных
│   │   ├── interfaces.go      # Collector, Exporter, Scorer, Comparer
│   │   └── registry.go        # DefaultRegistry
│   ├── usecase/               # Бизнес-логика
│   │   ├── scan.go            # Параллельный сбор
│   │   ├── compare.go         # Diff двух отчётов
│   │   ├── score.go           # Health + Security Score
│   │   ├── score_test.go      # 8 тестов
│   │   └── compare_test.go    # 3 теста
│   └── adapter/
│       ├── collector/         # Абстракции + хуки для платформ
│       │   ├── collectors.go  # 12 коллекторов с Register-функциями
│       │   └── errors.go      # ErrNotImplemented
│       ├── exporter/          # JSON + HTML (тёмная тема)
│       │   ├── json.go
│       │   ├── html.go
│       │   └── exporter_test.go  # 2 теста
│       └── platform/          # Платформенные реализации
│           ├── linux/platform.go   # ~500 строк
│           ├── windows/platform.go # ~600 строк
│           └── darwin/platform.go  # ~550 строк
├── .github/workflows/ci.yml
├── go.mod
├── Makefile
└── README.md
```

## 🚀 Быстрый старт

```bash
# Клонировать и собрать
git clone https://github.com/your-org/sysscope.git
cd sysscope
go mod tidy
make build

# Или cross-compile для всех платформ
make build-all
```

### Использование

```bash
# JSON-отчёт (по умолчанию)
sysscope scan

# HTML с красивым UI
sysscope scan --format html

# Указать путь
sysscope scan -f json -o my_system.json

# Сравнить два отчёта
sysscope compare scan_20250101.json scan_20250601.json

# Версия
sysscope version
```

### Пример вывода

```
🔍 Scanning system...
✅ Report saved: sysscope_report_20250723_131500.json
📊 Health Score:   85/100 (Good)
🔒 Security Score: 5/100 (Critical)

⚠️  5 module(s) had errors:
   • bios: DMI BIOS info not accessible (may require root)
   • motherboard: DMI motherboard info not accessible (may require root)
   • gpu: GPU info not available: open /sys/class/drm: no such file or directory
   • battery: no battery found on this system
   • monitor: monitor info not available (no display server)
```

## 📊 Health Score (алгоритм)

| Фактор | Штраф | Условие |
|--------|-------|---------|
| CPU usage | −15 / −8 | >95% / >80% |
| RAM usage | −25 / −12 | >95% / >85% |
| RAM free < 2GB | −15 | < 2048 MB свободных |
| Диск заполнен | −20 / −8 | >95% / >85% |
| Батарея | −20 / −10 | <50% / <80% здоровье |

## 🛡️ Security Score (алгоритм)

| Фактор | Штраф |
|--------|-------|
| Defender / Antivirus выключен | −20 |
| Real-time protection неактивен | −5 |
| Firewall выключен | −20 |
| Secure Boot выключен | −10 |
| TPM не обнаружен | −10 |
| Disk Encryption выключен | −15 |
| Обновления не текущие | −15 |
| Каждый threat | −5 |

## 🧪 Тестирование

```bash
go test -v ./...          # 13 тестов
go vet ./...              # статический анализ
go build -race ./...      # race detector
```

## 📦 Зависимости

| Пакет | Назначение |
|-------|-----------|
| `github.com/shirou/gopsutil/v4` | CPU, RAM, Disk, Network, Host, Process |
| `github.com/spf13/cobra` | CLI framework |
| `golang.org/x/sys` | Windows API (syscall, unsafe) |

## ⚠️ Ограничения

- **Linux**: DMI-информация (BIOS, motherboard) требует root
- **Linux**: GPU требует `lspci` (pciutils)
- **Linux**: Мониторы требуют X11 (xrandr) или Wayland-совместимость
- **macOS**: Нет TPM — используется Apple T2/SIP вместо Secure Boot
- **Linux**: Defender — нет аналога, поле `false`
- **Windows**: PowerShell 5.1+ требуется для WMI-запросов
- **Контейнеры (Docker)**: GPU, мониторы, батарея недоступны

## 📄 Лицензия

MIT © 2025
