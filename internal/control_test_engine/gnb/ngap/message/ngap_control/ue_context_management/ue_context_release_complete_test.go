package ue_context_management

import (
	"testing"

	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func TestUEContextReleaseComplete(t *testing.T) {
	ranUeNgapID := int64(10)
	amfUeNgapID := int64(210)

	encoded, err := GetUEContextReleaseComplete(ranUeNgapID, amfUeNgapID)
	if err != nil {
		t.Fatalf("Failed to encode UEContextReleaseComplete: %v", err)
	}

	pdu, err := ngap.Decoder(encoded)
	if err != nil {
		t.Fatalf("Failed to decode UEContextReleaseComplete: %v", err)
	}

	if pdu.Present != ngapType.NGAPPDUPresentSuccessfulOutcome {
		t.Errorf("Expected SuccessfulOutcome, got %v", pdu.Present)
	}
}
