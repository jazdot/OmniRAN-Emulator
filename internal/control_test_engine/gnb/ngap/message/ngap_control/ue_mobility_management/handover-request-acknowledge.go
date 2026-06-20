package ue_mobility_management

import (
	"OmniRAN-Emulator/lib/aper"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func GetHandoverRequestAcknowledge(ranUeNgapID int64, amfUeNgapID int64, pduSessionId uint8, gnbIp []byte, dlTeid []byte, qosId int64) ([]byte, error) {
	message := BuildHandoverRequestAcknowledge(ranUeNgapID, amfUeNgapID, pduSessionId, gnbIp, dlTeid, qosId)
	return ngap.Encoder(message)
}

func BuildHandoverRequestAcknowledge(ranUeNgapID int64, amfUeNgapID int64, pduSessionId uint8, gnbIp []byte, dlTeid []byte, qosId int64) (pdu ngapType.NGAPPDU) {
	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeHandoverResourceAllocation
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentHandoverRequestAcknowledge
	successfulOutcome.Value.HandoverRequestAcknowledge = new(ngapType.HandoverRequestAcknowledge)

	handoverRequestAck := successfulOutcome.Value.HandoverRequestAcknowledge
	handoverRequestAckIEs := &handoverRequestAck.ProtocolIEs

	// AMFUENGAPID
	ie := ngapType.HandoverRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.AMFUENGAPID.Value = amfUeNgapID
	handoverRequestAckIEs.List = append(handoverRequestAckIEs.List, ie)

	// RANUENGAPID
	ie = ngapType.HandoverRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ie.Value.RANUENGAPID.Value = ranUeNgapID
	handoverRequestAckIEs.List = append(handoverRequestAckIEs.List, ie)

	// PDUSessionResourceAdmittedList
	ie = ngapType.HandoverRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceAdmittedList
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentPDUSessionResourceAdmittedList
	ie.Value.PDUSessionResourceAdmittedList = new(ngapType.PDUSessionResourceAdmittedList)

	admittedList := ie.Value.PDUSessionResourceAdmittedList
	admittedItem := ngapType.PDUSessionResourceAdmittedItem{}
	admittedItem.PDUSessionID.Value = int64(pduSessionId)

	// Build HandoverRequestAcknowledgeTransfer
	transferPdu := ngapType.HandoverRequestAcknowledgeTransfer{}
	transferPdu.DLNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	transferPdu.DLNGUUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)
	gtp := transferPdu.DLNGUUPTNLInformation.GTPTunnel
	gtp.TransportLayerAddress.Value = aper.BitString{
		Bytes:     gnbIp,
		BitLength: uint64(len(gnbIp) * 8),
	}
	gtp.GTPTEID.Value = aper.OctetString(dlTeid)

	associatedQos := ngapType.QosFlowSetupResponseItemHOReqAck{}
	associatedQos.QosFlowIdentifier.Value = qosId
	transferPdu.QosFlowSetupResponseList.List = append(transferPdu.QosFlowSetupResponseList.List, associatedQos)

	// HandoverRequestAcknowledgeTransfer is an extensible container
	transferBytes, _ := aper.MarshalWithParams(transferPdu, "valueExt")
	admittedItem.HandoverRequestAcknowledgeTransfer = aper.OctetString(transferBytes)

	admittedList.List = append(admittedList.List, admittedItem)
	handoverRequestAckIEs.List = append(handoverRequestAckIEs.List, ie)

	// TargetToSourceTransparentContainer
	ie = ngapType.HandoverRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDTargetToSourceTransparentContainer
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentTargetToSourceTransparentContainer
	ie.Value.TargetToSourceTransparentContainer = new(ngapType.TargetToSourceTransparentContainer)
	// Add 4 dummy bytes RRC container to avoid being empty
	ie.Value.TargetToSourceTransparentContainer.Value = []byte{0x00, 0x01, 0x02, 0x03}
	handoverRequestAckIEs.List = append(handoverRequestAckIEs.List, ie)

	return
}
