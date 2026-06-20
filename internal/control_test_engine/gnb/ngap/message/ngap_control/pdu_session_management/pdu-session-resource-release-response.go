package pdu_session_management

import (
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
	"fmt"
)

// PDUSessionResourceReleaseResponse builds the NGAP PDU Session Resource Release Response
// sent by the gNB to the AMF after receiving and processing a PDU Session Resource Release Command.
func PDUSessionResourceReleaseResponse(ue *context.GNBUe) ([]byte, error) {
	pdu := buildPDUSessionResourceReleaseResponse(ue.GetAmfUeId(), ue.GetRanUeId())
	encoded, err := ngap.Encoder(pdu)
	if err != nil {
		return nil, fmt.Errorf("error encoding PDU Session Resource Release Response: %w", err)
	}
	return encoded, nil
}

func buildPDUSessionResourceReleaseResponse(amfUeNgapID, ranUeNgapID int64) (pdu ngapType.NGAPPDU) {
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

	return pdu
}
