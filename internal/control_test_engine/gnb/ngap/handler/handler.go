package handler

import (
	"encoding/binary"
	"fmt"
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	serviceGateway "OmniRAN-Emulator/internal/control_test_engine/gnb/data/service"
	serviceGtp "OmniRAN-Emulator/internal/control_test_engine/gnb/gtp/service"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/nas/message/sender"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/ue_context_management"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control/ue_mobility_management"
	senderNgap "OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/sender"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/trigger"
	"OmniRAN-Emulator/lib/aper"
	"OmniRAN-Emulator/lib/ngap/ngapType"
	"net"
	"time"
)

func HandlerDownlinkNasTransport(gnb *context.GNBContext, message *ngapType.NGAPPDU) {

	var ranUeId int64
	var amfUeId int64
	var messageNas []byte

	valueMessage := message.InitiatingMessage.Value.DownlinkNASTransport

	for _, ies := range valueMessage.ProtocolIEs.List {

		switch ies.Id.Value {

		case ngapType.ProtocolIEIDAMFUENGAPID:
			if ies.Value.AMFUENGAPID == nil {
				log.Fatal("[GNB][NGAP] AMF UE NGAP ID is missing")
				// TODO SEND ERROR INDICATION
			}
			amfUeId = ies.Value.AMFUENGAPID.Value

		case ngapType.ProtocolIEIDRANUENGAPID:
			if ies.Value.RANUENGAPID == nil {
				log.Fatal("[GNB][NGAP] RAN UE NGAP ID is missing")
				// TODO SEND ERROR INDICATION
			}
			ranUeId = ies.Value.RANUENGAPID.Value

		case ngapType.ProtocolIEIDNASPDU:
			if ies.Value.NASPDU == nil {
				log.Fatal("[GNB][NGAP] NAS PDU is missing")
				// TODO SEND ERROR INDICATION
			}
			messageNas = ies.Value.NASPDU.Value
		}
	}

	// check RanUeId and get UE.
	ue, err := gnb.GetGnbUe(ranUeId)
	if err != nil || ue == nil {
		log.Fatal("[GNB][NGAP] RAN UE NGAP ID is incorrect")
		// TODO SEND ERROR INDICATION
	}

	// update AMF UE id.
	if ue.GetAmfUeId() == 0 {
		ue.SetAmfUeId(amfUeId)
	} else {
		if ue.GetAmfUeId() != amfUeId {
			log.Fatal("[GNB][NGAP] AMF UE NGAP ID is incorrect")
		}
	}

	// send NAS message to UE.
	sender.SendToUe(ue, messageNas)
}

func HandlerInitialContextSetupRequest(gnb *context.GNBContext, message *ngapType.NGAPPDU) {

	var ranUeId int64
	var amfUeId int64
	var messageNas []byte
	var sst []string
	var sd []string
	var mobilityRestrict = "not informed"
	var maskedImeisv string
	// var securityKey []byte

	valueMessage := message.InitiatingMessage.Value.InitialContextSetupRequest

	for _, ies := range valueMessage.ProtocolIEs.List {

		// TODO MORE FIELDS TO CHECK HERE
		switch ies.Id.Value {

		case ngapType.ProtocolIEIDAMFUENGAPID:
			if ies.Value.AMFUENGAPID == nil {
				log.Fatal("[GNB][NGAP] AMF UE NGAP ID is missing")
				// TODO SEND ERROR INDICATION
			}
			amfUeId = ies.Value.AMFUENGAPID.Value

		case ngapType.ProtocolIEIDRANUENGAPID:
			if ies.Value.RANUENGAPID == nil {
				log.Fatal("[GNB][NGAP] RAN UE NGAP ID is missing")
				// TODO SEND ERROR INDICATION
			}
			ranUeId = ies.Value.RANUENGAPID.Value

		case ngapType.ProtocolIEIDNASPDU:
			if ies.Value.NASPDU == nil {
				log.Info("[GNB][NGAP] NAS PDU is missing")
				// TODO SEND ERROR INDICATION
			}
			messageNas = ies.Value.NASPDU.Value

		case ngapType.ProtocolIEIDSecurityKey:
			// TODO using for create new security context between GNB and UE.
			if ies.Value.SecurityKey == nil {
				log.Fatal("[GNB][NGAP] Security-Key is missing")
			}
			// securityKey = ies.Value.SecurityKey.Value.Bytes

		case ngapType.ProtocolIEIDGUAMI:
			if ies.Value.GUAMI == nil {
				log.Fatal("[GNB][NGAP] GUAMI is missing")
			}

		case ngapType.ProtocolIEIDAllowedNSSAI:
			if ies.Value.AllowedNSSAI == nil {
				log.Fatal("[GNB][NGAP] Allowed NSSAI is missing")
			}

			valor := len(ies.Value.AllowedNSSAI.List)
			sst = make([]string, valor)
			sd = make([]string, valor)

			// list S-NSSAI(Single – Network Slice Selection Assistance Information).
			for i, items := range ies.Value.AllowedNSSAI.List {

				if items.SNSSAI.SST.Value != nil {
					sst[i] = fmt.Sprintf("%x", items.SNSSAI.SST.Value)
				} else {
					sst[i] = "not informed"
				}

				if items.SNSSAI.SD != nil {
					sd[i] = fmt.Sprintf("%x", items.SNSSAI.SD.Value)
				} else {
					sd[i] = "not informed"
				}
			}

		case ngapType.ProtocolIEIDMobilityRestrictionList:
			// that field is not mandatory.
			if ies.Value.MobilityRestrictionList == nil {
				log.Info("[GNB][NGAP] Mobility Restriction is missing")
				mobilityRestrict = "not informed"
			} else {
				mobilityRestrict = fmt.Sprintf("%x", ies.Value.MobilityRestrictionList.ServingPLMN.Value)
			}

		case ngapType.ProtocolIEIDMaskedIMEISV:
			// that field is not mandatory.
			// TODO using for mapping UE context
			if ies.Value.MaskedIMEISV == nil {
				log.Info("[GNB][NGAP] Masked IMEISV is missing")
				maskedImeisv = "not informed"
			} else {
				maskedImeisv = fmt.Sprintf("%x", ies.Value.MaskedIMEISV.Value.Bytes)
			}

		case ngapType.ProtocolIEIDUESecurityCapabilities:
			// TODO using for create new security context between UE and GNB.
			// TODO algorithms for create new security context between UE and GNB.
			if ies.Value.UESecurityCapabilities == nil {
				log.Fatal("[GNB][NGAP] UE Security Capabilities is missing")
			}
		}

	}

	// check RanUeId and get UE.
	ue, err := gnb.GetGnbUe(ranUeId)
	if err != nil || ue == nil {
		log.Fatal("[GNB][NGAP] RAN UE NGAP ID is incorrect")
		// TODO SEND ERROR INDICATION
	}

	// check if AMF UE id.
	if ue.GetAmfUeId() != amfUeId {
		log.Fatal("[GNB][NGAP] AMF UE NGAP ID is incorrect")
		// TODO SEND ERROR INDICATION
	}

	// create UE context.
	ue.CreateUeContext(mobilityRestrict, maskedImeisv, sst, sd)

	// show UE context.
	log.Info("[GNB][UE] UE Context was created with successful")
	log.Info("[GNB][UE] UE RAN ID ", ue.GetRanUeId())
	log.Info("[GNB][UE] UE AMF ID ", ue.GetAmfUeId())
	mcc, mnc := ue.GetUeMobility()
	log.Info("[GNB][UE] UE Mobility Restrict --Plmn-- Mcc: ", mcc, " Mnc: ", mnc)
	log.Info("[GNB][UE] UE Masked Imeisv: ", ue.GetUeMaskedImeiSv())
	for i := 0; i < ue.GetLenSlice(); i++ {
		sst, sd := ue.GetAllowedNssai(i)
		log.Info("[GNB][UE] Allowed Nssai-- Sst: ", sst, " Sd: ", sd)
	}

	// send NAS message to UE.
	log.Info("[GNB][NAS][UE] Send Registration Accept.")
	sender.SendToUe(ue, messageNas)

	// send Initial Context Setup Response.
	log.Info("[GNB][NGAP][AMF] Send Initial Context Setup Response.")
	trigger.SendInitialContextSetupResponse(ue)
}

func HandlerPduSessionResourceSetupRequest(gnb *context.GNBContext, message *ngapType.NGAPPDU) {

	var ranUeId int64
	var amfUeId int64
	var pduSessionId int64
	var ulTeid uint32
	var upfAddress []byte
	var messageNas []byte
	var sst string
	var sd string
	var pduSType uint64
	var qosId int64
	var fiveQi int64
	var priArp int64

	valueMessage := message.InitiatingMessage.Value.PDUSessionResourceSetupRequest

	for _, ies := range valueMessage.ProtocolIEs.List {

		// TODO MORE FIELDS TO CHECK HERE
		switch ies.Id.Value {

		case ngapType.ProtocolIEIDAMFUENGAPID:

			if ies.Value.AMFUENGAPID == nil {
				log.Fatal("[GNB][NGAP] AMF UE ID is missing")
			}
			amfUeId = ies.Value.AMFUENGAPID.Value

		case ngapType.ProtocolIEIDRANUENGAPID:

			if ies.Value.RANUENGAPID == nil {
				log.Fatal("[GNB][NGAP] RAN UE ID is missing")
				// TODO SEND ERROR INDICATION
			}
			ranUeId = ies.Value.RANUENGAPID.Value

		case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:

			if ies.Value.PDUSessionResourceSetupListSUReq == nil {
				log.Fatal("[GNB][NGAP] PDU SESSION RESOURCE SETUP LIST SU REQ is missing")
			}
			pDUSessionResourceSetupList := ies.Value.PDUSessionResourceSetupListSUReq

			for _, item := range pDUSessionResourceSetupList.List {

				// check PDU Session NAS PDU.
				if item.PDUSessionNASPDU != nil {
					messageNas = item.PDUSessionNASPDU.Value
				} else {
					log.Fatal("[GNB][NGAP] NAS PDU is missing")
				}

				// check pdu session id and nssai information for create a PDU Session.

				// create a PDU session(PDU SESSION ID + NSSAI).
				pduSessionId = item.PDUSessionID.Value

				if item.SNSSAI.SD != nil {
					sd = fmt.Sprintf("%x", item.SNSSAI.SD.Value)
				} else {
					sd = "not informed"
				}

				if item.SNSSAI.SST.Value != nil {
					sst = fmt.Sprintf("%x", item.SNSSAI.SST.Value)
				} else {
					sst = "not informed"
				}

				if item.PDUSessionResourceSetupRequestTransfer != nil {

					pdu := &ngapType.PDUSessionResourceSetupRequestTransfer{}

					err := aper.UnmarshalWithParams(item.PDUSessionResourceSetupRequestTransfer, pdu, "valueExt")
					if err == nil {
						for _, ies := range pdu.ProtocolIEs.List {

							switch ies.Id.Value {

							case ngapType.ProtocolIEIDULNGUUPTNLInformation:
								ulTeid = binary.BigEndian.Uint32(ies.Value.ULNGUUPTNLInformation.GTPTunnel.GTPTEID.Value)
								upfAddress = ies.Value.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value.Bytes

							case ngapType.ProtocolIEIDQosFlowSetupRequestList:
								for _, itemsQos := range ies.Value.QosFlowSetupRequestList.List {
									qosId = itemsQos.QosFlowIdentifier.Value
									fiveQi = itemsQos.QosFlowLevelQosParameters.QosCharacteristics.NonDynamic5QI.FiveQI.Value
									priArp = itemsQos.QosFlowLevelQosParameters.AllocationAndRetentionPriority.PriorityLevelARP.Value
								}

							case ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate:

							case ngapType.ProtocolIEIDPDUSessionType:
								pduSType = uint64(ies.Value.PDUSessionType.Value)

							case ngapType.ProtocolIEIDSecurityIndication:

							}
						}
					} else {
						log.Info("[GNB][NGAP] Error in decode Pdu Session Resource Setup Request Transfer")
					}
				} else {
					log.Fatal("[GNB][NGAP] Error in Pdu Session Resource Setup Request, Pdu Session Resource Setup Request Transfer is missing")
				}

			}
		}
	}

	// check RanUeId and get UE.
	ue, err := gnb.GetGnbUe(ranUeId)
	if err != nil || ue == nil {
		log.Fatal("[GNB][NGAP] Error in Pdu Session Resource Setup Request. UE was not found in GNB POOL")
		// TODO SEND ERROR INDICATION
	}

	// check if AMF UE id.
	if ue.GetAmfUeId() != amfUeId {
		log.Fatal("[GNB][NGAP] Error in Pdu Session Resource Setup Request. Problem in AMF UE ID from CORE")
		// TODO SEND ERROR INDICATION
	}

	// allocate downlink TEID and IP for this PDU Session
	teidDown := gnb.GetUeTeid()
	gnb.StoreTeid(teidDown, ue)

	ueGnbIpVal := gnb.GetRanUeIp()
	ueGnbIp := net.IPv4(127, 0, 0, ueGnbIpVal)
	gnb.StoreUeIp(ueGnbIp.String(), ue)

	// create PDU Session for GNB UE.
	if ue.CreatePduSession(pduSessionId, sst, sd, pduSType, qosId, priArp, fiveQi, ulTeid, ueGnbIp, teidDown) != "" {
		log.Info("[GNB][NGAP] Error in Pdu Session Resource Setup Request. NSSAI informed was not found")
	} else {
		log.Info("[GNB][NGAP][UE] PDU Session was created with successful.")
		log.Info("[GNB][NGAP][UE] PDU Session Id: ", pduSessionId)
		sst, sd := ue.GetSelectedNssai()
		log.Info("[GNB][NGAP][UE] NSSAI Selected --- sst: ", sst, " sd: ", sd)
		log.Info("[GNB][NGAP][UE] PDU Session Type: ", ue.GetPduType())
		log.Info("[GNB][NGAP][UE] QOS Flow Identifier: ", ue.GetQosId())
		log.Info("[GNB][NGAP][UE] Uplink Teid: ", ulTeid)
		log.Info("[GNB][NGAP][UE] Downlink Teid: ", teidDown)
		log.Info("[GNB][NGAP][UE] Non-Dynamic-5QI: ", fiveQi)
		log.Info("[GNB][NGAP][UE] Priority Level ARP: ", priArp)
		log.Info("[GNB][NGAP][UE] UPF Address: ", fmt.Sprintf("%d.%d.%d.%d", upfAddress[0], upfAddress[1], upfAddress[2], upfAddress[3]), " :2152")
	}

	// get UPF ip.
	if gnb.GetUpfIp() == "" {
		upfIp := fmt.Sprintf("%d.%d.%d.%d", upfAddress[0], upfAddress[1], upfAddress[2], upfAddress[3])
		gnb.SetUpfIp(upfIp)
	}

	// send NAS message to UE.
	sender.SendToUe(ue, messageNas)

	// send PDU Session Resource Setup Response.
	trigger.SendPduSessionResourceSetupResponse(ue, gnb, pduSessionId)

	time.Sleep(20 * time.Millisecond)

	// configure GTP tunnel and listen.
	if gnb.GetN3Plane() == nil {
		// TODO check if GTP tunnel and gateway is ok.
		serviceGtp.InitGTPTunnel(gnb)
		serviceGateway.InitGatewayGnb(gnb)
	}
	
	// ue is ready for data plane.
	// send GNB UE IP message to UE (prepended with PDU Session ID).
	msg := append([]byte{byte(pduSessionId)}, ueGnbIp...)
	sender.SendToUe(ue, msg)
}

// HandlerPduSessionResourceReleaseCommand handles NGAP PDU Session Resource Release Command.
// It extracts the NAS PDU (PDU Session Release Command) carried inside the NGAP message
// and forwards it to the UE via the UNIX NAS socket, then sends the Release Response to AMF.
func HandlerPduSessionResourceReleaseCommand(gnb *context.GNBContext, message *ngapType.NGAPPDU) {
	var ranUeId int64
	var amfUeId int64
	var messageNas []byte

	valueMessage := message.InitiatingMessage.Value.PDUSessionResourceReleaseCommand

	for _, ies := range valueMessage.ProtocolIEs.List {
		switch ies.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			if ies.Value.AMFUENGAPID != nil {
				amfUeId = ies.Value.AMFUENGAPID.Value
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			if ies.Value.RANUENGAPID != nil {
				ranUeId = ies.Value.RANUENGAPID.Value
			}
		case ngapType.ProtocolIEIDNASPDU:
			if ies.Value.NASPDU != nil {
				messageNas = ies.Value.NASPDU.Value
			}
		}
	}

	// Look up the UE context by RAN UE NGAP ID
	ue, err := gnb.GetGnbUe(ranUeId)
	if err != nil || ue == nil {
		log.Warn("[GNB][NGAP] PDU Session Resource Release Command: RAN UE NGAP ID not found, ranUeId=", ranUeId)
		return
	}

	_ = amfUeId // may be used for future validation

	// Forward the NAS PDU Session Release Command to the UE
	if messageNas != nil {
		log.Info("[GNB][NGAP][UE] Forwarding PDU Session Release Command NAS to UE")
		sender.SendToUe(ue, messageNas)
	}

	// Send NGAP PDU Session Resource Release Response back to AMF
	trigger.SendPduSessionResourceReleaseResponse(ue, gnb)
}

func HandlerNgSetupResponse(amf *context.GNBAmf, gnb *context.GNBContext, message *ngapType.NGAPPDU) {

	err := false
	var plmn string

	// check information about AMF and add in AMF context.
	valueMessage := message.SuccessfulOutcome.Value.NGSetupResponse

	for _, ies := range valueMessage.ProtocolIEs.List {

		switch ies.Id.Value {

		case ngapType.ProtocolIEIDAMFName:
			if ies.Value.AMFName == nil {
				// TODO error indication. This field is mandatory critically reject
				log.Info("[GNB][NGAP] Error in NG SETUP RESPONSE,AMF Name is missing")
				log.Info("[GNB][NGAP] AMF is inactive")
				err = true
			} else {
				amfName := ies.Value.AMFName.Value
				amf.SetAmfName(amfName)
			}

		case ngapType.ProtocolIEIDServedGUAMIList:
			if ies.Value.ServedGUAMIList.List == nil {
				// TODO error indication. This field is mandatory critically reject
				log.Info("[GNB][NGAP] Error in NG SETUP RESPONSE,Serverd Guami list is missing")
				log.Info("[GNB][NGAP] AMF is inactive")
				err = true
			}
			for _, items := range ies.Value.ServedGUAMIList.List {
				if items.GUAMI.AMFRegionID.Value.Bytes == nil {
					log.Info("[GNB][NGAP] Error in NG SETUP RESPONSE,Served Guami list is inappropriate")
					log.Info("[GNB][NGAP] Error in NG SETUP RESPONSE, AMFRegionId is missing")
					log.Info("[GNB][NGAP] AMF is inactive")
					err = true
				}
				if items.GUAMI.AMFPointer.Value.Bytes == nil {
					log.Info("[GNB][NGAP] Error in NG SETUP RESPONSE,Served Guami list is inappropriate")
					log.Info("[GNB][NGAP] Error in NG SETUP RESPONSE, AMFPointer is missing")
					log.Info("[GNB][NGAP] AMF is inactive")
					err = true
				}
				if items.GUAMI.AMFSetID.Value.Bytes == nil {
					log.Info("[GNB][NGAP] Error in NG SETUP RESPONSE,Served Guami list is inappropriate")
					log.Info("[GNB][NGAP] Error in NG SETUP RESPONSE, AMFSetId is missing")
					log.Info("[GNB][NGAP] AMF is inactive")
					err = true
				}
			}

		case ngapType.ProtocolIEIDRelativeAMFCapacity:
			if ies.Value.RelativeAMFCapacity != nil {
				amfCapacity := ies.Value.RelativeAMFCapacity.Value
				amf.SetAmfCapacity(amfCapacity)
			}

		case ngapType.ProtocolIEIDPLMNSupportList:

			if ies.Value.PLMNSupportList == nil {
				log.Info("[GNB][NGAP] Error in NG SETUP RESPONSE, PLMN Support list is missing")
				err = true
			}

			for _, items := range ies.Value.PLMNSupportList.List {

				plmn = fmt.Sprintf("%x", items.PLMNIdentity.Value)
				amf.AddedPlmn(plmn)

				if items.SliceSupportList.List == nil {
					log.Info("[GNB][NGAP] Error in NG SETUP RESPONSE, PLMN Support list is inappropriate")
					log.Info("[GNB][NGAP] Error in NG SETUP RESPONSE, Slice Support list is missing")
					err = true
				}

				for _, slice := range items.SliceSupportList.List {

					var sd string
					var sst string

					if slice.SNSSAI.SST.Value != nil {
						sst = fmt.Sprintf("%x", slice.SNSSAI.SST.Value)
					} else {
						sst = "was not informed"
					}

					if slice.SNSSAI.SD != nil {
						sd = fmt.Sprintf("%x", slice.SNSSAI.SD.Value)
					} else {
						sd = "was not informed"
					}

					// update amf slice supported
					amf.AddedSlice(sst, sd)
				}
			}
		}

	}

	if err {
		log.Fatal("[GNB][AMF] AMF is inactive")
		amf.SetStateInactive()
	} else {
		amf.SetStateActive()
		log.Info("[GNB][AMF] AMF Name: ", amf.GetAmfName())
		log.Info("[GNB][AMF] State of AMF: Active")
		log.Info("[GNB][AMF] Capacity of AMF: ", amf.GetAmfCapacity())
		for i := 0; i < amf.GetLenPlmns(); i++ {
			mcc, mnc := amf.GetPlmnSupport(i)
			log.Info("[GNB][AMF] PLMNs Identities Supported by AMF -- mcc: ", mcc, " mnc:", mnc)
		}
		for i := 0; i < amf.GetLenSlice(); i++ {
			sst, sd := amf.GetSliceSupport(i)
			log.Info("[GNB][AMF] List of AMF slices Supported by AMF -- sst:", sst, " sd:", sd)
		}
	}

}

func HandlerNgSetupFailure(amf *context.GNBAmf, gnb *context.GNBContext, message *ngapType.NGAPPDU) {

	// check information about AMF and add in AMF context.
	valueMessage := message.UnsuccessfulOutcome.Value.NGSetupFailure

	for _, ies := range valueMessage.ProtocolIEs.List {

		switch ies.Id.Value {

		case ngapType.ProtocolIEIDCause:

			switch ies.Value.Cause.Present {

			case ngapType.CausePresentMisc:

				causeMisc := ies.Value.Cause.Misc.Value

				switch causeMisc {

				case ngapType.CauseMiscPresentUnknownPLMN:
					// Cannot find Served TAI in AMF.
					log.Info("[GNB][AMF][NGAP] Unknown PLMN in Supported TAI")

				case ngapType.CauseMiscPresentUnspecified:
					// No supported TA exist in NG-Setup request.
					log.Info("[GNB][AMF][NGAP] Error cause unspecified")

				}

			case ngapType.CausePresentRadioNetwork:
				// TODO treatment error

			case ngapType.CausePresentTransport:
				// TODO treatment error

			case ngapType.CausePresentProtocol:
				// TODO treatment error

			case ngapType.CausePresentNas:
				// TODO treatment error

			}

		case ngapType.ProtocolIEIDTimeToWait:

			switch ies.Value.TimeToWait.Value {

			case ngapType.TimeToWaitPresentV1s:
			case ngapType.TimeToWaitPresentV2s:
			case ngapType.TimeToWaitPresentV5s:
			case ngapType.TimeToWaitPresentV10s:
			case ngapType.TimeToWaitPresentV20s:
			case ngapType.TimeToWaitPresentV60s:

			}

		case ngapType.ProtocolIEIDCriticalityDiagnostics:

			// TODO treatment error

			// ies.Value.CriticalityDiagnostics
			// errors.IECriticality.Value
			// ngapType.CriticalityPresentReject:
			// ngapType.CriticalityPresentIgnore:
			// ngapType.CriticalityPresentNotify:
			// ngapType.TypeOfErrorPresentNotUnderstood:
			// ngapType.TypeOfErrorPresentMissing:

		}
	}

	// redundant but useful for information about code.
	amf.SetStateInactive()

	log.Info("[GNB][NGAP] AMF is inactive")
}

func HandlerPaging(gnb *context.GNBContext, message *ngapType.NGAPPDU) {
	log.Info("[GNB][NGAP] Handling Paging Request from AMF")

	valueMessage := message.InitiatingMessage.Value.Paging
	if valueMessage == nil {
		log.Warn("[GNB][NGAP] Paging value message is nil")
		return
	}

	var tmsiBytes []byte
	for _, ies := range valueMessage.ProtocolIEs.List {
		if ies.Id.Value == ngapType.ProtocolIEIDUEPagingIdentity {
			if ies.Value.UEPagingIdentity != nil {
				pagingId := ies.Value.UEPagingIdentity
				if pagingId.Present == ngapType.UEPagingIdentityPresentFiveGSTMSI {
					if pagingId.FiveGSTMSI != nil {
						tmsiBytes = pagingId.FiveGSTMSI.FiveGTMSI.Value
					}
				}
			}
		}
	}

	if tmsiBytes == nil {
		log.Warn("[GNB][NGAP] Paging identity (5G-S-TMSI) not found in Paging Request")
	}

	found := false
	gnb.RangeUePool(func(ranUeId int64, ue *context.GNBUe) bool {
		log.Infof("[GNB] Paging UE (RanUeId: %d)", ranUeId)
		conn := ue.GetUnixSocket()
		if conn != nil {
			_, err := conn.Write([]byte{0x00, 0x01})
			if err != nil {
				log.Errorf("[GNB] Error sending paging trigger to UE: %v", err)
			} else {
				log.Info("[GNB] Paging trigger sent successfully to UE")
				found = true
			}
		}
		return true
	})

	if !found {
		log.Warn("[GNB][NGAP] No active UE socket connection found to page")
	}
}

func HandlerPathSwitchRequestAcknowledge(gnb *context.GNBContext, message *ngapType.NGAPPDU) {
	var ranUeId int64
	var amfUeId int64

	valueMessage := message.SuccessfulOutcome.Value.PathSwitchRequestAcknowledge

	for _, ies := range valueMessage.ProtocolIEs.List {
		switch ies.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfUeId = ies.Value.AMFUENGAPID.Value
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranUeId = ies.Value.RANUENGAPID.Value
		case ngapType.ProtocolIEIDPDUSessionResourceSwitchedList:
			list := ies.Value.PDUSessionResourceSwitchedList
			for _, item := range list.List {
				pduSessionId := uint8(item.PDUSessionID.Value)
				var transfer ngapType.PathSwitchRequestAcknowledgeTransfer
				err := aper.UnmarshalWithParams(item.PathSwitchRequestAcknowledgeTransfer, &transfer, "valueExt")
				if err != nil {
					log.Info("[GNB][NGAP] Error decoding PathSwitchRequestAcknowledgeTransfer: ", err)
					continue
				}
				if transfer.ULNGUUPTNLInformation.Present == ngapType.UPTransportLayerInformationPresentGTPTunnel {
					gtp := transfer.ULNGUUPTNLInformation.GTPTunnel
					upfIp := gtp.TransportLayerAddress.Value.Bytes
					ulTeid := binary.BigEndian.Uint32(gtp.GTPTEID.Value)

					ue, err := gnb.GetGnbUe(ranUeId)
					if err == nil && ue != nil {
						gnb.StoreTeid(ulTeid, ue)
						log.Infof("[GNB][NGAP] Handover Path Switch Acknowledge: Session %d Switched. UPF IP: %v, UL TEID: 0x%08x", pduSessionId, net.IP(upfIp), ulTeid)
					}
				}
			}
		}
	}

	ue, err := gnb.GetGnbUe(ranUeId)
	if err == nil && ue != nil {
		ue.SetAmfUeId(amfUeId)
		log.Infof("[GNB][NGAP] Handover Completed for UE ID %d", ranUeId)
	}
}

func HandlerHandoverRequest(gnb *context.GNBContext, message *ngapType.NGAPPDU) {
	var amfUeId int64
	var pduSessionId uint8 = 1
	var qosId int64 = 1

	valueMessage := message.InitiatingMessage.Value.HandoverRequest
	if valueMessage == nil {
		log.Warn("[GNB][NGAP] Handover Request value is nil")
		return
	}

	for _, ies := range valueMessage.ProtocolIEs.List {
		switch ies.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfUeId = ies.Value.AMFUENGAPID.Value
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListHOReq:
			list := ies.Value.PDUSessionResourceSetupListHOReq
			if list != nil {
				for _, item := range list.List {
					pduSessionId = uint8(item.PDUSessionID.Value)
				}
			}
		}
	}

	log.Infof("[GNB-Target][NGAP] Received HANDOVER REQUEST from AMF. AMF UE ID: %d", amfUeId)

	// Create UE context in Target GNodeB (without socket connection yet)
	ue := gnb.NewGnBUe(nil)
	if ue == nil {
		log.Error("[GNB-Target][NGAP] Failed to create UE context in Target GNodeB")
		return
	}
	ue.SetAmfUeId(amfUeId)
	ue.SetStateOngoing()

	// Setup default PDU session info on Target
	ue.CreatePduSession(int64(pduSessionId), "01", "010203", 1, 1, 1, 1, ue.GetTeidDownlink(), ue.GetIp(), 0)

	// Build HandoverRequestAcknowledge
	dlTeid := make([]byte, 4)
	binary.BigEndian.PutUint32(dlTeid, ue.GetTeidDownlink())

	gnbIp := net.ParseIP(gnb.GetGnbIpByData())
	var gnbIpBytes []byte
	if gnbIp.To4() != nil {
		gnbIpBytes = gnbIp.To4()
	} else {
		gnbIpBytes = gnbIp
	}

	ackMsg, err := ue_mobility_management.GetHandoverRequestAcknowledge(
		ue.GetRanUeId(),
		amfUeId,
		pduSessionId,
		gnbIpBytes,
		dlTeid,
		qosId,
	)
	if err != nil {
		log.Errorf("[GNB-Target][NGAP] Error building Handover Request Acknowledge: %v", err)
		return
	}

	conn := ue.GetSCTP()
	err = senderNgap.SendToAmF(ackMsg, conn)
	if err != nil {
		log.Errorf("[GNB-Target][AMF] Error sending Handover Request Acknowledge: %v", err)
	} else {
		log.Infof("[GNB-Target][AMF] Handover Request Acknowledge sent successfully. Target RAN UE ID: %d", ue.GetRanUeId())
	}
}

func HandlerHandoverCommand(gnb *context.GNBContext, message *ngapType.NGAPPDU) {
	var ranUeId int64
	var amfUeId int64

	valueMessage := message.SuccessfulOutcome.Value.HandoverCommand
	if valueMessage == nil {
		log.Warn("[GNB-Source][NGAP] Handover Command value is nil")
		return
	}

	for _, ies := range valueMessage.ProtocolIEs.List {
		switch ies.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfUeId = ies.Value.AMFUENGAPID.Value
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranUeId = ies.Value.RANUENGAPID.Value
		}
	}

	ue, err := gnb.GetGnbUe(ranUeId)
	if err != nil || ue == nil {
		log.Errorf("[GNB-Source][NGAP] Handover Command: UE %d not found in GNodeB pool", ranUeId)
		return
	}

	log.Infof("[GNB-Source][NGAP] Handover Command received for RAN UE ID: %d, AMF UE ID: %d. Triggering cell switch on UE...", ranUeId, amfUeId)

	// Send Handover Command trigger to UE over UNIX/TCP socket: [0x00, 0x05]
	cmdMsg := []byte{0x00, 0x05}
	_, err = ue.GetUnixSocket().Write(cmdMsg)
	if err != nil {
		log.Errorf("[GNB-Source] Error writing Handover Command trigger to UE socket: %v", err)
	} else {
		log.Info("[GNB-Source] Handover Command trigger sent to UE socket successfully")
	}
}

func HandlerUeContextReleaseCommand(gnb *context.GNBContext, message *ngapType.NGAPPDU) {
	var ranUeId int64
	var amfUeId int64

	valueMessage := message.InitiatingMessage.Value.UEContextReleaseCommand
	if valueMessage == nil {
		log.Warn("[GNB][NGAP] UE Context Release Command value is nil")
		return
	}

	for _, ies := range valueMessage.ProtocolIEs.List {
		switch ies.Id.Value {
		case ngapType.ProtocolIEIDUENGAPIDs:
			ueGapIds := ies.Value.UENGAPIDs
			if ueGapIds != nil {
				if ueGapIds.Present == ngapType.UENGAPIDsPresentUENGAPIDPair {
					if ueGapIds.UENGAPIDPair != nil {
						amfUeId = ueGapIds.UENGAPIDPair.AMFUENGAPID.Value
						ranUeId = ueGapIds.UENGAPIDPair.RANUENGAPID.Value
					}
				} else if ueGapIds.Present == ngapType.UENGAPIDsPresentAMFUENGAPID {
					if ueGapIds.AMFUENGAPID != nil {
						amfUeId = ueGapIds.AMFUENGAPID.Value
					}
				}
			}
		}
	}

	if ranUeId == 0 && amfUeId != 0 {
		gnb.RangeUePool(func(id int64, ue *context.GNBUe) bool {
			if ue.GetAmfUeId() == amfUeId {
				ranUeId = id
				return false
			}
			return true
		})
	}

	log.Infof("[GNB-Source][NGAP] UE Context Release Command received for RAN UE ID: %d, AMF UE ID: %d", ranUeId, amfUeId)

	// Clean up context from GNodeB
	gnb.DeleteGnBUe(ranUeId)

	// Respond with UEContextReleaseComplete
	completeMsg, err := ue_context_management.GetUEContextReleaseComplete(ranUeId, amfUeId)
	if err != nil {
		log.Errorf("[GNB-Source][NGAP] Error building UE Context Release Complete: %v", err)
		return
	}

	n2Conn := gnb.GetN2()
	if n2Conn == nil {
		log.Error("[GNB-Source][NGAP] Failed to get N2 association to send UE Context Release Complete")
		return
	}

	err = senderNgap.SendToAmF(completeMsg, n2Conn)
	if err != nil {
		log.Errorf("[GNB-Source][AMF] Error sending UE Context Release Complete: %v", err)
	} else {
		log.Infof("[GNB-Source][AMF] UE Context Release Complete sent successfully")
	}
}


