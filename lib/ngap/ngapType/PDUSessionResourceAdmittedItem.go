package ngapType

import "OmniRAN-Emulator/lib/aper"

// Need to import "free5gc/lib/aper" if it uses "aper"

type PDUSessionResourceAdmittedItem struct {
	PDUSessionID                       PDUSessionID
	HandoverRequestAcknowledgeTransfer aper.OctetString
	IEExtensions                       *ProtocolExtensionContainerPDUSessionResourceAdmittedItemExtIEs `aper:"optional"`
}
