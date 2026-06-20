package ue_mobility_management

import (
	"OmniRAN-Emulator/lib/aper"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func GetHandoverNotify(ranUeNgapID int64, amfUeNgapID int64, plmn []byte, tac []byte) ([]byte, error) {
	message := BuildHandoverNotify(ranUeNgapID, amfUeNgapID, plmn, tac)
	return ngap.Encoder(message)
}

func BuildHandoverNotify(ranUeNgapID int64, amfUeNgapID int64, plmn []byte, tac []byte) (pdu ngapType.NGAPPDU) {
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeHandoverNotification
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore
	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentHandoverNotify
	initiatingMessage.Value.HandoverNotify = new(ngapType.HandoverNotify)

	handoverNotify := initiatingMessage.Value.HandoverNotify
	handoverNotifyIEs := &handoverNotify.ProtocolIEs

	// AMFUENGAPID
	ie := ngapType.HandoverNotifyIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverNotifyIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.AMFUENGAPID.Value = amfUeNgapID
	handoverNotifyIEs.List = append(handoverNotifyIEs.List, ie)

	// RANUENGAPID
	ie = ngapType.HandoverNotifyIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverNotifyIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ie.Value.RANUENGAPID.Value = ranUeNgapID
	handoverNotifyIEs.List = append(handoverNotifyIEs.List, ie)

	// UserLocationInformation
	ie = ngapType.HandoverNotifyIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUserLocationInformation
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverNotifyIEsPresentUserLocationInformation
	ie.Value.UserLocationInformation = new(ngapType.UserLocationInformation)

	userLoc := ie.Value.UserLocationInformation
	userLoc.Present = ngapType.UserLocationInformationPresentUserLocationInformationNR
	userLoc.UserLocationInformationNR = new(ngapType.UserLocationInformationNR)
	userLocNR := userLoc.UserLocationInformationNR
	userLocNR.NRCGI.PLMNIdentity.Value = plmn
	userLocNR.NRCGI.NRCellIdentity.Value = aper.BitString{
		Bytes:     []byte{0x00, 0x00, 0x00, 0x00, 0x10},
		BitLength: 36,
	}
	userLocNR.TAI.PLMNIdentity.Value = plmn
	userLocNR.TAI.TAC.Value = tac
	handoverNotifyIEs.List = append(handoverNotifyIEs.List, ie)

	return
}
