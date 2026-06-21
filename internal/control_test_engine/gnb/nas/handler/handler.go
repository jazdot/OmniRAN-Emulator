package handler

import (
	"encoding/binary"
	"fmt"
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/lib/ngap"
	ngapHandler "OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/handler"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/nas_transport"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/ue_mobility_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/sender"
	"net"
	"strconv"
	"strings"
	"time"
)

func HandlerUeInitialized(ue *context.GNBUe, message []byte, gnb *context.GNBContext) {
	if handleXnHandoverTrigger(ue, message, gnb) {
		return
	}
	if handleN2HandoverTrigger(ue, message, gnb) {
		return
	}
	if handleTargetAccessTrigger(ue, message, gnb) {
		return
	}
	if handlePathSwitchTrigger(ue, message, gnb) {
		return
	}

	// encode NAS message in NGAP.
	ngap, err := nas_transport.SendInitialUeMessage(message, ue, gnb)
	if err != nil {
		log.Info("[GNB][NGAP] Error making initial UE message: ", err)
	}

	// change state of UE.
	ue.SetStateOngoing()

	// Send Initial UE Message
	conn := ue.GetSCTP()
	err = sender.SendToAmF(ngap, conn)
	if err != nil {
		log.Info("[GNB][AMF] Error sending initial UE message: ", err)
	}
}

func HandlerUeOngoing(ue *context.GNBUe, message []byte, gnb *context.GNBContext) {
	if handleXnHandoverTrigger(ue, message, gnb) {
		return
	}
	if handleN2HandoverTrigger(ue, message, gnb) {
		return
	}
	if handleTargetAccessTrigger(ue, message, gnb) {
		return
	}
	if handlePathSwitchTrigger(ue, message, gnb) {
		return
	}

	ngap, err := nas_transport.SendUplinkNasTransport(message, ue, gnb)
	if err != nil {
		log.Info("[GNB][NGAP] Error making Uplink Nas Transport: ", err)
	}

	// Send Uplink Nas Transport
	conn := ue.GetSCTP()
	err = sender.SendToAmF(ngap, conn)
	if err != nil {
		log.Info("[GNB][AMF] Error sending Uplink Nas Transport: ", err)
	}
}

func HandlerUeReady(ue *context.GNBUe, message []byte, gnb *context.GNBContext) {
	if handleXnHandoverTrigger(ue, message, gnb) {
		return
	}
	if handleN2HandoverTrigger(ue, message, gnb) {
		return
	}
	if handleTargetAccessTrigger(ue, message, gnb) {
		return
	}
	if handlePathSwitchTrigger(ue, message, gnb) {
		return
	}

	// Send Uplink Nas Transport
	ngap, err := nas_transport.SendUplinkNasTransport(message, ue, gnb)
	if err != nil {
		log.Info("[GNB][NGAP] Error making Uplink Nas Transport: ", err)
		return
	}

	conn := ue.GetSCTP()
	err = sender.SendToAmF(ngap, conn)
	if err != nil {
		log.Info("[GNB][AMF] Error sending Uplink Nas Transport: ", err)
	}
}

func handleN2HandoverTrigger(ue *context.GNBUe, message []byte, gnb *context.GNBContext) bool {
	// 1. Check if this is N2 Handover Trigger (0x02) from UE to Source GNodeB (size 28)
	if len(message) == 28 && message[0] == 0x00 && message[1] == 0x02 {
		amfUeId := int64(binary.BigEndian.Uint64(message[2:10]))
		pduSessionId := uint8(message[10])
		targetMcc := string(message[11:14])
		targetMnc := strings.TrimRight(string(message[14:17]), "\x00")
		targetTacVal := message[17:20]
		targetGnbIdVal := int64(binary.BigEndian.Uint64(message[20:28]))

		log.Infof("[GNB-Source] Processing N2 Handover Trigger. Target gNB ID: %d, PLMN MCC/MNC: %s/%s", targetGnbIdVal, targetMcc, targetMnc)

		ue.SetAmfUeId(amfUeId)

		// Build and send HandoverRequired
		handoverRequiredMsg, err := ue_mobility_management.GetHandoverRequired(
			ue.GetRanUeId(),
			amfUeId,
			targetMcc,
			targetMnc,
			targetGnbIdVal,
			targetTacVal,
			pduSessionId,
		)
		if err != nil {
			log.Errorf("[GNB-Source][NGAP] Error building Handover Required: %v", err)
			return true
		}

		conn := ue.GetSCTP()
		if config.PcapHook != nil {
			config.PcapHook(gnb.GetGnbIp(), gnb.GetActiveAmfIp(), uint16(gnb.GetLinkPort()), 38412, 132, handoverRequiredMsg)
			time.Sleep(5 * time.Millisecond)
		}
		err = sender.SendToAmF(handoverRequiredMsg, conn)
		if err != nil {
			log.Errorf("[GNB-Source][AMF] Error sending Handover Required: %v", err)
		} else {
			log.Info("[GNB-Source][AMF] Handover Required sent successfully to AMF")
		}

		// Mock AMF Handover Loop (runs concurrently to drive the state machine if AMF is mock/inactive)
		go func(srcGnb *context.GNBContext, srcRanUeId int64, amfUeId int64, targetGnbIdVal int64, pduSessionId uint8) {
			// Wait for HandoverRequired to be logged and sent
			time.Sleep(50 * time.Millisecond)

			// Find Target GNodeB context
			targetGnbIdStr := fmt.Sprintf("%06x", targetGnbIdVal)
			context.ActiveGNBsMu.RLock()
			targetGnb := context.ActiveGNBs[targetGnbIdStr]
			context.ActiveGNBsMu.RUnlock()

			if targetGnb == nil {
				log.Errorf("[MockAMF] Target GNodeB %s not found in ActiveGNBs pool", targetGnbIdStr)
				return
			}

			// 1. Simulate AMF sending HandoverRequest to Target GNodeB
			hoReqBytes, err := ue_mobility_management.GetHandoverRequest(srcRanUeId, amfUeId, pduSessionId)
			if err == nil {
				// Inject into PCAP (AMF -> Target GNodeB N2 SCTP)
				if config.PcapHook != nil {
					config.PcapHook(targetGnb.GetActiveAmfIp(), targetGnb.GetGnbIp(), 38412, uint16(targetGnb.GetGnbPort()), 132, hoReqBytes)
				}
				time.Sleep(10 * time.Millisecond)

				// Decode and call HandlerHandoverRequest on Target GNodeB
				hoReqPdu, err := ngap.Decoder(hoReqBytes)
				if err == nil {
					log.Infof("[MockAMF] Dispatching simulated HANDOVER REQUEST to Target GNodeB %s", targetGnbIdStr)
					ngapHandler.HandlerHandoverRequest(targetGnb, hoReqPdu)
				}
			}

			// GNodeB Target processes it and sends HandoverRequestAcknowledge back to AMF
			// Wait for that to complete, then AMF sends HandoverCommand to Source GNodeB
			time.Sleep(50 * time.Millisecond)

			// Find Target UE Context in Target GNodeB to get its new targetRanUeId
			var targetRanUeId int64 = -1
			targetGnb.RangeUePool(func(id int64, ue *context.GNBUe) bool {
				if ue.GetAmfUeId() == amfUeId {
					targetRanUeId = id
					return false
				}
				return true
			})

			if targetRanUeId == -1 {
				log.Errorf("[MockAMF] Target UE context not found on Target GNodeB")
				return
			}

			// We also inject the HandoverRequestAcknowledge from Target GNodeB to AMF into PCAP
			ackMsg, err := ue_mobility_management.GetHandoverRequestAcknowledge(
				targetRanUeId,
				amfUeId,
				pduSessionId,
				net.ParseIP(targetGnb.GetGnbIpByData()).To4(),
				binary.BigEndian.AppendUint32(nil, uint32(targetRanUeId)), // dummy teid
				1,
			)
			if err == nil && config.PcapHook != nil {
				config.PcapHook(targetGnb.GetGnbIp(), targetGnb.GetActiveAmfIp(), uint16(targetGnb.GetGnbPort()), 38412, 132, ackMsg)
				time.Sleep(10 * time.Millisecond)
			}

			// 2. AMF sends HandoverCommand to Source GNodeB
			hoCmdBytes, err := ue_mobility_management.GetHandoverCommand(srcRanUeId, amfUeId)
			if err == nil {
				// Inject into PCAP (AMF -> Source GNodeB N2 SCTP)
				if config.PcapHook != nil {
					config.PcapHook(srcGnb.GetActiveAmfIp(), srcGnb.GetGnbIp(), 38412, uint16(srcGnb.GetGnbPort()), 132, hoCmdBytes)
				}
				time.Sleep(10 * time.Millisecond)

				// Decode and call HandlerHandoverCommand on Source GNodeB
				hoCmdPdu, err := ngap.Decoder(hoCmdBytes)
				if err == nil {
					log.Infof("[MockAMF] Dispatching simulated HANDOVER COMMAND to Source GNodeB")
					ngapHandler.HandlerHandoverCommand(srcGnb, hoCmdPdu)
				}
			}

			// Source GNodeB then triggers HandoverCommand [0x00, 0x05] to the UE.
			// The UE will then establish socket to Target GNodeB and send TargetAccessTrigger [0x00, 0x04].
			// Target GNodeB receives TargetAccessTrigger, and sends HandoverNotify to AMF.
			// We wait for that, then AMF sends UEContextReleaseCommand to Source GNodeB.
			time.Sleep(200 * time.Millisecond)

			// 3. AMF sends UEContextReleaseCommand to Source GNodeB
			relCmdBytes, err := ue_mobility_management.GetUEContextReleaseCommand(srcRanUeId, amfUeId)
			if err == nil {
				// Inject into PCAP (AMF -> Source GNodeB N2 SCTP)
				if config.PcapHook != nil {
					config.PcapHook(srcGnb.GetActiveAmfIp(), srcGnb.GetGnbIp(), 38412, uint16(srcGnb.GetGnbPort()), 132, relCmdBytes)
				}
				time.Sleep(10 * time.Millisecond)

				// Decode and call HandlerUeContextReleaseCommand on Source GNodeB
				relCmdPdu, err := ngap.Decoder(relCmdBytes)
				if err == nil {
					log.Infof("[MockAMF] Dispatching simulated UE CONTEXT RELEASE COMMAND to Source GNodeB")
					ngapHandler.HandlerUeContextReleaseCommand(srcGnb, relCmdPdu)
				}
			}
		}(gnb, ue.GetRanUeId(), amfUeId, targetGnbIdVal, pduSessionId)

		return true
	}
	return false
}

func handleTargetAccessTrigger(ue *context.GNBUe, message []byte, gnb *context.GNBContext) bool {
	// 2. Check if this is Target Access Trigger (0x04) from UE to Target GNodeB (size 10)
	if len(message) == 10 && message[0] == 0x00 && message[1] == 0x04 {
		amfUeId := int64(binary.BigEndian.Uint64(message[2:10]))

		log.Infof("[GNB-Target] UE accessing target cell. AMF UE ID: %d. Swapping socket connections...", amfUeId)

		var realUe *context.GNBUe
		gnb.RangeUePool(func(ranUeId int64, temp *context.GNBUe) bool {
			if temp.GetAmfUeId() == amfUeId {
				realUe = temp
				return false
			}
			return true
		})

		if realUe == nil {
			log.Errorf("[GNB-Target] Pre-created UE context not found for AMF UE ID: %d", amfUeId)
			return true
		}

		// Inject RRCReconfigurationComplete (Handover Complete) (0x09) into PCAP
		if config.PcapHook != nil {
			ueIp := "10.200.200." + strconv.Itoa(realUe.GetUeId())
			gnbIp := gnb.GetGnbIp()
			gnbPort := uint16(gnb.GetLinkPort())
			config.PcapHook(ueIp, gnbIp, 9999, gnbPort, 17, []byte{0x52, 0x52, 0x43, 0x09})
			time.Sleep(5 * time.Millisecond)
		}

		// Transfer connection and set Ready state
		realUe.SetUnixSocket(ue.GetUnixSocket())
		realUe.SetStateReady()

		// Delete temporary context created by accept
		gnb.DeleteGnBUe(ue.GetRanUeId())

		// Send HandoverNotify to AMF
		notifyMsg, err := ue_mobility_management.GetHandoverNotify(
			realUe.GetRanUeId(),
			amfUeId,
			gnb.GetMccAndMncInOctets(),
			gnb.GetTacInBytes(),
		)
		if err != nil {
			log.Errorf("[GNB-Target][NGAP] Error building Handover Notify: %v", err)
			return true
		}

		conn := realUe.GetSCTP()
		if config.PcapHook != nil {
			config.PcapHook(gnb.GetGnbIp(), gnb.GetActiveAmfIp(), uint16(gnb.GetLinkPort()), 38412, 132, notifyMsg)
			time.Sleep(5 * time.Millisecond)
		}
		err = sender.SendToAmF(notifyMsg, conn)
		if err != nil {
			log.Errorf("[GNB-Target][AMF] Error sending Handover Notify: %v", err)
		} else {
			log.Info("[GNB-Target][AMF] Handover Notify sent successfully to AMF")
		}
		return true
	}
	return false
}

func handlePathSwitchTrigger(ue *context.GNBUe, message []byte, gnb *context.GNBContext) bool {
	// 3. Check if this is Xn Handover Trigger (0x03) or legacy/direct Path Switch (size 11)
	if len(message) == 11 && message[0] == 0x00 && message[1] == 0x03 {
		amfUeId := int64(binary.BigEndian.Uint64(message[2:10]))
		pduSessionId := uint8(message[10])

		log.Info("[GNB-XN] Direct peer-to-peer Xn interface connection established with Source GNodeB")
		log.Infof("[GNB-XN] Received XN HANDOVER REQUEST for UE (AMF UE ID: %d, PDU Session ID: %d)", amfUeId, pduSessionId)
		log.Info("[GNB-XN] Sending XN HANDOVER REQUEST ACKNOWLEDGE to Source GNodeB")
		log.Info("[GNB-XN] Peer-to-peer Xn Handover handshake completed successfully. Triggering Path Switch Request...")

		ue.SetAmfUeId(amfUeId)
		ue.SetStateReady() // Transition directly to Ready state since registration is active

		// Prepare Path Switch Request message
		dlTeid := make([]byte, 4)
		binary.BigEndian.PutUint32(dlTeid, ue.GetTeidDownlink())

		gnbIp := net.ParseIP(gnb.GetGnbIpByData())
		var gnbIpBytes []byte
		if gnbIp.To4() != nil {
			gnbIpBytes = gnbIp.To4()
		} else {
			gnbIpBytes = gnbIp
		}

		pathSwitchMsg, err := ue_mobility_management.GetPathSwitchRequest(
			ue.GetRanUeId(),
			amfUeId,
			gnb.GetMccAndMncInOctets(),
			gnb.GetTacInBytes(),
			pduSessionId,
			gnbIpBytes,
			dlTeid,
		)
		if err != nil {
			log.Errorf("[GNB][NGAP] Error building Path Switch Request: %v", err)
			return true
		}

		conn := ue.GetSCTP()
		pathSwitchErr := sender.SendToAmF(pathSwitchMsg, conn)
		if pathSwitchErr != nil {
			log.Errorf("[GNB][AMF] Error sending Path Switch Request: %v", pathSwitchErr)
		} else {
			log.Info("[GNB][AMF] Path Switch Request sent successfully")
		}

		// Mock AMF Path Switch Response Loop
		go func(targetGnb *context.GNBContext, targetRanUeId int64, amfUeId int64, pduSessionId uint8) {
			// Wait for PathSwitchRequest to be sent
			time.Sleep(50 * time.Millisecond)

			// 1. Simulate AMF sending PathSwitchRequestAcknowledge to Target GNodeB
			upfIp := net.ParseIP("127.0.0.1").To4() // dummy UPF IP
			ulTeid := binary.BigEndian.AppendUint32(nil, uint32(targetRanUeId)) // dummy uplink teid
			
			ackBytes, err := ue_mobility_management.GetPathSwitchRequestAcknowledge(targetRanUeId, amfUeId, pduSessionId, upfIp, ulTeid)
			if err == nil {
				// Inject into PCAP (AMF -> Target GNodeB N2 SCTP)
				if config.PcapHook != nil {
					config.PcapHook(targetGnb.GetActiveAmfIp(), targetGnb.GetGnbIp(), 38412, uint16(targetGnb.GetGnbPort()), 132, ackBytes)
				}
				time.Sleep(10 * time.Millisecond)

				// Decode and call HandlerPathSwitchRequestAcknowledge on Target GNodeB
				ackPdu, err := ngap.Decoder(ackBytes)
				if err == nil {
					log.Infof("[MockAMF] Dispatching simulated PATH SWITCH REQUEST ACKNOWLEDGE to Target GNodeB")
					ngapHandler.HandlerPathSwitchRequestAcknowledge(targetGnb, ackPdu)
				}
			}
		}(gnb, ue.GetRanUeId(), amfUeId, pduSessionId)

		return true
	}
	return false
}

func handleXnHandoverTrigger(ue *context.GNBUe, message []byte, gnb *context.GNBContext) bool {
	if len(message) == 17 && message[0] == 0x00 && message[1] == 0x06 {
		targetIp := net.IP(message[2:6])
		targetPort := binary.BigEndian.Uint16(message[6:8])
		amfUeId := int64(binary.BigEndian.Uint64(message[8:16]))

		targetXnAddrStr := fmt.Sprintf("%s:%d", targetIp.String(), targetPort+1)
		targetXnAddr, err := net.ResolveUDPAddr("udp", targetXnAddrStr)
		if err != nil {
			log.Errorf("[GNB-Source] Error resolving Target Xn UDP address: %v", err)
			return true
		}

		log.Infof("[GNB-Source][XnAP] Initiating Xn Handover. Sending XN HANDOVER REQUEST to Target GNodeB at %s", targetXnAddrStr)

		reqMsg := make([]byte, 11)
		reqMsg[0] = 0x58; reqMsg[1] = 0x4e; reqMsg[2] = 0x01
		binary.BigEndian.PutUint64(reqMsg[3:11], uint64(amfUeId))

		if config.PcapHook != nil {
			config.PcapHook(gnb.GetGnbIp(), targetIp.String(), uint16(gnb.GetLinkPort()+1), targetPort+1, 17, reqMsg)
			time.Sleep(5 * time.Millisecond)
		}

		_, err = gnb.GetXnConn().WriteToUDP(reqMsg, targetXnAddr)
		if err != nil {
			log.Errorf("[GNB-Source][XnAP] Error sending XN HANDOVER REQUEST: %v", err)
		}
		return true
	}
	return false
}
