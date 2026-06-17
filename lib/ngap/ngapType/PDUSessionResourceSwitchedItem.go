package ngapType

import "OmniRAN-Emulator/lib/aper"

// Need to import "free5gc/lib/aper" if it uses "aper"

type PDUSessionResourceSwitchedItem struct {
	PDUSessionID                         PDUSessionID
	PathSwitchRequestAcknowledgeTransfer aper.OctetString
	IEExtensions                         *ProtocolExtensionContainerPDUSessionResourceSwitchedItemExtIEs `aper:"optional"`
}
