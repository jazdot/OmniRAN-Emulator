package nas_transport

import (
	"fmt"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/lib/aper"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func getUplinkNASTransport(amfUeNgapID, ranUeNgapID int64, nasPdu []byte, plmn []byte, tac []byte, gnbId []byte) ([]byte, error) {
	message := buildUplinkNasTransport(amfUeNgapID, ranUeNgapID, nasPdu, plmn, tac, gnbId)
	return ngap.Encoder(message)
}

func buildUplinkNasTransport(amfUeNgapID, ranUeNgapID int64, nasPdu []byte, plmn []byte, tac []byte, gnbId []byte) (pdu ngapType.NGAPPDU) {

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeUplinkNASTransport
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentUplinkNASTransport
	initiatingMessage.Value.UplinkNASTransport = new(ngapType.UplinkNASTransport)

	uplinkNasTransport := initiatingMessage.Value.UplinkNASTransport
	uplinkNasTransportIEs := &uplinkNasTransport.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUeNgapID

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUeNgapID

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	// NAS-PDU
	ie = ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDNASPDU
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentNASPDU
	ie.Value.NASPDU = new(ngapType.NASPDU)

	// TODO: complete NAS-PDU
	nASPDU := ie.Value.NASPDU
	nASPDU.Value = nasPdu

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	// User Location Information
	ie = ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUserLocationInformation
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentUserLocationInformation
	ie.Value.UserLocationInformation = new(ngapType.UserLocationInformation)

	userLocationInformation := ie.Value.UserLocationInformation
	userLocationInformation.Present = ngapType.UserLocationInformationPresentUserLocationInformationNR
	userLocationInformation.UserLocationInformationNR = new(ngapType.UserLocationInformationNR)

	userLocationInformationNR := userLocationInformation.UserLocationInformationNR
	userLocationInformationNR.NRCGI.PLMNIdentity.Value = plmn
	cellIdBytes := make([]byte, 5)
	if len(gnbId) >= 3 {
		copy(cellIdBytes[0:3], gnbId[0:3])
	}
	cellIdBytes[3] = 0x00
	cellIdBytes[4] = 0x10 // Cell ID 1 (36-bit: 24 bits gnb + 12 bits cell id)

	userLocationInformationNR.NRCGI.NRCellIdentity.Value = aper.BitString{
		Bytes:     cellIdBytes,
		BitLength: 36,
	}

	userLocationInformationNR.TAI.PLMNIdentity.Value = plmn
	userLocationInformationNR.TAI.TAC.Value = tac

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	return
}

func SendUplinkNasTransport(message []byte, ue *context.GNBUe, gnb *context.GNBContext) ([]byte, error) {

	sendMsg, err := getUplinkNASTransport(ue.GetAmfUeId(), ue.GetRanUeId(), message, gnb.GetMccAndMncInOctets(), gnb.GetTacInBytes(), gnb.GetGnbIdInBytes())
	if err != nil {
		return nil, fmt.Errorf("Error getting UE Id %d NAS Authentication Response", ue.GetRanUeId())
	}

	return sendMsg, nil
}

/*
func UplinkNasTransport(connN2 *sctp.SCTPConn, amfUeNgapID int64, ranUeNgapID int64, nasPdu []byte, gnb *context.RanGnbContext) error {

	sendMsg, err := getUplinkNASTransport(amfUeNgapID, ranUeNgapID, nasPdu, gnb.GetMccAndMncInOctets(), gnb.GetTacInBytes())
	if err != nil {
		return fmt.Errorf("Error getting ueId %d NAS Authentication Response", ranUeNgapID)
	}

	_, err = connN2.Write(sendMsg)
	if err != nil {
		return fmt.Errorf("Error sending ueId %d NAS Authentication Response", ranUeNgapID)
	}

	log.WithFields(log.Fields{
		"protocol":    "NGAP",
		"source":      fmt.Sprintf("GNB[ID:%s]", gnb.GetGnbId()),
		"destination": "AMF",
		"message":     "UPLINK NAS TRANSPORT",
	}).Info("Sending message")

	return nil
}
*/
