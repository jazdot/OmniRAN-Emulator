package mm_5gs

import (
	"bytes"

	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/lib/nas"
	"OmniRAN-Emulator/lib/nas/nasMessage"
	"OmniRAN-Emulator/lib/nas/nasType"
)

// IdentityResponse builds a NAS Identity Response message for the given identity type.
// Per TS 24.501 §5.4.4, the UE responds with the requested identity (SUCI, IMEI, 5G-GUTI, etc.).
func IdentityResponse(ue *context.UEContext, identityType uint8) []byte {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeIdentityResponse)

	identityResponse := nasMessage.NewIdentityResponse(nas.MsgTypeIdentityResponse)
	identityResponse.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	identityResponse.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)

	switch identityType {
	case nasMessage.MobileIdentity5GSTypeSuci: // 0x01 = SUCI
		suci := ue.GetSuci()
		identityResponse.MobileIdentity = nasType.MobileIdentity{
			Iei:    0,
			Len:    suci.Len,
			Buffer: suci.Buffer,
		}
		log.Info("[UE][NAS] Responding with SUCI identity")

	default:
		// For other identity types, respond with SUCI as fallback
		suci := ue.GetSuci()
		identityResponse.MobileIdentity = nasType.MobileIdentity{
			Iei:    0,
			Len:    suci.Len,
			Buffer: suci.Buffer,
		}
		log.Infof("[UE][NAS] Identity type %d requested, responding with SUCI as fallback", identityType)
	}

	m.GmmMessage.IdentityResponse = identityResponse

	var b bytes.Buffer
	m.GmmMessageEncode(&b)
	return b.Bytes()
}
