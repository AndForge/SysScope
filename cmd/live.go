package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	runtimeOS "runtime"
	"strings"
	"sync"
	"time"

	"sysscope/internal/adapter/collector"
	"sysscope/internal/domain"
	"sysscope/internal/usecase"

	gopsnet "github.com/shirou/gopsutil/v4/net"
)

// ─── Constants ───────────────────────────────────────────────────────────────

const (
	// LiveRefreshInterval is the interval between live data updates.
	LiveRefreshInterval = 5 * time.Second
	// LiveStaticRefreshInterval is the interval between static data updates.
	LiveStaticRefreshInterval = 10 * time.Minute
	// LiveScanTimeout is the timeout for each scan operation.
	LiveScanTimeout = 60 * time.Second
)

// ─── Live Cache ──────────────────────────────────────────────────────────────

// liveCache holds both static and dynamic data separately.
type liveCache struct {
	mu sync.RWMutex

	// Dynamic — updated every LiveRefreshInterval
	cpu         *domain.CPUInfo
	memory      *domain.MemoryInfo
	battery     *domain.BatteryInfo
	network     []domain.NetAdapter
	processes   []domain.ProcessInfo
	temps       *domain.TempInfo
	healthScore domain.Score
	secScore    domain.Score
	hostname    string
	osInfo      *domain.OSInfo
	// Network IO tracking
	netBytesRecvPrev uint64
	netBytesSentPrev uint64
	netRecvKBps      float64
	netSentKBps      float64
	netPrevTime      time.Time

	// Static — updated every LiveStaticRefreshInterval
	bios        *domain.BIOSInfo
	mobo        *domain.MoboInfo
	gpus        []domain.GPUInfo
	smart       []domain.SMARTInfo
	disks       []domain.DiskInfo
	monitors    []domain.MonitorInfo
	programs    []domain.ProgramInfo
	drivers     []domain.DriverInfo
	usb         []domain.USBDevice
	bluetooth   *domain.BluetoothInfo
	pci         []domain.PCIDevice
	winFeatures *domain.WinFeatures
	services    []domain.ServiceInfo
	startup     []domain.StartupItem
	security    *domain.SecurityInfo
	envVars     map[string]string
	errors      []domain.CollectError

	// Timestamps
	lastDynamic time.Time
	lastStatic  time.Time
}

var cache = &liveCache{}

// ─── Collectors ──────────────────────────────────────────────────────────────

func liveCollectDynamic() {
	ctx, cancel := context.WithTimeout(context.Background(), LiveScanTimeout)
	defer cancel()

	registry := domain.NewRegistry()
	// Only dynamic collectors
	registry.Register(collector.NewCPU())
	registry.Register(collector.NewMemory())
	registry.Register(collector.NewBattery())
	registry.Register(collector.NewNetwork())
	registry.Register(collector.NewProcesses())
	registry.Register(collector.NewTemperatures())
	registry.Register(collector.NewOS())

	type result struct {
		name string
		data any
		err  error
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results []result
	)

	for _, c := range registry.Collectors() {
		wg.Add(1)
		go func(col domain.Collector) {
			defer wg.Done()
			d, err := col.Collect(ctx)
			mu.Lock()
			results = append(results, result{col.Name(), d, err})
			mu.Unlock()
		}(c)
	}
	wg.Wait()

	// Compute scores from collected data
	r := &domain.Report{
		Hostname: cache.hostname,
	}
	var errs []domain.CollectError

	// Use cached static data for score computation
	cache.mu.RLock()
	r.Security = cache.security
	cache.mu.RUnlock()

	for _, res := range results {
		if res.err != nil {
			errs = append(errs, domain.CollectError{Module: res.name, Message: res.err.Error()})
			continue
		}
		switch res.name {
		case "cpu":
			if v, ok := res.data.(*domain.CPUInfo); ok {
				r.CPU = v
				cache.cpu = v
			}
		case "memory":
			if v, ok := res.data.(*domain.MemoryInfo); ok {
				r.Memory = v
				cache.memory = v
			}
		case "battery":
			if v, ok := res.data.(*domain.BatteryInfo); ok {
				r.Battery = v
				cache.battery = v
			}
		case "network":
			if v, ok := res.data.([]domain.NetAdapter); ok {
				r.Network = v
				cache.network = v
			}
		case "processes":
			if v, ok := res.data.([]domain.ProcessInfo); ok {
				r.Processes = v
				cache.processes = v
			}
		case "temperatures":
			if v, ok := res.data.(*domain.TempInfo); ok {
				r.Temperatures = v
				cache.temps = v
			}
		case "os":
			if v, ok := res.data.(*domain.OSInfo); ok {
				r.OS = v
				cache.osInfo = v
			}
		}
	}

	// Track network IO speed
	ioCounters, err := gopsnet.IOCounters(false)
	if err == nil && len(ioCounters) > 0 {
		io := ioCounters[0]
		now := time.Now()
		cache.mu.RLock()
		prevRecv := cache.netBytesRecvPrev
		prevSent := cache.netBytesSentPrev
		prevTime := cache.netPrevTime
		cache.mu.RUnlock()

		if !prevTime.IsZero() {
			elapsed := now.Sub(prevTime).Seconds()
			if elapsed > 0 {
				recvDelta := float64(io.BytesRecv-prevRecv) / elapsed / 1024 // KB/s
				sentDelta := float64(io.BytesSent-prevSent) / elapsed / 1024 // KB/s
				cache.mu.Lock()
				cache.netRecvKBps = recvDelta
				cache.netSentKBps = sentDelta
				cache.mu.Unlock()
			}
		}
		cache.mu.Lock()
		cache.netBytesRecvPrev = io.BytesRecv
		cache.netBytesSentPrev = io.BytesSent
		cache.netPrevTime = now
		cache.mu.Unlock()
	}

	scorer := usecase.NewScoreUseCase()
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.healthScore = scorer.ComputeHealth(r)
	cache.secScore = scorer.ComputeSecurity(r)
	cache.errors = errs
	cache.lastDynamic = time.Now()
}

func liveCollectStatic() {
	ctx, cancel := context.WithTimeout(context.Background(), LiveScanTimeout)
	defer cancel()

	registry := domain.NewRegistry()
	registry.Register(collector.NewBIOS())
	registry.Register(collector.NewMotherboard())
	registry.Register(collector.NewGPU())
	registry.Register(collector.NewSMART())
	registry.Register(collector.NewDisk())
	registry.Register(collector.NewMonitor())
	registry.Register(collector.NewPrograms())
	registry.Register(collector.NewDrivers())
	registry.Register(collector.NewUSB())
	registry.Register(collector.NewBluetooth())
	registry.Register(collector.NewPCI())
	registry.Register(collector.NewWinFeatures())
	registry.Register(collector.NewSecurity())
	registry.Register(collector.NewServices())
	registry.Register(collector.NewStartup())
	registry.Register(collector.NewEnvironment())

	type result struct {
		name string
		data any
		err  error
	}
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results []result
	)
	for _, c := range registry.Collectors() {
		wg.Add(1)
		go func(col domain.Collector) {
			defer wg.Done()
			d, err := col.Collect(ctx)
			mu.Lock()
			results = append(results, result{col.Name(), d, err})
			mu.Unlock()
		}(c)
	}
	wg.Wait()

	cache.mu.Lock()
	defer cache.mu.Unlock()

	for _, res := range results {
		if res.err != nil {
			continue
		}
		switch res.name {
		case "bios":
			if v, ok := res.data.(*domain.BIOSInfo); ok {
				cache.bios = v
			}
		case "motherboard":
			if v, ok := res.data.(*domain.MoboInfo); ok {
				cache.mobo = v
			}
		case "gpu":
			if v, ok := res.data.([]domain.GPUInfo); ok {
				cache.gpus = v
			}
		case "smart":
			if v, ok := res.data.([]domain.SMARTInfo); ok {
				cache.smart = v
			}
		case "disk":
			if v, ok := res.data.([]domain.DiskInfo); ok {
				cache.disks = v
			}
		case "monitor":
			if v, ok := res.data.([]domain.MonitorInfo); ok {
				cache.monitors = v
			}
		case "programs":
			if v, ok := res.data.([]domain.ProgramInfo); ok {
				cache.programs = v
			}
		case "drivers":
			if v, ok := res.data.([]domain.DriverInfo); ok {
				cache.drivers = v
			}
		case "usb":
			if v, ok := res.data.([]domain.USBDevice); ok {
				cache.usb = v
			}
		case "bluetooth":
			if v, ok := res.data.(*domain.BluetoothInfo); ok {
				cache.bluetooth = v
			}
		case "pci":
			if v, ok := res.data.([]domain.PCIDevice); ok {
				cache.pci = v
			}
		case "winfeatures":
			if v, ok := res.data.(*domain.WinFeatures); ok {
				cache.winFeatures = v
			}
		case "security":
			if v, ok := res.data.(*domain.SecurityInfo); ok {
				cache.security = v
			}
		case "services":
			if v, ok := res.data.([]domain.ServiceInfo); ok {
				cache.services = v
			}
		case "startup":
			if v, ok := res.data.([]domain.StartupItem); ok {
				cache.startup = v
			}
		case "environment":
			if v, ok := res.data.(map[string]string); ok {
				cache.envVars = v
			}
		}
	}
	cache.lastStatic = time.Now()
}

// ─── liveReport builds a full report from cache (no scan). ──────────────────

type liveReport struct {
	Hostname      string                `json:"hostname"`
	GeneratedAt   time.Time             `json:"generated_at"`
	OS            *domain.OSInfo        `json:"os,omitempty"`
	CPU           *domain.CPUInfo       `json:"cpu,omitempty"`
	Memory        *domain.MemoryInfo    `json:"memory,omitempty"`
	Disks         []domain.DiskInfo     `json:"disks,omitempty"`
	Network       []domain.NetAdapter   `json:"network,omitempty"`
	BIOS          *domain.BIOSInfo      `json:"bios,omitempty"`
	Motherboard   *domain.MoboInfo      `json:"motherboard,omitempty"`
	GPU           []domain.GPUInfo      `json:"gpu,omitempty"`
	Monitors      []domain.MonitorInfo  `json:"monitors,omitempty"`
	Battery       *domain.BatteryInfo   `json:"battery,omitempty"`
	Temperatures  *domain.TempInfo      `json:"temperatures,omitempty"`
	SMART         []domain.SMARTInfo    `json:"smart,omitempty"`
	Processes     []domain.ProcessInfo  `json:"processes,omitempty"`
	Services      []domain.ServiceInfo  `json:"services,omitempty"`
	Drivers       []domain.DriverInfo   `json:"drivers,omitempty"`
	Startup       []domain.StartupItem  `json:"startup,omitempty"`
	Programs      []domain.ProgramInfo  `json:"installed_programs,omitempty"`
	EnvVars       map[string]string     `json:"environment,omitempty"`
	USB           []domain.USBDevice    `json:"usb_devices,omitempty"`
	Bluetooth     *domain.BluetoothInfo `json:"bluetooth,omitempty"`
	PCI           []domain.PCIDevice    `json:"pci_devices,omitempty"`
	WinFeatures   *domain.WinFeatures   `json:"windows_features,omitempty"`
	Security      *domain.SecurityInfo  `json:"security,omitempty"`
	HealthScore   domain.Score          `json:"health_score"`
	SecurityScore domain.Score          `json:"security_score"`
	Errors        []domain.CollectError `json:"errors,omitempty"`
	LastDyn       string                `json:"last_dynamic"`
	LastStatic    string                `json:"last_static"`
	NetRecvKBps   float64               `json:"net_recv_kbps"`
	NetSentKBps   float64               `json:"net_sent_kbps"`
}

func buildLiveReport() liveReport {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return liveReport{
		Hostname:      cache.hostname,
		GeneratedAt:   time.Now(),
		OS:            cache.osInfo,
		CPU:           cache.cpu,
		Memory:        cache.memory,
		Disks:         cache.disks,
		Network:       cache.network,
		BIOS:          cache.bios,
		Motherboard:   cache.mobo,
		GPU:           cache.gpus,
		Monitors:      cache.monitors,
		Battery:       cache.battery,
		Temperatures:  cache.temps,
		SMART:         cache.smart,
		Processes:     cache.processes,
		Services:      cache.services,
		Drivers:       cache.drivers,
		Startup:       cache.startup,
		Programs:      cache.programs,
		EnvVars:       cache.envVars,
		USB:           cache.usb,
		Bluetooth:     cache.bluetooth,
		PCI:           cache.pci,
		WinFeatures:   cache.winFeatures,
		Security:      cache.security,
		HealthScore:   cache.healthScore,
		SecurityScore: cache.secScore,
		Errors:        cache.errors,
		LastDyn:       cache.lastDynamic.Format("15:04:05"),
		LastStatic:    cache.lastStatic.Format("15:04:05"),
		NetRecvKBps:   cache.netRecvKBps,
		NetSentKBps:   cache.netSentKBps,
	}
}

// ─── Server ──────────────────────────────────────────────────────────────────

func startLiveServer(port int) error {
	addr := fmt.Sprintf(":%d", port)

	// Initialize hostname & platform
	hostname, _ := ""
	if h, err := hostName(); err == nil {
		hostname = h
	}
	cache.mu.Lock()
	cache.hostname = hostname
	cache.mu.Unlock()

	// Background updater goroutine
	go func() {
		// Initial full scan
		fmt.Println("📊 Initial system scan...")
		liveCollectStatic()
		liveCollectDynamic()
		fmt.Println("✅ Initial scan complete")

		dynTicker := time.NewTicker(LiveRefreshInterval)
		staticTicker := time.NewTicker(LiveStaticRefreshInterval)
		defer dynTicker.Stop()
		defer staticTicker.Stop()

		for {
			select {
			case <-dynTicker.C:
				liveCollectDynamic()
			case <-staticTicker.C:
				liveCollectStatic()
				liveCollectDynamic() // also refresh dynamic after static
			}
		}
	}()

	// API endpoint — serves cached data, no scan
	http.HandleFunc("/api/report", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		report := buildLiveReport()
		data, err := json.Marshal(report)
		if err != nil {
			// Don't log — just return 500
			http.Error(w, `{"error":"marshal"}`, 500)
			return
		}

		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		// Write response — ignore write errors (client disconnect)
		_, _ = w.Write(data)
	})

	// Dashboard HTML
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = fmt.Fprint(w, liveHTML())
	})

	// Custom listener to handle errors silently
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	fmt.Printf("🔴 SysScope Live Monitor\n")
	fmt.Printf("📡 Dashboard: http://localhost%s\n", addr)
	fmt.Printf("📡 API:       http://localhost%s/api/report\n", addr)
	fmt.Printf("⏱️  Refresh:  every %v\n", LiveRefreshInterval)
	fmt.Printf("📊 Static:    every %v\n\n", LiveStaticRefreshInterval)
	fmt.Printf("Press Ctrl+C to stop.\n\n")

	// Auto-open browser
	go func() {
		time.Sleep(800 * time.Millisecond)
		url := fmt.Sprintf("http://localhost%s", addr)
		switch runtimeOS.GOOS {
		case "windows":
			_ = exec.Command("cmd", "/c", "start", url).Run()
		case "darwin":
			_ = exec.Command("open", url).Run()
		default:
			_ = exec.Command("xdg-open", url).Run()
		}
	}()

	// Serve with silent error handler
	srv := &http.Server{Handler: http.DefaultServeMux}
	err = srv.Serve(ln)
	// Ignore normal shutdown errors
	if err != nil && !isNormalShutdown(err) {
		return err
	}
	return nil
}

func hostName() (string, error) {
	return hostNameOS()
}

func hostNameOS() (string, error) {
	return os.Hostname()
}

func isNormalShutdown(err error) bool {
	if err == nil {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "Server closed") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "broken pipe")
}

// ─── HTML Dashboard ──────────────────────────────────────────────────────────

func liveHTML() string {
	return `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>SysScope Live</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4"><` + `/script>
<style>
*{box-sizing:border-box;margin:0;padding:0}
:root{--bg:#0a0e1a;--card:#111827;--border:#1e293b;--text:#e2e8f0;--muted:#64748b;--accent:#3b82f6;--green:#10b981;--yellow:#f59e0b;--red:#ef4444;--orange:#f97316}
body{font-family:'Inter','Segoe UI',system-ui,sans-serif;background:var(--bg);color:var(--text);min-height:100vh}
.header{background:var(--card);border-bottom:1px solid var(--border);padding:.8rem 1.5rem;display:flex;align-items:center;justify-content:space-between;position:sticky;top:0;z-index:100}
.header h1{font-size:1.1rem;display:flex;align-items:center;gap:.5rem}
.live-dot{width:10px;height:10px;border-radius:50%;animation:pulse 1.5s infinite}
.live-dot.ok{background:var(--green)}
.live-dot.err{background:var(--red)}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.3}}
.main{padding:1rem 1.5rem;max-width:1400px;margin:0 auto}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(260px,1fr));gap:.8rem;margin-bottom:1rem}
.card{background:var(--card);border:1px solid var(--border);border-radius:12px;padding:1rem;transition:border-color .2s}
.card:hover{border-color:var(--accent)}
.card-title{font-size:.72rem;text-transform:uppercase;letter-spacing:.05em;color:var(--muted);margin-bottom:.5rem}
.card-value{font-size:1.7rem;font-weight:700;line-height:1.2}
.card-sub{font-size:.78rem;color:var(--muted);margin-top:.15rem}
.chart-card{background:var(--card);border:1px solid var(--border);border-radius:12px;padding:1rem;margin-bottom:.8rem}
.chart-card h3{font-size:.82rem;color:var(--muted);margin-bottom:.5rem}
.chart-wrap{height:150px;position:relative}
.scores{display:grid;grid-template-columns:1fr 1fr;gap:.8rem;margin-bottom:1rem}
.score-card{background:var(--card);border:1px solid var(--border);border-radius:12px;padding:1.2rem;text-align:center}
.score-val{font-size:2.5rem;font-weight:800}
.score-lbl{font-size:.85rem;color:var(--muted);margin-top:.2rem}
.bar{background:var(--border);border-radius:4px;height:8px;margin-top:.5rem;overflow:hidden}
.bar-fill{height:100%;border-radius:4px;transition:width .5s ease}
.status-bar{display:flex;gap:1rem;align-items:center;font-size:.78rem}
.conn-ok{color:var(--green)}
.conn-err{color:var(--red)}
.conn-wait{color:var(--yellow)}
</style></head><body>
<div class="header"><h1>🔍 SysScope <span style="color:var(--accent)">Live</span> <span class="live-dot ok" id="dot"></span></h1>
<div class="status-bar"><span id="conn" class="conn-ok">🟢 Connected</span><span id="lu" style="color:var(--muted)">—</span><span id="rc" style="color:var(--muted)">—</span></div></div>
<div class="main">
<div class="scores">
<div class="score-card"><div class="card-title">📊 Health Score</div><div class="score-val" id="hv">—</div><div class="score-lbl" id="hl"></div><div class="bar"><div class="bar-fill" id="hb" style="width:0;background:var(--green)"></div></div></div>
<div class="score-card"><div class="card-title">🔒 Security Score</div><div class="score-val" id="sv">—</div><div class="score-lbl" id="sl"></div><div class="bar"><div class="bar-fill" id="sb" style="width:0;background:var(--accent)"></div></div></div>
</div>
<div class="grid" id="mg"></div>
<div class="grid" style="grid-template-columns:1fr 1fr">
<div class="chart-card"><h3>📈 CPU History</h3><div class="chart-wrap"><canvas id="cc"></canvas></div></div>
<div class="chart-card"><h3>📈 RAM History</h3><div class="chart-wrap"><canvas id="rc2"></canvas></div></div>
</div>
<div class="grid" style="grid-template-columns:1fr 1fr">
<div class="chart-card"><h3>📈 Disk Usage</h3><div class="chart-wrap"><canvas id="dc"></canvas></div></div>
<div class="chart-card"><h3>📈 Network</h3><div class="chart-wrap"><canvas id="nc"></canvas></div></div>
</div>
</div>
<script>
var REFRESH=5000,MP=60,rc=0,ch=[],rh=[],nh=[],lb=[],prev=null,failed=0;
for(var i=0;i<MP;i++){ch.push(0);rh.push(0);nh.push(0);lb.push('')}

function mc(id,cl,la){try{return new Chart(document.getElementById(id),{type:'line',data:{labels:lb.slice(),datasets:[{label:la,data:ch.slice(),borderColor:cl,backgroundColor:cl+'22',fill:true,tension:.3,pointRadius:0,borderWidth:2}]},options:{responsive:true,maintainAspectRatio:false,animation:{duration:300},plugins:{legend:{display:false}},scales:{y:{min:0,max:100,ticks:{color:'#64748b',font:{size:10}},grid:{color:'#1e293b'}},x:{display:false}}}})}catch(e){return null}}
function mcn(id,cl,la){try{return new Chart(document.getElementById(id),{type:'line',data:{labels:lb.slice(),datasets:[{label:la,data:nh.slice(),borderColor:cl,backgroundColor:cl+'22',fill:true,tension:.3,pointRadius:0,borderWidth:2}]},options:{responsive:true,maintainAspectRatio:false,animation:{duration:300},plugins:{legend:{display:false}},scales:{y:{min:0,ticks:{color:'#64748b',font:{size:10}},grid:{color:'#1e293b'}},x:{display:false}}}})}catch(e){return null}}
var cpuC=mc('cc','#3b82f6','CPU%'),ramC=mc('rc2','#10b981','RAM%'),diskC=mc('dc','#f59e0b','Disk%'),netC=mcn('nc','#f97316','KB/s');

function fb(b){if(!b||b===0)return'0 B';var u=1024,s=['B','KB','MB','GB','TB'],i=0;while(b>=u&&i<s.length-1){b/=u;i++}return b.toFixed(1)+' '+s[i]}
function bc(v){return v>90?'#ef4444':v>70?'#f59e0b':v>50?'#f97316':'#10b981'}
function sc(v){return v>=80?'#10b981':v>=60?'#f59e0b':v>=40?'#f97316':'#ef4444'}

function setConn(ok,msg){
  var d=document.getElementById('dot'),c=document.getElementById('conn');
  if(ok){d.className='live-dot ok';c.className='conn-ok';c.textContent='🟢 '+(msg||'Connected')}
  else{d.className='live-dot err';c.className='conn-err';c.textContent='🔴 '+(msg||'Disconnected')}
}

function upd(id,val){var el=document.getElementById(id);if(el&&el.textContent!==String(val))el.textContent=val}

async function fd(){
try{
var ctrl=new AbortController();
var timer=setTimeout(function(){ctrl.abort()},10000);
var r=await fetch('/api/report',{cache:'no-store',signal:ctrl.signal});
clearTimeout(timer);
if(!r.ok)throw new Error('HTTP '+r.status);
var d=await r.json();
if(!d)throw new Error('Empty');
rc++;failed=0;
setConn(true,'Connected');
upd('rc','Updates: '+rc);
upd('lu','Last: '+(d.last_dynamic||new Date().toLocaleTimeString()));

// Scores — update individual elements, no full redraw
if(d.health_score){
  var hv=Math.round(d.health_score.value||0);
  upd('hv',String(hv));
  document.getElementById('hv').style.color=sc(hv);
  upd('hl',d.health_score.label||'');
  document.getElementById('hb').style.width=hv+'%';
  document.getElementById('hb').style.background=sc(hv);
}
if(d.security_score){
  var sv=Math.round(d.security_score.value||0);
  upd('sv',String(sv));
  document.getElementById('sv').style.color=sc(sv);
  upd('sl',d.security_score.label||'');
  document.getElementById('sb').style.width=sv+'%';
  document.getElementById('sb').style.background=sc(sv);
}

// Metrics — only rebuild if data changed
var changed=!prev||JSON.stringify({
  cu:d.cpu?d.cpu.usage_percent:-1,
  mu:d.memory?d.memory.usage_percent:-1,
  du:d.disks?d.disks.length:-1,
  nu:d.network?d.network.length:-1,
  bu:d.battery?d.battery.charge_percent:-1,
  pu:d.processes?d.processes.length:-1,
  tu:d.temperatures?(d.temperatures.cpu_c||0):-1,
  nr:d.net_recv_kbps||0,
  ns:d.net_sent_kbps||0
})!==JSON.stringify({
  cu:prev.cpu?prev.cpu.usage_percent:-1,
  mu:prev.memory?prev.memory.usage_percent:-1,
  du:prev.disks?prev.disks.length:-1,
  nu:prev.network?prev.network.length:-1,
  bu:prev.battery?prev.battery.charge_percent:-1,
  pu:prev.processes?prev.processes.length:-1,
  tu:prev.temperatures?(prev.temperatures.cpu_c||0):-1,
  nr:prev.net_recv_kbps||0,
  ns:prev.net_sent_kbps||0
});

if(changed||!prev){
var h='';
if(d.os){h+='<div class="card"><div class="card-title">💻 OS</div><div class="card-sub">'+(d.os.product_name||'')+' '+(d.os.version||'')+'</div><div class="card-sub">Uptime: '+(d.os.uptime||'')+'</div></div>'}
if(d.cpu){var cu=d.cpu.usage_percent||0;h+='<div class="card"><div class="card-title">⚡ CPU</div><div class="card-value">'+cu.toFixed(1)+'%</div><div class="bar"><div class="bar-fill" style="width:'+cu+'%;background:'+bc(cu)+'"></div></div><div class="card-sub">'+(d.cpu.model||'')+'</div><div class="card-sub">'+(d.cpu.physical_cores||0)+' cores / '+(d.cpu.logical_cores||0)+' threads @ '+Math.round(d.cpu.max_mhz||0)+' MHz</div></div>';ch.push(cu);if(ch.length>MP)ch.shift()}
if(d.memory){var mu=d.memory.usage_percent||0;h+='<div class="card"><div class="card-title">🧠 RAM</div><div class="card-value">'+mu.toFixed(1)+'%</div><div class="bar"><div class="bar-fill" style="width:'+mu+'%;background:'+bc(mu)+'"></div></div><div class="card-sub">'+fb(d.memory.used_bytes||0)+' / '+fb(d.memory.total_bytes||0)+'</div></div>';rh.push(mu);if(rh.length>MP)rh.shift()}
if(d.gpu&&d.gpu.length>0){h+='<div class="card"><div class="card-title">🎮 GPU</div><div class="card-sub">'+(d.gpu[0].name||'')+'</div><div class="card-sub">'+(d.gpu[0].driver_version||'')+'</div></div>'}
if(d.disks&&d.disks.length>0){var maxU=0;for(var i=0;i<d.disks.length;i++){var dk=d.disks[i];if(dk.partitions)for(var j=0;j<dk.partitions.length;j++){if(dk.partitions[j].usage_percent>maxU)maxU=dk.partitions[j].usage_percent}}if(maxU>0)h+='<div class="card"><div class="card-title">💾 Disk</div><div class="card-value">'+maxU.toFixed(1)+'%</div><div class="bar"><div class="bar-fill" style="width:'+maxU+'%;background:'+bc(maxU)+'"></div></div><div class="card-sub">'+d.disks.length+' disk(s)</div></div>'}
if(d.network&&d.network.length>0){var up=0;for(var i=0;i<d.network.length;i++)if(d.network[i].is_up)up++;var nR=d.net_recv_kbps||0,nS=d.net_sent_kbps||0,nT=nR+nS;var nL=nT>1024?(nT/1024).toFixed(1)+' MB/s':nT.toFixed(1)+' KB/s';h+='<div class="card"><div class="card-title">🌐 Network</div><div class="card-value">'+nL+'</div><div class="bar"><div class="bar-fill" style="width:'+Math.min(nT/10,100)+'%;background:'+bc(Math.min(nT/10,100))+'"></div></div><div class="card-sub">↓ '+(nR>1024?(nR/1024).toFixed(1)+' MB/s':nR.toFixed(1)+' KB/s')+' ↑ '+(nS>1024?(nS/1024).toFixed(1)+' MB/s':nS.toFixed(1)+' KB/s')+'</div><div class="card-sub">'+d.network.length+' adapter(s), '+up+' up</div></div>';nh.push(nT);if(nh.length>MP)nh.shift()}
if(d.battery&&d.battery.is_present){h+='<div class="card"><div class="card-title">🔋 Battery</div><div class="card-value">'+Math.round(d.battery.charge_percent||0)+'%</div><div class="card-sub">'+(d.battery.is_charging?'⚡ Charging':'On battery')+'</div></div>'}
if(d.processes){h+='<div class="card"><div class="card-title">📊 Processes</div><div class="card-value">'+d.processes.length+'</div></div>'}
document.getElementById('mg').innerHTML=h;
}
prev=d;

// Charts — always update
if(cpuC){cpuC.data.datasets[0].data=ch.slice();cpuC.update('none')}
if(ramC){ramC.data.datasets[0].data=rh.slice();ramC.update('none')}
if(netC){netC.data.datasets[0].data=nh.slice();netC.update('none')}
if(diskC&&d.disks){var dl=[],dd=[],dc2=[];for(var i=0;i<d.disks.length;i++){var dk=d.disks[i];if(dk.partitions)for(var j=0;j<dk.partitions.length;j++){dl.push(dk.partitions[j].letter||'?');dd.push(dk.partitions[j].usage_percent);dc2.push(bc(dk.partitions[j].usage_percent))}}if(dl.length>0){diskC.data.labels=dl;diskC.data.datasets=[{label:'%',data:dd,backgroundColor:dc2,borderRadius:4}];diskC.update('none')}}

}catch(e){
failed++;
if(e.name==='AbortError')setConn(false,'Timeout');
else setConn(false,'Disconnected');
// Auto-reconnect with backoff
var delay=Math.min(REFRESH+failed*1000,30000);
setTimeout(fd,delay);
return;
}
setTimeout(fd,REFRESH);
}
fd();
<` + `/script></body></html>`
}
