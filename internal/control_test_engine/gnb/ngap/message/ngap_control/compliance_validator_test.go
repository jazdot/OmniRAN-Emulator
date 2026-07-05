package ngap_control

import (
	"net"
	"testing"
	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/interface_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/nas_transport"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/pdu_session_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/ue_context_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/ue_mobility_management"
	"OmniRAN-Emulator/lib/ngap"
)

func TestComplianceValidatorForGnbMessages(t *testing.T) {
	// Initialize GNodeB Context
	gnb := &context.GNBContext{}
	gnb.NewRanGnbContext("000001", "208", "93", "000001", "01", "010203", "127.0.0.1", "127.0.0.1", 9487, 2152)

	// Initialize UE Profile in GNodeB Context
	ue := &context.GNBUe{}
	ue.SetRanUeId(10)
	ue.SetAmfUeId(210)
	ue.SetTeidDownlink(5)
	ue.CreatePduSession(1, "01", "010203", 0, 9, 1, 5, 100, net.ParseIP("10.200.200.1"), 5)

	t.Run("NGSetupRequest", func(t *testing.T) {
		encoded, err := interface_management.NGSetupRequest(gnb, "GNB-Test")
		if err != nil {
			t.Fatalf("Failed to build NGSetupRequest: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("InitialUEMessage", func(t *testing.T) {
		// Test across releases
		originalRelease := config.GetActiveRelease()
		defer config.SetActiveRelease(originalRelease)

		for _, rel := range []string{"15", "17", "18", "19"} {
			config.SetActiveRelease(rel)
			encoded, err := nas_transport.SendInitialUeMessage([]byte{0x01, 0x02}, ue, gnb)
			if err != nil {
				t.Fatalf("Failed to build for Rel %s: %v", rel, err)
			}
			pdu, err := ngap.Decoder(encoded)
			if err != nil {
				t.Fatalf("Failed to decode for Rel %s: %v", rel, err)
			}
			if err := ValidateNGAPMessage(pdu); err != nil {
				t.Errorf("Validation failed for Rel %s: %v", rel, err)
			}
		}
	})

	t.Run("UplinkNASTransport", func(t *testing.T) {
		encoded, err := nas_transport.SendUplinkNasTransport([]byte{0x05, 0x06}, ue, gnb)
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("InitialContextSetupResponse", func(t *testing.T) {
		encoded, err := ue_context_management.InitialContextSetupResponse(ue, "127.0.0.1")
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("PDUSessionResourceSetupResponse", func(t *testing.T) {
		encoded, err := pdu_session_management.PDUSessionResourceSetupResponse(ue, "127.0.0.1", 1)
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("PDUSessionResourceReleaseResponse", func(t *testing.T) {
		encoded, err := pdu_session_management.PDUSessionResourceReleaseResponse(ue, []int64{1})
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("PDUSessionResourceModifyResponse", func(t *testing.T) {
		encoded, err := pdu_session_management.PDUSessionResourceModifyResponse(ue, []int64{1})
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("UEContextReleaseRequest", func(t *testing.T) {
		encoded, err := ue_context_management.GetUEContextReleaseRequest(ue.GetRanUeId(), ue.GetAmfUeId(), nil)
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("UEContextReleaseComplete", func(t *testing.T) {
		encoded, err := ue_context_management.GetUEContextReleaseComplete(ue.GetRanUeId(), ue.GetAmfUeId())
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("HandoverRequired", func(t *testing.T) {
		encoded, err := ue_mobility_management.GetHandoverRequired(
			ue.GetRanUeId(),
			ue.GetAmfUeId(),
			"208",
			"93",
			2,
			[]byte{0x00, 0x00, 0x02},
			1,
			gnb.GetGnbIdInBytes(),
		)
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("HandoverRequestAcknowledge", func(t *testing.T) {
		encoded, err := ue_mobility_management.GetHandoverRequestAcknowledge(
			ue.GetRanUeId(),
			ue.GetAmfUeId(),
			1,
			[]byte{127, 0, 0, 1},
			[]byte{0x00, 0x00, 0x00, 0x05},
			9,
		)
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("HandoverNotify", func(t *testing.T) {
		encoded, err := ue_mobility_management.GetHandoverNotify(
			ue.GetRanUeId(),
			ue.GetAmfUeId(),
			gnb.GetMccAndMncInOctets(),
			gnb.GetTacInBytes(),
			gnb.GetGnbIdInBytes(),
		)
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("PathSwitchRequest", func(t *testing.T) {
		encoded, err := ue_mobility_management.GetPathSwitchRequest(
			ue.GetRanUeId(),
			ue.GetAmfUeId(),
			gnb.GetMccAndMncInOctets(),
			gnb.GetTacInBytes(),
			1,
			[]byte{127, 0, 0, 1},
			[]byte{0x00, 0x00, 0x00, 0x05},
			gnb.GetGnbIdInBytes(),
		)
		if err != nil {
			t.Fatalf("Failed to build: %v", err)
		}
		pdu, err := ngap.Decoder(encoded)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if err := ValidateNGAPMessage(pdu); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})
}
