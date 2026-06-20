package trigger

import (
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/interface_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/pdu_session_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/ue_context_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/sender"
)

func SendPduSessionResourceSetupResponse(ue *context.GNBUe, gnb *context.GNBContext, pduSessionId int64) {

	// send PDU Session Resource Setup Response.
	gnbIp := gnb.GetGnbIpByData()
	ngapMsg, err := pdu_session_management.PDUSessionResourceSetupResponse(ue, gnbIp, pduSessionId)
	if err != nil {
		log.Fatal("[GNB][NGAP] Error sending PDU Session Resource Setup Response.")
	}

	ue.SetStateReady()

	// Send PDU Session Resource Setup Response.
	conn := ue.GetSCTP()
	err = sender.SendToAmF(ngapMsg, conn)
	if err != nil {
		log.Fatal("[GNB][AMF] Error sending PDU Session Resource Setup Response.: ", err)
	}
}

func SendInitialContextSetupResponse(ue *context.GNBUe) {

	// send Initial Context Setup Response.
	ngapMsg, err := ue_context_management.InitialContextSetupResponse(ue)
	if err != nil {
		log.Fatal("[GNB][NGAP] Error sending Initial Context Setup Response")
	}

	// Send Initial Context Setup Response.
	conn := ue.GetSCTP()
	err = sender.SendToAmF(ngapMsg, conn)
	if err != nil {
		log.Fatal("[GNB][AMF] Error sending Initial Context Setup Response: ", err)
	}
}

func SendNgSetupRequest(gnb *context.GNBContext, amf *context.GNBAmf) {

	// send NG setup response.
	ngapMsg, err := interface_management.NGSetupRequest(gnb, "my5gRANTester")
	if err != nil {
		log.Info("[GNB][NGAP] Error sending NG Setup Request")

	}

	conn := amf.GetSCTPConn()
	err = sender.SendToAmF(ngapMsg, conn)
	if err != nil {
		log.Info("[GNB][AMF] Error sending NG Setup Request: ", err)
	}

}

func SendPduSessionResourceReleaseResponse(ue *context.GNBUe, gnb *context.GNBContext) {
	_ = gnb // reserved for future per-gnb routing
	ngapMsg, err := pdu_session_management.PDUSessionResourceReleaseResponse(ue)
	if err != nil {
		log.Warn("[GNB][NGAP] Error building PDU Session Resource Release Response: ", err)
		return
	}

	conn := ue.GetSCTP()
	err = sender.SendToAmF(ngapMsg, conn)
	if err != nil {
		log.Warn("[GNB][AMF] Error sending PDU Session Resource Release Response: ", err)
	}
	log.Info("[GNB][NGAP][AMF] Sent PDU Session Resource Release Response.")
}
