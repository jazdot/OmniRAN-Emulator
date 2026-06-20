package webserver

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

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

	// Trigger initial registration in background
	go func() {
		trigger.InitRegistration(u)
		logrus.Infof("[FLEET][UE %d] Registration procedure complete (SUPI: %s)", ueID, u.GetSupi())
	}()

	logrus.Infof("[FLEET] Launched UE profile '%s' as UE ID %d (SUPI: %s)", profileName, ueID, u.GetSupi())
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
