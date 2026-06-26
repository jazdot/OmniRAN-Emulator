package ue_mobility_management

import (
	"OmniRAN-Emulator/lib/aper"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func GetPathSwitchRequest(ranUeNgapID int64, amfUeNgapID int64, plmn []byte, tac []byte, pduSessionId uint8, gnbIp []byte, dlTeid []byte, gnbId []byte, cellId ...int64) ([]byte, error) {
	message := BuildPathSwitchRequest(ranUeNgapID, amfUeNgapID, plmn, tac, pduSessionId, gnbIp, dlTeid, gnbId, cellId...)
	return ngap.Encoder(message)
}

func BuildPathSwitchRequest(ranUeNgapID int64, amfUeNgapID int64, plmn []byte, tac []byte, pduSessionId uint8, gnbIp []byte, dlTeid []byte, gnbId []byte, cellId ...int64) (pdu ngapType.NGAPPDU) {
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodePathSwitchRequest
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentPathSwitchRequest
	initiatingMessage.Value.PathSwitchRequest = new(ngapType.PathSwitchRequest)

	pathSwitchRequest := initiatingMessage.Value.PathSwitchRequest
	pathSwitchRequestIEs := &pathSwitchRequest.ProtocolIEs

	// RANUENGAPID
	ie := ngapType.PathSwitchRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PathSwitchRequestIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ie.Value.RANUENGAPID.Value = ranUeNgapID
	pathSwitchRequestIEs.List = append(pathSwitchRequestIEs.List, ie)

	// SourceAMFUENGAPID (populate with the UE's current AMF UE NGAP ID)
	ie = ngapType.PathSwitchRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSourceAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PathSwitchRequestIEsPresentSourceAMFUENGAPID
	ie.Value.SourceAMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.SourceAMFUENGAPID.Value = amfUeNgapID
	pathSwitchRequestIEs.List = append(pathSwitchRequestIEs.List, ie)

	// UserLocationInformation
	ie = ngapType.PathSwitchRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUserLocationInformation
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestIEsPresentUserLocationInformation
	ie.Value.UserLocationInformation = new(ngapType.UserLocationInformation)
	userLoc := ie.Value.UserLocationInformation
	userLoc.Present = ngapType.UserLocationInformationPresentUserLocationInformationNR
	userLoc.UserLocationInformationNR = new(ngapType.UserLocationInformationNR)
	userLocNR := userLoc.UserLocationInformationNR
	userLocNR.NRCGI.PLMNIdentity.Value = plmn
	// NRCellIdentity is 36 bits. We construct it by using the gnbId (24 bits = 3 bytes)
	// and padding it with cell ID bits. (gnbId + 0x00 + 0x10)
	cellIdBytes := make([]byte, 5)
	if len(gnbId) >= 3 {
		copy(cellIdBytes[0:3], gnbId[0:3])
	}
	cid := int64(1)
	if len(cellId) > 0 {
		cid = cellId[0]
	}
	cellIdBytes[3] = byte((cid >> 4) & 0xFF)
	cellIdBytes[4] = byte((cid & 0x0F) << 4)

	userLocNR.NRCGI.NRCellIdentity.Value = aper.BitString{
		Bytes:     cellIdBytes,
		BitLength: 36,
	}
	userLocNR.TAI.PLMNIdentity.Value = plmn
	userLocNR.TAI.TAC.Value = tac
	pathSwitchRequestIEs.List = append(pathSwitchRequestIEs.List, ie)

	// UESecurityCapabilities — field names use lowercase 'e' prefix per ngapType definition
	ie = ngapType.PathSwitchRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUESecurityCapabilities
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestIEsPresentUESecurityCapabilities
	ie.Value.UESecurityCapabilities = new(ngapType.UESecurityCapabilities)
	ueSec := ie.Value.UESecurityCapabilities
	ueSec.NRencryptionAlgorithms.Value = aper.BitString{Bytes: []byte{0xe0, 0x00}, BitLength: 16}
	ueSec.NRintegrityProtectionAlgorithms.Value = aper.BitString{Bytes: []byte{0xe0, 0x00}, BitLength: 16}
	ueSec.EUTRAencryptionAlgorithms.Value = aper.BitString{Bytes: []byte{0xe0, 0x00}, BitLength: 16}
	ueSec.EUTRAintegrityProtectionAlgorithms.Value = aper.BitString{Bytes: []byte{0xe0, 0x00}, BitLength: 16}
	pathSwitchRequestIEs.List = append(pathSwitchRequestIEs.List, ie)

	// PDUSessionResourceToBeSwitchedDLList
	ie = ngapType.PathSwitchRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PathSwitchRequestIEsPresentPDUSessionResourceToBeSwitchedDLList
	ie.Value.PDUSessionResourceToBeSwitchedDLList = new(ngapType.PDUSessionResourceToBeSwitchedDLList)

	pduList := ie.Value.PDUSessionResourceToBeSwitchedDLList
	pduItem := ngapType.PDUSessionResourceToBeSwitchedDLItem{}
	pduItem.PDUSessionID.Value = int64(pduSessionId)

	// Build PathSwitchRequestTransfer and APER-encode it into an OctetString
	transferPdu := ngapType.PathSwitchRequestTransfer{}
	transferPdu.DLNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	transferPdu.DLNGUUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)
	gtp := transferPdu.DLNGUUPTNLInformation.GTPTunnel
	gtp.TransportLayerAddress.Value = aper.BitString{
		Bytes:     gnbIp,
		BitLength: uint64(len(gnbIp) * 8),
	}
	gtp.GTPTEID.Value = aper.OctetString(dlTeid)

	transferPdu.QosFlowAcceptedList.List = append(transferPdu.QosFlowAcceptedList.List, ngapType.QosFlowAcceptedItem{
		QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: 1},
	})

	// PathSwitchRequestTransfer is a separate APER-encoded value (OctetString)
	transferBytes, _ := aper.MarshalWithParams(transferPdu, "valueExt")
	pduItem.PathSwitchRequestTransfer = aper.OctetString(transferBytes)

	pduList.List = append(pduList.List, pduItem)
	pathSwitchRequestIEs.List = append(pathSwitchRequestIEs.List, ie)

	return
}
