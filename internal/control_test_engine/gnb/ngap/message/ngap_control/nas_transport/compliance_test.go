package nas_transport

import (
	"testing"
	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func TestInitialUEMessageReleaseCompliance(t *testing.T) {
	ranUeNgapID := int64(10)
	nasPdu := []byte{0x01, 0x02, 0x03}
	plmn := []byte{0x02, 0xf8, 0x39}
	tac := []byte{0x00, 0x00, 0x01}
	gnbId := []byte{0x00, 0x00, 0x01}

	releases := []struct {
		release string
		expectedCause int
	}{
		{"15", int(ngapType.RRCEstablishmentCausePresentMoSignalling)},
		{"17", int(ngapType.RRCEstablishmentCausePresentMtAccess)},
		{"18", int(ngapType.RRCEstablishmentCausePresentHighPriorityAccess)},
		{"19", int(ngapType.RRCEstablishmentCausePresentMoVoiceCall)},
	}

	// Backup active release
	originalRelease := config.GetActiveRelease()
	defer config.SetActiveRelease(originalRelease)

	for _, tc := range releases {
		t.Run("Release_"+tc.release, func(t *testing.T) {
			config.SetActiveRelease(tc.release)

			// Get the RRC cause value dynamically using the same logic as SendInitialUeMessage
			rrcCause := int(ngapType.RRCEstablishmentCausePresentMoSignalling)
			switch tc.release {
			case "17":
				rrcCause = int(ngapType.RRCEstablishmentCausePresentMtAccess)
			case "18":
				rrcCause = int(ngapType.RRCEstablishmentCausePresentHighPriorityAccess)
			case "19":
				rrcCause = int(ngapType.RRCEstablishmentCausePresentMoVoiceCall)
			}

			encoded, err := GetInitialUEMessage(ranUeNgapID, nasPdu, "", plmn, tac, rrcCause, gnbId)
			if err != nil {
				t.Fatalf("Failed to encode InitialUEMessage for Release %s: %v", tc.release, err)
			}

			pdu, err := ngap.Decoder(encoded)
			if err != nil {
				t.Fatalf("Failed to decode InitialUEMessage for Release %s: %v", tc.release, err)
			}

			if pdu.Present != ngapType.NGAPPDUPresentInitiatingMessage {
				t.Fatalf("Expected InitiatingMessage, got %d", pdu.Present)
			}

			initMsg := pdu.InitiatingMessage.Value.InitialUEMessage
			var foundCause bool
			for _, ie := range initMsg.ProtocolIEs.List {
				if ie.Id.Value == ngapType.ProtocolIEIDRRCEstablishmentCause {
					foundCause = true
					actualCause := int(ie.Value.RRCEstablishmentCause.Value)
					if actualCause != tc.expectedCause {
						t.Errorf("For Release %s, expected RRCEstablishmentCause %d, got %d", tc.release, tc.expectedCause, actualCause)
					}
					break
				}
			}
			if !foundCause {
				t.Errorf("RRCEstablishmentCause IE not found in InitialUEMessage for Release %s", tc.release)
			}
		})
	}
}
