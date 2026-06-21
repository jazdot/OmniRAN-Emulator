package nas_control

import (
	"fmt"
	"OmniRAN-Emulator/internal/chaos"
	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/lib/nas"
	"OmniRAN-Emulator/lib/nas/nasMessage"
	"OmniRAN-Emulator/lib/nas/security"
)

func getNasMsgType(m *nas.Message) string {
	if m == nil {
		return "Unknown"
	}
	if m.GmmMessage != nil {
		msgType := m.GmmHeader.GetMessageType()
		switch msgType {
		case 0x41:
			return "RegistrationRequest"
		case 0x42:
			return "RegistrationAccept"
		case 0x43:
			return "RegistrationComplete"
		case 0x44:
			return "RegistrationReject"
		case 0x45:
			return "DeregistrationRequest"
		case 0x46:
			return "DeregistrationAccept"
		case 0x54:
			return "ServiceRequest"
		case 0x56:
			return "ServiceReject"
		case 0x57:
			return "ServiceAccept"
		case 0x64:
			return "AuthenticationRequest"
		case 0x65:
			return "AuthenticationResponse"
		case 0x67:
			return "AuthenticationFailure"
		case 0x68:
			return "SecurityModeCommand"
		case 0x69:
			return "SecurityModeComplete"
		case 0x6a:
			return "SecurityModeReject"
		case 0x6e:
			return "IdentityRequest"
		case 0x6f:
			return "IdentityResponse"
		}
	}
	if m.GsmMessage != nil {
		msgType := m.GsmHeader.GetMessageType()
		switch msgType {
		case 0xc1:
			return "PduSessionEstablishmentRequest"
		case 0xc2:
			return "PduSessionEstablishmentAccept"
		case 0xc3:
			return "PduSessionEstablishmentReject"
		case 0xc5:
			return "PduSessionModificationRequest"
		case 0xc6:
			return "PduSessionModificationCommand"
		case 0xc7:
			return "PduSessionModificationComplete"
		case 0xca:
			return "PduSessionReleaseRequest"
		case 0xcb:
			return "PduSessionReleaseCommand"
		case 0xcc:
			return "PduSessionReleaseComplete"
		}
	}
	return "NAS-Message"
}

func EncodeNasPduWithSecurity(ue *context.UEContext, pdu []byte, securityHeaderType uint8, securityContextAvailable, newSecurityContext bool) ([]byte, error) {
	m := nas.NewMessage()
	err := m.PlainNasDecode(&pdu)
	if err != nil {
		return nil, err
	}
	
	// Record plain message type in chaos state for drop/delay eval
	if ue != nil {
		chaos.SetLastNasMsgType(ue.GetUeId(), getNasMsgType(m))
	}

	m.SecurityHeader = nas.SecurityHeader{
		ProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		SecurityHeaderType:    securityHeaderType,
	}
	return NASEncode(ue, m, securityContextAvailable, newSecurityContext)
}

func NASEncode(ue *context.UEContext, msg *nas.Message, securityContextAvailable bool, newSecurityContext bool) (payload []byte, err error) {
	var sequenceNumber uint8
	if ue == nil {
		err = fmt.Errorf("amfUe is nil")
		return
	}
	if msg == nil {
		err = fmt.Errorf("Nas message is empty")
		return
	}

	if !securityContextAvailable {
		return msg.PlainNasEncode()
	} else {
		if newSecurityContext {
			ue.UeSecurity.ULCount.Set(0, 0)
			ue.UeSecurity.DLCount.Set(0, 0)
		}

		sequenceNumber = ue.UeSecurity.ULCount.SQN()
		payload, err = msg.PlainNasEncode()
		if err != nil {
			return
		}

		// TODO: Support for ue has nas connection in both accessType
		// make ciphering of NAS message.
		if err = security.NASEncrypt(ue.UeSecurity.CipheringAlg, ue.UeSecurity.KnasEnc, ue.UeSecurity.ULCount.Get(), security.Bearer3GPP,
			security.DirectionUplink, payload); err != nil {
			return
		}

		// add sequence number
		payload = append([]byte{sequenceNumber}, payload[:]...)
		mac32 := make([]byte, 4)

		mac32, err = security.NASMacCalculate(ue.UeSecurity.IntegrityAlg, ue.UeSecurity.KnasInt, ue.UeSecurity.ULCount.Get(), security.Bearer3GPP, security.DirectionUplink, payload)
		if err != nil {
			return
		}

		// Add mac value
		payload = append(mac32, payload[:]...)
		// Add EPD and Security Type
		msgSecurityHeader := []byte{msg.SecurityHeader.ProtocolDiscriminator, msg.SecurityHeader.SecurityHeaderType}
		payload = append(msgSecurityHeader, payload[:]...)

		// Increase UL Count
		ue.UeSecurity.ULCount.AddOne()
	}
	return
}
