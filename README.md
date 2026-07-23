# 🔍 SysScope

> Cross-platform system diagnostics, monitoring and reporting tool written in Go.

[![CI](https://github.com/AndForge/SysScope/actions/workflows/ci.yml/badge.svg)](https://github.com/AndForge/SysScope/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

SysScope collects real hardware, software and security information from your computer and generates detailed reports in JSON or HTML. It supports Windows, Linux and macOS and includes live monitoring, report comparison and health/security scoring.

---

# ✨ Features

- ✅ Cross-platform (Windows, Linux, macOS)
- ✅ Hardware & software inventory
- ✅ Live monitoring dashboard
- ✅ Health Score calculation
- ✅ Security Score calculation
- ✅ JSON reports
- ✅ HTML reports
- ✅ Report comparison
- ✅ Scan history
- ✅ GitHub Actions CI

---

# 📸 Screenshots

Live Dashboard

> _(Add screenshot here later)_

---

# 🚀 Quick Start

## Clone

```bash
git clone https://github.com/AndForge/SysScope.git
cd SysScope
```

## Install dependencies

```bash
go mod tidy
```

## Build

```bash
go build
```

---

# 💻 Usage

Generate JSON report

```bash
sysscope scan
```

Generate HTML report

```bash
sysscope scan --format html
```

System summary

```bash
sysscope summary
```

Run diagnostics

```bash
sysscope doctor
```

Live monitoring dashboard

```bash
sysscope live
```

Show history

```bash
sysscope history
```

Compare reports

```bash
sysscope compare old.json new.json
```

Version

```bash
sysscope version
```

Windows executable example

```powershell
.\sysscope.exe scan
```

---

# 📋 Example Output

```text
🔍 Scanning system...
✅ Report saved: sysscope_report_20250723_131500.json

📊 Health Score:   85/100 (Good)
🔒 Security Score: 70/100 (Good)

⚠ 5 module(s) had errors:
 • BIOS information not accessible
 • Motherboard information not accessible
 • GPU information unavailable
 • Battery not found
 • Monitor information unavailable
```

---

# 📊 Collected Information

| Module | Windows | Linux | macOS |
|----------|----------|----------|----------|
| Operating System | ✅ | ✅ | ✅ |
| CPU | ✅ | ✅ | ✅ |
| Memory | ✅ | ✅ | ✅ |
| Disks | ✅ | ✅ | ✅ |
| Network | ✅ | ✅ | ✅ |
| BIOS | ✅ | ✅ | ✅ |
| Motherboard | ✅ | ✅ | ✅ |
| GPU | ✅ | ✅ | ✅ |
| Monitors | ✅ | ✅ | ✅ |
| Battery | ✅ | ✅ | ✅ |
| Processes | ✅ | ✅ | ✅ |
| Security | ✅ | ✅ | ✅ |

---

# 🏗 Architecture

```
CLI (Cobra)
        │
        ▼
Use Cases
        │
        ▼
Domain
        │
        ▼
Adapters
 ├── Collectors
 ├── Exporters
 └── Platform implementations
```

Project follows Clean Architecture.

---

# 📁 Project Structure

```text
sysscope/
├── cmd/
├── internal/
│   ├── adapter/
│   ├── domain/
│   └── usecase/
├── .github/
├── main.go
├── go.mod
└── README.md
```

---

# 📈 Health Score

The Health Score is calculated using several system metrics.

| Metric | Penalty |
|---------|----------|
| High CPU usage | up to -15 |
| High RAM usage | up to -25 |
| Low free RAM | -15 |
| Full disk | up to -20 |
| Battery health | up to -20 |

Maximum score: 100

---

# 🔒 Security Score

The Security Score evaluates system protection.

Checks include:

- Windows Defender / Antivirus
- Firewall
- Secure Boot
- TPM
- Disk Encryption
- Operating System Updates
- Threat Detection

Maximum score: 100

---

# 🧪 Testing

```bash
go test ./...
go vet ./...
go build ./...
```

GitHub Actions automatically runs:

- Build
- Unit Tests
- golangci-lint

for

- Windows
- Linux
- macOS

---

# 📦 Dependencies

| Package | Purpose |
|----------|----------|
| gopsutil | Hardware information |
| Cobra | CLI |
| x/sys | Platform APIs |

---

# ⚠ Limitations

Linux

- BIOS information may require root
- GPU detection may require pciutils
- Monitor information requires X11 or Wayland

macOS

- TPM is not available

Windows

- PowerShell 5.1+ recommended

Docker

- GPU, monitors and battery are usually unavailable

---<img width="1906" height="971" alt="2026-07-23_18-20-06" src="https://github.com/user-attachments/assets/3dd102f3-3d05-4ad0-bca3-7498f0da900e" />


---

# 📄 License

MIT License © 2025 AndForge
