package context

import (
	"testing"

	"OmniRAN-Emulator/lib/nas/nasMessage"
)

// TestSetAndGetRegistrationType verifies the registration type setter/getter pair.
func TestSetAndGetRegistrationType(t *testing.T) {
	tests := []struct {
		name     string
		setValue uint8
		wantGet  uint8
	}{
		{
			name:     "InitialRegistration",
			setValue: nasMessage.RegistrationType5GSInitialRegistration,
			wantGet:  nasMessage.RegistrationType5GSInitialRegistration,
		},
		{
			name:     "MobilityRegistration",
			setValue: nasMessage.RegistrationType5GSMobilityRegistrationUpdating,
			wantGet:  nasMessage.RegistrationType5GSMobilityRegistrationUpdating,
		},
		{
			name:     "PeriodicRegistration",
			setValue: nasMessage.RegistrationType5GSPeriodicRegistrationUpdating,
			wantGet:  nasMessage.RegistrationType5GSPeriodicRegistrationUpdating,
		},
		{
			name:     "EmergencyRegistration",
			setValue: nasMessage.RegistrationType5GSEmergencyRegistration,
			wantGet:  nasMessage.RegistrationType5GSEmergencyRegistration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ue := &UEContext{}
			ue.SetRegistrationType(tt.setValue)
			got := ue.GetRegistrationType()
			if got != tt.wantGet {
				t.Errorf("GetRegistrationType() = 0x%02x, want 0x%02x", got, tt.wantGet)
			}
		})
	}
}

// TestGetRegistrationType_DefaultsToInitial verifies that a zero RegistrationType
// returns InitialRegistration as the safe default.
func TestGetRegistrationType_DefaultsToInitial(t *testing.T) {
	ue := &UEContext{}
	// Don't set — should default to Initial
	got := ue.GetRegistrationType()
	if got != nasMessage.RegistrationType5GSInitialRegistration {
		t.Errorf("expected default InitialRegistration (0x%02x), got 0x%02x",
			nasMessage.RegistrationType5GSInitialRegistration, got)
	}
}

// TestSetAndGetAmfUeId verifies the AMF UE NGAP ID setter/getter pair.
func TestSetAndGetAmfUeId(t *testing.T) {
	ue := &UEContext{}
	expected := int64(12345678)
	ue.SetAmfUeId(expected)
	got := ue.GetAmfUeId()
	if got != expected {
		t.Errorf("GetAmfUeId() = %d, want %d", got, expected)
	}
}

// TestAmfUeId_Zero verifies zero is returned when no AMF UE ID has been set.
func TestAmfUeId_Zero(t *testing.T) {
	ue := &UEContext{}
	if ue.GetAmfUeId() != 0 {
		t.Errorf("expected 0 before SetAmfUeId, got %d", ue.GetAmfUeId())
	}
}

// TestSetRegistrationType_Idempotent verifies setting the same value twice is safe.
func TestSetRegistrationType_Idempotent(t *testing.T) {
	ue := &UEContext{}
	ue.SetRegistrationType(nasMessage.RegistrationType5GSEmergencyRegistration)
	ue.SetRegistrationType(nasMessage.RegistrationType5GSEmergencyRegistration)
	got := ue.GetRegistrationType()
	if got != nasMessage.RegistrationType5GSEmergencyRegistration {
		t.Errorf("idempotent set failed: got 0x%02x", got)
	}
}
