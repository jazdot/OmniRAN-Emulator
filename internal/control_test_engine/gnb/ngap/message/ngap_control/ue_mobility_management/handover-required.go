package ue_mobility_management

import (
	"encoding/hex"
	"OmniRAN-Emulator/lib/aper"
	"OmniRAN-Emulator/lib/ngap"
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

	plmnBytes := getPlmnBytes(targetMcc, targetMnc)
	globalGNBID.PLMNIdentity.Value = plmnBytes
	globalGNBID.GNBID.Present = ngapType.GNBIDPresentGNBID
	globalGNBID.GNBID.GNBID = new(aper.BitString)
	*globalGNBID.GNBID.GNBID = aper.BitString{
		Bytes:     intTo3Bytes(targetGnbIdVal),
		BitLength: 24,
	}

	targetRANNodeID.SelectedTAI.PLMNIdentity.Value = plmnBytes
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
	// Add 4 dummy bytes RRC container to avoid being empty
	ie.Value.SourceToTargetTransparentContainer.Value = []byte{0x00, 0x01, 0x02, 0x03}
	handoverRequiredIEs.List = append(handoverRequiredIEs.List, ie)

	return
}

func getPlmnBytes(mccStr, mncStr string) []byte {
	mcc := reverse(mccStr)
	mnc := reverse(mncStr)

	oct5 := mcc[0:2]
	if len(mcc) >= 3 {
		oct5 = mcc[1:3]
	}
	var oct6 string
	var oct7 string
	if len(mncStr) == 2 {
		oct6 = "f" + string(mcc[0])
		oct7 = mnc
	} else {
		oct6 = string(mnc[0]) + string(mcc[0])
		oct7 = mnc[1:3]
	}
	res, _ := hex.DecodeString(oct5 + oct6 + oct7)
	if len(res) < 3 {
		// Fallback safe PLMN
		return []byte{0x02, 0xf8, 0x39}
	}
	return res
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func intTo3Bytes(val int64) []byte {
	return []byte{
		byte((val >> 16) & 0xff),
		byte((val >> 8) & 0xff),
		byte(val & 0xff),
	}
}
