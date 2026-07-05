package mm_5gs

import (
	"bytes"
	"fmt"
	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control"
	"OmniRAN-Emulator/lib/nas"
	"OmniRAN-Emulator/lib/nas/nasMessage"
)

func DeregistrationAcceptUETerminated(ue *context.UEContext) ([]byte, error) {

	pdu := getDeregistrationAcceptUETerminated()
	pdu, err := nas_control.EncodeNasPduWithSecurity(ue, pdu, nas.SecurityHeaderTypeIntegrityProtectedAndCiphered, true, false)
	if err != nil {
		return nil, fmt.Errorf("Error encoding %s IMSI UE NAS Deregistration Accept Msg", ue.UeSecurity.Supi)
	}

	return pdu, nil
}

func getDeregistrationAcceptUETerminated() (nasPdu []byte) {

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration)

	dereg := nasMessage.NewDeregistrationAcceptUETerminatedDeregistration(0)
	dereg.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	dereg.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	dereg.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	dereg.DeregistrationAcceptMessageIdentity.SetMessageType(nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration)

	m.GmmMessage.DeregistrationAcceptUETerminatedDeregistration = dereg

	data := new(bytes.Buffer)
	err := m.GmmMessageEncode(data)
	if err != nil {
		fmt.Println(err.Error())
	}

	nasPdu = data.Bytes()
	return
}
