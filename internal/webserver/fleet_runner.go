package webserver

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ishidawataru/sctp"
	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/control_test_engine/gnb"
	gnbContext "OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	ueContext "OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/service"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/trigger"
	"OmniRAN-Emulator/lib/nas/nasMessage"
	"OmniRAN-Emulator/lib/nas/security"

	"github.com/sirupsen/logrus"
)

// ─── Running GNB Fleet State ─────────────────────────────────────────────────

// RunningGNBInstance tracks a fleet-mode gNB lifecycle.
type RunningGNBInstance struct {
	ProfileName string
	GnbId       string
	StartedAt   time.Time
	Cancel      context.CancelFunc
	errCh       <-chan error
	GnbCtx      *gnbContext.GNBContext
	LinkType    string
	LinkPort    int
	ControlIp   string
	SocketPath  string
	Mcc         string
	Mnc         string
	Tac         string
}

// RunningGNBStatus is the JSON-serializable representation of a running gNB.
type RunningGNBStatus struct {
	ProfileName  string   `json:"profileName"`
	GnbId        string   `json:"gnbId"`
	StartedAt    string   `json:"startedAt"`
	State        string   `json:"state"`
	LinkType     string   `json:"linkType"`
	LinkPort     int      `json:"linkPort"`
	ControlIp    string   `json:"controlIp"`
	SocketPath   string   `json:"socketPath,omitempty"`
	Mcc          string   `json:"mcc"`
	Mnc          string   `json:"mnc"`
	Tac          string   `json:"tac"`
	ConnectedUes []string `json:"connectedUes"`
}

var (
	runningGNBsMu sync.RWMutex
	runningGNBs   = make(map[string]*RunningGNBInstance)
)

// LaunchGNBProfile starts a gNB from a named profile in fleet mode.
func LaunchGNBProfile(profileName string) error {
	prof, ok := config.GetGNBProfile(profileName)
	if !ok {
		return fmt.Errorf("gNB profile '%s' not found", profileName)
	}

	runningGNBsMu.Lock()
	defer runningGNBsMu.Unlock()

	if _, exists := runningGNBs[profileName]; exists {
		return fmt.Errorf("gNB profile '%s' is already running", profileName)
	}

	cfg := config.BuildConfigFromGNBProfile(prof)

	// Use a unique socket path for each gNB in fleet mode based on profile name
	socketPath := fmt.Sprintf("/tmp/gnb_%s.sock", profileName)

	ctx, cancel := context.WithCancel(context.Background())
	gCtx, errCh := gnb.InitGnbFleet(cfg, ctx, socketPath)

	// Wait briefly to catch immediate startup errors (e.g. connection refused, socket bind failure)
	select {
	case err := <-errCh:
		if err != nil {
			cancel()
			diag := DiagnoseGNBLaunchError(err, prof)
			return fmt.Errorf("%s", diag)
		}
	case <-time.After(600 * time.Millisecond):
		// gNB started with no immediate transport error
	}

	inst := &RunningGNBInstance{
		ProfileName: profileName,
		GnbId:       prof.GnbId,
		StartedAt:   time.Now(),
		Cancel:      cancel,
		errCh:       errCh,
		GnbCtx:      gCtx,
		LinkType:    prof.LinkType,
		LinkPort:    prof.LinkPort,
		ControlIp:   prof.ControlIp,
		SocketPath:  socketPath,
		Mcc:         prof.Mcc,
		Mnc:         prof.Mnc,
		Tac:         prof.Tac,
	}
	runningGNBs[profileName] = inst

	// Monitor the gNB and clean up when it exits
	go func() {
		<-errCh
		runningGNBsMu.Lock()
		delete(runningGNBs, profileName)
		runningGNBsMu.Unlock()
		logrus.Infof("[FLEET] gNB %s (%s) exited and cleaned up", profileName, prof.GnbId)
	}()

	logrus.Infof("[FLEET] Launched gNB profile '%s' (gNB-ID: %s)", profileName, prof.GnbId)
	return nil
}

// StopGNBProfile stops a running fleet gNB by profile name.
func StopGNBProfile(profileName string) error {
	runningGNBsMu.RLock()
	inst, ok := runningGNBs[profileName]
	runningGNBsMu.RUnlock()

	if !ok {
		return fmt.Errorf("no running gNB found with profile name '%s'", profileName)
	}

	inst.Cancel()
	logrus.Infof("[FLEET] Sent stop signal to gNB profile '%s'", profileName)
	return nil
}

// GetRunningGNBs returns all currently running gNB statuses.
func GetRunningGNBs() []RunningGNBStatus {
	runningGNBsMu.RLock()
	defer runningGNBsMu.RUnlock()

	ues := ueContext.GetAllActiveUEs()

	result := make([]RunningGNBStatus, 0, len(runningGNBs))
	for _, inst := range runningGNBs {
		connectedUes := make([]string, 0)
		for _, u := range ues {
			if u.GetGnbProfileName() == inst.ProfileName {
				connectedUes = append(connectedUes, fmt.Sprintf("UE-%d", u.GetUeId()))
			}
		}

		result = append(result, RunningGNBStatus{
			ProfileName:  inst.ProfileName,
			GnbId:        inst.GnbId,
			StartedAt:    inst.StartedAt.Format(time.RFC3339),
			State:        "running",
			LinkType:     inst.LinkType,
			LinkPort:     inst.LinkPort,
			ControlIp:    inst.ControlIp,
			SocketPath:   inst.SocketPath,
			Mcc:          inst.Mcc,
			Mnc:          inst.Mnc,
			Tac:          inst.Tac,
			ConnectedUes: connectedUes,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ProfileName < result[j].ProfileName
	})
	return result
}

// IsGNBProfileRunning returns true if a gNB profile is currently active.
func IsGNBProfileRunning(profileName string) bool {
	runningGNBsMu.RLock()
	defer runningGNBsMu.RUnlock()
	_, ok := runningGNBs[profileName]
	return ok
}

// ─── Fleet UE Launch ─────────────────────────────────────────────────────────

// nextFleetUEID generates the next available UE ID (1-254) not currently in use.
// It avoids IDs already taken by scenario-launched UEs in the active registry.
var fleetUEIDCounter uint32 = 100 // Fleet UEs start from ID 100 to avoid collisions with scenario UEs

func nextFleetUEID() (uint8, error) {
	for i := 0; i < 155; i++ {
		id := uint8(atomic.AddUint32(&fleetUEIDCounter, 1) % 256)
		if id == 0 {
			id = 1
		}
		if ueContext.GetActiveUE(id) == nil {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no available UE ID slots (max 155 simultaneous fleet UEs)")
}

// LaunchUEFromProfile registers and connects a UE from a named fleet profile.
// It returns the assigned UE ID or an error.
func LaunchUEFromProfile(profileName string, targetGnbProfile string) (uint8, error) {
	prof, ok := config.GetUEProfile(profileName)
	if !ok {
		return 0, fmt.Errorf("UE profile '%s' not found", profileName)
	}

	ueID, err := nextFleetUEID()
	if err != nil {
		return 0, err
	}

	cfg := config.BuildConfigFromUEProfile(prof)

	u := &ueContext.UEContext{}

	var linkType string = cfg.GNodeB.LinkType
	var linkPort int = cfg.GNodeB.LinkPort
	var controlIp string = cfg.GNodeB.ControlIF.Ip
	var socketPath string

	runningGNBsMu.RLock()
	var targetInstance *RunningGNBInstance
	if targetGnbProfile != "" {
		targetInstance = runningGNBs[targetGnbProfile]
	} else {
		// If not specified, and there is exactly one running GNB, use it
		if len(runningGNBs) == 1 {
			for _, inst := range runningGNBs {
				targetInstance = inst
			}
			logrus.Infof("[FLEET] No target specified, auto-selected the single running gNB: '%s'", targetInstance.ProfileName)
		}
	}

	if targetInstance != nil {
		linkType = targetInstance.LinkType
		linkPort = targetInstance.LinkPort
		controlIp = targetInstance.ControlIp
		socketPath = targetInstance.SocketPath
		logrus.Infof("[FLEET] Resolved target gNB '%s' (GNB-ID: %s, LinkType: %s, SocketPath/Port: %s/%d)", 
			targetInstance.ProfileName, targetInstance.GnbId, linkType, socketPath, linkPort)
	} else if targetGnbProfile != "" {
		runningGNBsMu.RUnlock()
		return 0, fmt.Errorf("target gNB profile '%s' is not running", targetGnbProfile)
	} else {
		if linkType == "unix" {
			socketPath = "/tmp/gnb.sock"
		}
		logrus.Warnf("[FLEET] No running fleet gNB resolved, defaulting to standard socket path: %s", socketPath)
	}
	runningGNBsMu.RUnlock()

	u.SetGnbLinkType(linkType)
	u.SetGnbLinkPort(linkPort)
	u.SetGnbControlIp(controlIp)
	if linkType == "unix" {
		if socketPath == "" {
			socketPath = "/tmp/gnb.sock"
		}
		u.SetGnbSocketPath(socketPath)
	}

	if targetInstance != nil {
		u.SetGnbId(targetInstance.GnbId)
		u.SetGnbProfileName(targetInstance.ProfileName)
	} else {
		u.SetGnbId("000001")
		u.SetGnbProfileName("gNB-Default")
	}

	u.NewRanUeContext(
		cfg.Ue.Msin,
		security.AlgCiphering128NEA0,
		security.AlgIntegrity128NIA2,
		cfg.Ue.Key,
		cfg.Ue.Opc,
		"c9e8763286b5b9ffbdf56e1297d0887b", // OP (default)
		cfg.Ue.Amf,
		cfg.Ue.Sqn,
		cfg.Ue.Hplmn.Mcc,
		cfg.Ue.Hplmn.Mnc,
		cfg.Ue.Dnn,
		cfg.Ue.PduSessionType,
		int32(cfg.Ue.Snssai.Sst),
		cfg.Ue.Snssai.Sd,
		ueID,
		cfg.Ue.PduSessions,
	)
	u.SetRegistrationType(nasMessage.RegistrationType5GSInitialRegistration)

	if err := service.InitConn(u); err != nil {
		ueContext.UnregisterUE(ueID)
		return 0, fmt.Errorf("failed to connect UE %d to gNB: %w", ueID, err)
	}

	// Trigger initial registration
	trigger.InitRegistration(u)
	logrus.Infof("[FLEET][UE %d] Registration procedure initiated (SUPI: %s)", ueID, u.GetSupi())

	// Wait up to 3.5s to verify 5GMM registration and PDU Session outcome
	startWait := time.Now()
	for time.Since(startWait) < 3500*time.Millisecond {
		if u.GetStateMM() == 0x01 { // 5GMM-REGISTERED
			defaultSess := u.GetPduSession(1)
			if defaultSess != nil && (defaultSess.State == ueContext.SM5G_PDU_SESSION_ACTIVE || defaultSess.Error != "") {
				break
			}
		}
		if u.GetMMError() != "" || u.GetSMError() != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if u.GetStateMM() != 0x01 {
		errMsg := u.GetMMError()
		if errMsg == "" {
			errMsg = fmt.Sprintf("AMF 5G Core (%s:%d) did not respond to 5GMM Registration Request (Timeout)", cfg.AMF.Ip, cfg.AMF.Port)
		}
		u.Terminate()
		ueContext.UnregisterUE(ueID)
		return 0, fmt.Errorf("UE %d Registration Failed: %s", ueID, errMsg)
	}

	// Check PDU Session outcome
	defaultSess := u.GetPduSession(1)
	if defaultSess != nil {
		if defaultSess.State == ueContext.SM5G_PDU_SESSION_ACTIVE_PENDING {
			if u.GetSMError() != "" {
				defaultSess.State = ueContext.SM5G_PDU_SESSION_INACTIVE
				defaultSess.Error = u.GetSMError()
			} else {
				defaultSess.State = ueContext.SM5G_PDU_SESSION_INACTIVE
				defaultSess.Error = fmt.Sprintf("PDU Session #1 Establishment Timed Out: 5G Core network (%s:%d) did not send 5GSM Accept or Resource Setup Request within 3.5s. Verify Core DNN ('%s'), S-NSSAI (SST: %d, SD: %s), UPF N4 tunnel, or gNB N3 GTP interface (2152).", cfg.AMF.Ip, cfg.AMF.Port, defaultSess.Dnn, defaultSess.Snssai.Sst, defaultSess.Snssai.Sd)
				u.SetSMError(defaultSess.Error)
			}
		}
		if defaultSess.Error != "" {
			logrus.Warnf("[FLEET] UE %d registered but PDU Session failed: %s", ueID, defaultSess.Error)
		}
	}

	logrus.Infof("[FLEET] Successfully launched UE profile '%s' as UE ID %d (SUPI: %s)", profileName, ueID, u.GetSupi())
	return ueID, nil
}

// ─── Fleet Running Summary ────────────────────────────────────────────────────

// FleetRunningSummary bundles active UEs and gNBs for the UI live view.
type FleetRunningSummary struct {
	RunningUEs  []UEStatus         `json:"runningUes"`
	RunningGNBs []RunningGNBStatus `json:"runningGnbs"`
}

// GetFleetRunningSummary collects all currently active fleet entities.
func GetFleetRunningSummary() FleetRunningSummary {
	ues := ueContext.GetAllActiveUEs()
	ueStatuses := make([]UEStatus, 0, len(ues))
	for _, u := range ues {
		resolveUeConnectionDetails(u)

		pduSessions := make([]PDUSessionStatus, 0)
		for _, sess := range u.PduSessions {
			// Synchronize global MM/SM error if session error was recorded or if MM/SM error exists
			if sess.Error == "" {
				if u.GetSMError() != "" {
					sess.Error = u.GetSMError()
				} else if u.GetMMError() != "" {
					sess.Error = u.GetMMError()
				}
			}

			// If UE 5GMM registration failed or is DEREGISTERED, PDU session CANNOT be pending
			if u.GetStateMM() == ueContext.MM5G_DEREGISTERED && (u.GetMMError() != "" || u.GetSMError() != "") {
				sess.State = ueContext.SM5G_PDU_SESSION_INACTIVE
				if sess.Error == "" {
					sess.Error = u.GetMMError()
				}
			}

			// Evaluate PDU session establishment timeout at 2.5s or force inactive if error exists
			if sess.State == ueContext.SM5G_PDU_SESSION_ACTIVE_PENDING {
				if sess.Error != "" {
					sess.State = ueContext.SM5G_PDU_SESSION_INACTIVE
				} else if sess.RequestedAt.IsZero() || time.Since(sess.RequestedAt) > 2500*time.Millisecond {
					sess.State = ueContext.SM5G_PDU_SESSION_INACTIVE
					sess.Error = fmt.Sprintf("PDU Session #%d Establishment Timed Out: AMF/SMF Core network did not send PDUSessionResourceSetupRequest or 5GSM Accept within 2.5s. Verify Core DNN ('%s'), S-NSSAI (SST: %d, SD: %s), UPF N4 tunnel, or gNB N3 GTP interface (2152).", sess.Id, sess.Dnn, sess.Snssai.Sst, sess.Snssai.Sd)
					u.SetSMError(sess.Error)
				}
			}

			pduSessions = append(pduSessions, PDUSessionStatus{
				ID:             sess.Id,
				UeIP:           u.GetIp(sess.Id),
				Dnn:            sess.Dnn,
				PduSessionType: sess.PduSessionType,
				Sst:            sess.Snssai.Sst,
				Sd:             sess.Snssai.Sd,
				State:          sess.State,
				StateDesc:      ueContext.GetStateSMDesc(sess.State),
				Error:          sess.Error,
			})
		}
		ueStatuses = append(ueStatuses, UEStatus{
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
			MmError:          u.GetMMError(),
			SmError:          u.GetSMError(),
		})
	}
	sort.Slice(ueStatuses, func(i, j int) bool {
		return ueStatuses[i].ID < ueStatuses[j].ID
	})

	return FleetRunningSummary{
		RunningUEs:  ueStatuses,
		RunningGNBs: GetRunningGNBs(),
	}
}

func resolveUeConnectionDetails(u *ueContext.UEContext) {
	runningGNBsMu.RLock()
	defer runningGNBsMu.RUnlock()

	ueLinkType := u.GetGnbLinkType()
	ueSocketPath := u.GetGnbSocketPath()
	uePort := u.GetGnbLinkPort()

	for _, inst := range runningGNBs {
		match := false
		if ueLinkType == "unix" && inst.LinkType == "unix" {
			if inst.SocketPath == ueSocketPath {
				match = true
			}
		} else if ueLinkType == "tcp" && inst.LinkType == "tcp" {
			if inst.LinkPort == uePort {
				match = true
			}
		}

		if match {
			u.SetGnbId(inst.GnbId)
			u.SetGnbProfileName(inst.ProfileName)

			// Resolve the AmfUeId from the GNB's UE pool
			if inst.GnbCtx != nil {
				inst.GnbCtx.RangeUePool(func(ranUeId int64, gUe *gnbContext.GNBUe) bool {
					uConn := u.GetUnixConn()
					gConn := gUe.GetUnixSocket()
					if uConn != nil && gConn != nil {
						// Compare networks and local-to-remote addresses (stable comparison)
						if gConn.LocalAddr().Network() == uConn.LocalAddr().Network() &&
							gConn.LocalAddr().String() == uConn.RemoteAddr().String() &&
							gConn.RemoteAddr().String() == uConn.LocalAddr().String() {
							if gUe.GetAmfUeId() != 0 {
								u.SetAmfUeId(gUe.GetAmfUeId())
							}
							return false // stop iteration
						}
					}
					return true
				})
			}
			return
		}
	}
}

// CleanUpAll terminates all running fleet UEs and gNBs.
func CleanUpAll() {
	logrus.Info("[FLEET] Cleaning up all running UEs and gNBs...")
	
	// Terminate UEs
	ues := ueContext.GetAllActiveUEs()
	for _, u := range ues {
		logrus.Infof("[FLEET] Terminating UE %d (SUPI: %s)", u.GetUeId(), u.GetSupi())
		u.Terminate()
	}

	// Terminate GNBs
	runningGNBsMu.Lock()
	for name, inst := range runningGNBs {
		logrus.Infof("[FLEET] Terminating gNB profile '%s'", name)
		inst.Cancel()
	}
	// Clear the map
	runningGNBs = make(map[string]*RunningGNBInstance)
	runningGNBsMu.Unlock()
}

type BatchLaunchUERequest struct {
	BaseProfileName  string `json:"baseProfileName"`
	Count            int    `json:"count"`
	TargetGnbProfile string `json:"targetGnbProfile"`
	StartMsin        string `json:"startMsin,omitempty"`
}

// BatchLaunchUE creates and connects multiple UEs in a single request.
func BatchLaunchUE(req BatchLaunchUERequest) ([]uint8, []string, error) {
	if req.Count <= 0 || req.Count > 100 {
		return nil, nil, fmt.Errorf("count must be between 1 and 100")
	}

	prof, ok := config.GetUEProfile(req.BaseProfileName)
	if !ok {
		return nil, nil, fmt.Errorf("base UE profile '%s' not found", req.BaseProfileName)
	}

	var successfulIDs []uint8
	var errorsList []string

	startMsinInt := 1
	if req.StartMsin != "" {
		fmt.Sscanf(req.StartMsin, "%d", &startMsinInt)
	} else if prof.Msin != "" {
		fmt.Sscanf(prof.Msin, "%d", &startMsinInt)
	}

	for i := 0; i < req.Count; i++ {
		currentMsin := fmt.Sprintf("%010d", startMsinInt+i)
		tempProfile := prof
		tempProfile.Name = fmt.Sprintf("%s-batch-%d-%d", prof.Name, time.Now().UnixNano()%1000, i+1)
		tempProfile.Msin = currentMsin

		_ = config.UpsertUEProfile(tempProfile)
		ueID, err := LaunchUEFromProfile(tempProfile.Name, req.TargetGnbProfile)
		_ = config.DeleteUEProfile(tempProfile.Name)

		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("UE #%d (MSIN %s) failed: %v", i+1, currentMsin, err))
		} else {
			successfulIDs = append(successfulIDs, ueID)
		}
	}

	return successfulIDs, errorsList, nil
}

// QuickStartAllGNBs starts all non-running gNB profiles.
func QuickStartAllGNBs() (int, []string) {
	fleet := config.GetFleet()
	startedCount := 0
	var errs []string

	for _, prof := range fleet.GNBProfiles {
		if !IsGNBProfileRunning(prof.Name) {
			if err := LaunchGNBProfile(prof.Name); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", prof.Name, err))
			} else {
				startedCount++
			}
		}
	}
	return startedCount, errs
}

// QuickStopAllGNBs stops all running fleet gNBs.
func QuickStopAllGNBs() int {
	runningGNBsMu.RLock()
	names := make([]string, 0, len(runningGNBs))
	for name := range runningGNBs {
		names = append(names, name)
	}
	runningGNBsMu.RUnlock()

	stopped := 0
	for _, name := range names {
		if err := StopGNBProfile(name); err == nil {
			stopped++
		}
	}
	return stopped
}

// AMFTestReport holds detailed diagnostic results for testing AMF SCTP link connectivity.
type AMFTestReport struct {
	ProfileName         string   `json:"profileName,omitempty"`
	AmfIp               string   `json:"amfIp"`
	AmfPort             int      `json:"amfPort"`
	ControlIp           string   `json:"controlIp"`
	PingSuccess         bool     `json:"pingSuccess"`
	KernelSctpSupported bool     `json:"kernelSctpSupported"`
	SctpConnected       bool     `json:"sctpConnected"`
	Error               string   `json:"error,omitempty"`
	Diagnostic          string   `json:"diagnostic"`
	SuggestedActions    []string `json:"suggestedActions"`
}

// DiagnoseGNBLaunchError converts raw transport/NGAP errors into structured human-readable troubleshooting guidance.
func DiagnoseGNBLaunchError(err error, prof config.GNBProfile) string {
	if err == nil {
		return ""
	}
	errStr := err.Error()
	errStrLower := strings.ToLower(errStr)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("❌ gNB '%s' Launch Failed: %s\n\n", prof.Name, errStr))
	sb.WriteString("🔍 3GPP NGAP / SCTP Diagnostics:\n")

	if strings.Contains(errStrLower, "protocol not supported") || strings.Contains(errStrLower, "eprotonosupport") {
		sb.WriteString("• ROOT CAUSE: Linux Kernel SCTP module is NOT loaded on this host system.\n")
		sb.WriteString("• EXPLANATION: ICMP ping works because it uses raw IP/ICMP, but 5G NGAP uses SCTP (IP protocol 132). Linux does not load kernel module 'sctp' by default on some network environments or Linux distros.\n")
		sb.WriteString("• ACTION REQUIRED: Run the following command on the host terminal to load SCTP kernel support:\n")
		sb.WriteString("    sudo modprobe sctp\n")
	} else if strings.Contains(errStrLower, "connection refused") || strings.Contains(errStrLower, "econnrefused") {
		sb.WriteString(fmt.Sprintf("• ROOT CAUSE: SCTP Connection Refused by 5G Core AMF at %s:%d.\n", prof.AmfIp, prof.AmfPort))
		sb.WriteString(fmt.Sprintf("• EXPLANATION: Host IP is pingable, but NO 5G Core AMF service (Open5GS, Free5GC, etc.) is listening on SCTP port %d, or the AMF is bound ONLY to loopback (127.0.0.1 / 127.0.0.18) rather than 0.0.0.0 / external interface.\n", prof.AmfPort))
		sb.WriteString("• ACTION REQUIRED:\n")
		sb.WriteString("  1. Check if AMF process is running:  ps aux | grep amf\n")
		sb.WriteString(fmt.Sprintf("  2. Verify AMF listening ports:      sudo ss -sctp -l  or  sudo netstat -sctp\n"))
		sb.WriteString(fmt.Sprintf("  3. Check AMF config (e.g. /etc/open5gs/amf.yaml or amfcfg.yaml) and ensure 'ngap.server' or 'ngapIp' is bound to 0.0.0.0 or %s.\n", prof.AmfIp))
	} else if strings.Contains(errStrLower, "timeout") || strings.Contains(errStrLower, "no route") || strings.Contains(errStrLower, "unreachable") || strings.Contains(errStrLower, "etimedout") {
		sb.WriteString(fmt.Sprintf("• ROOT CAUSE: SCTP packets to AMF at %s:%d timed out or were blocked by firewall.\n", prof.AmfIp, prof.AmfPort))
		sb.WriteString(fmt.Sprintf("• EXPLANATION: Ping (ICMP protocol 1) is allowed, but SCTP (IP protocol 132 / port %d) packets are blocked by ufw/iptables/firewalld or a network security group/router.\n", prof.AmfPort))
		sb.WriteString("• ACTION REQUIRED:\n")
		sb.WriteString(fmt.Sprintf("  1. Allow SCTP traffic on host firewall:  sudo ufw allow %d/sctp  or  sudo iptables -A INPUT -p sctp --dport %d -j ACCEPT\n", prof.AmfPort, prof.AmfPort))
		sb.WriteString("  2. If running across AWS/GCP/Docker/Subnets, ensure Security Groups pass IP Protocol 132 (SCTP).\n")
	} else if strings.Contains(errStrLower, "cannot assign requested address") || strings.Contains(errStrLower, "eaddrnotavail") {
		sb.WriteString(fmt.Sprintf("• ROOT CAUSE: Local Control IP '%s' is not assigned to any network interface on this machine.\n", prof.ControlIp))
		sb.WriteString(fmt.Sprintf("• ACTION REQUIRED: Edit gNB profile '%s' and change Control IF IP to 127.0.0.1 or an active local IP address.\n", prof.Name))
	} else if strings.Contains(errStrLower, "address already in use") || strings.Contains(errStrLower, "eaddrinuse") {
		sb.WriteString(fmt.Sprintf("• ROOT CAUSE: Control IF Port %d is already bound by another running process.\n", prof.ControlPort))
		sb.WriteString(fmt.Sprintf("• ACTION REQUIRED: Stop conflicting gNB or change Control IF Port in gNB profile.\n"))
	} else if strings.Contains(errStrLower, "unknown plmn") || strings.Contains(errStrLower, "ngsetupfailure") || strings.Contains(errStrLower, "plmn") {
		sb.WriteString(fmt.Sprintf("• ROOT CAUSE: 5G Core AMF (%s:%d) rejected gNB NG Setup Request (PLMN: MCC %s, MNC %s, TAC %s).\n", prof.AmfIp, prof.AmfPort, prof.Mcc, prof.Mnc, prof.Tac))
		sb.WriteString(fmt.Sprintf("• ACTION REQUIRED: Ensure MCC (%s) and MNC (%s) in gNB profile match the supported PLMN / TAI in your 5G Core AMF config.\n", prof.Mcc, prof.Mnc))
	} else {
		sb.WriteString("• GENERAL TROUBLESHOOTING CHECKLIST:\n")
		sb.WriteString("  1. Ensure SCTP kernel module is loaded:  sudo modprobe sctp\n")
		sb.WriteString(fmt.Sprintf("  2. Test SCTP connectivity manually:      nc -z -v -u %s %d\n", prof.AmfIp, prof.AmfPort))
		sb.WriteString("  3. Verify AMF service status and logs.\n")
	}

	return sb.String()
}

// TestAMFConnection performs a full multi-layer diagnostic check (ICMP, SCTP, Kernel Module) against a target AMF endpoint.
func TestAMFConnection(prof config.GNBProfile) AMFTestReport {
	report := AMFTestReport{
		ProfileName:      prof.Name,
		AmfIp:            prof.AmfIp,
		AmfPort:          prof.AmfPort,
		ControlIp:        prof.ControlIp,
		SuggestedActions: make([]string, 0),
	}

	// 1. Check ICMP ping (L3 IP reachability)
	cmd := exec.Command("ping", "-c", "1", "-W", "1", prof.AmfIp)
	if err := cmd.Run(); err == nil {
		report.PingSuccess = true
	}

	// 2. Check SCTP Dial
	remote := fmt.Sprintf("%s:%d", prof.AmfIp, prof.AmfPort)
	local := fmt.Sprintf("%s:0", prof.ControlIp)

	rem, errRem := sctp.ResolveSCTPAddr("sctp", remote)
	loc, errLoc := sctp.ResolveSCTPAddr("sctp", local)

	if errRem == nil && errLoc == nil {
		conn, errDial := sctp.DialSCTPExt("sctp", loc, rem, sctp.InitMsg{NumOstreams: 2, MaxInstreams: 2})
		if errDial == nil {
			report.KernelSctpSupported = true
			report.SctpConnected = true
			_ = conn.Close()
		} else {
			report.Error = errDial.Error()
			errLower := strings.ToLower(errDial.Error())
			if !strings.Contains(errLower, "protocol not supported") {
				report.KernelSctpSupported = true
			}
		}
	} else {
		if errLoc != nil {
			report.Error = fmt.Sprintf("Local SCTP address resolve error: %v", errLoc)
		} else {
			report.Error = fmt.Sprintf("Remote SCTP address resolve error: %v", errRem)
		}
	}

	if report.SctpConnected {
		report.Diagnostic = fmt.Sprintf("✅ SCTP Connection Successful! AMF Core at %s:%d is reachable and accepting 5G NGAP associations.", prof.AmfIp, prof.AmfPort)
	} else {
		report.Diagnostic = DiagnoseGNBLaunchError(fmt.Errorf("%s", report.Error), prof)
		if !report.KernelSctpSupported {
			report.SuggestedActions = append(report.SuggestedActions, "Run 'sudo modprobe sctp' on host terminal to load Linux SCTP module")
		}
		if report.PingSuccess && !report.SctpConnected {
			report.SuggestedActions = append(report.SuggestedActions, fmt.Sprintf("Verify AMF is listening on %s:%d (check /etc/open5gs/amf.yaml or amfcfg.yaml)", prof.AmfIp, prof.AmfPort))
			report.SuggestedActions = append(report.SuggestedActions, fmt.Sprintf("Allow SCTP protocol 132 on host firewall (sudo ufw allow %d/sctp)", prof.AmfPort))
		} else if !report.PingSuccess {
			report.SuggestedActions = append(report.SuggestedActions, fmt.Sprintf("Check network routing to %s (host IP is unreachable)", prof.AmfIp))
		}
	}

	return report
}


