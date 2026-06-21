package handler

import (
	"encoding/binary"
	"fmt"
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/nas_transport"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/ue_mobility_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/sender"
	"net"
	"strings"
)

func HandlerUeInitialized(ue *context.GNBUe, message []byte, gnb *context.GNBContext) {
	if handleXnHandoverTrigger(ue, message, gnb) {
		return
	}

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
			return
		}

		conn := ue.GetSCTP()
		err = sender.SendToAmF(handoverRequiredMsg, conn)
		if err != nil {
			log.Errorf("[GNB-Source][AMF] Error sending Handover Required: %v", err)
		} else {
			log.Info("[GNB-Source][AMF] Handover Required sent successfully to AMF")
		}
		return
	}

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
			return
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
			return
		}

		conn := realUe.GetSCTP()
		err = sender.SendToAmF(notifyMsg, conn)
		if err != nil {
			log.Errorf("[GNB-Target][AMF] Error sending Handover Notify: %v", err)
		} else {
			log.Info("[GNB-Target][AMF] Handover Notify sent successfully to AMF")
		}
		return
	}

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
			return
		}

		conn := ue.GetSCTP()
		pathSwitchErr := sender.SendToAmF(pathSwitchMsg, conn)
		if pathSwitchErr != nil {
			log.Errorf("[GNB][AMF] Error sending Path Switch Request: %v", pathSwitchErr)
		} else {
			log.Info("[GNB][AMF] Path Switch Request sent successfully")
		}
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

		_, err = gnb.GetXnConn().WriteToUDP(reqMsg, targetXnAddr)
		if err != nil {
			log.Errorf("[GNB-Source][XnAP] Error sending XN HANDOVER REQUEST: %v", err)
		}
		return true
	}
	return false
}
