package webserver

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/control_test_engine/ue"
	ueContext "OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control/mm_5gs"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/sender"
	"OmniRAN-Emulator/internal/templates"
	"OmniRAN-Emulator/lib/nas/nasMessage"
	"OmniRAN-Emulator/web"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// WebLogHook intercepts Logrus logs and broadcasts them to SSE clients.
type WebLogHook struct {
	mu      sync.Mutex
	clients map[chan string]bool
}

var (
	logHook      *WebLogHook
	runningState int32 // 0 = Idle, 1 = Running
	runningName  string
	runningMu    sync.Mutex
)

func init() {
	logHook = &WebLogHook{
		clients: make(map[chan string]bool),
	}
	logrus.AddHook(logHook)
}

func (h *WebLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *WebLogHook) Fire(entry *logrus.Entry) error {
	msg, err := entry.String()
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for clientChan := range h.clients {
		select {
		case clientChan <- msg:
		default:
			// Non-blocking write: skip slow clients
		}
	}
	return nil
}

func (h *WebLogHook) RegisterClient(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[ch] = true
}

func (h *WebLogHook) UnregisterClient(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, ch)
}

// StartServer starts the Go HTTP backend and serves the embedded Web UI.
func StartServer(host string, port int) error {
	// Clean up stale socket files on startup
	if files, err := filepath.Glob("/tmp/gnb_*.sock"); err == nil {
		for _, f := range files {
			_ = os.Remove(f)
		}
	}
	_ = os.Remove("/tmp/gnb.sock")

	// Initialize fleet config from disk
	if err := config.LoadFleet(); err != nil {
		logrus.Warnf("[WEB] Failed to load fleet config: %v (starting with empty fleet)", err)
	}

	mux := http.NewServeMux()

	// REST API Routes
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/config", handleConfig)
	mux.HandleFunc("/api/scenarios", handleScenariosList)
	mux.HandleFunc("/api/scenarios/run", handleScenarioRun)
	mux.HandleFunc("/api/scenarios/stop", handleScenarioStop)
	mux.HandleFunc("/api/ue/active", handleActiveUEs)
	mux.HandleFunc("/api/ue/action", handleUEAction)
	mux.HandleFunc("/api/ping", handlePingTest)
	mux.HandleFunc("/api/logs/stream", handleLogStream)

	// User Plane Traffic API Routes
	mux.HandleFunc("/api/ue/ping", handleUEPing)
	mux.HandleFunc("/api/ue/http", handleUEHttp)
	mux.HandleFunc("/api/ue/stream", handleUEStream)
	mux.HandleFunc("/api/ue/vonr/dial", handleUEVonrDial)
	mux.HandleFunc("/api/ue/vonr/hangup", handleUEVonrHangup)
	mux.HandleFunc("/api/ue/traffic/stats", handleUETrafficStats)

	// Fleet Manager API Routes
	mux.HandleFunc("/api/fleet/ue", handleFleetUEProfiles)
	mux.HandleFunc("/api/fleet/ue/", handleFleetUEProfileDelete)
	mux.HandleFunc("/api/fleet/gnb", handleFleetGNBProfiles)
	mux.HandleFunc("/api/fleet/gnb/", handleFleetGNBProfileDelete)
	mux.HandleFunc("/api/fleet/launch/ue", handleFleetLaunchUE)
	mux.HandleFunc("/api/fleet/launch/gnb", handleFleetLaunchGNB)
	mux.HandleFunc("/api/fleet/stop/ue", handleFleetStopUE)
	mux.HandleFunc("/api/fleet/stop/gnb/", handleFleetStopGNB)
	mux.HandleFunc("/api/fleet/running", handleFleetRunning)

	// Embedded static React files
	assetsFS, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		return fmt.Errorf("failed to load embedded web assets: %v", err)
	}

	fileServer := http.FileServer(http.FS(assetsFS))
	
	// Fallback handler to support React Client-side routing
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check if requesting API
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			http.NotFound(w, r)
			return
		}

		// Try to serve static file
		f, err := assetsFS.Open(r.URL.Path[1:])
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for React SPA Router
		indexFile, err := assetsFS.Open("index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusNotFound)
			return
		}
		defer indexFile.Close()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = ioCopy(w, indexFile)
	})

	addr := fmt.Sprintf("%s:%d", host, port)
	logrus.Infof("[WEB] Starting dashboard server at http://%s", addr)
	
	// Wrap with a basic CORS middleware to facilitate npm run dev local frontend development
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		mux.ServeHTTP(w, r)
	})

	return http.ListenAndServe(addr, corsHandler)
}

func ioCopy(w http.ResponseWriter, r fs.File) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		nr, er := r.Read(buf)
		if nr > 0 {
			nw, ew := w.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write")
				}
			}
			written += int64(nw)
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, fmt.Errorf("short write")
			}
		}
		if er != nil {
			if er.Error() == "EOF" {
				return written, nil
			}
			return written, er
		}
	}
}

// REST API Handlers

type StatusResponse struct {
	IsRunning     bool               `json:"isRunning"`
	RunningName   string             `json:"runningName"`
	Interfaces    []NetworkInterface `json:"interfaces"`
	GnbLinkState  string             `json:"gnbLinkState"`
	ConfigSummary ConfigSummary      `json:"configSummary"`
	RunningGnbs   []RunningGNBStatus `json:"runningGnbs,omitempty"`
	RunningUes    []UEStatus         `json:"runningUes,omitempty"`
}

type NetworkInterface struct {
	Name string   `json:"name"`
	IPs  []string `json:"ips"`
}

type ConfigSummary struct {
	UeIMSI    string `json:"ueImsi"`
	UeKey     string `json:"ueKey"`
	UeOPC     string `json:"ueOpc"`
	UeSlice   string `json:"ueSlice"`
	GnbControl string `json:"gnbControl"`
	AmfTarget  string `json:"amfTarget"`
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := StatusResponse{
		IsRunning:   atomic.LoadInt32(&runningState) == 1,
		RunningName: runningName,
	}

	// Fetch network interfaces (looking for uetun / tun / uesimtun)
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			var ips []string
			for _, addr := range addrs {
				ips = append(ips, addr.String())
			}
			resp.Interfaces = append(resp.Interfaces, NetworkInterface{
				Name: iface.Name,
				IPs:  ips,
			})
		}
	}

	cfg := config.Data

	// Check if local gNodeB socket/port is listening
	runningGNBsMu.RLock()
	hasFleetGNBs := len(runningGNBs) > 0
	runningGNBsMu.RUnlock()

	if hasFleetGNBs {
		resp.GnbLinkState = "socket_active"
		resp.RunningGnbs = GetRunningGNBs()
	} else {
		if cfg.GNodeB.LinkType == "tcp" {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.GNodeB.LinkPort), 100*time.Millisecond)
			if err == nil {
				resp.GnbLinkState = "listening"
				conn.Close()
			} else {
				resp.GnbLinkState = "offline"
			}
		} else {
			// UNIX socket check - actively dial to see if alive or stale
			conn, err := net.DialTimeout("unix", "/tmp/gnb.sock", 50*time.Millisecond)
			if err == nil {
				resp.GnbLinkState = "socket_active"
				conn.Close()
			} else {
				// Clean up stale socket file if it exists but connection was refused
				if _, statErr := os.Stat("/tmp/gnb.sock"); statErr == nil {
					_ = os.Remove("/tmp/gnb.sock")
				}
				resp.GnbLinkState = "offline"
			}
		}
	}

	runningUEs := make([]UEStatus, 0)
	ues := ueContext.GetAllActiveUEs()
	for _, u := range ues {
		resolveUeConnectionDetails(u)
		pduSessions := make([]PDUSessionStatus, 0)
		for _, sess := range u.PduSessions {
			pduSessions = append(pduSessions, PDUSessionStatus{
				ID:             sess.Id,
				UeIP:           u.GetIp(sess.Id),
				Dnn:            sess.Dnn,
				PduSessionType: sess.PduSessionType,
				Sst:            sess.Snssai.Sst,
				Sd:             sess.Snssai.Sd,
				State:          sess.State,
				StateDesc:      ueContext.GetStateSMDesc(sess.State),
			})
		}
		runningUEs = append(runningUEs, UEStatus{
			ID:               u.GetUeId(),
			Supi:             u.GetSupi(),
			StateMM:          u.GetStateMM(),
			StateMMDesc:      ueContext.GetStateMMDesc(u.GetStateMM()),
			StateSM:          u.GetStateSM(),
			StateSMDesc:      ueContext.GetStateSMDesc(u.GetStateSM()),
			RegistrationType: u.GetRegistrationType(),
			AmfUeNgapId:      u.GetAmfUeId(),
			GnbLinkType:      u.GetGnbLinkType(),
			GnbLinkPort:      u.GetGnbLinkPort(),
			GnbControlIp:     u.GetGnbControlIp(),
			GnbId:            u.GetGnbId(),
			GnbProfileName:   u.GetGnbProfileName(),
			PduSessions:      pduSessions,
		})
	}
	resp.RunningUes = runningUEs

	// Config summary
	resp.ConfigSummary = ConfigSummary{
		UeIMSI:    fmt.Sprintf("%s%s%s", cfg.Ue.Hplmn.Mcc, cfg.Ue.Hplmn.Mnc, cfg.Ue.Msin),
		UeKey:     cfg.Ue.Key,
		UeOPC:     cfg.Ue.Opc,
		UeSlice:   fmt.Sprintf("SST: %d, SD: %s", cfg.Ue.Snssai.Sst, cfg.Ue.Snssai.Sd),
		GnbControl: fmt.Sprintf("%s:%d", cfg.GNodeB.ControlIF.Ip, cfg.GNodeB.ControlIF.Port),
		AmfTarget:  fmt.Sprintf("%s:%d", cfg.AMF.Ip, cfg.AMF.Port),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(config.Data)
		return
	}

	if r.Method == http.MethodPost {
		var newCfg config.Config
		err := json.NewDecoder(r.Body).Decode(&newCfg)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}

		// Validate configuration parameter formats and types
		if err := newCfg.Validate(); err != nil {
			http.Error(w, fmt.Sprintf("Validation failed: %v", err), http.StatusBadRequest)
			return
		}

		// Update global config
		config.Data = newCfg

		// Save back to YAML file
		yamlData, err := yaml.Marshal(&newCfg)
		if err != nil {
			http.Error(w, fmt.Sprintf("YAML marshal error: %v", err), http.StatusInternalServerError)
			return
		}

		// Save to standard config location
		err = os.WriteFile("config/config.yml", yamlData, 0644)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to write config file: %v", err), http.StatusInternalServerError)
			return
		}

		logrus.Info("[WEB] Configuration updated dynamically from dashboard UI")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

type ScenarioItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func handleScenariosList(w http.ResponseWriter, r *http.Request) {
	scenarios := []ScenarioItem{
		{"interactive-ue", "Interactive UE Control Session", "Registers a UE and keeps it active/running, allowing dynamic operations via the UE Controller card"},
		{"periodic-reg", "Periodic Registration Update", "Registers a UE, simulates T3512 expiration, and triggers a Periodic Registration Update"},
		{"mobility-reg", "Mobility Registration Update (TAU)", "Registers a UE, simulates cell crossing, and triggers a Mobility Tracking Area Update (TAU)"},
		{"emergency-reg", "Emergency Registration", "Triggers unauthenticated Emergency Registration to the core network"},
		{"handover", "N2 Handover (Path Switch)", "Simulates cell change by executing a Path Switch Request between gNodeBs"},
		{"xn-handover", "Xn Handover (Inter-gNB)", "Starts two GNodeBs (Source and Target), registers a UE, establishes a PDU session, and performs Xn-based handover between GNodeBs"},
		{"pdu-lifecycle", "PDU Session Lifecycle", "Registers a UE, establishes a PDU session, releases the PDU session, and validates state transitions back to INACTIVE"},
		{"full-lifecycle", "Full UE Lifecycle", "Executes full sequence: Attach → PDU Active → CM-IDLE → Service Request → Detach"},
		{"deregister", "UE-initiated Deregistration", "Registers a UE and performs a clean power-off Deregistration Request"},
		{"load-test", "Multi-UE Load Endurance Test", "Stress tests the AMF by attaching multiple simulated UEs sequentially in a queue"},
		{"amf-load-loop", "AMF Load Loop (Stress Test)", "Periodically generates heavy registration requests to evaluate AMF throughput under stress"},
		{"ue-latency-interval", "UE Registration Latency", "Evaluates and measures the average registration latency for a queue of UEs"},
		{"amf-availability", "AMF Core Uptime Availability", "Performs reachability checks to evaluate the core uptime over a specified interval"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(scenarios)
}

type RunRequest struct {
	Scenario      string `json:"scenario"`
	TargetGnbIP   string `json:"targetGnbIp"`
	TargetGnbPort int    `json:"targetGnbPort"`
	Delay         int    `json:"delay"`
	IdleSeconds   int    `json:"idleSeconds"`
	UeCount       int    `json:"ueCount"`
	Requests      int    `json:"requests"`
	Duration      int    `json:"duration"`
	UeOnly        bool   `json:"ueOnly"`
}

func handleScenarioRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !atomic.CompareAndSwapInt32(&runningState, 0, 1) {
		http.Error(w, "Another scenario is already executing", http.StatusConflict)
		return
	}

	var req RunRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	runningName = req.Scenario
	if runningName == "" {
		runningName = "Default Scenario"
	}

	// Run scenario in background
	go func() {
		logrus.Infof("[WEB][SCENARIO] Running %s...", runningName)
		
		// Clean up socket resources before starting
		_ = os.Remove("/tmp/gnb.sock")
		_ = os.Remove("/tmp/ue1.sock")

		defer func() {
			atomic.StoreInt32(&runningState, 0)
			runningName = ""
			logrus.Info("[WEB][SCENARIO] Execution finished.")
		}()

		switch req.Scenario {
		case "interactive-ue":
			templates.ScenarioInteractiveUE(func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "periodic-reg":
			templates.ScenarioPeriodicRegistration(func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "mobility-reg":
			templates.ScenarioMobilityRegistration(func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "emergency-reg":
			templates.ScenarioEmergencyRegistration(func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "handover":
			ip := req.TargetGnbIP
			if ip == "" {
				ip = "127.0.0.1"
			}
			port := req.TargetGnbPort
			if port == 0 {
				port = 9489
			}
			delay := req.Delay
			if delay == 0 {
				delay = 5
			}
			templates.ScenarioHandover(ip, port, delay, func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "xn-handover":
			templates.ScenarioXnHandover(func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "pdu-lifecycle":
			templates.ScenarioPduLifecycle(func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "full-lifecycle":
			idle := req.IdleSeconds
			if idle == 0 {
				idle = 5
			}
			templates.ScenarioFullLifecycle(idle, func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "deregister":
			templates.ScenarioDeregistration(func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "load-test":
			ueCount := req.UeCount
			if ueCount == 0 {
				ueCount = 5
			}
			templates.TestMultiUesInQueue(ueCount, req.UeOnly)
		case "amf-load-loop":
			reqs := req.Requests
			if reqs == 0 {
				reqs = 10
			}
			dur := req.Duration
			if dur == 0 {
				dur = 10
			}
			templates.TestRqsLoop(reqs, dur)
		case "ue-latency-interval":
			reqs := req.Requests
			if reqs == 0 {
				reqs = 10
			}
			templates.TestUesLatencyInInterval(reqs)
		case "amf-availability":
			dur := req.Duration
			if dur == 0 {
				dur = 10
			}
			templates.TestAvailability(dur)
		default:
			logrus.Warnf("[WEB] Unknown scenario: %s", req.Scenario)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"started"}`))
}

type PingRequest struct {
	Host string `json:"host"`
}

type PingResponse struct {
	Output  string `json:"output"`
	Success bool   `json:"success"`
}

func handlePingTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PingRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	host := req.Host
	if host == "" {
		host = config.Data.AMF.Ip
	}

	w.Header().Set("Content-Type", "application/json")
	
	// Perform ping check (ping -c 3 <host>)
	cmd := exec.Command("ping", "-c", "3", "-W", "2", host)
	out, err := cmd.CombinedOutput()
	
	resp := PingResponse{
		Output:  string(out),
		Success: err == nil,
	}

	if err != nil && len(resp.Output) == 0 {
		resp.Output = fmt.Sprintf("Ping execution failed: %v", err)
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func handleLogStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	logChan := make(chan string, 100)
	logHook.RegisterClient(logChan)
	defer logHook.UnregisterClient(logChan)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Initial message
	_, _ = fmt.Fprintf(w, "data: log_stream_started\n\n")
	flusher.Flush()

	for {
		select {
		case msg := <-logChan:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// Structs for Active UE Status
type UEStatus struct {
	ID               uint8              `json:"id"`
	Supi             string             `json:"supi"`
	StateMM          int                `json:"stateMm"`
	StateMMDesc      string             `json:"stateMmDesc"`
	StateSM          int                `json:"stateSm"`
	StateSMDesc      string             `json:"stateSmDesc"`
	RegistrationType uint8              `json:"registrationType"`
	AmfUeNgapId      int64              `json:"amfUeNgapId"`
	GnbLinkType      string             `json:"gnbLinkType"`
	GnbLinkPort      int                `json:"gnbLinkPort"`
	GnbControlIp     string             `json:"gnbControlIp"`
	GnbId            string             `json:"gnbId"`
	GnbProfileName   string             `json:"gnbProfileName"`
	PduSessions      []PDUSessionStatus `json:"pduSessions"`
}

type PDUSessionStatus struct {
	ID             uint8  `json:"id"`
	UeIP           string `json:"ueIp"`
	Dnn            string `json:"dnn"`
	PduSessionType string `json:"pduSessionType"`
	Sst            int32  `json:"sst"`
	Sd             string `json:"sd"`
	State          int    `json:"state"`
	StateDesc      string `json:"stateDesc"`
}

type ActionRequest struct {
	UeId                uint8  `json:"ueId"`
	Action              string `json:"action"`
	PduSessionId        uint8  `json:"pduSessionId"`
	Dnn                 string `json:"dnn"`
	Sst                 int32  `json:"sst"`
	Sd                  string `json:"sd"`
	SessionType         string `json:"sessionType"`
	TargetGnbIP         string `json:"targetGnbIp"`
	TargetGnbPort       int    `json:"targetGnbPort"`
	TargetGnbLinkType   string `json:"targetGnbLinkType"`
	TargetGnbSocketPath string `json:"targetGnbSocketPath"`
	TargetGnbId         string `json:"targetGnbId"`
	TargetGnbName       string `json:"targetGnbName"`
}

func handleScenarioStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if atomic.LoadInt32(&runningState) == 0 {
		http.Error(w, "No scenario is currently running", http.StatusBadRequest)
		return
	}

	logrus.Info("[WEB] Stop request received. Terminating running scenario...")
	atomic.StoreInt32(&runningState, 0)

	// Clean terminate all active UEs
	ues := ueContext.GetAllActiveUEs()
	for _, u := range ues {
		u.Terminate()
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"success"}`))
}

func handleActiveUEs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ues := ueContext.GetAllActiveUEs()
	resp := make([]UEStatus, 0)

	for _, u := range ues {
		resolveUeConnectionDetails(u)
		pduSessions := make([]PDUSessionStatus, 0)
		for _, sess := range u.PduSessions {
			pduSessions = append(pduSessions, PDUSessionStatus{
				ID:             sess.Id,
				UeIP:           u.GetIp(sess.Id),
				Dnn:            sess.Dnn,
				PduSessionType: sess.PduSessionType,
				Sst:            sess.Snssai.Sst,
				Sd:             sess.Snssai.Sd,
				State:          sess.State,
				StateDesc:      ueContext.GetStateSMDesc(sess.State),
			})
		}

		resp = append(resp, UEStatus{
			ID:               u.GetUeId(),
			Supi:             u.GetSupi(),
			StateMM:          u.GetStateMM(),
			StateMMDesc:      ueContext.GetStateMMDesc(u.GetStateMM()),
			StateSM:          u.GetStateSM(),
			StateSMDesc:      ueContext.GetStateSMDesc(u.GetStateSM()),
			RegistrationType: u.GetRegistrationType(),
			AmfUeNgapId:      u.GetAmfUeId(),
			GnbLinkType:      u.GetGnbLinkType(),
			GnbLinkPort:      u.GetGnbLinkPort(),
			GnbControlIp:     u.GetGnbControlIp(),
			GnbId:            u.GetGnbId(),
			GnbProfileName:   u.GetGnbProfileName(),
			PduSessions:      pduSessions,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func handleUEAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ActionRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	u := ueContext.GetActiveUE(req.UeId)
	if u == nil {
		http.Error(w, fmt.Sprintf("UE with ID %d is not active", req.UeId), http.StatusNotFound)
		return
	}

	logrus.Infof("[WEB][ACTION] Executing %s on UE %d...", req.Action, req.UeId)

	switch req.Action {
	case "service-request":
		u.SetStateMM_MM5G_SERVICE_REQ_INIT()
		svcReq, err := mm_5gs.ServiceRequest(u, nasMessage.ServiceTypeMobileTerminatedServices)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to build Service Request: %v", err), http.StatusInternalServerError)
			return
		}
		sender.SendToGnb(u, svcReq)
		logrus.Info("[WEB][ACTION] Service Request sent successfully")

	case "deregister-normal":
		deregReq, err := mm_5gs.DeregistrationRequest(u, false)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to build Deregistration Request: %v", err), http.StatusInternalServerError)
			return
		}
		sender.SendToGnb(u, deregReq)
		logrus.Info("[WEB][ACTION] Clean Deregistration Request sent successfully")
		time.Sleep(1 * time.Second)
		u.Terminate()

	case "deregister-poweroff":
		deregReq, err := mm_5gs.DeregistrationRequest(u, true)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to build Deregistration Request: %v", err), http.StatusInternalServerError)
			return
		}
		sender.SendToGnb(u, deregReq)
		logrus.Info("[WEB][ACTION] Power-off Deregistration Request sent successfully")
		time.Sleep(1 * time.Second)
		u.Terminate()

	case "pdu-establish":
		if req.PduSessionId == 0 || req.PduSessionId > 15 {
			http.Error(w, "PDU Session ID must be between 1 and 15", http.StatusBadRequest)
			return
		}
		u.SetupPduSession(req.PduSessionId, req.Dnn, req.SessionType, req.Sst, req.Sd)
		ulNasTransport, err := mm_5gs.UlNasTransport(u, req.PduSessionId, nasMessage.ULNASTransportRequestTypeInitialRequest)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to build UlNasTransport: %v", err), http.StatusInternalServerError)
			return
		}
		u.GetPduSession(req.PduSessionId).State = ueContext.SM5G_PDU_SESSION_ACTIVE_PENDING
		sender.SendToGnb(u, ulNasTransport)
		logrus.Infof("[WEB][ACTION] Secondary PDU Session establishment sent for ID %d", req.PduSessionId)

	case "handover", "xn-handover":
		ip := req.TargetGnbIP
		if ip == "" {
			ip = "127.0.0.1"
		}
		port := req.TargetGnbPort
		if port == 0 {
			port = 9489
		}
		linkType := req.TargetGnbLinkType
		if linkType == "" {
			linkType = u.GetGnbLinkType()
		}
		socketPath := req.TargetGnbSocketPath
		isXn := req.Action == "xn-handover"

		err := ue.TriggerHandover(u, ip, port, linkType, socketPath, isXn, req.TargetGnbId, req.TargetGnbName)
		if err != nil {
			http.Error(w, fmt.Sprintf("Handover trigger failed: %v", err), http.StatusInternalServerError)
			return
		}
		if isXn {
			logrus.Infof("[WEB][ACTION] Xn Handover triggered successfully to %s:%d (SocketPath: %s, TargetId: %s)", ip, port, socketPath, req.TargetGnbId)
		} else {
			logrus.Infof("[WEB][ACTION] Handover triggered successfully to %s:%d (SocketPath: %s, TargetId: %s)", ip, port, socketPath, req.TargetGnbId)
		}

	default:
		http.Error(w, fmt.Sprintf("Unsupported action: %s", req.Action), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"success"}`))
}

// ─── Fleet Manager API Handlers ────────────────────────────────────────────────

// handleFleetUEProfiles handles GET (list) and POST (upsert) for UE profiles.
func handleFleetUEProfiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		fleet := config.GetFleet()
		_ = json.NewEncoder(w).Encode(fleet.UEProfiles)

	case http.MethodPost:
		var p config.UEProfile
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
		if err := config.ValidateUEProfile(p); err != nil {
			http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
			return
		}
		if err := config.UpsertUEProfile(p); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save UE profile: %v", err), http.StatusInternalServerError)
			return
		}
		logrus.Infof("[FLEET] UE profile '%s' saved", p.Name)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"saved"}`))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFleetUEProfileDelete handles DELETE /api/fleet/ue/{name}
func handleFleetUEProfileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Path[len("/api/fleet/ue/"):]
	if name == "" {
		http.Error(w, "Profile name required", http.StatusBadRequest)
		return
	}
	if err := config.DeleteUEProfile(name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	logrus.Infof("[FLEET] UE profile '%s' deleted", name)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"deleted"}`))
}

// handleFleetGNBProfiles handles GET (list) and POST (upsert) for gNB profiles.
func handleFleetGNBProfiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		fleet := config.GetFleet()
		_ = json.NewEncoder(w).Encode(fleet.GNBProfiles)

	case http.MethodPost:
		var p config.GNBProfile
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
		if err := config.ValidateGNBProfile(p); err != nil {
			http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
			return
		}
		if err := config.UpsertGNBProfile(p); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save gNB profile: %v", err), http.StatusInternalServerError)
			return
		}
		logrus.Infof("[FLEET] gNB profile '%s' saved", p.Name)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"saved"}`))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFleetGNBProfileDelete handles DELETE /api/fleet/gnb/{name}
func handleFleetGNBProfileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Path[len("/api/fleet/gnb/"):]
	if name == "" {
		http.Error(w, "Profile name required", http.StatusBadRequest)
		return
	}
	// Cannot delete a running gNB profile
	if IsGNBProfileRunning(name) {
		http.Error(w, fmt.Sprintf("gNB profile '%s' is currently running — stop it first", name), http.StatusConflict)
		return
	}
	if err := config.DeleteGNBProfile(name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	logrus.Infof("[FLEET] gNB profile '%s' deleted", name)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"deleted"}`))
}

// FleetLaunchRequest carries the profile name to launch.
type FleetLaunchRequest struct {
	ProfileName    string `json:"profileName"`
	GnbProfileName string `json:"gnbProfileName"`
}

// FleetLaunchUEResponse includes the assigned UE ID.
type FleetLaunchUEResponse struct {
	Status  string `json:"status"`
	UeId    uint8  `json:"ueId"`
	Supi    string `json:"supi,omitempty"`
	Message string `json:"message,omitempty"`
}

// handleFleetLaunchUE handles POST /api/fleet/launch/ue
func handleFleetLaunchUE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FleetLaunchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if req.ProfileName == "" {
		http.Error(w, "profileName is required", http.StatusBadRequest)
		return
	}

	ueID, err := LaunchUEFromProfile(req.ProfileName, req.GnbProfileName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to launch UE: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(FleetLaunchUEResponse{
		Status:  "launched",
		UeId:    ueID,
		Message: fmt.Sprintf("UE %d registration initiated", ueID),
	})
}

// handleFleetLaunchGNB handles POST /api/fleet/launch/gnb
func handleFleetLaunchGNB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FleetLaunchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if req.ProfileName == "" {
		http.Error(w, "profileName is required", http.StatusBadRequest)
		return
	}

	if err := LaunchGNBProfile(req.ProfileName); err != nil {
		http.Error(w, fmt.Sprintf("Failed to launch gNB: %v", err), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"launched"}`))
}

// FleetStopUERequest carries the UE ID to stop.
type FleetStopUERequest struct {
	UeId uint8 `json:"ueId"`
}

// handleFleetStopUE handles POST /api/fleet/stop/ue with body {"ueId": N}
func handleFleetStopUE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FleetStopUERequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	u := ueContext.GetActiveUE(req.UeId)
	if u == nil {
		http.Error(w, fmt.Sprintf("UE %d is not active", req.UeId), http.StatusNotFound)
		return
	}

	go u.Terminate()
	logrus.Infof("[FLEET] Terminated UE %d", req.UeId)

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"terminated"}`))
}

// handleFleetStopGNB handles POST /api/fleet/stop/gnb/{name}
func handleFleetStopGNB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Path[len("/api/fleet/stop/gnb/"):]
	if name == "" {
		http.Error(w, "Profile name required in URL path", http.StatusBadRequest)
		return
	}

	if err := StopGNBProfile(name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"stopping"}`))
}

// handleFleetRunning handles GET /api/fleet/running — returns live fleet state.
func handleFleetRunning(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	summary := GetFleetRunningSummary()
	_ = json.NewEncoder(w).Encode(summary)
}
