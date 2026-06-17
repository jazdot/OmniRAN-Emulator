package ngapType

import "OmniRAN-Emulator/lib/aper"

// Need to import "free5gc/lib/aper" if it uses "aper"

type PDUSessionResourceFailedToSetupItemSURes struct {
	PDUSessionID                                PDUSessionID
	PDUSessionResourceSetupUnsuccessfulTransfer aper.OctetString
	IEExtensions                                *ProtocolExtensionContainerPDUSessionResourceFailedToSetupItemSUResExtIEs `aper:"optional"`
}
