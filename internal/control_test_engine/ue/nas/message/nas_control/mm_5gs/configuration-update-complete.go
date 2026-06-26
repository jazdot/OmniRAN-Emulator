package mm_5gs

import (
	"bytes"
	"fmt"
	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control"
	"OmniRAN-Emulator/lib/nas"
	"OmniRAN-Emulator/lib/nas/nasMessage"
)

func ConfigurationUpdateComplete(ue *context.UEContext) ([]byte, error) {

	pdu := getConfigurationUpdateComplete()
	pdu, err := nas_control.EncodeNasPduWithSecurity(ue, pdu, nas.SecurityHeaderTypeIntegrityProtectedAndCiphered, true, false)
	if err != nil {
		return nil, fmt.Errorf("Error encoding %s IMSI UE NAS Configuration Update Complete Msg", ue.UeSecurity.Supi)
	}

	return pdu, nil
}

func getConfigurationUpdateComplete() (nasPdu []byte) {

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeConfigurationUpdateComplete)

	configurationUpdateComplete := nasMessage.NewConfigurationUpdateComplete(0)
	configurationUpdateComplete.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	configurationUpdateComplete.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	configurationUpdateComplete.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	configurationUpdateComplete.ConfigurationUpdateCompleteMessageIdentity.SetMessageType(nas.MsgTypeConfigurationUpdateComplete)

	m.GmmMessage.ConfigurationUpdateComplete = configurationUpdateComplete

	data := new(bytes.Buffer)
	err := m.GmmMessageEncode(data)
	if err != nil {
		fmt.Println(err.Error())
	}

	nasPdu = data.Bytes()
	return
}
