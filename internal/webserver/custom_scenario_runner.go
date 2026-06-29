package webserver

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/chaos"
	"OmniRAN-Emulator/internal/control_test_engine/gnb"
	gnbContext "OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/interface_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/pdu_session_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/ue_context_management"
	gnbSender "OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/sender"
	"OmniRAN-Emulator/internal/control_test_engine/ue"
	ueContext "OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control/mm_5gs"
	ueSender "OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/sender"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/service"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/trigger"
	"OmniRAN-Emulator/lib/aper"
	"OmniRAN-Emulator/lib/nas/nasMessage"
	"OmniRAN-Emulator/lib/nas/security"
	"OmniRAN-Emulator/lib/ngap/ngapType"

	log "github.com/sirupsen/logrus"
)

type CustomStep struct {
	Type   string                 `json:"type"` // "start_gnb", "start_ue", "wait_ue_state", "trigger_handover", "chaos_inject", "sleep"
	Params map[string]interface{} `json:"params"`
}

type CustomScenario struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Steps       []CustomStep `json:"steps"`
}

type CustomRunnerState struct {
	mu          sync.RWMutex
	Name        string       `json:"name"`
	Status      string       `json:"status"` // "idle", "running", "success", "failed"
	CurrentStep int          `json:"currentStep"`
	TotalSteps  int          `json:"totalSteps"`
	Logs        []string     `json:"logs"`
	Error       string       `json:"error"`
	StartTime   time.Time    `json:"startTime"`
	EndTime     time.Time    `json:"endTime"`

	cancelFunc stdctx.CancelFunc
	activeUes  map[uint8]*ueContext.UEContext
	sockets    []string
}

var GlobalCustomRunner = &CustomRunnerState{
	Status:    "idle",
	activeUes: make(map[uint8]*ueContext.UEContext),
}

func (s *CustomRunnerState) AddLog(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Logs = append(s.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	log.Infof("[CUSTOM_SCENARIO] %s", msg)
}

func (s *CustomRunnerState) SetError(errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Error = errMsg
	s.Status = "failed"
	s.EndTime = time.Now()
}

func (s *CustomRunnerState) GetStateJSON() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, _ := json.Marshal(s)
	return data
}

// Stop terminates the currently running custom scenario.
func (s *CustomRunnerState) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status != "running" {
		return
	}

	s.Logs = append(s.Logs, fmt.Sprintf("[%s] Scenario aborted by user.", time.Now().Format("15:04:05")))
	s.Status = "failed"
	s.Error = "Scenario cancelled by user"
	s.EndTime = time.Now()

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	for id, u := range s.activeUes {
		u.Terminate()
		delete(s.activeUes, id)
	}

	for _, sock := range s.sockets {
		_ = os.Remove(sock)
	}
	s.sockets = nil
}

// Run executes the custom scenario in the background.
func (s *CustomRunnerState) Run(scen CustomScenario) {
	s.mu.Lock()
	s.Name = scen.Name
	s.Status = "running"
	s.CurrentStep = 0
	s.TotalSteps = len(scen.Steps)
	s.Logs = []string{}
	s.Error = ""
	s.StartTime = time.Now()
	s.activeUes = make(map[uint8]*ueContext.UEContext)
	s.sockets = []string{}

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	s.cancelFunc = cancel
	s.mu.Unlock()

	s.AddLog(fmt.Sprintf("Starting custom scenario: %s", scen.Name))

	go func() {
		defer func() {
			s.mu.Lock()
			if s.Status == "running" {
				s.Status = "success"
				s.EndTime = time.Now()
				s.AddLog("✅ Custom scenario completed successfully.")
			}
			cancel()
			for id, u := range s.activeUes {
				u.Terminate()
				delete(s.activeUes, id)
			}
			for _, sock := range s.sockets {
				_ = os.Remove(sock)
			}
			s.sockets = nil
			s.mu.Unlock()
		}()

		cfg, err := config.GetConfig()
		if err != nil {
			s.SetError(fmt.Sprintf("Failed to get configurations: %v", err))
			return
		}

		for i, step := range scen.Steps {
			select {
			case <-ctx.Done():
				return
			default:
			}

			s.mu.Lock()
			s.CurrentStep = i + 1
			s.mu.Unlock()

			s.AddLog(fmt.Sprintf("[Step %d/%d] Executing action: %s", i+1, len(scen.Steps), step.Type))

			switch step.Type {
			case "start_gnb":
				gnbId, _ := step.Params["id"].(string)
				tac, _ := step.Params["tac"].(string)
				socketPath, _ := step.Params["socketPath"].(string)
				if gnbId == "" {
					gnbId = "000001"
				}
				if tac == "" {
					tac = "000001"
				}
				if socketPath == "" {
					socketPath = "/tmp/gnb.sock"
				}

				// Clean up previous socket if exists
				_ = os.Remove(socketPath)
				s.mu.Lock()
				s.sockets = append(s.sockets, socketPath)
				s.mu.Unlock()

				cfgGnb := cfg
				cfgGnb.GNodeB.PlmnList.GnbId = gnbId
				cfgGnb.GNodeB.PlmnList.Tac = tac
				cfgGnb.GNodeB.LinkType = "unix"

				if p, ok := step.Params["port"].(float64); ok {
					cfgGnb.GNodeB.ControlIF.Port = int(p)
				}
				if p, ok := step.Params["dataPort"].(float64); ok {
					cfgGnb.GNodeB.DataIF.Port = int(p)
				}
				if p, ok := step.Params["linkPort"].(float64); ok {
					cfgGnb.GNodeB.LinkPort = int(p)
				}

				s.AddLog(fmt.Sprintf("Launching GNodeB %s (TAC: %s) on socket %s", gnbId, tac, socketPath))
				_, _ = gnb.InitGnbFleet(cfgGnb, ctx, socketPath)
				time.Sleep(1 * time.Second) // wait for server socket bind

			case "start_ue":
				ueIdVal, _ := step.Params["ueId"].(float64)
				ueId := uint8(ueIdVal)
				if ueId == 0 {
					ueId = 1
				}
				gnbSocket, _ := step.Params["gnbSocket"].(string)
				if gnbSocket == "" {
					gnbSocket = "/tmp/gnb.sock"
				}

				regTypeStr, _ := step.Params["regType"].(string)
				regType := nasMessage.RegistrationType5GSInitialRegistration
				switch regTypeStr {
				case "mobility":
					regType = nasMessage.RegistrationType5GSMobilityRegistrationUpdating
				case "periodic":
					regType = nasMessage.RegistrationType5GSPeriodicRegistrationUpdating
				case "emergency":
					regType = nasMessage.RegistrationType5GSEmergencyRegistration
				}

				s.AddLog(fmt.Sprintf("Building and registering UE %d on GNodeB socket %s", ueId, gnbSocket))
				
				u := &ueContext.UEContext{}
				u.SetGnbLinkType("unix")
				u.SetGnbSocketPath(gnbSocket)
				u.SetGnbControlIp(cfg.GNodeB.ControlIF.Ip)
				u.SetGnbLinkPort(cfg.GNodeB.LinkPort)

				u.NewRanUeContext(
					cfg.Ue.Msin,
					security.AlgCiphering128NEA0,
					security.AlgIntegrity128NIA2,
					cfg.Ue.Key,
					cfg.Ue.Opc,
					"c9e8763286b5b9ffbdf56e1297d0887b",
					cfg.Ue.Amf,
					cfg.Ue.Sqn,
					cfg.Ue.Hplmn.Mcc,
					cfg.Ue.Hplmn.Mnc,
					cfg.Ue.Dnn,
					cfg.Ue.PduSessionType,
					int32(cfg.Ue.Snssai.Sst),
					cfg.Ue.Snssai.Sd,
					ueId,
					cfg.Ue.PduSessions,
				)
				u.SetRegistrationType(regType)

				if err := service.InitConn(u); err != nil {
					s.SetError(fmt.Sprintf("UE %d connection init failed: %v", ueId, err))
					return
				}

				s.mu.Lock()
				s.activeUes[ueId] = u
				s.mu.Unlock()

				trigger.InitRegistration(u)

			case "wait_ue_state":
				ueIdVal, _ := step.Params["ueId"].(float64)
				ueId := uint8(ueIdVal)
				targetStateStr, _ := step.Params["state"].(string)
				timeoutSec, _ := step.Params["timeout"].(float64)
				if timeoutSec == 0 {
					timeoutSec = 15
				}

				targetState := ueContext.MM5G_REGISTERED
				if targetStateStr == "MM5G_DEREGISTERED" {
					targetState = ueContext.MM5G_DEREGISTERED
				}

				s.AddLog(fmt.Sprintf("Waiting for UE %d to reach state %s (timeout: %.0fs)...", ueId, targetStateStr, timeoutSec))
				
				s.mu.RLock()
				u, exists := s.activeUes[ueId]
				s.mu.RUnlock()

				if !exists {
					s.SetError(fmt.Sprintf("UE %d not found in active context", ueId))
					return
				}

				deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
				matched := false
				for time.Now().Before(deadline) {
					select {
					case <-ctx.Done():
						return
					default:
					}
					if u.GetStateMM() == targetState {
						matched = true
						break
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !matched {
					s.SetError(fmt.Sprintf("Timeout: UE %d failed to reach state %s", ueId, targetStateStr))
					return
				}
				s.AddLog(fmt.Sprintf("UE %d successfully reached state %s.", ueId, targetStateStr))

			case "trigger_handover":
				ueIdVal, _ := step.Params["ueId"].(float64)
				ueId := uint8(ueIdVal)
				targetGnbIp, _ := step.Params["targetGnbIp"].(string)
				targetGnbPortVal, _ := step.Params["targetGnbPort"].(float64)
				targetGnbSocket, _ := step.Params["targetGnbSocket"].(string)
				isXn, _ := step.Params["isXn"].(bool)
				targetGnbId, _ := step.Params["targetGnbId"].(string)
				targetGnbName, _ := step.Params["targetGnbName"].(string)

				if targetGnbIp == "" {
					targetGnbIp = "127.0.0.1"
				}
				if targetGnbPortVal == 0 {
					targetGnbPortVal = 9499
				}
				if targetGnbSocket == "" {
					targetGnbSocket = "/tmp/gnb_target.sock"
				}

				s.AddLog(fmt.Sprintf("Triggering handover for UE %d -> target GNodeB %s (isXn: %t)", ueId, targetGnbId, isXn))

				s.mu.RLock()
				u, exists := s.activeUes[ueId]
				s.mu.RUnlock()

				if !exists {
					s.SetError(fmt.Sprintf("UE %d not found in active context", ueId))
					return
				}

				err := ue.TriggerHandover(u, targetGnbIp, int(targetGnbPortVal), "unix", targetGnbSocket, isXn, targetGnbId, targetGnbName)
				if err != nil {
					s.SetError(fmt.Sprintf("Handover failed: %v", err))
					return
				}
				s.AddLog("Handover trigger successfully sent.")

			case "chaos_inject":
				target, _ := step.Params["target"].(string) // "nas" or "ngap"
				dropRate, _ := step.Params["dropRate"].(float64)
				delayMs, _ := step.Params["delayMs"].(float64)
				msgType, _ := step.Params["msgType"].(string)
				enabled, _ := step.Params["enabled"].(bool)

				chaosCfg := chaos.ChaosConfig{
					DropProbability: dropRate,
					DelayDuration:   time.Duration(delayMs) * time.Millisecond,
					TargetMsgType:   msgType,
					Enabled:         enabled,
				}

				if target == "nas" {
					ueIdVal, _ := step.Params["ueId"].(float64)
					ueId := uint8(ueIdVal)
					s.AddLog(fmt.Sprintf("Configuring NAS chaos for UE %d: drop=%.2f, delay=%.0fms, type=%s, enabled=%t",
						ueId, dropRate, delayMs, msgType, enabled))
					chaos.GlobalChaosManager.ConfigureNas(ueId, chaosCfg)
				} else {
					gnbId, _ := step.Params["gnbId"].(string)
					s.AddLog(fmt.Sprintf("Configuring NGAP chaos for GNodeB %s: drop=%.2f, delay=%.0fms, type=%s, enabled=%t",
						gnbId, dropRate, delayMs, msgType, enabled))
					chaos.GlobalChaosManager.ConfigureNgap(gnbId, chaosCfg)
				}

			case "ue_nas_trigger":
				ueIdVal, _ := step.Params["ueId"].(float64)
				ueId := uint8(ueIdVal)
				msgType, _ := step.Params["msgType"].(string)

				s.mu.RLock()
				u, exists := s.activeUes[ueId]
				s.mu.RUnlock()

				if !exists {
					s.SetError(fmt.Sprintf("UE %d not found in active context", ueId))
					return
				}

				// Active Release Override support
				if rel, ok := step.Params["release"].(string); ok && rel != "" {
					config.SetActiveRelease(rel)
					s.AddLog(fmt.Sprintf("Setting active 3GPP release to Rel %s", rel))
				}

				// Dynamically switch GNodeB connection if requested (for mobility)
				if socket, ok := step.Params["switchGnbSocket"].(string); ok && socket != "" {
					s.AddLog(fmt.Sprintf("UE %d switching connection to GNodeB socket: %s", ueId, socket))
					u.SetGnbSocketPath(socket)
					u.SetGnbLinkType("unix")
					
					// Resolve target profile details
					if gnbId, ok := step.Params["switchGnbId"].(string); ok {
						u.SetGnbId(gnbId)
					}
					if profile, ok := step.Params["switchGnbProfile"].(string); ok {
						u.SetGnbProfileName(profile)
					}

					if u.GetUnixConn() != nil {
						_ = u.GetUnixConn().Close()
					}
					if err := service.InitConn(u); err != nil {
						s.SetError(fmt.Sprintf("UE %d GNodeB reconnect failed: %v", ueId, err))
						return
					}
				}

				var payload []byte
				var err error

				pduSessIdVal, _ := step.Params["pduSessionId"].(float64)
				pduSessId := uint8(pduSessIdVal)
				if pduSessId == 0 {
					pduSessId = 1
				}

				s.AddLog(fmt.Sprintf("UE %d triggering NAS message: %s", ueId, msgType))

				switch msgType {
				case "registration_request":
					regType := u.GetRegistrationType()
					if rt, ok := step.Params["registrationType"].(float64); ok {
						regType = uint8(rt)
					}
					payload = mm_5gs.GetRegistrationRequest(regType, nil, nil, false, u)
					u.SetStateMM_DEREGISTERED()

				case "service_request":
					svcType := nasMessage.ServiceTypeSignalling
					if st, ok := step.Params["serviceType"].(float64); ok {
						svcType = uint8(st)
					}
					u.SetStateMM_MM5G_SERVICE_REQ_INIT()
					payload, err = mm_5gs.ServiceRequest(u, svcType)

				case "deregistration_request":
					switchOff := false
					if so, ok := step.Params["switchOff"].(bool); ok {
						switchOff = so
					}
					payload, err = mm_5gs.DeregistrationRequest(u, switchOff)

				case "pdu_establishment":
					dnn, _ := step.Params["dnn"].(string)
					sstVal, _ := step.Params["sst"].(float64)
					sd, _ := step.Params["sd"].(string)
					sessType, _ := step.Params["sessionType"].(string)

					if dnn == "" {
						dnn = u.PduSession.Dnn
					}
					if sstVal == 0 {
						sstVal = float64(u.PduSession.Snssai.Sst)
					}
					if sd == "" {
						sd = u.PduSession.Snssai.Sd
					}
					if sessType == "" {
						sessType = u.PduSession.PduSessionType
					}

					u.SetupPduSession(pduSessId, dnn, sessType, int32(sstVal), sd)
					payload, err = mm_5gs.UlNasTransport(u, pduSessId, nasMessage.ULNASTransportRequestTypeInitialRequest)
					if err == nil {
						u.GetPduSession(pduSessId).State = ueContext.SM5G_PDU_SESSION_ACTIVE_PENDING
					}

				case "pdu_modification":
					sess := u.GetPduSession(pduSessId)
					if sess == nil {
						s.SetError(fmt.Sprintf("PDU Session %d not active on UE %d", pduSessId, ueId))
						return
					}
					payload, err = mm_5gs.UlNasTransportModification(u, pduSessId)

				case "pdu_release":
					sess := u.GetPduSession(pduSessId)
					if sess == nil {
						s.SetError(fmt.Sprintf("PDU Session %d not active on UE %d", pduSessId, ueId))
						return
					}
					payload, err = mm_5gs.UlNasTransportRelease(u, pduSessId)

				default:
					s.SetError(fmt.Sprintf("Unknown NAS message type: %s", msgType))
					return
				}

				if err != nil {
					s.SetError(fmt.Sprintf("Failed to build NAS message %s: %v", msgType, err))
					return
				}

				ueSender.SendToGnb(u, payload)
				time.Sleep(100 * time.Millisecond)

			case "gnb_ngap_trigger":
				gnbIdStr, _ := step.Params["gnbId"].(string)
				if gnbIdStr == "" {
					gnbIdStr = "000001"
				}
				msgType, _ := step.Params["msgType"].(string)

				gnbContext.ActiveGNBsMu.RLock()
				g, ok := gnbContext.ActiveGNBs[gnbIdStr]
				gnbContext.ActiveGNBsMu.RUnlock()

				if !ok || g == nil {
					s.SetError(fmt.Sprintf("GNodeB %s not found in ActiveGNBs pool", gnbIdStr))
					return
				}

				var payload []byte
				var err error

				s.AddLog(fmt.Sprintf("GNodeB %s triggering NGAP message: %s", gnbIdStr, msgType))

				switch msgType {
				case "ng_setup_request":
					payload, err = interface_management.NGSetupRequest(g, "OmniRANEmulator")

				case "ue_context_release_request":
					ranUeIdVal, _ := step.Params["ranUeId"].(float64)
					amfUeIdVal, _ := step.Params["amfUeId"].(float64)
					causeType, _ := step.Params["causeType"].(string)
					causeVal, _ := step.Params["causeValue"].(float64)

					var cause *ngapType.Cause
					if causeType != "" {
						cause = buildNgapCause(causeType, int64(causeVal))
					}
					payload, err = ue_context_management.GetUEContextReleaseRequest(int64(ranUeIdVal), int64(amfUeIdVal), cause)

				case "error_indication":
					var ranUeIdPtr, amfUeIdPtr *int64
					if ranVal, ok := step.Params["ranUeId"].(float64); ok {
						val := int64(ranVal)
						ranUeIdPtr = &val
					}
					if amfVal, ok := step.Params["amfUeId"].(float64); ok {
						val := int64(amfVal)
						amfUeIdPtr = &val
					}
					causeType, _ := step.Params["causeType"].(string)
					causeVal, _ := step.Params["causeValue"].(float64)

					var cause *ngapType.Cause
					if causeType != "" {
						cause = buildNgapCause(causeType, int64(causeVal))
					}
					payload, err = interface_management.ErrorIndication(amfUeIdPtr, ranUeIdPtr, cause)

				case "pdu_session_resource_modify_response":
					ranUeIdVal, _ := step.Params["ranUeId"].(float64)
					gUe, errGue := g.GetGnbUe(int64(ranUeIdVal))
					if errGue != nil || gUe == nil {
						s.SetError(fmt.Sprintf("GNBUe %d not found on GNodeB %s", int64(ranUeIdVal), gnbIdStr))
						return
					}

					var modifySessIds []int64
					if idsList, ok := step.Params["modifySessIds"].([]interface{}); ok {
						for _, idVal := range idsList {
							if idFloat, ok := idVal.(float64); ok {
								modifySessIds = append(modifySessIds, int64(idFloat))
							}
						}
					}
					if len(modifySessIds) == 0 {
						modifySessIds = append(modifySessIds, 1)
					}

					payload, err = pdu_session_management.PDUSessionResourceModifyResponse(gUe, modifySessIds)

				default:
					s.SetError(fmt.Sprintf("Unknown NGAP message type: %s", msgType))
					return
				}

				if err != nil {
					s.SetError(fmt.Sprintf("Failed to build NGAP message %s: %v", msgType, err))
					return
				}

				// Resolve target SCTP association (AMF connection)
				targetAmf := g.GetActiveAmf()
				if targetAmf == nil || targetAmf.GetSCTPConn() == nil {
					s.SetError(fmt.Sprintf("GNodeB %s N2 connection to AMF is not active", gnbIdStr))
					return
				}

				err = gnbSender.SendToAmF(payload, targetAmf.GetSCTPConn())
				if err != nil {
					s.SetError(fmt.Sprintf("Failed to send NGAP message %s: %v", msgType, err))
					return
				}
				time.Sleep(100 * time.Millisecond)

			case "sleep":
				seconds, _ := step.Params["seconds"].(float64)
				if seconds == 0 {
					seconds = 1
				}
				s.AddLog(fmt.Sprintf("Sleeping for %.0fs...", seconds))
				time.Sleep(time.Duration(seconds) * time.Second)
			}
		}
	}()
}

func buildNgapCause(causeType string, causeVal int64) *ngapType.Cause {
	cause := new(ngapType.Cause)
	switch causeType {
	case "radioNetwork":
		cause.Present = ngapType.CausePresentRadioNetwork
		cause.RadioNetwork = new(ngapType.CauseRadioNetwork)
		cause.RadioNetwork.Value = aper.Enumerated(causeVal)
	case "nas":
		cause.Present = ngapType.CausePresentNas
		cause.Nas = new(ngapType.CauseNas)
		cause.Nas.Value = aper.Enumerated(causeVal)
	case "protocol":
		cause.Present = ngapType.CausePresentProtocol
		cause.Protocol = new(ngapType.CauseProtocol)
		cause.Protocol.Value = aper.Enumerated(causeVal)
	case "transport":
		cause.Present = ngapType.CausePresentTransport
		cause.Transport = new(ngapType.CauseTransport)
		cause.Transport.Value = aper.Enumerated(causeVal)
	case "misc":
		cause.Present = ngapType.CausePresentMisc
		cause.Misc = new(ngapType.CauseMisc)
		cause.Misc.Value = aper.Enumerated(causeVal)
	default:
		return nil
	}
	return cause
}
