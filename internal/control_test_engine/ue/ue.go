package ue

import (
	"encoding/binary"
	"fmt"
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/service"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/trigger"
	"OmniRAN-Emulator/internal/monitoring"
	"OmniRAN-Emulator/lib/nas/nasMessage"
	"OmniRAN-Emulator/lib/nas/security"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
)

func RegistrationUe(conf config.Config, id uint8, wg *sync.WaitGroup) {

	// wg := sync.WaitGroup{}

	// new UE instance.
	ue := &context.UEContext{}
	ue.SetGnbLinkType(conf.GNodeB.LinkType)
	ue.SetGnbLinkPort(conf.GNodeB.LinkPort)
	ue.SetGnbControlIp(conf.GNodeB.ControlIF.Ip)

	// new UE context
	ue.NewRanUeContext(
		conf.Ue.Msin,
		security.AlgCiphering128NEA0,
		security.AlgIntegrity128NIA2,
		conf.Ue.Key,
		conf.Ue.Opc,
		"c9e8763286b5b9ffbdf56e1297d0887b",
		conf.Ue.Amf,
		conf.Ue.Sqn,
		conf.Ue.Hplmn.Mcc,
		conf.Ue.Hplmn.Mnc,
		conf.Ue.Dnn,
		conf.Ue.PduSessionType,
		int32(conf.Ue.Snssai.Sst),
		conf.Ue.Snssai.Sd,
		id,
		conf.Ue.PduSessions)

	// Parse registration type from config
	regType := nasMessage.RegistrationType5GSInitialRegistration
	switch conf.Ue.RegistrationType {
	case "mobility":
		regType = nasMessage.RegistrationType5GSMobilityRegistrationUpdating
	case "periodic":
		regType = nasMessage.RegistrationType5GSPeriodicRegistrationUpdating
	case "emergency":
		regType = nasMessage.RegistrationType5GSEmergencyRegistration
	}
	ue.SetRegistrationType(regType)

	// starting communication with GNB and listen.
	err := service.InitConn(ue)
	if err != nil {
		log.Fatal("Error in", err)
	} else {
		log.Info("[UE] UNIX/NAS service is running")
		// wg.Add(1)
	}

	// registration procedure started.
	trigger.InitRegistration(ue)

	// wg.Wait()

	// control the signals
	sigUe := make(chan os.Signal, 1)
	signal.Notify(sigUe, os.Interrupt)

	// Block until a signal is received.
	<-sigUe
	ue.Terminate()
	wg.Done()
	// os.Exit(0)

}

func RegistrationUeMonitor(conf config.Config,
	id uint8, monitor *monitoring.Monitor, wg *sync.WaitGroup, start time.Time) {

	// new UE instance.
	ue := &context.UEContext{}
	ue.SetGnbLinkType(conf.GNodeB.LinkType)
	ue.SetGnbLinkPort(conf.GNodeB.LinkPort)
	ue.SetGnbControlIp(conf.GNodeB.ControlIF.Ip)

	// new UE context
	ue.NewRanUeContext(
		conf.Ue.Msin,
		security.AlgCiphering128NEA0,
		security.AlgIntegrity128NIA2,
		conf.Ue.Key,
		conf.Ue.Opc,
		"c9e8763286b5b9ffbdf56e1297d0887b",
		conf.Ue.Amf,
		conf.Ue.Sqn,
		conf.Ue.Hplmn.Mcc,
		conf.Ue.Hplmn.Mnc,
		conf.Ue.Dnn,
		conf.Ue.PduSessionType,
		int32(conf.Ue.Snssai.Sst),
		conf.Ue.Snssai.Sd,
		id,
		conf.Ue.PduSessions)

	// Parse registration type from config
	regTypeMonitor := nasMessage.RegistrationType5GSInitialRegistration
	switch conf.Ue.RegistrationType {
	case "mobility":
		regTypeMonitor = nasMessage.RegistrationType5GSMobilityRegistrationUpdating
	case "periodic":
		regTypeMonitor = nasMessage.RegistrationType5GSPeriodicRegistrationUpdating
	case "emergency":
		regTypeMonitor = nasMessage.RegistrationType5GSEmergencyRegistration
	}
	ue.SetRegistrationType(regTypeMonitor)

	// starting communication with GNB and listen.
	err := service.InitConn(ue)
	if err != nil {
		log.Fatal("Error in", err)
	} else {
		log.Info("[UE] UNIX/NAS service is running")
		// wg.Add(1)
	}

	// registration procedure started.
	trigger.InitRegistration(ue)

	for {

		// UE is register in network
		if ue.GetStateMM() == 0x03 {
			elapsed := time.Since(start)
			monitor.LtRegisterLocal = elapsed.Milliseconds()
			log.Warn("[TESTER][UE] UE LATENCY IN REGISTRATION: ", monitor.LtRegisterLocal, " ms")
			break
		}

		// timeout is 10 000 ms
		if time.Since(start).Milliseconds() >= 10000 {
			log.Warn("[TESTER][UE] TIME EXPIRED IN UE REGISTRATION 10 000 ms")
			break
		}
	}

	wg.Done()
	ue.Terminate()
}

func TriggerHandover(ue *context.UEContext, targetGnbIp string, targetGnbPort int, targetGnbLinkType string, targetGnbSocketPath string, isXn bool, targetGnbId string, targetGnbName string) error {
	log.Infof("[UE] Initiating Handover to Target GNodeB: %s:%d (LinkType: %s, SocketPath: %s, isXn: %t, targetGnbId: %s, targetGnbName: %s)", targetGnbIp, targetGnbPort, targetGnbLinkType, targetGnbSocketPath, isXn, targetGnbId, targetGnbName)

	if !isXn {
		// N2 Handover Flow: Keep source connection open, set target params, send trigger to Source GNodeB
		oldConn := ue.GetUnixConn()
		if oldConn == nil {
			return fmt.Errorf("no active connection to Source GNodeB")
		}

		// Update GNodeB target parameters in context so we can dial when HandoverCommand is received
		ue.SetGnbLinkType(targetGnbLinkType)
		ue.SetGnbLinkPort(targetGnbPort)
		ue.SetGnbControlIp(targetGnbIp)
		if targetGnbLinkType == "unix" {
			ue.SetGnbSocketPath(targetGnbSocketPath)
		}
		if targetGnbId != "" {
			ue.SetGnbId(targetGnbId)
		}
		if targetGnbName != "" {
			ue.SetGnbProfileName(targetGnbName)
		}

		amfUeId := ue.GetAmfUeId()
		var pduSessionId uint8 = 1
		for id := range ue.PduSessions {
			pduSessionId = id
			break
		}

		// Format 28-byte trigger message
		triggerMsg := make([]byte, 28)
		triggerMsg[0] = 0x00
		triggerMsg[1] = 0x02
		binary.BigEndian.PutUint64(triggerMsg[2:10], uint64(amfUeId))
		triggerMsg[10] = pduSessionId

		mccBytes := []byte(ue.GetMcc())
		copy(triggerMsg[11:14], mccBytes)

		mncBytes := []byte(ue.GetMnc())
		if len(mncBytes) < 3 {
			// Pad with zero byte to keep fixed length
			mncBytes = append(mncBytes, 0x00)
		}
		copy(triggerMsg[14:17], mncBytes)

		// Set target TAC and Target gNB ID by parsing targetGnbId (default is 2 for target in scenario)
		val := int64(2)
		if targetGnbId != "" {
			if v, err := strconv.ParseInt(targetGnbId, 10, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseInt(targetGnbId, 16, 64); err == nil {
				val = v
			}
		}
		triggerMsg[17] = byte((val >> 16) & 0xff)
		triggerMsg[18] = byte((val >> 8) & 0xff)
		triggerMsg[19] = byte(val & 0xff)

		binary.BigEndian.PutUint64(triggerMsg[20:28], uint64(val))

		_, err := oldConn.Write(triggerMsg)
		if err != nil {
			return fmt.Errorf("error writing N2 handover trigger to Source GNodeB: %w", err)
		}

		log.Infof("[UE] N2 Handover trigger sent successfully to Source GNodeB. AMF UE ID: %d, Target gNB ID: %d", amfUeId, val)
		return nil
	}

	// Legacy / Xn Handover Flow: Close source conn, connect to target immediately
	oldConn := ue.GetUnixConn()
	if oldConn != nil {
		_ = oldConn.Close()
	}

	// Establish new connection to Target GNodeB
	var conn net.Conn
	var err error

	if targetGnbLinkType == "tcp" {
		addr := fmt.Sprintf("%s:%d", targetGnbIp, targetGnbPort)
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			return fmt.Errorf("error connecting to Target GNodeB via TCP: %w", err)
		}
	} else {
		socketPath := targetGnbSocketPath
		if socketPath == "" {
			socketPath = "/tmp/gnb.sock"
		}
		dialer := net.Dialer{
			LocalAddr: &net.UnixAddr{
				Name: fmt.Sprintf("@ue_%d", ue.GetUeId()),
				Net:  "unix",
			},
		}
		for i := 0; i < 10; i++ {
			conn, err = dialer.Dial("unix", socketPath)
			if err == nil {
				break
			}
			if i < 9 {
				log.Warnf("[UE] Dial Target GNodeB UNIX socket failed: %v. Retrying in 100ms...", err)
				time.Sleep(100 * time.Millisecond)
			}
		}
		if err != nil {
			return fmt.Errorf("error connecting to Target GNodeB via UNIX socket %s after retries: %w", socketPath, err)
		}
	}

	// Set new connection
	ue.SetUnixConn(conn)

	// Update GNodeB connection parameters in context
	ue.SetGnbLinkType(targetGnbLinkType)
	ue.SetGnbLinkPort(targetGnbPort)
	ue.SetGnbControlIp(targetGnbIp)
	if targetGnbLinkType == "unix" {
		ue.SetGnbSocketPath(targetGnbSocketPath)
	}
	if targetGnbId != "" {
		ue.SetGnbId(targetGnbId)
	}
	if targetGnbName != "" {
		ue.SetGnbProfileName(targetGnbName)
	}

	// Start listening on the new connection
	go service.UeListen(ue)

	// Send Handover Path Switch Trigger: [0x00, 0x03] [AMF UE ID (8 bytes)] [PDU Session ID (1 byte)]
	amfUeId := ue.GetAmfUeId()
	var pduSessionId uint8 = 1
	for id := range ue.PduSessions {
		pduSessionId = id
		break
	}

	triggerMsg := make([]byte, 11)
	triggerMsg[0] = 0x00
	triggerMsg[1] = 0x03 // Xn Handover Trigger
	binary.BigEndian.PutUint64(triggerMsg[2:10], uint64(amfUeId))
	triggerMsg[10] = pduSessionId

	_, err = conn.Write(triggerMsg)
	if err != nil {
		return fmt.Errorf("error writing Xn handover trigger to target GNodeB: %w", err)
	}

	log.Infof("[UE] Xn Handover trigger sent successfully to target GNodeB. AMF UE ID: %d, PDU Session ID: %d", amfUeId, pduSessionId)
	return nil
}
