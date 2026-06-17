package mm_5gs

import (
	"bytes"
	"fmt"
	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control"
	"OmniRAN-Emulator/lib/nas"
	"OmniRAN-Emulator/lib/nas/nasMessage"
)

func ServiceRequest(ue *context.UEContext, serviceType uint8) ([]byte, error) {

	pdu := getServiceRequest(ue, serviceType)
	pdu, err := nas_control.EncodeNasPduWithSecurity(ue, pdu, nas.SecurityHeaderTypeIntegrityProtectedAndCiphered, true, false)
	if err != nil {
		return nil, fmt.Errorf("Error encoding %s IMSI UE NAS Service Request Msg", ue.UeSecurity.Supi)
	}

	return pdu, nil
}

func getServiceRequest(ue *context.UEContext, serviceType uint8) (nasPdu []byte) {

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeServiceRequest)

	serviceRequest := nasMessage.NewServiceRequest(0)
	serviceRequest.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	serviceRequest.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	serviceRequest.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(0)
	serviceRequest.ServiceRequestMessageIdentity.SetMessageType(nas.MsgTypeServiceRequest)

	// Set Service Type and KSI
	serviceRequest.ServiceTypeAndNgksi.SetServiceTypeValue(serviceType)
	serviceRequest.ServiceTypeAndNgksi.SetNasKeySetIdentifiler(0x07) // No key is available or TSC key index

	// Set 5G-S-TMSI
	serviceRequest.TMSI5GS.SetLen(7)
	serviceRequest.TMSI5GS.SetTypeOfIdentity(nasMessage.MobileIdentity5GSType5gSTmsi)
	serviceRequest.TMSI5GS.SetAMFSetID(ue.GetAmfSetId())
	serviceRequest.TMSI5GS.SetAMFPointer(ue.GetAmfPointer())
	serviceRequest.TMSI5GS.SetTMSI5G(ue.Get5gGuti())

	m.GmmMessage.ServiceRequest = serviceRequest

	data := new(bytes.Buffer)
	err := m.GmmMessageEncode(data)
	if err != nil {
		fmt.Println(err.Error())
	}

	nasPdu = data.Bytes()
	return
}
