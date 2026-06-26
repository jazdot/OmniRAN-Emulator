package ue_mobility_management

import (
	"testing"

	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func TestHandoverRequired(t *testing.T) {
	ranUeNgapID := int64(5)
	amfUeNgapID := int64(105)
	targetMcc := "999"
	targetMnc := "70"
	targetGnbIdVal := int64(2)
	targetTacVal := []byte{0x00, 0x00, 0x02}
	pduSessionId := uint8(1)
	sourceGnbId := []byte{0x00, 0x00, 0x01}

	encoded, err := GetHandoverRequired(ranUeNgapID, amfUeNgapID, targetMcc, targetMnc, targetGnbIdVal, targetTacVal, pduSessionId, sourceGnbId)
	if err != nil {
		t.Fatalf("Failed to encode HandoverRequired: %v", err)
	}

	pdu, err := ngap.Decoder(encoded)
	if err != nil {
		t.Fatalf("Failed to decode HandoverRequired: %v", err)
	}

	if err := ngap_control.ValidateNGAPMessage(pdu); err != nil {
		t.Errorf("HandoverRequired failed 3GPP schema validation: %v", err)
	}

	if pdu.Present != ngapType.NGAPPDUPresentInitiatingMessage {
		t.Errorf("Expected InitiatingMessage, got %v", pdu.Present)
	}
}

func TestHandoverRequestAcknowledge(t *testing.T) {
	ranUeNgapID := int64(5)
	amfUeNgapID := int64(105)
	pduSessionId := uint8(1)
	gnbIp := []byte{127, 0, 0, 1}
	dlTeid := []byte{0x00, 0x00, 0x00, 0x05}
	qosId := int64(9)

	encoded, err := GetHandoverRequestAcknowledge(ranUeNgapID, amfUeNgapID, pduSessionId, gnbIp, dlTeid, qosId)
	if err != nil {
		t.Fatalf("Failed to encode HandoverRequestAcknowledge: %v", err)
	}

	pdu, err := ngap.Decoder(encoded)
	if err != nil {
		t.Fatalf("Failed to decode HandoverRequestAcknowledge: %v", err)
	}

	if err := ngap_control.ValidateNGAPMessage(pdu); err != nil {
		t.Errorf("HandoverRequestAcknowledge failed 3GPP schema validation: %v", err)
	}

	if pdu.Present != ngapType.NGAPPDUPresentSuccessfulOutcome {
		t.Errorf("Expected SuccessfulOutcome, got %v", pdu.Present)
	}
}

func TestHandoverNotify(t *testing.T) {
	ranUeNgapID := int64(5)
	amfUeNgapID := int64(105)
	plmn := []byte{0x02, 0xf8, 0x39}
	tac := []byte{0x00, 0x00, 0x02}
	gnbId := []byte{0x00, 0x00, 0x01}

	encoded, err := GetHandoverNotify(ranUeNgapID, amfUeNgapID, plmn, tac, gnbId)
	if err != nil {
		t.Fatalf("Failed to encode HandoverNotify: %v", err)
	}

	pdu, err := ngap.Decoder(encoded)
	if err != nil {
		t.Fatalf("Failed to decode HandoverNotify: %v", err)
	}

	if err := ngap_control.ValidateNGAPMessage(pdu); err != nil {
		t.Errorf("HandoverNotify failed 3GPP schema validation: %v", err)
	}

	if pdu.Present != ngapType.NGAPPDUPresentInitiatingMessage {
		t.Errorf("Expected InitiatingMessage, got %v", pdu.Present)
	}
}
