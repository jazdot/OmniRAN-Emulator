package ue_mobility_management

import (
	"testing"

	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

// TestBuildPathSwitchRequest validates that the NGAP PathSwitchRequest
// PDU is correctly constructed and can round-trip through the encoder.
func TestBuildPathSwitchRequest(t *testing.T) {
	ranUeNgapID := int64(1)
	amfUeNgapID := int64(100)
	plmn := []byte{0x02, 0xF8, 0x39} // MCC=208, MNC=93
	tac := []byte{0x00, 0x00, 0x01}
	pduSessionId := uint8(1)
	gnbIp := []byte{10, 0, 0, 1}
	dlTeid := []byte{0x00, 0x00, 0x00, 0x01}

	gnbId := []byte{0x00, 0x00, 0x02} // Dummy target gNB ID (2)
	pdu := BuildPathSwitchRequest(ranUeNgapID, amfUeNgapID, plmn, tac, pduSessionId, gnbIp, dlTeid, gnbId)

	// Verify PDU type
	if pdu.Present != ngapType.NGAPPDUPresentInitiatingMessage {
		t.Fatalf("expected InitiatingMessage, got %d", pdu.Present)
	}

	msg := pdu.InitiatingMessage
	if msg == nil {
		t.Fatal("InitiatingMessage is nil")
	}
	if msg.ProcedureCode.Value != ngapType.ProcedureCodePathSwitchRequest {
		t.Fatalf("expected ProcedureCodePathSwitchRequest, got %d", msg.ProcedureCode.Value)
	}
	if msg.Value.Present != ngapType.InitiatingMessagePresentPathSwitchRequest {
		t.Fatalf("expected PathSwitchRequest present, got %d", msg.Value.Present)
	}

	psReq := msg.Value.PathSwitchRequest
	if psReq == nil {
		t.Fatal("PathSwitchRequest is nil")
	}

	// Verify mandatory IEs are present
	foundRanUeId := false
	foundAmfUeId := false
	foundUserLoc := false
	foundUeSec := false
	foundPduList := false

	for _, ie := range psReq.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDRANUENGAPID:
			foundRanUeId = true
			if ie.Value.RANUENGAPID.Value != ranUeNgapID {
				t.Errorf("RAN UE NGAP ID: expected %d, got %d", ranUeNgapID, ie.Value.RANUENGAPID.Value)
			}
		case ngapType.ProtocolIEIDSourceAMFUENGAPID:
			foundAmfUeId = true
			if ie.Value.SourceAMFUENGAPID.Value != amfUeNgapID {
				t.Errorf("AMF UE NGAP ID: expected %d, got %d", amfUeNgapID, ie.Value.SourceAMFUENGAPID.Value)
			}
		case ngapType.ProtocolIEIDUserLocationInformation:
			foundUserLoc = true
		case ngapType.ProtocolIEIDUESecurityCapabilities:
			foundUeSec = true
		case ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList:
			foundPduList = true
			if len(ie.Value.PDUSessionResourceToBeSwitchedDLList.List) != 1 {
				t.Errorf("expected 1 PDU session item, got %d", len(ie.Value.PDUSessionResourceToBeSwitchedDLList.List))
			}
			item := ie.Value.PDUSessionResourceToBeSwitchedDLList.List[0]
			if item.PDUSessionID.Value != int64(pduSessionId) {
				t.Errorf("PDU session ID: expected %d, got %d", pduSessionId, item.PDUSessionID.Value)
			}
		}
	}

	if !foundRanUeId {
		t.Error("missing RANUENGAPID IE")
	}
	if !foundAmfUeId {
		t.Error("missing SourceAMFUENGAPID IE")
	}
	if !foundUserLoc {
		t.Error("missing UserLocationInformation IE")
	}
	if !foundUeSec {
		t.Error("missing UESecurityCapabilities IE")
	}
	if !foundPduList {
		t.Error("missing PDUSessionResourceToBeSwitchedDLList IE")
	}
}

// TestGetPathSwitchRequest verifies the PDU encodes without error.
func TestGetPathSwitchRequest(t *testing.T) {
	plmn := []byte{0x02, 0xF8, 0x39}
	tac := []byte{0x00, 0x00, 0x01}
	gnbIp := []byte{192, 168, 0, 1}
	dlTeid := []byte{0x00, 0x00, 0x00, 0x42}

	gnbId := []byte{0x00, 0x00, 0x02}
	encoded, err := GetPathSwitchRequest(1, 200, plmn, tac, 1, gnbIp, dlTeid, gnbId)
	if err != nil {
		t.Fatalf("GetPathSwitchRequest encoding failed: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("encoded bytes are empty")
	}

	// Verify it decodes back
	pdu, err := ngap.Decoder(encoded)
	if err != nil {
		t.Fatalf("ngap.Decoder failed on encoded message: %v", err)
	}
	if pdu.Present != ngapType.NGAPPDUPresentInitiatingMessage {
		t.Errorf("decoded PDU present mismatch: got %d", pdu.Present)
	}
}
