package nas_control

import (
	"OmniRAN-Emulator/lib/nas"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func GetNasPduFromDownlink(msg *ngapType.DownlinkNASTransport) (m *nas.Message) {
	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDNASPDU {
			pkg := []byte(ie.Value.NASPDU.Value)
			m = new(nas.Message)
			err := m.PlainNasDecode(&pkg)
			if err != nil {
				return nil
			}
			return
		}
	}
	return nil
}

func GetNasPduFromPduAccept(dlNas *nas.Message) (m *nas.Message) {
	if dlNas == nil || dlNas.DLNASTransport == nil {
		return nil
	}

	// get payload container from DL NAS.
	payload := dlNas.DLNASTransport.GetPayloadContainerContents()
	if len(payload) == 0 {
		return nil
	}

	m = new(nas.Message)
	err := m.PlainNasDecode(&payload)
	if err == nil && m.GsmHeader.GetMessageType() != 0 {
		return m
	}

	// If plain NAS decode failed or message type is empty, check for 7-byte security header (e.g. 0x7E ...)
	if len(payload) > 7 && payload[0] == 0x7e {
		stripped := payload[7:]
		m2 := new(nas.Message)
		if err2 := m2.PlainNasDecode(&stripped); err2 == nil && m2.GsmHeader.GetMessageType() != 0 {
			return m2
		}
	}

	return nil
}

func GetNasPduFromDlNas(msg *ngapType.PDUSessionResourceSetupRequest) (m *nas.Message) {
	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq {
			pDUSessionResourceSetupList := ie.Value.PDUSessionResourceSetupListSUReq
			for _, item := range pDUSessionResourceSetupList.List {
				// get PDUSessionNas-PDU
				payload := []byte(item.PDUSessionNASPDU.Value)
				// remove security header.
				payload = payload[7:]
				m := new(nas.Message)
				err := m.PlainNasDecode(&payload)
				if err != nil {
					return nil
				}
				return m
			}
		}
	}
	return nil
}
