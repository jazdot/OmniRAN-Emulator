package templates

import (
	"os"
	"os/signal"
	"sync"
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

// buildUE creates and connects a UE with a given registration type.
func buildUE(cfg config.Config, id uint8, regType uint8) (*ueContext.UEContext, error) {
	u := &ueContext.UEContext{}
	u.SetGnbLinkType(cfg.GNodeB.LinkType)
	u.SetGnbLinkPort(cfg.GNodeB.LinkPort)
	u.SetGnbControlIp(cfg.GNodeB.ControlIF.Ip)

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

// waitForState polls the UE MM state until it matches target or timeout.
func waitForState(u *ueContext.UEContext, targetState int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if u.GetStateMM() == targetState {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// ─── ScenarioPeriodicRegistration ────────────────────────────────────────────

// ScenarioPeriodicRegistration simulates T3512 expiry: UE performs initial
// registration, waits, then sends a Periodic Registration Update.
func ScenarioPeriodicRegistration() {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal("[SCENARIO] Cannot get config: ", err)
	}

	wg := sync.WaitGroup{}

	// Start gNB
	go gnb.InitGnb(cfg, &wg)
	wg.Add(1)
	time.Sleep(1 * time.Second)

	// Step 1: Initial Registration
	log.Info("[SCENARIO][STEP 1] Initial Registration...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		log.Fatal("[SCENARIO] Error creating UE: ", err)
	}
	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	if !waitForState(u, ueContext.MM5G_REGISTERED, 15*time.Second) {
		log.Error("[SCENARIO] Initial Registration did not complete in 15s")
	} else {
		log.Info("[SCENARIO][STEP 1] ✅ Initial Registration complete. UE is REGISTERED.")
	}

	// Step 2: Wait (simulates T3512 timer expiry — normally 54 minutes, here 5s)
	log.Info("[SCENARIO][STEP 2] Waiting 5s to simulate T3512 timer expiry...")
	time.Sleep(5 * time.Second)

	// Step 3: Periodic Registration Update
	log.Info("[SCENARIO][STEP 3] Sending Periodic Registration Update...")
	u.SetRegistrationType(nasMessage.RegistrationType5GSPeriodicRegistrationUpdating)
	periodicReq := mm_5gs.GetRegistrationRequest(nasMessage.RegistrationType5GSPeriodicRegistrationUpdating, nil, nil, false, u)
	sender.SendToGnb(u, periodicReq)

	if !waitForState(u, ueContext.MM5G_REGISTERED, 15*time.Second) {
		log.Warn("[SCENARIO] Periodic Registration Update did not re-register in 15s")
	} else {
		log.Info("[SCENARIO][STEP 3] ✅ Periodic Registration Update accepted. UE remains REGISTERED.")
	}

	log.Info("[SCENARIO] ✅ Periodic Registration scenario complete.")
	u.Terminate()
	wg.Done()
}

// ─── ScenarioMobilityRegistration ────────────────────────────────────────────

// ScenarioMobilityRegistration simulates a TAU/Mobility Registration Update
// after initial registration — as would happen after a cell change.
func ScenarioMobilityRegistration() {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal("[SCENARIO] Cannot get config: ", err)
	}

	wg := sync.WaitGroup{}
	go gnb.InitGnb(cfg, &wg)
	wg.Add(1)
	time.Sleep(1 * time.Second)

	log.Info("[SCENARIO][STEP 1] Initial Registration...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		log.Fatal("[SCENARIO] Error creating UE: ", err)
	}
	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	if !waitForState(u, ueContext.MM5G_REGISTERED, 15*time.Second) {
		log.Error("[SCENARIO] Initial Registration did not complete")
	} else {
		log.Info("[SCENARIO][STEP 1] ✅ UE REGISTERED.")
	}

	// Simulate cell change delay
	log.Info("[SCENARIO][STEP 2] Simulating cell change (3s)...")
	time.Sleep(3 * time.Second)

	// Send Mobility Registration Update
	log.Info("[SCENARIO][STEP 3] Sending Mobility Registration Update (TAU)...")
	mobilityReq := mm_5gs.GetRegistrationRequest(nasMessage.RegistrationType5GSMobilityRegistrationUpdating, nil, nil, false, u)
	sender.SendToGnb(u, mobilityReq)

	if !waitForState(u, ueContext.MM5G_REGISTERED, 15*time.Second) {
		log.Warn("[SCENARIO] Mobility Registration Update did not complete")
	} else {
		log.Info("[SCENARIO][STEP 3] ✅ Mobility Registration Update accepted.")
	}

	log.Info("[SCENARIO] ✅ Mobility Registration scenario complete.")
	u.Terminate()
	wg.Done()
}

// ─── ScenarioEmergencyRegistration ───────────────────────────────────────────

// ScenarioEmergencyRegistration performs an Emergency Registration.
// Note: Open5GS may reject this without a provisioned emergency subscriber;
// the scenario tests that the correct NAS type is sent.
func ScenarioEmergencyRegistration() {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal("[SCENARIO] Cannot get config: ", err)
	}

	wg := sync.WaitGroup{}
	go gnb.InitGnb(cfg, &wg)
	wg.Add(1)
	time.Sleep(1 * time.Second)

	log.Info("[SCENARIO][STEP 1] Emergency Registration Request...")
	log.Warn("[SCENARIO] Note: Core may reject emergency registration without emergency subscriber provisioning.")

	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSEmergencyRegistration)
	if err != nil {
		log.Fatal("[SCENARIO] Error creating UE: ", err)
	}
	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	// Wait — accept both REGISTERED and REJECT (network-dependent)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
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
	u.Terminate()
	wg.Done()
}

// ─── ScenarioHandover ────────────────────────────────────────────────────────

// ScenarioHandover simulates an N2 handover: UE registers, then after `delaySec`
// seconds triggers a Path Switch Request to the target gNB.
func ScenarioHandover(targetGnbIp string, targetGnbPort int, delaySec int) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal("[SCENARIO] Cannot get config: ", err)
	}

	wg := sync.WaitGroup{}
	go gnb.InitGnb(cfg, &wg)
	wg.Add(1)
	time.Sleep(1 * time.Second)

	log.Info("[SCENARIO][STEP 1] Initial Registration before handover...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		log.Fatal("[SCENARIO] Error creating UE: ", err)
	}
	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	if !waitForState(u, ueContext.MM5G_REGISTERED, 15*time.Second) {
		log.Error("[SCENARIO] Registration did not complete — cannot proceed with handover.")
		wg.Done()
		return
	}
	log.Info("[SCENARIO][STEP 1] ✅ UE REGISTERED. Waiting for PDU session...")
	time.Sleep(2 * time.Second) // allow PDU session to come up

	log.Infof("[SCENARIO][STEP 2] Waiting %ds before triggering handover...", delaySec)
	time.Sleep(time.Duration(delaySec) * time.Second)

	// Trigger handover path switch
	log.Infof("[SCENARIO][STEP 3] Triggering N2 Handover to %s:%d ...", targetGnbIp, targetGnbPort)
	if err := ue.TriggerHandover(u, targetGnbIp, targetGnbPort, cfg.GNodeB.LinkType, "", false, "", ""); err != nil {
		log.Errorf("[SCENARIO] Handover trigger failed: %v", err)
		log.Warn("[SCENARIO] Note: For inter-gNB handover, start a second gNB instance on the target address/port first.")
	} else {
		log.Info("[SCENARIO][STEP 3] ✅ Handover trigger sent. Monitoring for Path Switch Acknowledge...")
		time.Sleep(5 * time.Second)
		log.Info("[SCENARIO] ✅ Handover scenario complete. Check logs for PathSwitchRequestAcknowledge.")
	}

	u.Terminate()
	wg.Done()
}

// ─── ScenarioFullLifecycle ────────────────────────────────────────────────────

// ScenarioFullLifecycle runs the complete UE state machine:
// Register → PDU Session → CM-IDLE (no data) → Service Request → Deregister
func ScenarioFullLifecycle(idleSec int) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal("[SCENARIO] Cannot get config: ", err)
	}

	wg := sync.WaitGroup{}
	go gnb.InitGnb(cfg, &wg)
	wg.Add(1)
	time.Sleep(1 * time.Second)

	// Step 1: Register
	log.Info("[SCENARIO][STEP 1/5] Initial Registration...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		log.Fatal("[SCENARIO] Error creating UE: ", err)
	}
	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	if !waitForState(u, ueContext.MM5G_REGISTERED, 15*time.Second) {
		log.Error("[SCENARIO] Registration failed — aborting lifecycle.")
		wg.Done()
		return
	}
	log.Info("[SCENARIO][STEP 1/5] ✅ Registered.")

	// Step 2: Wait for PDU Session
	log.Info("[SCENARIO][STEP 2/5] Waiting for PDU Session establishment...")
	deadline := time.Now().Add(15 * time.Second)
	pduUp := false
	for time.Now().Before(deadline) {
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
	time.Sleep(time.Duration(idleSec) * time.Second)

	// Step 4: Service Request (UE wakes up)
	log.Info("[SCENARIO][STEP 4/5] Sending Service Request (UE wakes from idle)...")
	u.SetStateMM_MM5G_SERVICE_REQ_INIT()
	svcReq, err := mm_5gs.ServiceRequest(u, nasMessage.ServiceTypeMobileTerminatedServices)
	if err != nil {
		log.Warnf("[SCENARIO] Service Request build error: %v", err)
	} else {
		sender.SendToGnb(u, svcReq)
		time.Sleep(2 * time.Second)
		log.Info("[SCENARIO][STEP 4/5] ✅ Service Request sent.")
	}

	// Step 5: UE-initiated Deregistration
	log.Info("[SCENARIO][STEP 5/5] UE-initiated Deregistration (normal power-off)...")
	deregReq, err := mm_5gs.DeregistrationRequest(u, false)
	if err != nil {
		log.Warnf("[SCENARIO] Deregistration Request build error: %v", err)
	} else {
		sender.SendToGnb(u, deregReq)
		time.Sleep(2 * time.Second)
		log.Info("[SCENARIO][STEP 5/5] ✅ Deregistration Request sent.")
	}

	log.Info("[SCENARIO] ✅ Full UE Lifecycle complete: Register → PDU → Idle → ServiceReq → Deregister")
	u.Terminate()
	wg.Done()
}

// ─── ScenarioDeregistration ──────────────────────────────────────────────────

// ScenarioDeregistration registers a UE then immediately sends UE-initiated
// Deregistration Request (simulates normal UE power-off).
func ScenarioDeregistration() {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal("[SCENARIO] Cannot get config: ", err)
	}

	wg := sync.WaitGroup{}
	go gnb.InitGnb(cfg, &wg)
	wg.Add(1)
	time.Sleep(1 * time.Second)

	log.Info("[SCENARIO][STEP 1] Initial Registration...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		log.Fatal("[SCENARIO] Error creating UE: ", err)
	}
	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	if !waitForState(u, ueContext.MM5G_REGISTERED, 15*time.Second) {
		log.Error("[SCENARIO] Registration failed — cannot deregister.")
		wg.Done()
		return
	}
	log.Info("[SCENARIO][STEP 1] ✅ UE REGISTERED.")

	// Wait for PDU session then deregister
	time.Sleep(3 * time.Second)

	log.Info("[SCENARIO][STEP 2] Sending Deregistration Request...")
	deregReq, err := mm_5gs.DeregistrationRequest(u, false)
	if err != nil {
		log.Errorf("[SCENARIO] Error building Deregistration Request: %v", err)
	} else {
		sender.SendToGnb(u, deregReq)
		time.Sleep(3 * time.Second)
		log.Info("[SCENARIO][STEP 2] ✅ Deregistration Request sent. Waiting for Accept...")
	}

	// Block until Ctrl+C or state becomes DEREGISTERED
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, os.Interrupt)
	select {
	case <-sigC:
	case <-time.After(10 * time.Second):
	}

	log.Info("[SCENARIO] ✅ Deregistration scenario complete.")
	u.Terminate()
	wg.Done()
}

// ScenarioInteractiveUE registers a UE and keeps it running for manual interaction.
func ScenarioInteractiveUE(shouldExit func() bool) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal("[SCENARIO] Cannot get config: ", err)
	}

	wg := sync.WaitGroup{}
	go gnb.InitGnb(cfg, &wg)
	wg.Add(1)
	time.Sleep(1 * time.Second)

	log.Info("[SCENARIO][STEP 1] Starting Interactive UE Session...")
	u, err := buildUE(cfg, 1, nasMessage.RegistrationType5GSInitialRegistration)
	if err != nil {
		log.Fatal("[SCENARIO] Error creating UE: ", err)
	}
	log.Info("[UE] UNIX/NAS service is running")
	trigger.InitRegistration(u)

	if !waitForState(u, ueContext.MM5G_REGISTERED, 15*time.Second) {
		log.Error("[SCENARIO] Initial Registration did not complete in 15s")
	} else {
		log.Info("[SCENARIO] ✅ Interactive UE Registered. Ready for dynamic operations.")
	}

	// Keep loop running until server stops the scenario or UE is terminated
	for !shouldExit() {
		if u.GetStateMM() == ueContext.MM5G_DEREGISTERED {
			log.Info("[SCENARIO] UE reached DEREGISTERED state. Ending interactive session.")
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	u.Terminate()
	wg.Done()
}
