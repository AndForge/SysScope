package exporter

import (
	"bytes"
	"fmt"
	"html/template"

	"sysscope/internal/domain"
)

type HTMLExporter struct{}

func NewHTML() *HTMLExporter { return &HTMLExporter{} }
func (e *HTMLExporter) Extension() string { return "html" }

func (e *HTMLExporter) Export(r *domain.Report) ([]byte, error) {
	fm := template.FuncMap{
		"bytes":   fmtBytes,
		"pct":     func(v float64) string { return fmt.Sprintf("%.1f%%", v) },
		"pct0":    func(v float64) string { return fmt.Sprintf("%.0f%%", v) },
		"mhz":     func(v float64) string { return fmt.Sprintf("%.0f", v) },
		"gb":      func(b uint64) string { return fmt.Sprintf("%.1f GB", float64(b)/(1<<30)) },
		"sub":     func(a, b int) int { return a - b },
		"add":     func(a, b int) int { return a + b },
		"scoreColor": func(v, max float64) string {
			p := v / max * 100
			switch {
			case p >= 80: return "var(--green)"
			case p >= 60: return "var(--yellow)"
			case p >= 40: return "var(--orange)"
			default: return "var(--red)"
			}
		},
		"scoreBg": func(v, max float64) string {
			p := v / max * 100
			switch {
			case p >= 80: return "var(--green-bg)"
			case p >= 60: return "var(--yellow-bg)"
			case p >= 40: return "var(--orange-bg)"
			default: return "var(--red-bg)"
			}
		},
		"bar": func(pct float64) template.HTML {
			c := "var(--green)"
			if pct > 90 { c = "var(--red)" } else if pct > 70 { c = "var(--yellow)" } else if pct > 50 { c = "var(--orange)" }
			return template.HTML(fmt.Sprintf(`<div class="bar"><div class="bar-fill" style="width:%.1f%%;background:%s"></div></div>`, pct, c))
		},
		"badge": func(s string) template.HTML {
			c := "var(--green-bg)"; tc := "var(--green)"
			sl := ""
			switch s {
			case "running","Running","OK","Started","Active","loaded":
				sl = "● Active"
			case "stopped","Stopped","disabled","Disabled","dead","Failed":
				c = "var(--red-bg)"; tc = "var(--red)"; sl = "● " + s
			default:
				c = "var(--yellow-bg)"; tc = "var(--yellow)"; sl = "● " + s
			}
			return template.HTML(fmt.Sprintf(`<span class="badge" style="background:%s;color:%s">%s</span>`, c, tc, sl))
		},
		"yesno": func(b bool) template.HTML {
			if b { return template.HTML(`<span class="badge" style="background:var(--green-bg);color:var(--green)">● Yes</span>`) }
			return template.HTML(`<span class="badge" style="background:var(--red-bg);color:var(--red)">● No</span>`)
		},
		"dict": func(m map[string]string) []kv {
			var r []kv
			for k, v := range m { r = append(r, kv{k, v}) }
			return r
		},
	}
	t, err := template.New("r").Funcs(fm).Parse(htmlTmpl)
	if err != nil { return nil, err }
	var buf bytes.Buffer
	if err := t.Execute(&buf, r); err != nil { return nil, err }
	return buf.Bytes(), nil
}

type kv struct{ K, V string }

func fmtBytes(b uint64) string {
	const u = 1024
	if b < u { return fmt.Sprintf("%d B", b) }
	d, e := uint64(u), 0
	for n := b / u; n >= u; n /= u { d *= u; e++ }
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(d), "KMGTPE"[e])
}

const htmlTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>SysScope — {{.Hostname}}</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
:root{--bg:#0a0e1a;--card:#111827;--card2:#1a2235;--border:#1e293b;--border2:#2d3a52;--text:#e2e8f0;--muted:#64748b;--accent:#3b82f6;--green:#10b981;--green-bg:#10b98118;--yellow:#f59e0b;--yellow-bg:#f59e0b18;--orange:#f97316;--orange-bg:#f9731618;--red:#ef4444;--red-bg:#ef444418;--blue-bg:#3b82f618}
body{font-family:'Inter','Segoe UI',system-ui,-apple-system,sans-serif;background:var(--bg);color:var(--text);line-height:1.5;font-size:14px}
.layout{display:flex;min-height:100vh}
.sidebar{width:260px;background:var(--card);border-right:1px solid var(--border);position:fixed;top:0;left:0;bottom:0;overflow-y:auto;z-index:10;padding:1rem}
.sidebar h2{font-size:.85rem;color:var(--muted);text-transform:uppercase;letter-spacing:.05em;margin:1.2rem 0 .5rem;padding:0 .5rem}
.sidebar a{display:flex;align-items:center;gap:.5rem;padding:.45rem .75rem;border-radius:8px;color:var(--text);text-decoration:none;font-size:.82rem;transition:all .15s}
.sidebar a:hover{background:var(--card2);color:var(--accent)}
.sidebar .logo{font-size:1.1rem;font-weight:700;color:var(--accent);padding:.75rem;margin-bottom:.5rem;display:flex;align-items:center;gap:.5rem}
.main{margin-left:260px;flex:1;padding:1.5rem 2rem}
.topbar{display:flex;justify-content:space-between;align-items:center;margin-bottom:1.5rem;gap:1rem;flex-wrap:wrap}
.search{background:var(--card);border:1px solid var(--border);color:var(--text);padding:.6rem 1rem .6rem 2.5rem;border-radius:10px;width:320px;font-size:.85rem;transition:border-color .2s}
.search:focus{outline:none;border-color:var(--accent)}
.search-wrap{position:relative}
.search-wrap::before{content:"🔍";position:absolute;left:.8rem;top:50%;transform:translateY(-50%);font-size:.85rem}
.meta-bar{display:flex;gap:.5rem;flex-wrap:wrap;align-items:center}
.meta-tag{background:var(--card);border:1px solid var(--border);padding:.3rem .7rem;border-radius:8px;font-size:.8rem;color:var(--muted)}
.meta-tag b{color:var(--text)}

/* Score cards */
.scores{display:grid;grid-template-columns:1fr 1fr;gap:1rem;margin-bottom:1.5rem}
.score-card{background:var(--card);border:1px solid var(--border);border-radius:14px;padding:1.3rem;position:relative;overflow:hidden}
.score-card::before{content:"";position:absolute;top:0;left:0;right:0;height:3px}
.score-card.health::before{background:linear-gradient(90deg,var(--green),var(--accent))}
.score-card.security::before{background:linear-gradient(90deg,var(--accent),var(--orange))}
.score-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:.8rem}
.score-title{font-size:.85rem;color:var(--muted);font-weight:500}
.score-value{font-size:2.8rem;font-weight:800;line-height:1}
.score-max{font-size:1rem;color:var(--muted);font-weight:400}
.score-label{display:inline-block;padding:.2rem .6rem;border-radius:6px;font-size:.8rem;font-weight:600;margin-top:.3rem}
.score-reasons{margin-top:.8rem}
.score-reason{font-size:.8rem;color:var(--muted);padding:.25rem 0;border-bottom:1px solid var(--border)}
.score-reason:last-child{border:none}
.score-rec{font-size:.8rem;padding:.3rem .6rem;margin:.2rem 0;border-radius:6px;background:var(--blue-bg);color:var(--accent);border-left:3px solid var(--accent)}

/* Section */
.section{background:var(--card);border:1px solid var(--border);border-radius:12px;margin-bottom:1rem;overflow:hidden}
.section-header{display:flex;justify-content:space-between;align-items:center;padding:.8rem 1.2rem;cursor:pointer;user-select:none;transition:background .15s}
.section-header:hover{background:var(--card2)}
.section-title{font-size:.95rem;font-weight:600;display:flex;align-items:center;gap:.5rem}
.section-title .icon{font-size:1.1rem}
.section-arrow{color:var(--muted);transition:transform .2s;font-size:.8rem}
.section.open .section-arrow{transform:rotate(180deg)}
.section-body{display:none;border-top:1px solid var(--border)}
.section.open .section-body{display:block}
.section-content{padding:1rem 1.2rem}
.section-count{background:var(--accent);color:#fff;padding:.1rem .5rem;border-radius:10px;font-size:.7rem;font-weight:600}

/* Table */
table{width:100%;border-collapse:collapse}
th{text-align:left;padding:.5rem .6rem;font-size:.75rem;text-transform:uppercase;letter-spacing:.03em;color:var(--muted);border-bottom:2px solid var(--border);font-weight:600;position:sticky;top:0;background:var(--card)}
td{padding:.45rem .6rem;border-bottom:1px solid var(--border);font-size:.82rem;vertical-align:middle}
tr:hover td{background:var(--card2)}
.tbl-wrap{max-height:500px;overflow-y:auto;border-radius:8px}

/* Field row */
.field{display:flex;justify-content:space-between;padding:.35rem 0;border-bottom:1px solid var(--border)}
.field:last-child{border:none}
.field-label{color:var(--muted);font-size:.82rem}
.field-value{font-weight:500;font-size:.82rem;text-align:right;max-width:60%;word-break:break-word}

/* Bar */
.bar{background:var(--border);border-radius:4px;height:6px;width:120px;display:inline-block;vertical-align:middle;margin-left:.5rem}
.bar-fill{height:100%;border-radius:4px;transition:width .3s}

/* Badge */
.badge{display:inline-block;padding:.15rem .5rem;border-radius:6px;font-size:.75rem;font-weight:500}

/* Recommendations */
.recs{padding:1rem 1.2rem}
.rec-item{display:flex;gap:.6rem;padding:.5rem;margin:.3rem 0;border-radius:8px;font-size:.85rem;line-height:1.4}
.rec-item.info{background:var(--blue-bg);color:var(--accent)}
.rec-item.warn{background:var(--yellow-bg);color:var(--yellow)}
.rec-item.ok{background:var(--green-bg);color:var(--green)}
.rec-item.err{background:var(--red-bg);color:var(--red)}

/* Error block */
.errors{background:var(--red-bg);border:1px solid var(--red);border-radius:10px;padding:1rem;margin-bottom:1rem}
.err-item{font-size:.82rem;color:var(--red);padding:.2rem 0}
.err-item b{color:#fca5a5}

/* Grid */
.grid2{display:grid;grid-template-columns:1fr 1fr;gap:.8rem}
.grid3{display:grid;grid-template-columns:repeat(3,1fr);gap:.8rem}
@media(max-width:900px){.sidebar{display:none}.main{margin-left:0}.grid2,.grid3{grid-template-columns:1fr}.scores{grid-template-columns:1fr}}

/* Chip */
.chip{display:inline-block;background:var(--card2);border:1px solid var(--border2);padding:.15rem .5rem;border-radius:6px;font-size:.75rem;margin:.1rem}

/* Footer */
.footer{text-align:center;padding:2rem 0 1rem;color:var(--muted);font-size:.75rem}
</style>
</head>
<body>
<div class="layout">
<nav class="sidebar">
<div class="logo">🔍 SysScope</div>
<h2>Scores</h2>
<a href="#health">📊 Health Score</a>
<a href="#security">🔒 Security Score</a>
<h2>System</h2>
<a href="#os">💻 Operating System</a>
<a href="#cpu">⚡ CPU</a>
<a href="#memory">🧠 Memory</a>
<a href="#disks">💾 Disks</a>
<a href="#smart">💽 SMART</a>
<a href="#temps">🌡️ Temperatures</a>
<a href="#gpu">🎮 GPU</a>
<a href="#monitors">🖥️ Monitors</a>
<a href="#battery">🔋 Battery</a>
<a href="#network">🌐 Network</a>
<h2>Hardware</h2>
<a href="#bios">🔧 BIOS</a>
<a href="#mobo">📟 Motherboard</a>
<a href="#usb">🔗 USB</a>
<a href="#bluetooth">📶 Bluetooth</a>
<a href="#pci">🔧 PCI</a>
<h2>Software</h2>
<a href="#services">🛠️ Services</a>
<a href="#drivers">🔌 Drivers</a>
<a href="#startup">🚀 Startup</a>
<a href="#programs">📦 Programs</a>
<a href="#env">⚙️ Environment</a>
<a href="#winfeatures">🪟 Windows Features</a>
<h2>Other</h2>
<a href="#security-detail">🛡️ Security</a>
<a href="#processes">📊 Processes</a>
<a href="#recommendations">💡 Recommendations</a>
{{if .Errors}}<a href="#errors">⚠️ Errors ({{len .Errors}})</a>{{end}}
</nav>
<main class="main">
<div class="topbar">
<div>
<h1 style="font-size:1.3rem;margin-bottom:.2rem">System Report</h1>
<div class="meta-bar">
{{if .OS}}<span class="meta-tag">💻 <b>{{.OS.ProductName}}</b> {{.OS.Version}}</span>{{end}}
<span class="meta-tag">🏠 <b>{{.Hostname}}</b></span>
<span class="meta-tag">📅 {{.GeneratedAt.Format "2006-01-02 15:04"}}</span>
{{if .OS}}<span class="meta-tag">⏱️ {{.OS.Uptime}}</span>{{end}}
</div>
</div>
<div class="search-wrap"><input class="search" type="text" id="search" placeholder="Search in report..." oninput="filterSections(this.value)"></div>
</div>

<!-- Scores -->
<div class="scores">
<div class="score-card health" id="health">
<div class="score-header">
<div><div class="score-title">Health Score</div><div class="score-value" style="color:{{scoreColor .HealthScore.Value .HealthScore.MaxValue}}">{{printf "%.0f" .HealthScore.Value}}<span class="score-max">/{{printf "%.0f" .HealthScore.MaxValue}}</span></div><span class="score-label" style="background:{{scoreBg .HealthScore.Value .HealthScore.MaxValue}};color:{{scoreColor .HealthScore.Value .HealthScore.MaxValue}}">{{.HealthScore.Label}}</span></div>
</div>
{{if .HealthScore.Reasons}}<div class="score-reasons">{{range .HealthScore.Reasons}}<div class="score-reason">{{.}}</div>{{end}}</div>{{end}}
{{if .HealthScore.Recommendations}}{{range .HealthScore.Recommendations}}<div class="score-rec">💡 {{.}}</div>{{end}}{{end}}
</div>
<div class="score-card security" id="security">
<div class="score-header">
<div><div class="score-title">Security Score</div><div class="score-value" style="color:{{scoreColor .SecurityScore.Value .SecurityScore.MaxValue}}">{{printf "%.0f" .SecurityScore.Value}}<span class="score-max">/{{printf "%.0f" .SecurityScore.MaxValue}}</span></div><span class="score-label" style="background:{{scoreBg .SecurityScore.Value .SecurityScore.MaxValue}};color:{{scoreColor .SecurityScore.Value .SecurityScore.MaxValue}}">{{.SecurityScore.Label}}</span></div>
</div>
{{if .SecurityScore.Reasons}}<div class="score-reasons">{{range .SecurityScore.Reasons}}<div class="score-reason">{{.}}</div>{{end}}</div>{{end}}
{{if .SecurityScore.Recommendations}}{{range .SecurityScore.Recommendations}}<div class="score-rec">💡 {{.}}</div>{{end}}{{end}}
</div>
</div>

{{if .OS}}
<!-- OS -->
<div class="section open" id="os" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">💻</span>Operating System</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="grid2">
<div>
<div class="field"><span class="field-label">Product</span><span class="field-value">{{.OS.ProductName}}</span></div>
<div class="field"><span class="field-label">Edition</span><span class="field-value">{{.OS.Edition}}</span></div>
<div class="field"><span class="field-label">Version</span><span class="field-value">{{.OS.Version}}</span></div>
<div class="field"><span class="field-label">Build</span><span class="field-value">{{.OS.Build}}</span></div>
</div><div>
<div class="field"><span class="field-label">Architecture</span><span class="field-value">{{.OS.Architecture}}</span></div>
<div class="field"><span class="field-label">Kernel</span><span class="field-value">{{.OS.Kernel}}</span></div>
{{if .OS.InstallDate}}<div class="field"><span class="field-label">Install Date</span><span class="field-value">{{.OS.InstallDate}}</span></div>{{end}}
<div class="field"><span class="field-label">Uptime</span><span class="field-value">{{.OS.Uptime}}</span></div>
</div>
</div>
</div></div></div>
{{end}}

{{if .CPU}}
<!-- CPU -->
<div class="section open" id="cpu" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">⚡</span>CPU — {{.CPU.Model}}</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="grid2">
<div>
<div class="field"><span class="field-label">Model</span><span class="field-value">{{.CPU.Model}}</span></div>
<div class="field"><span class="field-label">Vendor</span><span class="field-value">{{.CPU.Vendor}}</span></div>
<div class="field"><span class="field-label">Family</span><span class="field-value">{{.CPU.Family}}</span></div>
<div class="field"><span class="field-label">Architecture</span><span class="field-value">{{.CPU.Architecture}}</span></div>
</div><div>
<div class="field"><span class="field-label">Cores / Threads</span><span class="field-value">{{.CPU.PhysicalCores}} / {{.CPU.LogicalCores}}</span></div>
<div class="field"><span class="field-label">Current / Max</span><span class="field-value">{{mhz .CPU.CurrentMHz}} / {{mhz .CPU.MaxMHz}} MHz</span></div>
<div class="field"><span class="field-label">Usage</span><span class="field-value">{{pct .CPU.UsagePercent}} {{bar .CPU.UsagePercent}}</span></div>
</div>
</div>
</div></div></div>
{{end}}

{{if .Memory}}
<!-- Memory -->
<div class="section open" id="memory" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🧠</span>Memory — {{bytes .Memory.TotalBytes}}</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="field"><span class="field-label">Total</span><span class="field-value">{{bytes .Memory.TotalBytes}}</span></div>
<div class="field"><span class="field-label">Used</span><span class="field-value">{{bytes .Memory.UsedBytes}} ({{pct .Memory.UsagePercent}})</span></div>
<div class="field"><span class="field-label">Free</span><span class="field-value">{{bytes .Memory.FreeBytes}}</span></div>
<div style="margin:.5rem 0">{{bar .Memory.UsagePercent}}</div>
{{if .Memory.Sticks}}
<h4 style="margin:.8rem 0 .4rem;font-size:.85rem;color:var(--accent)">RAM Sticks ({{len .Memory.Sticks}})</h4>
<div class="tbl-wrap"><table><tr><th>Bank</th><th>Size</th><th>Speed</th><th>Manufacturer</th><th>Part Number</th></tr>
{{range .Memory.Sticks}}<tr><td>{{.BankLabel}}</td><td>{{gb .SizeBytes}}</td><td>{{.SpeedMHz}} MHz</td><td>{{.Manufacturer}}</td><td><code style="font-size:.78rem">{{.PartNumber}}</code></td></tr>{{end}}
</table></div>
{{end}}
</div></div></div>
{{end}}

{{if .Disks}}
<!-- Disks -->
<div class="section open" id="disks" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">💾</span>Disks ({{len .Disks}})</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
{{range .Disks}}
<div style="border:1px solid var(--border);border-radius:10px;padding:.8rem;margin-bottom:.8rem">
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:.5rem">
<strong>{{.Model}}</strong>
<div style="display:flex;gap:.3rem"><span class="chip">{{.MediaType}}</span>{{if .SerialNumber}}<span class="chip">SN: {{.SerialNumber}}</span>{{end}}</div>
</div>
<div class="field"><span class="field-label">Total</span><span class="field-value">{{bytes .SizeBytes}}</span></div>
{{if .Partitions}}
<table style="margin-top:.5rem"><tr><th>Drive</th><th>FS</th><th>Size</th><th>Free</th><th>Usage</th></tr>
{{range .Partitions}}<tr><td><strong>{{.Letter}}</strong></td><td>{{.FileSystem}}</td><td>{{bytes .TotalBytes}}</td><td>{{bytes .FreeBytes}}</td><td>{{pct .UsagePercent}} {{bar .UsagePercent}}</td></tr>{{end}}
</table>
{{end}}
</div>
{{end}}
</div></div></div>
{{end}}

{{if .SMART}}
<div class="section" id="smart" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">💽</span>SMART ({{len .SMART}})</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
{{range .SMART}}
<div style="border:1px solid var(--border);border-radius:10px;padding:.8rem;margin-bottom:.6rem">
<strong>{{.Model}}</strong> {{badge .Health}}
<div class="grid3" style="margin-top:.5rem">
{{if gt .TemperatureC 0.0}}<div class="field"><span class="field-label">Temp</span><span class="field-value">{{printf "%.0f" .TemperatureC}}°C</span></div>{{end}}
{{if gt .PowerOnHours 0}}<div class="field"><span class="field-label">Power On</span><span class="field-value">{{.PowerOnHours}} hrs</span></div>{{end}}
{{if gt .PowerCycles 0}}<div class="field"><span class="field-label">Cycles</span><span class="field-value">{{.PowerCycles}}</span></div>{{end}}
{{if gt .RemainingLife 0.0}}<div class="field"><span class="field-label">Life</span><span class="field-value">{{pct0 .RemainingLife}}</span></div>{{end}}
</div>
</div>
{{end}}
</div></div></div>
{{end}}

{{if .Temperatures}}
<div class="section open" id="temps" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🌡️</span>Temperatures</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="grid3">
{{if gt .Temperatures.CPU 0.0}}<div style="text-align:center;padding:.8rem;border:1px solid var(--border);border-radius:10px"><div style="font-size:2rem;font-weight:700;color:{{if gt .Temperatures.CPU 85.0}}var(--red){{else if gt .Temperatures.CPU 70.0}}var(--yellow){{else}}var(--green){{end}}">{{printf "%.0f" .Temperatures.CPU}}°C</div><div style="font-size:.8rem;color:var(--muted)">CPU</div></div>{{end}}
{{if gt .Temperatures.GPU 0.0}}<div style="text-align:center;padding:.8rem;border:1px solid var(--border);border-radius:10px"><div style="font-size:2rem;font-weight:700;color:{{if gt .Temperatures.GPU 85.0}}var(--red){{else if gt .Temperatures.GPU 70.0}}var(--yellow){{else}}var(--green){{end}}">{{printf "%.0f" .Temperatures.GPU}}°C</div><div style="font-size:.8rem;color:var(--muted)">GPU</div></div>{{end}}
{{if gt .Temperatures.SSD 0.0}}<div style="text-align:center;padding:.8rem;border:1px solid var(--border);border-radius:10px"><div style="font-size:2rem;font-weight:700">{{printf "%.0f" .Temperatures.SSD}}°C</div><div style="font-size:.8rem;color:var(--muted)">SSD</div></div>{{end}}
{{if gt .Temperatures.Motherboard 0.0}}<div style="text-align:center;padding:.8rem;border:1px solid var(--border);border-radius:10px"><div style="font-size:2rem;font-weight:700">{{printf "%.0f" .Temperatures.Motherboard}}°C</div><div style="font-size:.8rem;color:var(--muted)">Motherboard</div></div>{{end}}
</div>
</div></div></div>
{{end}}

{{if .GPU}}
<div class="section open" id="gpu" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🎮</span>GPU ({{len .GPU}})</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
{{range .GPU}}
<div style="border:1px solid var(--border);border-radius:10px;padding:.8rem;margin-bottom:.6rem">
<div class="field"><span class="field-label">Name</span><span class="field-value">{{.Name}}</span></div>
<div class="field"><span class="field-label">Vendor</span><span class="field-value">{{.Vendor}}</span></div>
{{if gt .VRAMBytes 0}}<div class="field"><span class="field-label">VRAM</span><span class="field-value">{{bytes .VRAMBytes}}</span></div>{{end}}
<div class="field"><span class="field-label">Driver</span><span class="field-value"><code>{{.DriverVersion}}</code></span></div>
</div>
{{end}}
</div></div></div>
{{end}}

{{if .Monitors}}
<div class="section open" id="monitors" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🖥️</span>Monitors ({{len .Monitors}})</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
{{range .Monitors}}
<div style="border:1px solid var(--border);border-radius:10px;padding:.8rem;margin-bottom:.6rem">
<div class="field"><span class="field-label">Name</span><span class="field-value">{{.Name}}</span></div>
{{if .Manufacturer}}<div class="field"><span class="field-label">Manufacturer</span><span class="field-value">{{.Manufacturer}}</span></div>{{end}}
{{if .SerialNumber}}<div class="field"><span class="field-label">Serial</span><span class="field-value">{{.SerialNumber}}</span></div>{{end}}
{{if gt .ResolutionX 0}}<div class="field"><span class="field-label">Resolution</span><span class="field-value">{{.ResolutionX}} × {{.ResolutionY}}</span></div>{{end}}
{{if gt .RefreshRateHz 0}}<div class="field"><span class="field-label">Refresh Rate</span><span class="field-value">{{.RefreshRateHz}} Hz</span></div>{{end}}
{{if gt .DiagonalInch 0.0}}<div class="field"><span class="field-label">Diagonal</span><span class="field-value">{{printf "%.1f" .DiagonalInch}}"</span></div>{{end}}
{{if .HDR}}<div class="field"><span class="field-label">HDR</span><span class="field-value">{{yesno .HDR}}</span></div>{{end}}
{{if gt .ScalePercent 0}}<div class="field"><span class="field-label">Scale</span><span class="field-value">{{.ScalePercent}}%</span></div>{{end}}
</div>
{{end}}
</div></div></div>
{{end}}

{{if .Battery}}
<div class="section open" id="battery" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🔋</span>Battery</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="field"><span class="field-label">Charge</span><span class="field-value">{{pct0 .Battery.ChargePercent}} {{bar .Battery.ChargePercent}}</span></div>
<div class="field"><span class="field-label">Charging</span><span class="field-value">{{yesno .Battery.IsCharging}}</span></div>
{{if gt .Battery.HealthPercent 0.0}}<div class="field"><span class="field-label">Health</span><span class="field-value">{{pct0 .Battery.HealthPercent}}</span></div>{{end}}
{{if gt .Battery.WearLevelPercent 0.0}}<div class="field"><span class="field-label">Wear Level</span><span class="field-value">{{pct0 .Battery.WearLevelPercent}}</span></div>{{end}}
{{if gt .Battery.DesignCapacityWh 0.0}}<div class="field"><span class="field-label">Design Capacity</span><span class="field-value">{{printf "%.1f" .Battery.DesignCapacityWh}} Wh</span></div>{{end}}
{{if gt .Battery.FullChargeCapWh 0.0}}<div class="field"><span class="field-label">Full Charge</span><span class="field-value">{{printf "%.1f" .Battery.FullChargeCapWh}} Wh</span></div>{{end}}
{{if gt .Battery.CycleCount 0}}<div class="field"><span class="field-label">Cycle Count</span><span class="field-value">{{.Battery.CycleCount}}</span></div>{{end}}
{{if .Battery.RemainingTime}}<div class="field"><span class="field-label">Remaining</span><span class="field-value">{{.Battery.RemainingTime}}</span></div>{{end}}
</div></div></div>
{{end}}

{{if .Network}}
<div class="section open" id="network" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🌐</span>Network ({{len .Network}})</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
{{range .Network}}
<div style="border:1px solid var(--border);border-radius:10px;padding:.8rem;margin-bottom:.6rem">
<div style="display:flex;justify-content:space-between;margin-bottom:.4rem"><strong>{{.Name}}</strong>{{if .IsUp}}<span class="badge" style="background:var(--green-bg);color:var(--green)">● Up</span>{{else}}<span class="badge" style="background:var(--red-bg);color:var(--red)">● Down</span>{{end}}</div>
{{if .Description}}<div class="field"><span class="field-label">Description</span><span class="field-value">{{.Description}}</span></div>{{end}}
<div class="field"><span class="field-label">Type</span><span class="field-value"><span class="chip">{{.Type}}</span></span></div>
<div class="field"><span class="field-label">MAC</span><span class="field-value"><code>{{.MAC}}</code></span></div>
{{if .IPv4}}<div class="field"><span class="field-label">IPv4</span><span class="field-value">{{range .IPv4}}{{.}} {{end}}</span></div>{{end}}
{{if .IPv6}}<div class="field"><span class="field-label">IPv6</span><span class="field-value" style="font-size:.78rem">{{range .IPv6}}{{.}} {{end}}</span></div>{{end}}
{{if .DNS}}<div class="field"><span class="field-label">DNS</span><span class="field-value">{{range .DNS}}{{.}} {{end}}</span></div>{{end}}
{{if .Gateway}}<div class="field"><span class="field-label">Gateway</span><span class="field-value">{{range .Gateway}}{{.}} {{end}}</span></div>{{end}}
{{if .SpeedMbps}}<div class="field"><span class="field-label">Speed</span><span class="field-value">{{.SpeedMbps}} Mbps</span></div>{{end}}
<div class="field"><span class="field-label">DHCP</span><span class="field-value">{{yesno .DHCP}}</span></div>
</div>
{{end}}
</div></div></div>
{{end}}

{{if .BIOS}}
<div class="section" id="bios" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🔧</span>BIOS</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="field"><span class="field-label">Manufacturer</span><span class="field-value">{{.BIOS.Manufacturer}}</span></div>
<div class="field"><span class="field-label">Version</span><span class="field-value">{{.BIOS.Version}}</span></div>
<div class="field"><span class="field-label">Date</span><span class="field-value">{{.BIOS.ReleaseDate}}</span></div>
</div></div></div>
{{end}}

{{if .Motherboard}}
<div class="section" id="mobo" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">📟</span>Motherboard</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="field"><span class="field-label">Manufacturer</span><span class="field-value">{{.Motherboard.Manufacturer}}</span></div>
<div class="field"><span class="field-label">Model</span><span class="field-value">{{.Motherboard.Model}}</span></div>
{{if .Motherboard.SerialNumber}}<div class="field"><span class="field-label">Serial</span><span class="field-value">{{.Motherboard.SerialNumber}}</span></div>{{end}}
</div></div></div>
{{end}}

{{if .USB}}
<div class="section" id="usb" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🔗</span>USB Devices <span class="section-count">{{len .USB}}</span></div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="tbl-wrap"><table><tr><th>Device</th><th>Vendor ID</th><th>Product ID</th><th>Status</th></tr>
{{range .USB}}<tr><td>{{.Name}}</td><td><code>{{.VendorID}}</code></td><td><code>{{.ProductID}}</code></td><td>{{badge .Status}}</td></tr>{{end}}
</table></div>
</div></div></div>
{{end}}

{{if .Bluetooth}}
<div class="section" id="bluetooth" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">📶</span>Bluetooth</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
{{if .Bluetooth.Adapters}}<h4 style="font-size:.85rem;color:var(--accent);margin-bottom:.4rem">Adapters</h4>
{{range .Bluetooth.Adapters}}<div class="field"><span class="field-label">{{.Name}}</span><span class="field-value">{{badge .Status}}</span></div>{{end}}{{end}}
{{if .Bluetooth.Devices}}<h4 style="font-size:.85rem;color:var(--accent);margin:.6rem 0 .4rem">Devices</h4>
{{range .Bluetooth.Devices}}<div class="field"><span class="field-label">{{.Name}}</span><span class="field-value">{{if .Connected}}{{yesno true}}{{else}}{{yesno false}}{{end}}</span></div>{{end}}{{end}}
</div></div></div>
{{end}}

{{if .PCI}}
<div class="section" id="pci" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🔧</span>PCI Devices <span class="section-count">{{len .PCI}}</span></div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="tbl-wrap"><table><tr><th>Device</th><th>Class</th><th>Vendor</th><th>Device ID</th><th>Status</th></tr>
{{range .PCI}}<tr><td>{{.Name}}</td><td>{{.Description}}</td><td><code>{{.VendorID}}</code></td><td><code>{{.DeviceID}}</code></td><td>{{badge .Status}}</td></tr>{{end}}
</table></div>
</div></div></div>
{{end}}

{{if .Services}}
<div class="section" id="services" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🛠️</span>Services <span class="section-count">{{len .Services}}</span></div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="tbl-wrap"><table><tr><th>Name</th><th>Display Name</th><th>Status</th><th>Start Type</th></tr>
{{range .Services}}<tr><td><code style="font-size:.78rem">{{.Name}}</code></td><td>{{.DisplayName}}</td><td>{{badge .Status}}</td><td><span class="chip">{{.StartType}}</span></td></tr>{{end}}
</table></div>
</div></div></div>
{{end}}

{{if .Drivers}}
<div class="section" id="drivers" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🔌</span>Drivers <span class="section-count">{{len .Drivers}}</span></div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="tbl-wrap"><table><tr><th>Name</th><th>Version</th><th>Provider</th><th>Date</th><th>Status</th></tr>
{{range .Drivers}}<tr><td>{{.Name}}</td><td><code style="font-size:.78rem">{{.Version}}</code></td><td>{{.Provider}}</td><td>{{.Date}}</td><td>{{badge .Status}}</td></tr>{{end}}
</table></div>
</div></div></div>
{{end}}

{{if .Startup}}
<div class="section" id="startup" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🚀</span>Startup <span class="section-count">{{len .Startup}}</span></div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="tbl-wrap"><table><tr><th>Name</th><th>Source</th><th>Enabled</th><th>Command</th></tr>
{{range .Startup}}<tr><td>{{.Name}}</td><td><span class="chip">{{.Source}}</span></td><td>{{yesno .Enabled}}</td><td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap"><code style="font-size:.75rem">{{.Command}}</code></td></tr>{{end}}
</table></div>
</div></div></div>
{{end}}

{{if .Programs}}
<div class="section" id="programs" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">📦</span>Installed Programs <span class="section-count">{{len .Programs}}</span></div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="tbl-wrap"><table><tr><th>Name</th><th>Version</th><th>Publisher</th><th>Date</th><th>Size</th></tr>
{{range .Programs}}<tr><td>{{.Name}}</td><td><code style="font-size:.78rem">{{.Version}}</code></td><td>{{.Publisher}}</td><td>{{.InstallDate}}</td><td>{{if gt .EstimatedMB 0}}{{.EstimatedMB}} MB{{end}}</td></tr>{{end}}
</table></div>
</div></div></div>
{{end}}

{{if .EnvVars}}
<div class="section" id="env" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">⚙️</span>Environment Variables <span class="section-count">{{len .EnvVars}}</span></div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="tbl-wrap"><table><tr><th>Variable</th><th>Value</th></tr>
{{range $k,$v := .EnvVars}}<tr><td><code style="color:var(--accent)">{{$k}}</code></td><td style="word-break:break-all;font-size:.78rem">{{$v}}</td></tr>{{end}}
</table></div>
</div></div></div>
{{end}}

{{if .WinFeatures}}
<div class="section" id="winfeatures" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🪟</span>Windows Features</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
{{if .WinFeatures.HyperV}}<div class="field"><span class="field-label">Hyper-V</span><span class="field-value">{{badge .WinFeatures.HyperV}}</span></div>{{end}}
{{if .WinFeatures.WSL}}<div class="field"><span class="field-label">WSL</span><span class="field-value">{{badge .WinFeatures.WSL}}</span></div>{{end}}
{{if .WinFeatures.Sandbox}}<div class="field"><span class="field-label">Sandbox</span><span class="field-value">{{badge .WinFeatures.Sandbox}}</span></div>{{end}}
{{if .WinFeatures.VirtualMachinePlatform}}<div class="field"><span class="field-label">VM Platform</span><span class="field-value">{{badge .WinFeatures.VirtualMachinePlatform}}</span></div>{{end}}
{{if .WinFeatures.NetFramework}}<div class="field"><span class="field-label">.NET Framework</span><span class="field-value">{{badge .WinFeatures.NetFramework}}</span></div>{{end}}
{{if .WinFeatures.PowerShellVersion}}<div class="field"><span class="field-label">PowerShell</span><span class="field-value">{{.WinFeatures.PowerShellVersion}}</span></div>{{end}}
</div></div></div>
{{end}}

{{if .Security}}
<div class="section open" id="security-detail" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">🛡️</span>Security Details</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="grid2">
<div>
<div class="field"><span class="field-label">Antivirus</span><span class="field-value">{{yesno .Security.DefenderEnabled}}</span></div>
<div class="field"><span class="field-label">Real-time Protection</span><span class="field-value">{{yesno .Security.DefenderRealtime}}</span></div>
<div class="field"><span class="field-label">Controlled Folder</span><span class="field-value">{{yesno .Security.ControlledFolderAccess}}</span></div>
<div class="field"><span class="field-label">Firewall</span><span class="field-value">{{yesno .Security.FirewallEnabled}}</span></div>
<div class="field"><span class="field-label">Secure Boot</span><span class="field-value">{{yesno .Security.SecureBootEnabled}}</span></div>
<div class="field"><span class="field-label">TPM</span><span class="field-value">{{yesno .Security.TPMPresent}}{{if .Security.TPMVersion}} <span class="chip">{{.Security.TPMVersion}}</span>{{end}}</span></div>
</div><div>
<div class="field"><span class="field-label">Disk Encryption</span><span class="field-value">{{yesno .Security.BitLockerEnabled}}</span></div>
<div class="field"><span class="field-label">Credential Guard</span><span class="field-value">{{yesno .Security.CredentialGuard}}</span></div>
<div class="field"><span class="field-label">Core Isolation</span><span class="field-value">{{yesno .Security.CoreIsolation}}</span></div>
<div class="field"><span class="field-label">Memory Integrity</span><span class="field-value">{{yesno .Security.MemoryIntegrity}}</span></div>
<div class="field"><span class="field-label">SmartScreen</span><span class="field-value">{{yesno .Security.SmartScreen}}</span></div>
<div class="field"><span class="field-label">OS Updated</span><span class="field-value">{{yesno .Security.OSUpdateCurrent}}{{if .Security.LastUpdateDate}} <span class="chip">{{.Security.LastUpdateDate}}</span>{{end}}</span></div>
</div>
</div>
</div></div></div>
{{end}}

{{if .Processes}}
<div class="section" id="processes" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">📊</span>Processes <span class="section-count">{{len .Processes}}</span></div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="section-content">
<div class="tbl-wrap"><table><tr><th>PID</th><th>Name</th><th>CPU%</th><th>Mem%</th><th>Status</th></tr>
{{range .Processes}}<tr><td>{{.PID}}</td><td>{{.Name}}</td><td>{{printf "%.1f" .CPUPercent}}</td><td>{{printf "%.1f" .MemPercent}}</td><td><span class="chip">{{.Status}}</span></td></tr>{{end}}
</table></div>
</div></div></div>
{{end}}

{{if .Recommendations}}
<div class="section open" id="recommendations" data-section>
<div class="section-header" onclick="toggle(this)"><div class="section-title"><span class="icon">💡</span>Recommendations</div><span class="section-arrow">▼</span></div>
<div class="section-body"><div class="recs">
{{range .Recommendations}}<div class="rec-item info">{{.}}</div>{{end}}
</div></div></div>
{{end}}

{{if .Errors}}
<div class="errors" id="errors">
<strong>⚠️ Collection Errors ({{len .Errors}})</strong>
{{range .Errors}}<div class="err-item"><b>{{.Module}}</b>: {{.Message}}</div>{{end}}
</div>
{{end}}

<div class="footer">Generated by SysScope v0.3.0 — {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</div>
</main>
</div>

<script>
function toggle(el){el.parentElement.classList.toggle('open')}
function filterSections(q){
  q=q.toLowerCase();
  document.querySelectorAll('[data-section]').forEach(s=>{
    const t=s.textContent.toLowerCase();
    s.style.display=t.includes(q)?'':'none';
  });
}
// Auto-expand first sections
document.querySelectorAll('.section').forEach((s,i)=>{if(i<3)s.classList.add('open')});
</script>
</body>
</html>`
