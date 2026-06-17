package handler

import (
	"encoding/binary"
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/nas_transport"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/ue_mobility_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/sender"
	"net"
)

func HandlerUeInitialized(ue *context.GNBUe, message []byte, gnb *context.GNBContext) {

	// Check if this is a handover path switch trigger
	if len(message) == 11 && message[0] == 0x00 && message[1] == 0x02 {
		amfUeId := int64(binary.BigEndian.Uint64(message[2:10]))
		pduSessionId := uint8(message[10])

		log.Infof("[GNB] Processing Handover Path Switch Trigger from UE. AMF UE ID: %d, PDU Session ID: %d", amfUeId, pduSessionId)

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
		err = sender.SendToAmF(pathSwitchMsg, conn)
		if err != nil {
			log.Errorf("[GNB][AMF] Error sending Path Switch Request: %v", err)
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
