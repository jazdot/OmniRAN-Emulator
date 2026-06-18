package mm_5gs

import (
	"fmt"

	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
	nas_control "OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control"
	"OmniRAN-Emulator/lib/nas"
	"OmniRAN-Emulator/lib/nas/nasMessage"
)

// DeregistrationRequest builds an encoded 5GS UE-initiated Deregistration
// Request NAS PDU (3GPP TS 24.501 §8.2.11).
//
// switchOff=true  → power-off (UE goes off immediately, no response expected).
// switchOff=false → normal deregistration (AMF sends DeregistrationAccept).
func DeregistrationRequest(ue *context.UEContext, switchOff bool) ([]byte, error) {

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration)

	dereg := nasMessage.NewDeregistrationRequestUEOriginatingDeregistration(0)
	dereg.DeregistrationRequestMessageIdentity.SetMessageType(nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration)

	// NgksiAndDeregistrationType: TSC | KSI | SwitchOff | ReReg | AccessType
	dereg.NgksiAndDeregistrationType.SetTSC(nasMessage.TypeOfSecurityContextFlagNative)
	dereg.NgksiAndDeregistrationType.SetNasKeySetIdentifiler(ue.GetUeId())
	if switchOff {
		dereg.NgksiAndDeregistrationType.SetSwitchOff(1) // 1 = switch-off
	} else {
		dereg.NgksiAndDeregistrationType.SetSwitchOff(0) // 0 = normal
	}
	dereg.NgksiAndDeregistrationType.SetAccessType(0x01) // 01 = 3GPP access

	// Mobile identity: use SUCI (always populated after NewRanUeContext)
	dereg.MobileIdentity5GS = ue.GetSuci()

	m.GmmMessage.DeregistrationRequestUEOriginatingDeregistration = dereg

	// Plain encode first
	pdu, err := m.PlainNasEncode()
	if err != nil {
		return nil, fmt.Errorf("error encoding DeregistrationRequest for %s: %w", ue.UeSecurity.Supi, err)
	}

	// Apply NAS security if UE is in REGISTERED or SERVICE_REQ_INIT state
	if ue.GetStateMM() == context.MM5G_REGISTERED || ue.GetStateMM() == context.MM5G_SERVICE_REQ_INIT {
		secured, secErr := nas_control.EncodeNasPduWithSecurity(
			ue, pdu,
			nas.SecurityHeaderTypeIntegrityProtectedAndCiphered,
			true, false,
		)
		if secErr == nil {
			return secured, nil
		}
		// Fall back to plain NAS on security failure (shouldn't happen)
	}

	return pdu, nil
}
