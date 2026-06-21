package ue_mobility_management

import (
	"OmniRAN-Emulator/lib/aper"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

// GetHandoverRequest builds and APER-encodes a mock HandoverRequest (InitiatingMessage, procedure code 14)
func GetHandoverRequest(ranUeNgapID int64, amfUeNgapID int64, pduSessionId uint8) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)
	pdu.InitiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeHandoverResourceAllocation
	pdu.InitiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject
	pdu.InitiatingMessage.Value.Present = ngapType.InitiatingMessagePresentHandoverRequest
	pdu.InitiatingMessage.Value.HandoverRequest = new(ngapType.HandoverRequest)

	hoReq := pdu.InitiatingMessage.Value.HandoverRequest
	hoReqIEs := &hoReq.ProtocolIEs

	// AMFUENGAPID
	ie := ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.AMFUENGAPID.Value = amfUeNgapID
	hoReqIEs.List = append(hoReqIEs.List, ie)

	// HandoverType
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDHandoverType
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentHandoverType
	ie.Value.HandoverType = new(ngapType.HandoverType)
	ie.Value.HandoverType.Value = ngapType.HandoverTypePresentIntra5gs
	hoReqIEs.List = append(hoReqIEs.List, ie)

	// Cause
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequestIEsPresentCause
	ie.Value.Cause = new(ngapType.Cause)
	ie.Value.Cause.Present = ngapType.CausePresentRadioNetwork
	ie.Value.Cause.RadioNetwork = new(ngapType.CauseRadioNetwork)
	ie.Value.Cause.RadioNetwork.Value = ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason
	hoReqIEs.List = append(hoReqIEs.List, ie)

	// PDUSessionResourceSetupListHOReq
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceSetupListHOReq
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentPDUSessionResourceSetupListHOReq
	ie.Value.PDUSessionResourceSetupListHOReq = new(ngapType.PDUSessionResourceSetupListHOReq)

	pduList := ie.Value.PDUSessionResourceSetupListHOReq
	pduItem := ngapType.PDUSessionResourceSetupItemHOReq{}
	pduItem.PDUSessionID.Value = int64(pduSessionId)
	
	// Add dummy SNSSAI (SST=1, SD="010203")
	pduItem.SNSSAI.SST.Value = []byte{0x01}
	pduItem.SNSSAI.SD = new(ngapType.SD)
	pduItem.SNSSAI.SD.Value = []byte{0x01, 0x02, 0x03}

	// Add dummy HandoverRequestTransfer bytes (type is aper.OctetString)
	pduItem.HandoverRequestTransfer = []byte{0x00, 0x01, 0x02, 0x03}

	pduList.List = append(pduList.List, pduItem)
	hoReqIEs.List = append(hoReqIEs.List, ie)

	// UEAggregateMaximumBitRate
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUEAggregateMaximumBitRate
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentUEAggregateMaximumBitRate
	ie.Value.UEAggregateMaximumBitRate = new(ngapType.UEAggregateMaximumBitRate)
	ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateDL.Value = 100000000
	ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateUL.Value = 50000000
	hoReqIEs.List = append(hoReqIEs.List, ie)

	// UESecurityCapabilities
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUESecurityCapabilities
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentUESecurityCapabilities
	ie.Value.UESecurityCapabilities = new(ngapType.UESecurityCapabilities)
	ie.Value.UESecurityCapabilities.NRencryptionAlgorithms.Value = aper.BitString{Bytes: []byte{0xe0, 0x00}, BitLength: 16}
	ie.Value.UESecurityCapabilities.NRintegrityProtectionAlgorithms.Value = aper.BitString{Bytes: []byte{0xe0, 0x00}, BitLength: 16}
	ie.Value.UESecurityCapabilities.EUTRAencryptionAlgorithms.Value = aper.BitString{Bytes: []byte{0xe0, 0x00}, BitLength: 16}
	ie.Value.UESecurityCapabilities.EUTRAintegrityProtectionAlgorithms.Value = aper.BitString{Bytes: []byte{0xe0, 0x00}, BitLength: 16}
	hoReqIEs.List = append(hoReqIEs.List, ie)

	// SecurityContext
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSecurityContext
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentSecurityContext
	ie.Value.SecurityContext = new(ngapType.SecurityContext)
	ie.Value.SecurityContext.NextHopChainingCount.Value = 1
	ie.Value.SecurityContext.NextHopNH.Value = aper.BitString{Bytes: make([]byte, 32), BitLength: 256}
	hoReqIEs.List = append(hoReqIEs.List, ie)

	return ngap.Encoder(pdu)
}

// GetHandoverCommand builds and APER-encodes a mock HandoverCommand (SuccessfulOutcome, procedure code 14)
func GetHandoverCommand(ranUeNgapID int64, amfUeNgapID int64) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)
	pdu.SuccessfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeHandoverPreparation
	pdu.SuccessfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	pdu.SuccessfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentHandoverCommand
	pdu.SuccessfulOutcome.Value.HandoverCommand = new(ngapType.HandoverCommand)

	hoCmd := pdu.SuccessfulOutcome.Value.HandoverCommand
	hoCmdIEs := &hoCmd.ProtocolIEs

	// AMFUENGAPID
	ie := ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCommandIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.AMFUENGAPID.Value = amfUeNgapID
	hoCmdIEs.List = append(hoCmdIEs.List, ie)

	// RANUENGAPID
	ie = ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCommandIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ie.Value.RANUENGAPID.Value = ranUeNgapID
	hoCmdIEs.List = append(hoCmdIEs.List, ie)

	// HandoverType
	ie = ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDHandoverType
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverCommandIEsPresentHandoverType
	ie.Value.HandoverType = new(ngapType.HandoverType)
	ie.Value.HandoverType.Value = ngapType.HandoverTypePresentIntra5gs
	hoCmdIEs.List = append(hoCmdIEs.List, ie)

	return ngap.Encoder(pdu)
}

// GetUEContextReleaseCommand builds and APER-encodes a mock UEContextReleaseCommand (InitiatingMessage, procedure code 41)
func GetUEContextReleaseCommand(ranUeNgapID int64, amfUeNgapID int64) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)
	pdu.InitiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeUEContextRelease
	pdu.InitiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject
	pdu.InitiatingMessage.Value.Present = ngapType.InitiatingMessagePresentUEContextReleaseCommand
	pdu.InitiatingMessage.Value.UEContextReleaseCommand = new(ngapType.UEContextReleaseCommand)

	releaseCmd := pdu.InitiatingMessage.Value.UEContextReleaseCommand
	releaseCmdIEs := &releaseCmd.ProtocolIEs

	// UENGAPIDs
	ie := ngapType.UEContextReleaseCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUENGAPIDs
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UEContextReleaseCommandIEsPresentUENGAPIDs
	ie.Value.UENGAPIDs = new(ngapType.UENGAPIDs)
	ie.Value.UENGAPIDs.Present = ngapType.UENGAPIDsPresentUENGAPIDPair
	ie.Value.UENGAPIDs.UENGAPIDPair = new(ngapType.UENGAPIDPair)
	ie.Value.UENGAPIDs.UENGAPIDPair.AMFUENGAPID.Value = amfUeNgapID
	ie.Value.UENGAPIDs.UENGAPIDPair.RANUENGAPID.Value = ranUeNgapID
	releaseCmdIEs.List = append(releaseCmdIEs.List, ie)

	// Cause
	ie = ngapType.UEContextReleaseCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.UEContextReleaseCommandIEsPresentCause
	ie.Value.Cause = new(ngapType.Cause)
	ie.Value.Cause.Present = ngapType.CausePresentRadioNetwork
	ie.Value.Cause.RadioNetwork = new(ngapType.CauseRadioNetwork)
	ie.Value.Cause.RadioNetwork.Value = ngapType.CauseRadioNetworkPresentSuccessfulHandover
	releaseCmdIEs.List = append(releaseCmdIEs.List, ie)

	return ngap.Encoder(pdu)
}

// GetPathSwitchRequestAcknowledge builds and APER-encodes a mock PathSwitchRequestAcknowledge (SuccessfulOutcome, procedure code 30)
func GetPathSwitchRequestAcknowledge(ranUeNgapID int64, amfUeNgapID int64, pduSessionId uint8, upfIp []byte, ulTeid []byte) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)
	pdu.SuccessfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodePathSwitchRequest
	pdu.SuccessfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	pdu.SuccessfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentPathSwitchRequestAcknowledge
	pdu.SuccessfulOutcome.Value.PathSwitchRequestAcknowledge = new(ngapType.PathSwitchRequestAcknowledge)

	ack := pdu.SuccessfulOutcome.Value.PathSwitchRequestAcknowledge
	ackIEs := &ack.ProtocolIEs

	// AMFUENGAPID
	ie := ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.AMFUENGAPID.Value = amfUeNgapID
	ackIEs.List = append(ackIEs.List, ie)

	// RANUENGAPID
	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ie.Value.RANUENGAPID.Value = ranUeNgapID
	ackIEs.List = append(ackIEs.List, ie)

	// PDUSessionResourceSwitchedList
	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceSwitchedList
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentPDUSessionResourceSwitchedList
	ie.Value.PDUSessionResourceSwitchedList = new(ngapType.PDUSessionResourceSwitchedList)

	switchedList := ie.Value.PDUSessionResourceSwitchedList
	switchedItem := ngapType.PDUSessionResourceSwitchedItem{}
	switchedItem.PDUSessionID.Value = int64(pduSessionId)

	// Build PathSwitchRequestAcknowledgeTransfer
	transfer := ngapType.PathSwitchRequestAcknowledgeTransfer{}
	transfer.ULNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	transfer.ULNGUUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)
	gtp := transfer.ULNGUUPTNLInformation.GTPTunnel
	gtp.TransportLayerAddress.Value = aper.BitString{
		Bytes:     upfIp,
		BitLength: uint64(len(upfIp) * 8),
	}
	gtp.GTPTEID.Value = aper.OctetString(ulTeid)

	transferBytes, _ := aper.MarshalWithParams(transfer, "valueExt")
	switchedItem.PathSwitchRequestAcknowledgeTransfer = aper.OctetString(transferBytes)

	switchedList.List = append(switchedList.List, switchedItem)
	ackIEs.List = append(ackIEs.List, ie)

	return ngap.Encoder(pdu)
}
