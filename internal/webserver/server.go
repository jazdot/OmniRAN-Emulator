package webserver

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/chaos"
	gnbContext "OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	serviceNgap "OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/service"
	triggerNgap "OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/trigger"
	"OmniRAN-Emulator/internal/control_test_engine/ue"
	ueContext "OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control/mm_5gs"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/sender"
	"OmniRAN-Emulator/internal/templates"
	ueContextMgmt "OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/ue_context_management"
	senderNgap "OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/sender"
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

func isAnyFleetGNBOrUERunning() bool {
	runningGNBsMu.RLock()
	hasGNBs := len(runningGNBs) > 0
	runningGNBsMu.RUnlock()

	hasUEs := len(ueContext.GetAllActiveUEs()) > 0
	return hasGNBs || hasUEs
}

func isScenarioOrCustomRunning() bool {
	if atomic.LoadInt32(&runningState) == 1 {
		return true
	}
	GlobalCustomRunner.mu.RLock()
	isCustom := GlobalCustomRunner.Status == "running"
	GlobalCustomRunner.mu.RUnlock()
	return isCustom
}

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
	AppendLogToFile(msg)
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

	// Initialize users and custom scenarios on server start
	if err := loadUsers(); err != nil {
		logrus.Errorf("[WEB] Failed to load users database: %v", err)
	}
	if err := loadSavedScenarios(); err != nil {
		logrus.Errorf("[WEB] Failed to load custom scenarios: %v", err)
	}

	mux := http.NewServeMux()

	// Public Authentication APIs
	mux.HandleFunc("/api/auth/session", handleAuthSession)
	mux.HandleFunc("/api/auth/setup", handleAuthSetup)
	mux.HandleFunc("/api/auth/login", handleAuthLogin)
	mux.HandleFunc("/api/auth/logout", handleAuthLogout)

	// User Management (Admin Only)
	mux.HandleFunc("/api/auth/users", withAdminAuth(handleListUsers))
	mux.HandleFunc("/api/auth/users/create", withAdminAuth(handleCreateUser))
	mux.HandleFunc("/api/auth/users/delete", withAdminAuth(handleDeleteUser))
	mux.HandleFunc("/api/auth/users/update", withAdminAuth(handleUpdateUser))

	// Custom Scenario Management
	mux.HandleFunc("/api/scenarios/save", withAuth(handleSaveScenario))
	mux.HandleFunc("/api/scenarios/edit", withAdminAuth(handleEditScenario))

	// REST API Routes (Wrapped in Authentication middleware)
	mux.HandleFunc("/api/status", withAuth(handleStatus))
	mux.HandleFunc("/api/config", withAuth(handleConfig))
	mux.HandleFunc("/api/config/release", withAuth(handleConfigRelease))
	mux.HandleFunc("/api/scenarios", withAuth(handleScenariosList))
	mux.HandleFunc("/api/scenarios/run", withAuth(handleScenarioRun))
	mux.HandleFunc("/api/scenarios/stop", withAuth(handleScenarioStop))
	mux.HandleFunc("/api/scenarios/custom/run", withAuth(handleCustomScenarioRun))
	mux.HandleFunc("/api/scenarios/custom/status", withAuth(handleCustomScenarioStatus))
	mux.HandleFunc("/api/scenarios/custom/stop", withAuth(handleCustomScenarioStop))
	mux.HandleFunc("/api/chaos/configure", withAuth(handleChaosConfigure))
	mux.HandleFunc("/api/chaos/status", withAuth(handleChaosStatus))
	mux.HandleFunc("/api/chaos/reset", withAuth(handleChaosReset))
	mux.HandleFunc("/api/chaos/fuzz/configure", withAuth(handleChaosFuzzConfigure))
	mux.HandleFunc("/api/chaos/fuzz/status", withAuth(handleChaosFuzzStatus))
	mux.HandleFunc("/api/chaos/sctp-failover", withAuth(handleChaosSctpFailover))
	mux.HandleFunc("/api/ue/active", withAuth(handleActiveUEs))
	mux.HandleFunc("/api/ue/action", withAuth(handleUEAction))
	mux.HandleFunc("/api/ping", withAuth(handlePingTest))
	mux.HandleFunc("/api/logs/stream", withAuth(handleLogStream))
	mux.HandleFunc("/api/ue/ping", withAuth(handleUEPing))
	mux.HandleFunc("/api/ue/http", withAuth(handleUEHttp))
	mux.HandleFunc("/api/ue/stream", withAuth(handleUEStream))
	mux.HandleFunc("/api/ue/vonr/dial", withAuth(handleUEVonrDial))
	mux.HandleFunc("/api/ue/vonr/hangup", withAuth(handleUEVonrHangup))
	mux.HandleFunc("/api/ue/traffic/stats", withAuth(handleUETrafficStats))
	mux.HandleFunc("/api/ue/traffic/performance", withAuth(handleUETrafficPerformance))
	mux.HandleFunc("/api/ue/traffic/packets", withAuth(handleUETrafficPackets))
	mux.HandleFunc("/api/slices/sla", withAuth(handleSlicesSla))
	mux.HandleFunc("/api/test/performance", withAuth(handlePerformanceTest))
	
	// Fleet Manager Routes (Wrapped in Authentication middleware)
	mux.HandleFunc("/api/fleet/ue", withAuth(handleFleetUEProfiles))
	mux.HandleFunc("/api/fleet/ue/", withAuth(handleFleetUEProfileDelete))
	mux.HandleFunc("/api/fleet/gnb", withAuth(handleFleetGNBProfiles))
	mux.HandleFunc("/api/fleet/gnb/", withAuth(handleFleetGNBProfileDelete))
	mux.HandleFunc("/api/fleet/launch/ue", withAuth(handleFleetLaunchUE))
	mux.HandleFunc("/api/fleet/launch/gnb", withAuth(handleFleetLaunchGNB))
	mux.HandleFunc("/api/fleet/stop/ue", withAuth(handleFleetStopUE))
	mux.HandleFunc("/api/fleet/stop/gnb/", withAuth(handleFleetStopGNB))
	mux.HandleFunc("/api/fleet/running", withAuth(handleFleetRunning))
	
	// Diagnostics / PCAP API Routes (Wrapped in Authentication middleware)
	mux.HandleFunc("/api/diagnostics/pcap/interfaces", withAuth(handleGetInterfaces))
	mux.HandleFunc("/api/diagnostics/pcap/start", withAuth(handleStartPcap))
	mux.HandleFunc("/api/diagnostics/pcap/stop", withAuth(handleStopPcap))
	mux.HandleFunc("/api/diagnostics/pcap/status", withAuth(handleGetPcapStatus))
	mux.HandleFunc("/api/diagnostics/pcap/list", withAuth(handleListPcaps))
	mux.HandleFunc("/api/diagnostics/pcap/download", withAuth(handleDownloadPcap))
	mux.HandleFunc("/api/diagnostics/pcap/delete", withAuth(handleDeletePcap))
	mux.HandleFunc("/api/diagnostics/pcap/parse", withAuth(handleParsePcap))
	mux.HandleFunc("/api/diagnostics/logs/download", withAuth(handleDownloadLogs))
	mux.HandleFunc("/api/diagnostics/logs/clear", withAuth(handleClearLogs))
	mux.HandleFunc("/api/diagnostics/logs/history", withAuth(handleGetLogsHistory))
	mux.HandleFunc("/api/diagnostics/logs/parse", withAuth(handleParseLogs))
	mux.HandleFunc("/api/docs", withAuth(handleDocs))

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
	ActiveRelease string             `json:"activeRelease"`
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
		IsRunning:     atomic.LoadInt32(&runningState) == 1,
		RunningName:   runningName,
		ActiveRelease: config.GetActiveRelease(),
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
			ConnectionState:  getUeConnectionState(u),
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

func handleConfigRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"release":"%s"}`, config.GetActiveRelease())))
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Release string `json:"release"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
		if req.Release != "15" && req.Release != "17" && req.Release != "18" && req.Release != "19" {
			http.Error(w, "Unsupported release. Must be '15', '17', '18', or '19'", http.StatusBadRequest)
			return
		}
		config.SetActiveRelease(req.Release)
		logrus.Infof("[WEB] Active 3GPP Release updated dynamically to Release %s", req.Release)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success"}`))
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

type ScenarioItem struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	IsCustom    bool         `json:"isCustom"`
	Steps       []CustomStep `json:"steps,omitempty"`
}

func handleScenariosList(w http.ResponseWriter, r *http.Request) {
	defaults := []ScenarioItem{
		{ID: "interactive-ue", Name: "Interactive UE Control Session", Description: "Registers a UE and keeps it active/running, allowing dynamic operations via the UE Controller card"},
		{ID: "periodic-reg", Name: "Periodic Registration Update", Description: "Registers a UE, simulates T3512 expiration, and triggers a Periodic Registration Update"},
		{ID: "mobility-reg", Name: "Mobility Registration Update (TAU)", Description: "Registers a UE, simulates cell crossing, and triggers a Mobility Tracking Area Update (TAU)"},
		{ID: "emergency-reg", Name: "Emergency Registration", Description: "Triggers unauthenticated Emergency Registration to the core network"},
		{ID: "handover", Name: "N2 Handover (Path Switch)", Description: "Simulates cell change by executing a Path Switch Request between gNodeBs"},
		{ID: "xn-handover", Name: "Xn Handover (Inter-gNB)", Description: "Starts two GNodeBs (Source and Target), registers a UE, establishes a PDU session, and performs Xn-based handover between GNodeBs"},
		{ID: "pdu-lifecycle", Name: "PDU Session Lifecycle", Description: "Registers a UE, establishes a PDU session, releases the PDU session, and validates state transitions back to INACTIVE"},
		{ID: "pdu-mod-lifecycle", Name: "PDU Session Modification Lifecycle", Description: "Registers a UE, establishes a PDU session, modifies session parameters, and clean releases the session"},
		{ID: "full-lifecycle", Name: "Full UE Lifecycle", Description: "Executes full sequence: Attach → PDU Active → CM-IDLE → Service Request → Detach"},
		{ID: "deregister", Name: "UE-initiated Deregistration", Description: "Registers a UE and performs a clean power-off Deregistration Request"},
		{ID: "load-test", Name: "Multi-UE Load Endurance Test", Description: "Stress tests the AMF by attaching multiple simulated UEs sequentially in a queue"},
		{ID: "amf-load-loop", Name: "AMF Load Loop (Stress Test)", Description: "Periodically generates heavy registration requests to evaluate AMF throughput under stress"},
		{ID: "ue-latency-interval", Name: "UE Registration Latency", Description: "Evaluates and measures the average registration latency for a queue of UEs"},
		{ID: "amf-availability", Name: "AMF Core Uptime Availability", Description: "Performs reachability checks to evaluate the core uptime over a specified interval"},
		{ID: "r17-ntn", Name: "3GPP Rel 17: RedCap & NTN Attachment", Description: "Demonstrates Release 17 capabilities, including RedCap cell access, satellite orbit parameter IEs, and XR low-latency QoS flows"},
		{ID: "r18-uav", Name: "3GPP Rel 18: UAV Flight & Slicing", Description: "Demonstrates Release 18 capabilities: Aerial drone trajectory registration, PEI support, and Slice Groups handover"},
		{ID: "r19-sensing", Name: "3GPP Rel 19: AI & ISAC Sensing", Description: "Demonstrates Release 19 capabilities: Ambient IoT passive sensor relay tags, direct RAN AI model inference deployment, and ISAC radar target sweeps"},
		{ID: "storm", Name: "AMF Registration Storm", Description: "Simulates cell-congestion by launching a rapid burst of concurrent registration requests to stress-test the AMF core"},
	}

	scenariosMu.RLock()
	merged := append(defaults, savedScenarios...)
	scenariosMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(merged)
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

	if isScenarioOrCustomRunning() {
		http.Error(w, "Another scenario is already executing", http.StatusConflict)
		return
	}

	if isAnyFleetGNBOrUERunning() {
		http.Error(w, "Cannot run scenario: active gNBs or UEs exist in the Fleet Manager. Please stop them first.", http.StatusConflict)
		return
	}

	atomic.StoreInt32(&runningState, 1)

	var req RunRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.Scenario != "" && strings.HasPrefix(req.Scenario, "custom-") {
		var targetScen *ScenarioItem
		scenariosMu.RLock()
		for _, s := range savedScenarios {
			if s.ID == req.Scenario {
				targetScen = &s
				break
			}
		}
		scenariosMu.RUnlock()
		if targetScen != nil {
			customScen := CustomScenario{
				Name:        targetScen.Name,
				Description: targetScen.Description,
				Steps:       targetScen.Steps,
			}
			GlobalCustomRunner.Run(customScen)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"running"}`))
			return
		} else {
			http.Error(w, "Custom scenario not found", http.StatusNotFound)
			return
		}
	}

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
		case "pdu-mod-lifecycle":
			templates.ScenarioPduModificationLifecycle(func() bool {
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
		case "r17-ntn":
			templates.ScenarioRelease17RedCapNTN(func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "r18-uav":
			templates.ScenarioRelease18UAVSlicing(func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "r19-sensing":
			templates.ScenarioRelease19AISensing(func() bool {
				return atomic.LoadInt32(&runningState) == 0
			})
		case "storm":
			templates.ScenarioRegistrationStorm(func() bool {
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
	ConnectionState  string             `json:"connectionState"` // "CONNECTED" or "IDLE"
}

func getUeConnectionState(u *ueContext.UEContext) string {
	if u.GetStateMM() != 3 { // MM5G_REGISTERED = 3
		return "IDLE"
	}
	gnbContext.ActiveGNBsMu.RLock()
	gnb, exists := gnbContext.ActiveGNBs[u.GetGnbId()]
	gnbContext.ActiveGNBsMu.RUnlock()
	if !exists || gnb == nil {
		return "IDLE"
	}
	isConnected := false
	gnb.RangeUePool(func(ranUeId int64, gUe *gnbContext.GNBUe) bool {
		if gUe.GetAmfUeId() == u.GetAmfUeId() {
			if gUe.GetState() == 2 { // Ready = 2
				isConnected = true
			}
			return false
		}
		return true
	})
	if isConnected {
		return "CONNECTED"
	}
	return "IDLE"
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
			ConnectionState:  getUeConnectionState(u),
		})
	}
	sort.Slice(resp, func(i, j int) bool {
		return resp[i].ID < resp[j].ID
	})

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

	case "pdu-modify":
		if req.PduSessionId == 0 || req.PduSessionId > 15 {
			http.Error(w, "PDU Session ID must be between 1 and 15", http.StatusBadRequest)
			return
		}
		sess := u.GetPduSession(req.PduSessionId)
		if sess == nil {
			http.Error(w, fmt.Sprintf("PDU Session with ID %d is not active", req.PduSessionId), http.StatusNotFound)
			return
		}
		ulNasTransport, err := mm_5gs.UlNasTransportModification(u, req.PduSessionId)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to build UlNasTransportModification: %v", err), http.StatusInternalServerError)
			return
		}
		sender.SendToGnb(u, ulNasTransport)
		logrus.Infof("[WEB][ACTION] PDU Session Modification sent for ID %d", req.PduSessionId)

	case "pdu-release":
		if req.PduSessionId == 0 || req.PduSessionId > 15 {
			http.Error(w, "PDU Session ID must be between 1 and 15", http.StatusBadRequest)
			return
		}
		sess := u.GetPduSession(req.PduSessionId)
		if sess == nil {
			http.Error(w, fmt.Sprintf("PDU Session with ID %d is not active", req.PduSessionId), http.StatusNotFound)
			return
		}
		ulNasTransport, err := mm_5gs.UlNasTransportRelease(u, req.PduSessionId)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to build UlNasTransportRelease: %v", err), http.StatusInternalServerError)
			return
		}
		sender.SendToGnb(u, ulNasTransport)
		logrus.Infof("[WEB][ACTION] PDU Session Release sent for ID %d", req.PduSessionId)

	case "handover", "xn-handover":
		ip := req.TargetGnbIP
		port := req.TargetGnbPort
		linkType := req.TargetGnbLinkType
		socketPath := req.TargetGnbSocketPath
		targetName := req.TargetGnbName

		if req.TargetGnbId != "" {
			gnbContext.ActiveGNBsMu.RLock()
			targetGnb, exists := gnbContext.ActiveGNBs[req.TargetGnbId]
			gnbContext.ActiveGNBsMu.RUnlock()
			if exists && targetGnb != nil {
				ip = targetGnb.GetGnbIp()
				if targetGnb.GetLinkType() == "tcp" {
					port = targetGnb.GetLinkPort()
				} else {
					port = targetGnb.GetGnbPort()
				}
				linkType = targetGnb.GetLinkType()
				socketPath = targetGnb.GetSocketPath()
				targetName = targetGnb.GetGnbId()
			}
		}

		if ip == "" {
			ip = "127.0.0.1"
		}
		if port == 0 {
			port = 9489
		}
		if linkType == "" {
			linkType = u.GetGnbLinkType()
		}
		isXn := req.Action == "xn-handover"

		err := ue.TriggerHandover(u, ip, port, linkType, socketPath, isXn, req.TargetGnbId, targetName)
		if err != nil {
			http.Error(w, fmt.Sprintf("Handover trigger failed: %v", err), http.StatusInternalServerError)
			return
		}
		if isXn {
			logrus.Infof("[WEB][ACTION] Xn Handover triggered successfully to %s:%d (SocketPath: %s, TargetId: %s)", ip, port, socketPath, req.TargetGnbId)
		} else {
			logrus.Infof("[WEB][ACTION] Handover triggered successfully to %s:%d (SocketPath: %s, TargetId: %s)", ip, port, socketPath, req.TargetGnbId)
		}

	case "connection-release":
		gnbContext.ActiveGNBsMu.RLock()
		gnb, exists := gnbContext.ActiveGNBs[u.GetGnbId()]
		gnbContext.ActiveGNBsMu.RUnlock()
		if !exists || gnb == nil {
			http.Error(w, fmt.Sprintf("GNodeB with ID %s is not active", u.GetGnbId()), http.StatusNotFound)
			return
		}

		var targetGnBUe *gnbContext.GNBUe
		gnb.RangeUePool(func(ranUeId int64, ue *gnbContext.GNBUe) bool {
			if ue.GetAmfUeId() == u.GetAmfUeId() {
				targetGnBUe = ue
				return false
			}
			return true
		})

		if targetGnBUe == nil {
			http.Error(w, fmt.Sprintf("UE with AMF ID %d not found in GNodeB", u.GetAmfUeId()), http.StatusNotFound)
			return
		}

		releaseMsg, err := ueContextMgmt.GetUEContextReleaseRequest(targetGnBUe.GetRanUeId(), targetGnBUe.GetAmfUeId(), nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to build UE Context Release Request: %v", err), http.StatusInternalServerError)
			return
		}

		err = senderNgap.SendToAmF(releaseMsg, targetGnBUe.GetSCTP())
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to send UE Context Release Request to AMF: %v", err), http.StatusInternalServerError)
			return
		}
		logrus.Infof("[WEB][ACTION] Connection Release Request sent for UE %d", u.GetUeId())

	case "paging":
		gnbContext.ActiveGNBsMu.RLock()
		gnb, exists := gnbContext.ActiveGNBs[u.GetGnbId()]
		gnbContext.ActiveGNBsMu.RUnlock()
		if !exists || gnb == nil {
			http.Error(w, fmt.Sprintf("GNodeB with ID %s is not active", u.GetGnbId()), http.StatusNotFound)
			return
		}

		var targetGnBUe *gnbContext.GNBUe
		gnb.RangeUePool(func(ranUeId int64, ue *gnbContext.GNBUe) bool {
			if ue.GetAmfUeId() == u.GetAmfUeId() {
				targetGnBUe = ue
				return false
			}
			return true
		})

		if targetGnBUe == nil {
			http.Error(w, fmt.Sprintf("UE with AMF ID %d not found in GNodeB", u.GetAmfUeId()), http.StatusNotFound)
			return
		}

		conn := targetGnBUe.GetUnixSocket()
		if conn == nil {
			http.Error(w, "UNIX/TCP control socket connection to UE is nil, cannot page", http.StatusInternalServerError)
			return
		}

		_, err := conn.Write([]byte{0x00, 0x01})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to write paging trigger to UE socket: %v", err), http.StatusInternalServerError)
			return
		}

		logrus.Infof("[WEB][ACTION] Paging trigger sent to UE %d", u.GetUeId())

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

	if isScenarioOrCustomRunning() {
		http.Error(w, "Cannot launch UE: a scenario is currently executing. Please stop the scenario first.", http.StatusConflict)
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

	if isScenarioOrCustomRunning() {
		http.Error(w, "Cannot launch gNB: a scenario is currently executing. Please stop the scenario first.", http.StatusConflict)
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

// handleCustomScenarioRun handles POST /api/scenarios/custom/run
func handleCustomScenarioRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if isScenarioOrCustomRunning() {
		http.Error(w, "Another scenario is already executing", http.StatusConflict)
		return
	}

	if isAnyFleetGNBOrUERunning() {
		http.Error(w, "Cannot run scenario: active gNBs or UEs exist in the Fleet Manager. Please stop them first.", http.StatusConflict)
		return
	}

	var scen CustomScenario
	err := json.NewDecoder(r.Body).Decode(&scen)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	GlobalCustomRunner.Run(scen)

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"started"}`))
}

// handleCustomScenarioStatus handles GET /api/scenarios/custom/status
func handleCustomScenarioStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(GlobalCustomRunner.GetStateJSON())
}

// handleCustomScenarioStop handles POST /api/scenarios/custom/stop
func handleCustomScenarioStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	GlobalCustomRunner.Stop()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"stopped"}`))
}

// handleChaosConfigure handles POST /api/chaos/configure
func handleChaosConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type ChaosConfigRequest struct {
		Target          string  `json:"target"` // "nas" or "ngap"
		UeId            uint8   `json:"ueId"`
		GnbId           string  `json:"gnbId"`
		DropProbability float64 `json:"dropProbability"`
		DelayMs         int64   `json:"delayMs"`
		TargetMsgType   string  `json:"targetMsgType"`
		Enabled         bool    `json:"enabled"`
	}
	var req ChaosConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg := chaos.ChaosConfig{
		DropProbability: req.DropProbability,
		DelayDuration:   time.Duration(req.DelayMs) * time.Millisecond,
		TargetMsgType:   req.TargetMsgType,
		Enabled:         req.Enabled,
	}

	if req.Target == "nas" {
		chaos.GlobalChaosManager.ConfigureNas(req.UeId, cfg)
	} else if req.Target == "ngap" {
		chaos.GlobalChaosManager.ConfigureNgap(req.GnbId, cfg)
	} else {
		http.Error(w, "invalid target", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"configured"}`))
}

// handleChaosStatus handles GET /api/chaos/status
func handleChaosStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	stats := chaos.GlobalChaosManager.GetStats()
	
	nasRules, ngapRules := chaos.GlobalChaosManager.GetRules()
	
	response := map[string]interface{}{
		"stats": stats,
		"nas":   nasRules,
		"ngap":  ngapRules,
	}
	_ = json.NewEncoder(w).Encode(response)
}

// handleChaosReset handles POST /api/chaos/reset
func handleChaosReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	chaos.GlobalChaosManager.Reset()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"reset"}`))
}

// handleChaosFuzzConfigure handles POST /api/chaos/fuzz/configure
func handleChaosFuzzConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req chaos.FuzzConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chaos.GlobalChaosManager.ConfigureFuzz(req)

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"configured"}`))
}

// handleChaosFuzzStatus handles GET /api/chaos/fuzz/status
func handleChaosFuzzStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fuzz := chaos.GlobalChaosManager.GetFuzz()
	_ = json.NewEncoder(w).Encode(fuzz)
}

// handleChaosSctpFailover handles POST /api/chaos/sctp-failover
func handleChaosSctpFailover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type FailoverRequest struct {
		GnbId string `json:"gnbId"`
	}
	var req FailoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	go triggerSctpFailover(req.GnbId)

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"failover_triggered"}`))
}

func triggerSctpFailover(gnbId string) {
	logrus.Infof("[CHAOS] Initiating SCTP path failover recovery test for GNodeB %s", gnbId)

	gnbContext.ActiveGNBsMu.RLock()
	g, ok := gnbContext.ActiveGNBs[gnbId]
	gnbContext.ActiveGNBsMu.RUnlock()

	if !ok || g == nil {
		logrus.Errorf("[CHAOS] GNodeB %s not found in active fleet", gnbId)
		return
	}

	amf := g.GetActiveAmf()
	if amf == nil {
		logrus.Errorf("[CHAOS] GNodeB %s has no active AMF association", gnbId)
		return
	}

	n2Conn := g.GetN2()
	if n2Conn == nil {
		logrus.Errorf("[CHAOS] GNodeB %s has no active SCTP association", gnbId)
		return
	}

	// 1. Simulating link drop (close socket)
	logrus.Warnf("[CHAOS] DROPPING SCTP link for GNodeB %s", gnbId)
	_ = n2Conn.Close()

	// 2. Sleep to simulate down time
	time.Sleep(3 * time.Second)

	// 3. Re-dialing to reconnect
	logrus.Infof("[CHAOS] RECONNECTING SCTP path to AMF for GNodeB %s", gnbId)
	if err := serviceNgap.InitConn(amf, g); err != nil {
		logrus.Errorf("[CHAOS] Reconnection failed for GNodeB %s: %v", gnbId, err)
		return
	}

	// 4. Send NGSetupRequest
	triggerNgap.SendNgSetupRequest(g, amf)
	logrus.Infof("[CHAOS] Sent NG Setup Request over new SCTP connection for GNodeB %s", gnbId)
}

type perfRequest struct {
	NumUEs      int `json:"numUes"`
	DurationSec int `json:"duration"`
}

func handlePerformanceTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if isScenarioOrCustomRunning() {
		http.Error(w, "Cannot run performance test: another scenario is already executing", http.StatusConflict)
		return
	}

	if isAnyFleetGNBOrUERunning() {
		http.Error(w, "Cannot run performance test: active gNBs or UEs exist in the Fleet Manager. Please stop them first.", http.StatusConflict)
		return
	}

	atomic.StoreInt32(&runningState, 1)
	runningName = "Performance Test"
	defer func() {
		atomic.StoreInt32(&runningState, 0)
		runningName = ""
	}()

	var req perfRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if req.NumUEs <= 0 {
		req.NumUEs = 10
	}
	if req.DurationSec <= 0 {
		req.DurationSec = 5
	}

	report, err := templates.RunPerformanceSuite(req.NumUEs, req.DurationSec)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error running performance suite: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

func handleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	content, err := os.ReadFile("docs/technical_reference.md")
	if err != nil {
		content = []byte("# OmniRAN Technical Documentation\n\nFailed to load docs/technical_reference.md. Please ensure the file is present in the docs/ directory.")
	}

	type DocsResponse struct {
		Content string `json:"content"`
	}

	_ = json.NewEncoder(w).Encode(DocsResponse{
		Content: string(content),
	})
}

// ─── Authentication, Session & User Management ───────────────────────────────

type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
	Salt         string `json:"salt"`
	Role         string `json:"role"` // "admin" or "user"
}

type SessionInfo struct {
	Username string
	Role     string
	Expires  time.Time
}

var (
	usersFile      = "config/users.json"
	users          = make(map[string]User)
	usersMu        sync.RWMutex
	activeSessions = make(map[string]*SessionInfo)
	sessionMu      sync.RWMutex
)

// Hashing & Salt functions
func hashPassword(password string, salt []byte) string {
	hasher := sha256.New()
	hasher.Write(salt)
	hasher.Write([]byte(password))
	hash := hasher.Sum(nil)
	for i := 0; i < 9999; i++ {
		h := sha256.New()
		h.Write(hash)
		hash = h.Sum(nil)
	}
	return hex.EncodeToString(hash)
}

func generateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, err
	}
	return salt, nil
}

func verifyPassword(password, saltHex, hashHex string) bool {
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}
	computedHash := hashPassword(password, salt)
	computedBytes, _ := hex.DecodeString(computedHash)
	storedBytes, _ := hex.DecodeString(hashHex)
	return subtle.ConstantTimeCompare(computedBytes, storedBytes) == 1
}

// Load & Save Users
func loadUsers() error {
	usersMu.Lock()
	defer usersMu.Unlock()

	file, err := os.Open(usersFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	var uList []User
	if err := json.NewDecoder(file).Decode(&uList); err != nil {
		return err
	}

	users = make(map[string]User)
	for _, u := range uList {
		users[u.Username] = u
	}
	return nil
}

func saveUsers() error {
	var uList []User
	for _, u := range users {
		uList = append(uList, u)
	}

	_ = os.MkdirAll("config", 0755)
	data, err := json.MarshalIndent(uList, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(usersFile, data, 0600)
}

// Authentication Middlewares
func authenticate(r *http.Request) (*SessionInfo, error) {
	token := r.Header.Get("X-Session-Token")
	if token == "" {
		if cookie, err := r.Cookie("session_token"); err == nil {
			token = cookie.Value
		}
	}
	if token == "" {
		return nil, fmt.Errorf("missing session token")
	}

	sessionMu.RLock()
	session, ok := activeSessions[token]
	sessionMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("invalid session")
	}
	if time.Now().After(session.Expires) {
		sessionMu.Lock()
		delete(activeSessions, token)
		sessionMu.Unlock()
		return nil, fmt.Errorf("session expired")
	}
	return session, nil
}

func withAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Session-Token")
			w.WriteHeader(http.StatusOK)
			return
		}

		_, err := authenticate(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Unauthorized: login required"}`))
			return
		}
		handler(w, r)
	}
}

func withAdminAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Session-Token")
			w.WriteHeader(http.StatusOK)
			return
		}

		session, err := authenticate(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Unauthorized: login required"}`))
			return
		}

		if session.Role != "admin" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"Forbidden: admin role required"}`))
			return
		}
		handler(w, r)
	}
}

// ─── Public Authentication Endpoints ─────────────────────────────────────────

func handleAuthSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	usersMu.RLock()
	hasAdmin := false
	for _, u := range users {
		if u.Role == "admin" {
			hasAdmin = true
			break
		}
	}
	usersMu.RUnlock()

	status := struct {
		SetupRequired bool   `json:"setupRequired"`
		LoggedIn      bool   `json:"loggedIn"`
		Username      string `json:"username"`
		Role          string `json:"role"`
	}{
		SetupRequired: !hasAdmin,
	}

	session, err := authenticate(r)
	if err == nil {
		status.LoggedIn = true
		status.Username = session.Username
		status.Role = session.Role
	}

	_ = json.NewEncoder(w).Encode(status)
}

func handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	usersMu.Lock()
	defer usersMu.Unlock()

	hasAdmin := false
	for _, u := range users {
		if u.Role == "admin" {
			hasAdmin = true
			break
		}
	}
	if hasAdmin {
		http.Error(w, "Setup already completed", http.StatusConflict)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Username == "" || len(req.Password) < 8 {
		http.Error(w, "Username cannot be empty and password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	salt, err := generateSalt()
	if err != nil {
		http.Error(w, "Failed to generate salt", http.StatusInternalServerError)
		return
	}

	hash := hashPassword(req.Password, salt)
	adminUser := User{
		Username:     req.Username,
		PasswordHash: hash,
		Salt:         hex.EncodeToString(salt),
		Role:         "admin",
	}

	users[req.Username] = adminUser
	if err := saveUsers(); err != nil {
		http.Error(w, "Failed to save user database", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "setup_completed"})
}

func handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	usersMu.RLock()
	u, ok := users[req.Username]
	usersMu.RUnlock()

	if !ok || !verifyPassword(req.Password, u.Salt, u.PasswordHash) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Invalid credentials"}`))
		return
	}

	tokenBytes := make([]byte, 32)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	sessionMu.Lock()
	activeSessions[token] = &SessionInfo{
		Username: u.Username,
		Role:     u.Role,
		Expires:  time.Now().Add(2 * time.Hour),
	}
	sessionMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(2 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	_ = json.NewEncoder(w).Encode(map[string]string{
		"token":    token,
		"username": u.Username,
		"role":     u.Role,
	})
}

func handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Session-Token")
	if token == "" {
		if cookie, err := r.Cookie("session_token"); err == nil {
			token = cookie.Value
		}
	}

	if token != "" {
		sessionMu.Lock()
		delete(activeSessions, token)
		sessionMu.Unlock()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"logged_out"}`))
}

// ─── User Administration Endpoints (Admin Only) ─────────────────────────────

type PublicUser struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

func handleListUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	usersMu.RLock()
	var list []PublicUser
	for _, u := range users {
		list = append(list, PublicUser{Username: u.Username, Role: u.Role})
	}
	usersMu.RUnlock()
	_ = json.NewEncoder(w).Encode(list)
}

func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Username == "" || len(req.Password) < 8 {
		http.Error(w, "Username cannot be empty and password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if req.Role != "admin" && req.Role != "user" {
		req.Role = "user"
	}

	usersMu.Lock()
	defer usersMu.Unlock()

	if _, exists := users[req.Username]; exists {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	salt, _ := generateSalt()
	hash := hashPassword(req.Password, salt)

	users[req.Username] = User{
		Username:     req.Username,
		PasswordHash: hash,
		Salt:         hex.EncodeToString(salt),
		Role:         req.Role,
	}

	if err := saveUsers(); err != nil {
		http.Error(w, "Failed to save user database", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "user_created"})
}

func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, _ := authenticate(r)
	if session != nil && session.Username == req.Username {
		http.Error(w, "Cannot delete currently logged-in administrator", http.StatusConflict)
		return
	}

	usersMu.Lock()
	defer usersMu.Unlock()

	if _, exists := users[req.Username]; !exists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	delete(users, req.Username)
	if err := saveUsers(); err != nil {
		http.Error(w, "Failed to save user database", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "user_deleted"})
}

func handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password,omitempty"`
		Role     string `json:"role,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	usersMu.Lock()
	defer usersMu.Unlock()

	u, exists := users[req.Username]
	if !exists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if req.Password != "" {
		if len(req.Password) < 8 {
			http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
			return
		}
		salt, _ := generateSalt()
		u.PasswordHash = hashPassword(req.Password, salt)
		u.Salt = hex.EncodeToString(salt)
	}

	if req.Role == "admin" || req.Role == "user" {
		u.Role = req.Role
	}

	users[req.Username] = u
	if err := saveUsers(); err != nil {
		http.Error(w, "Failed to save user database", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "user_updated"})
}

// ─── Scenario Customization & Storage Endpoints ──────────────────────────────

var (
	scenariosFile  = "config/scenarios.json"
	savedScenarios = []ScenarioItem{}
	scenariosMu    sync.RWMutex
)

func loadSavedScenarios() error {
	scenariosMu.Lock()
	defer scenariosMu.Unlock()

	file, err := os.Open(scenariosFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(&savedScenarios)
}

func saveSavedScenarios() error {
	_ = os.MkdirAll("config", 0755)
	data, err := json.MarshalIndent(savedScenarios, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(scenariosFile, data, 0644)
}

func handleSaveScenario(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name        string       `json:"name"`
		Description string       `json:"description"`
		Steps       []CustomStep `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" || len(req.Steps) == 0 {
		http.Error(w, "Scenario Name and steps are required", http.StatusBadRequest)
		return
	}

	scenariosMu.Lock()
	defer scenariosMu.Unlock()

	customId := "custom-" + strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	
	foundIdx := -1
	for idx, s := range savedScenarios {
		if s.ID == customId {
			foundIdx = idx
			break
		}
	}

	newScen := ScenarioItem{
		ID:          customId,
		Name:        req.Name,
		Description: req.Description,
		IsCustom:    true,
		Steps:       req.Steps,
	}

	if foundIdx >= 0 {
		savedScenarios[foundIdx] = newScen
	} else {
		savedScenarios = append(savedScenarios, newScen)
	}

	if err := saveSavedScenarios(); err != nil {
		http.Error(w, "Failed to save custom scenarios", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "scenario_saved", "id": customId})
}

func handleEditScenario(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID          string       `json:"id"`
		Name        string       `json:"name"`
		Description string       `json:"description"`
		Steps       []CustomStep `json:"steps"`
		Delete      bool         `json:"delete,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "Scenario ID is required", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(req.ID, "custom-") {
		http.Error(w, "Cannot modify built-in default scenarios", http.StatusForbidden)
		return
	}

	scenariosMu.Lock()
	defer scenariosMu.Unlock()

	foundIdx := -1
	for idx, s := range savedScenarios {
		if s.ID == req.ID {
			foundIdx = idx
			break
		}
	}

	if foundIdx < 0 {
		http.Error(w, "Scenario not found", http.StatusNotFound)
		return
	}

	if req.Delete {
		savedScenarios = append(savedScenarios[:foundIdx], savedScenarios[foundIdx+1:]...)
	} else {
		savedScenarios[foundIdx].Name = req.Name
		savedScenarios[foundIdx].Description = req.Description
		if len(req.Steps) > 0 {
			savedScenarios[foundIdx].Steps = req.Steps
		}
	}

	if err := saveSavedScenarios(); err != nil {
		http.Error(w, "Failed to save custom scenarios", http.StatusInternalServerError)
		return
	}

	status := "scenario_updated"
	if req.Delete {
		status = "scenario_deleted"
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}
