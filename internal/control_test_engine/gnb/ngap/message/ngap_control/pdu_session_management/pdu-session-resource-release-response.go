package pdu_session_management

import (
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/lib/aper"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
	"fmt"
)

// PDUSessionResourceReleaseResponse builds the NGAP PDU Session Resource Release Response
// sent by the gNB to the AMF after receiving and processing a PDU Session Resource Release Command.
// Per TS 38.413, the response MUST include the PDUSessionResourceReleasedListRelRes IE.
func PDUSessionResourceReleaseResponse(ue *context.GNBUe, pduSessionIds []int64) ([]byte, error) {
	pdu := buildPDUSessionResourceReleaseResponse(ue.GetAmfUeId(), ue.GetRanUeId(), pduSessionIds)
	encoded, err := ngap.Encoder(pdu)
	if err != nil {
		return nil, fmt.Errorf("error encoding PDU Session Resource Release Response: %w", err)
	}
	return encoded, nil
}

func buildPDUSessionResourceReleaseResponse(amfUeNgapID, ranUeNgapID int64, pduSessionIds []int64) (pdu ngapType.NGAPPDU) {
	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)
	successfulOutcome := pdu.SuccessfulOutcome

	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodePDUSessionResourceRelease
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentPDUSessionResourceReleaseResponse
	successfulOutcome.Value.PDUSessionResourceReleaseResponse = new(ngapType.PDUSessionResourceReleaseResponse)

	resp := successfulOutcome.Value.PDUSessionResourceReleaseResponse
	respIEs := &resp.ProtocolIEs

	// AMF UE NGAP ID
	{
		ie := ngapType.PDUSessionResourceReleaseResponseIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PDUSessionResourceReleaseResponseIEsPresentAMFUENGAPID
		ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
		ie.Value.AMFUENGAPID.Value = amfUeNgapID
		respIEs.List = append(respIEs.List, ie)
	}

	// RAN UE NGAP ID
	{
		ie := ngapType.PDUSessionResourceReleaseResponseIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PDUSessionResourceReleaseResponseIEsPresentRANUENGAPID
		ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
		ie.Value.RANUENGAPID.Value = ranUeNgapID
		respIEs.List = append(respIEs.List, ie)
	}

	// PDU Session Resource Released List (Mandatory per TS 38.413 §9.2.3.6)
	{
		ie := ngapType.PDUSessionResourceReleaseResponseIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PDUSessionResourceReleaseResponseIEsPresentPDUSessionResourceReleasedListRelRes
		ie.Value.PDUSessionResourceReleasedListRelRes = new(ngapType.PDUSessionResourceReleasedListRelRes)

		releasedList := ie.Value.PDUSessionResourceReleasedListRelRes

		for _, sessionId := range pduSessionIds {
			// Build an empty PDUSessionResourceReleaseResponseTransfer and APER-encode it
			transfer := ngapType.PDUSessionResourceReleaseResponseTransfer{}
			transferBytes, err := aper.MarshalWithParams(transfer, "valueExt")
			if err != nil {
				// Fallback: use a minimal valid APER encoding (single zero byte)
				transferBytes = []byte{0x00}
			}

			item := ngapType.PDUSessionResourceReleasedItemRelRes{}
			item.PDUSessionID.Value = sessionId
			item.PDUSessionResourceReleaseResponseTransfer = transferBytes
			releasedList.List = append(releasedList.List, item)
		}

		respIEs.List = append(respIEs.List, ie)
	}

	return pdu
}

