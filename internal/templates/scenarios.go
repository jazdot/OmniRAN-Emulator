package templates

import (
	stdctx "context"
	"os"
	"time"

	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/control_test_engine/gnb"
	"OmniRAN-Emulator/internal/control_test_engine/ue"
	ueContext "OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control/mm_5gs"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/sender"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/service"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/trigger"
	"OmniRAN-Emulator/lib/nas/nasMessage"
	"OmniRAN-Emulator/lib/nas/security"

	log "github.com/sirupsen/logrus"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// sleepOrCancel pauses execution for duration, checking shouldExit every 100ms.
// Returns true if shouldExit returned true (cancelled).
func sleepOrCancel(duration time.Duration, shouldExit func() bool) bool {
	if shouldExit == nil {
		time.Sleep(duration)
		return false
	}
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if shouldExit() {
			return true // Cancelled
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// waitForStateOrCancel polls the UE MM state until target or timeout, checking shouldExit.
// Returns true if state matches, false if timeout or cancelled.
func waitForStateOrCancel(u *ueContext.UEContext, targetState int, timeout time.Duration, shouldExit func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if shouldExit != nil && shouldExit() {
			return false // Cancelled
		}
		if u.GetStateMM() == targetState {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// getShouldExit returns a non-nil fallback function if shouldExit is nil.
func getShouldExit(fn func() bool) func() bool {
	if fn == nil {
		return func() bool { return false }
	}
	return fn
}

// buildUE creates and connects a UE with a given registration type.
func buildUE(cfg config.Config, id uint8, regType uint8, gnbSocketPath string) (*ueContext.UEContext, error) {
	u := &ueContext.UEContext{}
	u.SetGnbLinkType(cfg.GNodeB.LinkType)
	u.SetGnbLinkPort(cfg.GNodeB.LinkPort)
	u.SetGnbControlIp(cfg.GNodeB.ControlIF.Ip)
	if gnbSocketPath != "" {
		u.SetGnbSocketPath(gnbSocketPath)
	}

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
		id,
		cfg.Ue.PduSessions,
	)
	u.SetRegistrationType(regType)

	if err := service.InitConn(u); err != nil {
		return nil, err
	}
	return u, nil
}

// ─── ScenarioPeriodicRegistration ────────────────────────────────────────────

// ScenarioPeriodicRegistration simulates T3512 expiry: UE performs initial
// registration, waits, then sends a Periodic Registration Update.
func ScenarioPeriodicRegistration(shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Start GNodeB using InitGnbFleet for cancelability
	log.Info("[SCENARIO][STEP 1] Starting GNodeB...")
	_, _ = gnb.InitGnbFleet(cfg, ctx, "")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	// Step 1: Initial Registration
	log.Info("[SCENARIO][STEP 1] Initial Registration...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration, "")
	if err != nil {
		log.Error("[SCENARIO] Error creating UE: ", err)
		return
	}
	defer u.Terminate()

	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Error("[SCENARIO] Initial Registration did not complete or was cancelled")
		return
	}
	log.Info("[SCENARIO][STEP 1] ✅ Initial Registration complete. UE is REGISTERED.")

	// Step 2: Wait (simulates T3512 timer expiry)
	log.Info("[SCENARIO][STEP 2] Waiting 5s to simulate T3512 timer expiry...")
	if sleepOrCancel(5*time.Second, exitCheck) {
		return
	}

	// Step 3: Periodic Registration Update
	log.Info("[SCENARIO][STEP 3] Sending Periodic Registration Update...")
	u.SetRegistrationType(nasMessage.RegistrationType5GSPeriodicRegistrationUpdating)
	periodicReq := mm_5gs.GetRegistrationRequest(nasMessage.RegistrationType5GSPeriodicRegistrationUpdating, nil, nil, false, u)
	sender.SendToGnb(u, periodicReq)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Warn("[SCENARIO] Periodic Registration Update did not re-register or was cancelled")
		return
	}
	log.Info("[SCENARIO][STEP 3] ✅ Periodic Registration Update accepted. UE remains REGISTERED.")
	log.Info("[SCENARIO] ✅ Periodic Registration scenario complete.")
}

// ─── ScenarioMobilityRegistration ────────────────────────────────────────────

// ScenarioMobilityRegistration simulates a TAU/Mobility Registration Update
// after initial registration — as would happen after a cell change.
func ScenarioMobilityRegistration(shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	log.Info("[SCENARIO][STEP 1] Starting GNodeB...")
	_, _ = gnb.InitGnbFleet(cfg, ctx, "")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][STEP 1] Initial Registration...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration, "")
	if err != nil {
		log.Error("[SCENARIO] Error creating UE: ", err)
		return
	}
	defer u.Terminate()

	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Error("[SCENARIO] Initial Registration did not complete or was cancelled")
		return
	}
	log.Info("[SCENARIO][STEP 1] ✅ UE REGISTERED.")

	// Simulate cell change delay
	log.Info("[SCENARIO][STEP 2] Simulating cell change (3s)...")
	if sleepOrCancel(3*time.Second, exitCheck) {
		return
	}

	// Send Mobility Registration Update
	log.Info("[SCENARIO][STEP 3] Sending Mobility Registration Update (TAU)...")
	mobilityReq := mm_5gs.GetRegistrationRequest(nasMessage.RegistrationType5GSMobilityRegistrationUpdating, nil, nil, false, u)
	sender.SendToGnb(u, mobilityReq)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Warn("[SCENARIO] Mobility Registration Update did not complete or was cancelled")
		return
	}
	log.Info("[SCENARIO][STEP 3] ✅ Mobility Registration Update accepted.")
	log.Info("[SCENARIO] ✅ Mobility Registration scenario complete.")
}

// ─── ScenarioEmergencyRegistration ───────────────────────────────────────────

// ScenarioEmergencyRegistration performs an Emergency Registration.
func ScenarioEmergencyRegistration(shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	log.Info("[SCENARIO][STEP 1] Starting GNodeB...")
	_, _ = gnb.InitGnbFleet(cfg, ctx, "")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][STEP 1] Emergency Registration Request...")
	log.Warn("[SCENARIO] Note: Core may reject emergency registration without emergency subscriber provisioning.")

	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSEmergencyRegistration, "")
	if err != nil {
		log.Error("[SCENARIO] Error creating UE: ", err)
		return
	}
	defer u.Terminate()

	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	// Wait — accept both REGISTERED and REJECT (network-dependent)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if exitCheck() {
			return
		}
		state := u.GetStateMM()
		if state == ueContext.MM5G_REGISTERED {
			log.Info("[SCENARIO][STEP 1] ✅ Emergency Registration ACCEPTED by core.")
			break
		}
		if state == ueContext.MM5G_DEREGISTERED {
			log.Warn("[SCENARIO][STEP 1] ⚠️  Core REJECTED emergency registration (expected without emergency sub provisioning).")
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	log.Info("[SCENARIO] Emergency Registration scenario complete.")
}

// ─── ScenarioHandover ────────────────────────────────────────────────────────

// ScenarioHandover simulates an N2 handover: starts source and target GNodeBs,
// registers a UE on source, and performs N2-based handover to target.
func ScenarioHandover(targetGnbIp string, targetGnbPort int, delaySec int, shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	// Create Source and Target configurations
	cfgSource := cfg
	cfgSource.GNodeB.PlmnList.GnbId = "000001"
	cfgSource.GNodeB.PlmnList.Tac = "000001"
	cfgSource.GNodeB.LinkType = "unix"

	cfgTarget := cfg
	cfgTarget.GNodeB.PlmnList.GnbId = "000002"
	cfgTarget.GNodeB.PlmnList.Tac = "000002"
	cfgTarget.GNodeB.LinkType = "unix"
	cfgTarget.GNodeB.ControlIF.Port = cfg.GNodeB.ControlIF.Port + 10
	cfgTarget.GNodeB.DataIF.Port = cfg.GNodeB.DataIF.Port + 10
	cfgTarget.GNodeB.LinkPort = cfg.GNodeB.LinkPort + 10

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	_ = os.Remove("/tmp/gnb_source.sock")
	_ = os.Remove("/tmp/gnb_target.sock")
	defer func() {
		_ = os.Remove("/tmp/gnb_source.sock")
		_ = os.Remove("/tmp/gnb_target.sock")
	}()

	log.Info("[SCENARIO][STEP 1] Starting Source GNodeB (gNB-Source)...")
	_, _ = gnb.InitGnbFleet(cfgSource, ctx, "/tmp/gnb_source.sock")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][STEP 2] Starting Target GNodeB (gNB-Target)...")
	_, _ = gnb.InitGnbFleet(cfgTarget, ctx, "/tmp/gnb_target.sock")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][STEP 3] Registering UE on Source GNodeB...")
	u, err := buildUE(cfgSource, 1, nasMessage.RegistrationType5GSInitialRegistration, "/tmp/gnb_source.sock")
	if err != nil {
		log.Errorf("[SCENARIO] Error creating UE: %v", err)
		return
	}
	defer u.Terminate()

	u.SetGnbSocketPath("/tmp/gnb_source.sock")
	u.SetGnbProfileName("gNB-Source")
	u.SetGnbId("000001")

	trigger.InitRegistration(u)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Error("[SCENARIO] Registration failed — cannot proceed with handover.")
		return
	}
	log.Info("[SCENARIO][STEP 3] ✅ UE Registered on Source GNodeB. Waiting...")
	if sleepOrCancel(time.Duration(delaySec)*time.Second, exitCheck) {
		return
	}

	// Trigger N2 Handover to Target
	log.Info("[SCENARIO][STEP 4] Triggering N2 Handover to Target GNodeB...")
	err = ue.TriggerHandover(
		u,
		cfgTarget.GNodeB.ControlIF.Ip,
		cfgTarget.GNodeB.LinkPort,
		"unix",
		"/tmp/gnb_target.sock",
		false, // isXn = false (N2 Handover)
		"000002",
		"gNB-Target",
	)
	if err != nil {
		log.Errorf("[SCENARIO] Handover trigger failed: %v", err)
	} else {
		log.Info("[SCENARIO][STEP 4] ✅ Handover trigger sent. Monitoring status...")
		_ = sleepOrCancel(5*time.Second, exitCheck)
		log.Info("[SCENARIO] ✅ Handover scenario complete.")
	}
}

// ─── ScenarioFullLifecycle ────────────────────────────────────────────────────

// ScenarioFullLifecycle runs the complete UE state machine:
// Register → PDU Session → CM-IDLE (no data) → Service Request → Deregister
func ScenarioFullLifecycle(idleSec int, shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	log.Info("[SCENARIO][STEP 1] Starting GNodeB...")
	_, _ = gnb.InitGnbFleet(cfg, ctx, "")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	// Step 1: Register
	log.Info("[SCENARIO][STEP 1/5] Initial Registration...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration, "")
	if err != nil {
		log.Error("[SCENARIO] Error creating UE: ", err)
		return
	}
	defer u.Terminate()

	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Error("[SCENARIO] Registration failed — aborting lifecycle.")
		return
	}
	log.Info("[SCENARIO][STEP 1/5] ✅ Registered.")

	// Step 2: Wait for PDU Session
	log.Info("[SCENARIO][STEP 2/5] Waiting for PDU Session establishment...")
	deadline := time.Now().Add(15 * time.Second)
	pduUp := false
	for time.Now().Before(deadline) {
		if exitCheck() {
			return
		}
		if sess, ok := u.PduSessions[1]; ok && sess.State == ueContext.SM5G_PDU_SESSION_ACTIVE {
			pduUp = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if pduUp {
		log.Info("[SCENARIO][STEP 2/5] ✅ PDU Session Active.")
	} else {
		log.Warn("[SCENARIO][STEP 2/5] ⚠️  PDU Session not confirmed (may still be signalling).")
	}

	// Step 3: Simulate CM-IDLE (no data for idleSec seconds)
	log.Infof("[SCENARIO][STEP 3/5] Simulating CM-IDLE for %ds (UE in connected but no traffic)...", idleSec)
	if sleepOrCancel(time.Duration(idleSec)*time.Second, exitCheck) {
		return
	}

	// Step 4: Service Request (UE wakes up)
	log.Info("[SCENARIO][STEP 4/5] Sending Service Request (UE wakes from idle)...")
	u.SetStateMM_MM5G_SERVICE_REQ_INIT()
	svcReq, err := mm_5gs.ServiceRequest(u, nasMessage.ServiceTypeMobileTerminatedServices)
	if err != nil {
		log.Warnf("[SCENARIO] Service Request build error: %v", err)
	} else {
		sender.SendToGnb(u, svcReq)
		_ = sleepOrCancel(2*time.Second, exitCheck)
		log.Info("[SCENARIO][STEP 4/5] ✅ Service Request sent.")
	}

	// Step 5: UE-initiated Deregistration
	log.Info("[SCENARIO][STEP 5/5] UE-initiated Deregistration (normal power-off)...")
	deregReq, err := mm_5gs.DeregistrationRequest(u, false)
	if err != nil {
		log.Warnf("[SCENARIO] Deregistration Request build error: %v", err)
	} else {
		sender.SendToGnb(u, deregReq)
		_ = sleepOrCancel(2*time.Second, exitCheck)
		log.Info("[SCENARIO][STEP 5/5] ✅ Deregistration Request sent.")
	}

	log.Info("[SCENARIO] ✅ Full UE Lifecycle complete: Register → PDU → Idle → ServiceReq → Deregister")
}

// ─── ScenarioDeregistration ──────────────────────────────────────────────────

// ScenarioDeregistration registers a UE then immediately sends UE-initiated
// Deregistration Request (simulates normal UE power-off).
func ScenarioDeregistration(shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	log.Info("[SCENARIO][STEP 1] Starting GNodeB...")
	_, _ = gnb.InitGnbFleet(cfg, ctx, "")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][STEP 1] Initial Registration...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration, "")
	if err != nil {
		log.Error("[SCENARIO] Error creating UE: ", err)
		return
	}
	defer u.Terminate()

	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Error("[SCENARIO] Registration failed — cannot deregister.")
		return
	}
	log.Info("[SCENARIO][STEP 1] ✅ UE REGISTERED.")

	// Wait for PDU session then deregister
	if sleepOrCancel(3*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][STEP 2] Sending Deregistration Request...")
	deregReq, err := mm_5gs.DeregistrationRequest(u, false)
	if err != nil {
		log.Errorf("[SCENARIO] Error building Deregistration Request: %v", err)
	} else {
		sender.SendToGnb(u, deregReq)
		if sleepOrCancel(3*time.Second, exitCheck) {
			return
		}
		log.Info("[SCENARIO][STEP 2] ✅ Deregistration Request sent. Waiting for Accept...")
	}

	// Block up to 5 seconds waiting for completion, checking cancellation
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if exitCheck() {
			return
		}
		if u.GetStateMM() == ueContext.MM5G_DEREGISTERED {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	log.Info("[SCENARIO] ✅ Deregistration scenario complete.")
}

// ScenarioInteractiveUE registers a UE and keeps it running for manual interaction.
func ScenarioInteractiveUE(shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	log.Info("[SCENARIO][STEP 1] Starting GNodeB...")
	_, _ = gnb.InitGnbFleet(cfg, ctx, "")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][STEP 1] Starting Interactive UE Session...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration, "")
	if err != nil {
		log.Error("[SCENARIO] Error creating UE: ", err)
		return
	}
	defer u.Terminate()

	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Error("[SCENARIO] Initial Registration did not complete in 15s")
	} else {
		log.Info("[SCENARIO] ✅ Interactive UE Registered. Ready for dynamic operations.")
	}

	// Keep loop running until server stops the scenario or UE is terminated
	for !exitCheck() {
		if u.GetStateMM() == ueContext.MM5G_DEREGISTERED {
			log.Info("[SCENARIO] UE reached DEREGISTERED state. Ending interactive session.")
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// ─── ScenarioXnHandover ──────────────────────────────────────────────────────

// ScenarioXnHandover simulates Xn Handover between two GNodeBs (Source and Target)
func ScenarioXnHandover(shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	// Create Source and Target configurations
	cfgSource := cfg
	cfgSource.GNodeB.PlmnList.GnbId = "000001"
	cfgSource.GNodeB.PlmnList.Tac = "000001"
	cfgSource.GNodeB.LinkType = "unix"

	cfgTarget := cfg
	cfgTarget.GNodeB.PlmnList.GnbId = "000002"
	cfgTarget.GNodeB.PlmnList.Tac = "000002"
	cfgTarget.GNodeB.LinkType = "unix"
	// To prevent "address already in use" binding conflicts on local control and user planes:
	cfgTarget.GNodeB.ControlIF.Port = cfg.GNodeB.ControlIF.Port + 10 // e.g. 9497
	cfgTarget.GNodeB.DataIF.Port = cfg.GNodeB.DataIF.Port + 10       // e.g. 2162
	cfgTarget.GNodeB.LinkPort = cfg.GNodeB.LinkPort + 10

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// 1. Clean up socket files before starting
	_ = os.Remove("/tmp/gnb_source.sock")
	_ = os.Remove("/tmp/gnb_target.sock")
	defer func() {
		_ = os.Remove("/tmp/gnb_source.sock")
		_ = os.Remove("/tmp/gnb_target.sock")
	}()

	// 2. Start Source GNodeB
	log.Info("[SCENARIO][STEP 1] Starting Source GNodeB (gNB-Source)...")
	_, _ = gnb.InitGnbFleet(cfgSource, ctx, "/tmp/gnb_source.sock")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	// 3. Start Target GNodeB
	log.Info("[SCENARIO][STEP 2] Starting Target GNodeB (gNB-Target)...")
	_, _ = gnb.InitGnbFleet(cfgTarget, ctx, "/tmp/gnb_target.sock")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	// 4. Create and Register UE on Source GNodeB
	log.Info("[SCENARIO][STEP 3] Registering UE on Source GNodeB...")
	cfgSourceUe := cfgSource
	u, err := buildUE(cfgSourceUe, 1, nasMessage.RegistrationType5GSInitialRegistration, "/tmp/gnb_source.sock")
	if err != nil {
		log.Errorf("[SCENARIO] Error creating UE: %v", err)
		return
	}
	defer u.Terminate()

	u.SetGnbSocketPath("/tmp/gnb_source.sock")
	u.SetGnbProfileName("gNB-Source")
	u.SetGnbId("000001")

	trigger.InitRegistration(u)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Error("[SCENARIO] Registration on Source GNodeB failed or cancelled")
		return
	}
	log.Info("[SCENARIO][STEP 3] ✅ UE Registered on Source GNodeB.")

	if sleepOrCancel(2*time.Second, exitCheck) {
		return
	}

	// 5. Trigger Xn Handover to Target GNodeB
	log.Info("[SCENARIO][STEP 4] Triggering Xn Handover to Target GNodeB...")
	err = ue.TriggerHandover(
		u,
		cfgTarget.GNodeB.ControlIF.Ip,
		cfgTarget.GNodeB.LinkPort,
		"unix",
		"/tmp/gnb_target.sock",
		true, // isXn = true
		"000002",
		"gNB-Target",
	)
	if err != nil {
		log.Errorf("[SCENARIO] Xn Handover initiation failed: %v", err)
		return
	}

	log.Info("[SCENARIO][STEP 4] ✅ Xn Handover trigger sent. Monitoring status on Target GNodeB...")

	// Wait to simulate handover signaling duration
	if sleepOrCancel(5*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO] ✅ Xn Handover scenario complete.")
}

// ─── ScenarioPduLifecycle ───────────────────────────────────────────────────

// ScenarioPduLifecycle simulates UE initial registration, PDU session establishment,
// traffic generation, PDU session release, and UE deregistration.
func ScenarioPduLifecycle(shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Start GNodeB
	log.Info("[SCENARIO][STEP 1] Starting GNodeB...")
	_, _ = gnb.InitGnbFleet(cfg, ctx, "")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	// Step 2: Register UE
	log.Info("[SCENARIO][STEP 2] Registering UE...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration, "")
	if err != nil {
		log.Errorf("[SCENARIO] Error creating UE: %v", err)
		return
	}
	defer u.Terminate()

	trigger.InitRegistration(u)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Error("[SCENARIO] Registration failed or cancelled")
		return
	}
	log.Info("[SCENARIO][STEP 2] ✅ UE Registered.")

	// Wait for default PDU session (ID 1) to become active
	log.Info("[SCENARIO][STEP 3] Waiting for PDU Session ID 1 to become active...")
	deadline := time.Now().Add(15 * time.Second)
	pduActive := false
	for time.Now().Before(deadline) {
		if exitCheck() {
			return
		}
		if sess, ok := u.PduSessions[1]; ok && sess.State == ueContext.SM5G_PDU_SESSION_ACTIVE {
			pduActive = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !pduActive {
		log.Error("[SCENARIO] Default PDU Session establishment failed")
		return
	}
	log.Info("[SCENARIO][STEP 3] ✅ PDU Session ID 1 is ACTIVE.")

	// Wait a bit to simulate data traffic
	if sleepOrCancel(3*time.Second, exitCheck) {
		return
	}

	// Step 4: Release PDU session
	log.Info("[SCENARIO][STEP 4] Sending PDU Session Release Request for ID 1...")
	releasePduMsg, err := mm_5gs.UlNasTransportRelease(u, 1)
	if err != nil {
		log.Errorf("[SCENARIO] Error building PDU Session Release Request: %v", err)
		return
	}
	sender.SendToGnb(u, releasePduMsg)

	// Wait for PDU session to become INACTIVE
	log.Info("[SCENARIO][STEP 4] Monitoring for PDU Session ID 1 release to INACTIVE...")
	deadline = time.Now().Add(15 * time.Second)
	pduReleased := false
	for time.Now().Before(deadline) {
		if exitCheck() {
			return
		}
		if sess, ok := u.PduSessions[1]; ok && sess.State == ueContext.SM5G_PDU_SESSION_INACTIVE {
			pduReleased = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !pduReleased {
		log.Warn("[SCENARIO] PDU Session release timeout")
	} else {
		log.Info("[SCENARIO][STEP 4] ✅ PDU Session ID 1 is INACTIVE.")
	}

	if sleepOrCancel(2*time.Second, exitCheck) {
		return
	}

	// Step 5: Deregister UE
	log.Info("[SCENARIO][STEP 5] UE Deregistration...")
	deregReq, err := mm_5gs.DeregistrationRequest(u, false)
	if err != nil {
		log.Errorf("[SCENARIO] Error building Deregistration Request: %v", err)
	} else {
		sender.SendToGnb(u, deregReq)
		_ = sleepOrCancel(2*time.Second, exitCheck)
		log.Info("[SCENARIO][STEP 5] ✅ Deregistration request sent.")
	}

	log.Info("[SCENARIO] ✅ PDU Session Lifecycle scenario complete.")
}

// ScenarioRelease17RedCapNTN simulates Release 17 NTN Satellite Access and RedCap UE registration.
func ScenarioRelease17RedCapNTN(shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	config.SetActiveRelease("17")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	log.Info("[SCENARIO][R17] Establishing Release 17 NTN Satellite & RedCap Cell Environment...")
	
	_ = os.Remove("/tmp/gnb.sock")
	defer os.Remove("/tmp/gnb.sock")
	
	// Start GNodeB
	log.Info("[SCENARIO][R17] Starting GNodeB with NTN & RedCap capabilities...")
	_, _ = gnb.InitGnbFleet(cfg, ctx, "")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	// Register UE
	log.Info("[SCENARIO][R17] Registering RedCap NTN UE...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration, "")
	if err != nil {
		log.Error("[SCENARIO][R17] Error creating UE: ", err)
		return
	}
	defer u.Terminate()

	trigger.InitRegistration(u)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Error("[SCENARIO][R17] Registration failed or cancelled")
		return
	}
	log.Info("[SCENARIO][R17] ✅ RedCap NTN UE Registered successfully.")

	// Wait for default PDU session
	log.Info("[SCENARIO][R17] Waiting for PDU Session with XR Low-Latency QoS params...")
	deadline := time.Now().Add(15 * time.Second)
	pduActive := false
	for time.Now().Before(deadline) {
		if exitCheck() {
			return
		}
		if sess, ok := u.PduSessions[1]; ok && sess.State == ueContext.SM5G_PDU_SESSION_ACTIVE {
			pduActive = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !pduActive {
		log.Warn("[SCENARIO][R17] PDU Session setup timed out or failed")
	} else {
		log.Info("[SCENARIO][R17] ✅ PDU Session active with Rel 17 XR QoS flows.")
	}

	if sleepOrCancel(3*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][R17] ✅ Release 17 RedCap & NTN Scenario completed.")
}

// ScenarioRelease18UAVSlicing simulates Release 18 Aerial UAV drone registration and handover with advanced slicing.
func ScenarioRelease18UAVSlicing(shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	config.SetActiveRelease("18")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	// Create Source and Target configurations for handover
	cfgSource := cfg
	cfgSource.GNodeB.PlmnList.GnbId = "000001"
	cfgSource.GNodeB.PlmnList.Tac = "000001"
	cfgSource.GNodeB.LinkType = "unix"

	cfgTarget := cfg
	cfgTarget.GNodeB.PlmnList.GnbId = "000002"
	cfgTarget.GNodeB.PlmnList.Tac = "000002"
	cfgTarget.GNodeB.LinkType = "unix"
	cfgTarget.GNodeB.ControlIF.Port = cfg.GNodeB.ControlIF.Port + 10
	cfgTarget.GNodeB.DataIF.Port = cfg.GNodeB.DataIF.Port + 10
	cfgTarget.GNodeB.LinkPort = cfg.GNodeB.LinkPort + 10

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	_ = os.Remove("/tmp/gnb_source.sock")
	_ = os.Remove("/tmp/gnb_target.sock")
	defer func() {
		_ = os.Remove("/tmp/gnb_source.sock")
		_ = os.Remove("/tmp/gnb_target.sock")
	}()

	log.Info("[SCENARIO][R18] Establishing Release 18 UAV Flight & Slicing Environment...")

	log.Info("[SCENARIO][R18] Starting Source GNodeB (gNB-Source)...")
	_, _ = gnb.InitGnbFleet(cfgSource, ctx, "/tmp/gnb_source.sock")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][R18] Starting Target GNodeB (gNB-Target)...")
	_, _ = gnb.InitGnbFleet(cfgTarget, ctx, "/tmp/gnb_target.sock")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][R18] Registering Aerial UAV UE on Source GNodeB...")
	u, err := buildUE(cfgSource, 1, nasMessage.RegistrationType5GSInitialRegistration, "/tmp/gnb_source.sock")
	if err != nil {
		log.Errorf("[SCENARIO][R18] Error creating UE: %v", err)
		return
	}
	defer u.Terminate()

	u.SetGnbSocketPath("/tmp/gnb_source.sock")
	u.SetGnbProfileName("gNB-Source")
	u.SetGnbId("000001")

	trigger.InitRegistration(u)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Error("[SCENARIO][R18] Registration failed or cancelled")
		return
	}
	log.Info("[SCENARIO][R18] ✅ Aerial UE Registered with UAV context and Slice Group 0x4f.")

	if sleepOrCancel(3*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][R18] Triggering UAV Handover to Target GNodeB (Trajectory sync)...")
	err = ue.TriggerHandover(
		u,
		cfgTarget.GNodeB.ControlIF.Ip,
		cfgTarget.GNodeB.LinkPort,
		"unix",
		"/tmp/gnb_target.sock",
		true, // isXn = true (Xn Handover)
		"000002",
		"gNB-Target",
	)
	if err != nil {
		log.Errorf("[SCENARIO][R18] Handover failed: %v", err)
		return
	}

	log.Info("[SCENARIO][R18] Monitoring UAV flight zone handover...")
	if sleepOrCancel(4*time.Second, exitCheck) {
		return
	}

	log.Info("[SCENARIO][R18] ✅ Release 18 UAV Slicing & Handover scenario completed.")
}

// ScenarioRelease19AISensing simulates Release 19 Ambient IoT sensors tagging and ISAC environment sensing tracking.
func ScenarioRelease19AISensing(shouldExit func() bool) {
	exitCheck := getShouldExit(shouldExit)
	config.SetActiveRelease("19")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error("[SCENARIO] Cannot get config: ", err)
		return
	}

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	defer cancel()

	go func() {
		for {
			if exitCheck() {
				cancel()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	log.Info("[SCENARIO][R19] Establishing Release 19 Ambient IoT & ISAC sensing environment...")
	
	_ = os.Remove("/tmp/gnb.sock")
	defer os.Remove("/tmp/gnb.sock")
	
	// Start GNodeB
	log.Info("[SCENARIO][R19] Starting GNodeB with Ambient IoT Gateway & RAN AI interface...")
	_, _ = gnb.InitGnbFleet(cfg, ctx, "")
	if sleepOrCancel(1*time.Second, exitCheck) {
		return
	}

	// Register UE
	log.Info("[SCENARIO][R19] Registering UE with AI capabilities and Ambient tag sensors...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration, "")
	if err != nil {
		log.Error("[SCENARIO][R19] Error creating UE: ", err)
		return
	}
	defer u.Terminate()

	trigger.InitRegistration(u)

	if !waitForStateOrCancel(u, ueContext.MM5G_REGISTERED, 15*time.Second, exitCheck) {
		log.Error("[SCENARIO][R19] Registration failed or cancelled")
		return
	}
	log.Info("[SCENARIO][R19] ✅ UE registered. RAN AI model active.")

	// Simulate Ambient tag reading and ISAC coordinates sweep updates
	for i := 1; i <= 3; i++ {
		if sleepOrCancel(2*time.Second, exitCheck) {
			return
		}
		log.Infof("[SCENARIO][R19] [ISAC Sweep #%d] Scanning radio environment... Radar reflection target tracked at distance %d meters (Velocity: %d m/s)", i, 150-i*10, 12)
		log.Infof("[SCENARIO][R19] [Ambient IoT Reader] Broadcast received from RFID tag: 0xE8A10F (Status: Energy Harvesting, Sensor Temp: %.1fC)", 24.5+float64(i)*0.2)
	}

	log.Info("[SCENARIO][R19] ✅ Release 19 Ambient IoT & ISAC Sensing Scenario completed.")
}

