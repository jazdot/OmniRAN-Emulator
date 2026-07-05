package pdu_session_management

import (
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/lib/aper"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func PDUSessionResourceModifyResponse(ue *context.GNBUe, modifiedPduSessionIds []int64) ([]byte, error) {
	message := buildPDUSessionResourceModifyResponse(ue.GetAmfUeId(), ue.GetRanUeId(), ue, modifiedPduSessionIds)
	return ngap.Encoder(message)
}

func buildPDUSessionResourceModifyResponse(amfUeNgapID, ranUeNgapID int64, ue *context.GNBUe, modifiedPduSessionIds []int64) (pdu ngapType.NGAPPDU) {

	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodePDUSessionResourceModify
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject

	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyResponse
	successfulOutcome.Value.PDUSessionResourceModifyResponse = new(ngapType.PDUSessionResourceModifyResponse)

	pduSessionResourceModifyResponse := successfulOutcome.Value.PDUSessionResourceModifyResponse
	pduSessionResourceModifyResponseIEs := &pduSessionResourceModifyResponse.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.PDUSessionResourceModifyResponseIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PDUSessionResourceModifyResponseIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.AMFUENGAPID.Value = amfUeNgapID
	pduSessionResourceModifyResponseIEs.List = append(pduSessionResourceModifyResponseIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.PDUSessionResourceModifyResponseIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PDUSessionResourceModifyResponseIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ie.Value.RANUENGAPID.Value = ranUeNgapID
	pduSessionResourceModifyResponseIEs.List = append(pduSessionResourceModifyResponseIEs.List, ie)

	// PDU Session Resource Modify Response List
	if len(modifiedPduSessionIds) > 0 {
		ie = ngapType.PDUSessionResourceModifyResponseIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceModifyListModRes
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PDUSessionResourceModifyResponseIEsPresentPDUSessionResourceModifyListModRes
		ie.Value.PDUSessionResourceModifyListModRes = new(ngapType.PDUSessionResourceModifyListModRes)

		listModRes := ie.Value.PDUSessionResourceModifyListModRes
		for _, pduSessionId := range modifiedPduSessionIds {
			item := ngapType.PDUSessionResourceModifyItemModRes{}
			item.PDUSessionID.Value = pduSessionId
			
			// Build Transfer
			transfer := ngapType.PDUSessionResourceModifyResponseTransfer{}
			transfer.QosFlowAddOrModifyResponseList = new(ngapType.QosFlowAddOrModifyResponseList)
			
			qosId := ue.GetQosIdOfSession(pduSessionId)
			qosItem := ngapType.QosFlowAddOrModifyResponseItem{}
			qosItem.QosFlowIdentifier.Value = qosId
			transfer.QosFlowAddOrModifyResponseList.List = append(transfer.QosFlowAddOrModifyResponseList.List, qosItem)

			transferBytes, _ := aper.MarshalWithParams(transfer, "valueExt")
			octetBytes := aper.OctetString(transferBytes)
			item.PDUSessionResourceModifyResponseTransfer = &octetBytes

			listModRes.List = append(listModRes.List, item)
		}
		pduSessionResourceModifyResponseIEs.List = append(pduSessionResourceModifyResponseIEs.List, ie)
	}

	return
}
