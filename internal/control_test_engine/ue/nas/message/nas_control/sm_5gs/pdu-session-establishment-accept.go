package sm_5gs

import (
	"fmt"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control"
	"OmniRAN-Emulator/lib/nas"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func DecodeNasPduAccept(ngapMsg *ngapType.NGAPPDU) (*nas.Message, error) {

	// get NasPdu from DlNas.
	nasPdu := nas_control.GetNasPduFromDlNas(ngapMsg.InitiatingMessage.Value.PDUSessionResourceSetupRequest)
	if nasPdu == nil {
		return nil, fmt.Errorf("Error in get NasPdu from DL NAS message")
	}

	// get NasPdu from Pdu Session establishment accept.
	nasPduPayload := nas_control.GetNasPduFromPduAccept(nasPdu)
	if nasPduPayload == nil {
		return nil, fmt.Errorf("Error in get NasPdu from Pdu Session establishment accept message")
	}

	return nasPduPayload, nil
}

func GetPduAdress(m *nas.Message) [12]uint8 {
	return m.PDUSessionEstablishmentAccept.GetPDUAddressInformation()
}
