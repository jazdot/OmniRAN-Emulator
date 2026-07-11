package ngapType

type UPTransportLayerInformationItem struct {
	NGUUPTNLInformation UPTransportLayerInformation
	IEExtensions        *ProtocolExtensionContainerUPTransportLayerInformationItemExtIEs `aper:"optional"`
}

type ProtocolExtensionContainerUPTransportLayerInformationItemExtIEs struct {
	List []UPTransportLayerInformationItemExtIEs `aper:"sizeLB:1,sizeUB:65535"`
}

type UPTransportLayerInformationItemExtIEs struct {
	// Dummy struct
}
