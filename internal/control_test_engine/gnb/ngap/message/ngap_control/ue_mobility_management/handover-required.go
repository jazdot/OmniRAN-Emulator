package ue_mobility_management

import (
	"OmniRAN-Emulator/lib/aper"
	"OmniRAN-Emulator/lib/ngap"
	importModels "OmniRAN-Emulator/lib/openapi/models"
	"OmniRAN-Emulator/lib/ngap/ngapConvert"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func GetHandoverRequired(ranUeNgapID int64, amfUeNgapID int64, targetMcc, targetMnc string, targetGnbIdVal int64, targetTacVal []byte, pduSessionId uint8) ([]byte, error) {
	message := BuildHandoverRequired(ranUeNgapID, amfUeNgapID, targetMcc, targetMnc, targetGnbIdVal, targetTacVal, pduSessionId)
	return ngap.Encoder(message)
}

func BuildHandoverRequired(ranUeNgapID int64, amfUeNgapID int64, targetMcc, targetMnc string, targetGnbIdVal int64, targetTacVal []byte, pduSessionId uint8) (pdu ngapType.NGAPPDU) {
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeHandoverPreparation
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject
	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentHandoverRequired
	initiatingMessage.Value.HandoverRequired = new(ngapType.HandoverRequired)

	handoverRequired := initiatingMessage.Value.HandoverRequired
	handoverRequiredIEs := &handoverRequired.ProtocolIEs

	// AMFUENGAPID
	ie := ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.AMFUENGAPID.Value = amfUeNgapID
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	// RANUENGAPID
	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ie.Value.RANUENGAPID.Value = ranUeNgapID
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	// HandoverType
	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDHandoverType
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentHandoverType
	ie.Value.HandoverType = new(ngapType.HandoverType)
	ie.Value.HandoverType.Value = ngapType.HandoverTypePresentIntra5gs
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	// Cause
	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentCause
	ie.Value.Cause = new(ngapType.Cause)
	ie.Value.Cause.Present = ngapType.CausePresentRadioNetwork
	ie.Value.Cause.RadioNetwork = new(ngapType.CauseRadioNetwork)
	ie.Value.Cause.RadioNetwork.Value = ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	// TargetID
	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDTargetID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentTargetID
	ie.Value.TargetID = new(ngapType.TargetID)
	
	targetID := ie.Value.TargetID
	targetID.Present = ngapType.TargetIDPresentTargetRANNodeID
	targetID.TargetRANNodeID = new(ngapType.TargetRANNodeID)

	targetRANNodeID := targetID.TargetRANNodeID
	targetRANNodeID.GlobalRANNodeID.Present = ngapType.GlobalRANNodeIDPresentGlobalGNBID
	targetRANNodeID.GlobalRANNodeID.GlobalGNBID = new(ngapType.GlobalGNBID)
	globalGNBID := targetRANNodeID.GlobalRANNodeID.GlobalGNBID

	plmnID := ngapConvert.PlmnIdToNgap(importModels.PlmnId{
		Mcc: targetMcc,
		Mnc: targetMnc,
	})
	globalGNBID.PLMNIdentity.Value = plmnID.Value
	globalGNBID.GNBID.Present = ngapType.GNBIDPresentGNBID
	globalGNBID.GNBID.GNBID = new(aper.BitString)
	*globalGNBID.GNBID.GNBID = aper.BitString{
		Bytes:     intTo3Bytes(targetGnbIdVal),
		BitLength: 24,
	}

	targetRANNodeID.SelectedTAI.PLMNIdentity.Value = plmnID.Value
	targetRANNodeID.SelectedTAI.TAC.Value = targetTacVal

	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	// PDUSessionResourceListHORqd
	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceListHORqd
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentPDUSessionResourceListHORqd
	ie.Value.PDUSessionResourceListHORqd = new(ngapType.PDUSessionResourceListHORqd)

	pduList := ie.Value.PDUSessionResourceListHORqd
	pduItem := ngapType.PDUSessionResourceItemHORqd{}
	pduItem.PDUSessionID.Value = int64(pduSessionId)

	transferPdu := ngapType.HandoverRequiredTransfer{}
	transferBytes, _ := aper.MarshalWithParams(transferPdu, "valueExt")
	pduItem.HandoverRequiredTransfer = aper.OctetString(transferBytes)

	pduList.List = append(pduList.List, pduItem)
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	// SourceToTargetTransparentContainer
	ie = ngapType.HandoverRequiredIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSourceToTargetTransparentContainer
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentSourceToTargetTransparentContainer
	ie.Value.SourceToTargetTransparentContainer = new(ngapType.SourceToTargetTransparentContainer)
	// Use an empty byte slice or a properly formatted empty OCTET STRING
	ie.Value.SourceToTargetTransparentContainer.Value = []byte{}
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	return
}



func intTo3Bytes(val int64) []byte {
	return []byte{
		byte((val >> 16) & 0xff),
		byte((val >> 8) & 0xff),
		byte(val & 0xff),
	}
}
