package ngap_control

import (
	"fmt"
	"sync"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

var (
	ValidationMutex  sync.Mutex
	TotalValidated   int64
	TotalFailed      int64
	ValidationErrors []string
)

func ResetValidationStats() {
	ValidationMutex.Lock()
	defer ValidationMutex.Unlock()
	TotalValidated = 0
	TotalFailed = 0
	ValidationErrors = nil
}

// ValidateNGAPMessage checks a decoded NGAP PDU and verifies that all mandatory
// IEs required by TS 38.413 for that message type are present.
func ValidateNGAPMessage(pdu *ngapType.NGAPPDU) error {
	ValidationMutex.Lock()
	TotalValidated++
	ValidationMutex.Unlock()

	err := validateMessageInternal(pdu)
	if err != nil {
		ValidationMutex.Lock()
		TotalFailed++
		ValidationErrors = append(ValidationErrors, err.Error())
		ValidationMutex.Unlock()
	}
	return err
}

func validateMessageInternal(pdu *ngapType.NGAPPDU) error {
	if pdu == nil {
		return fmt.Errorf("PDU is nil")
	}
	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		initMsg := pdu.InitiatingMessage
		if initMsg == nil {
			return fmt.Errorf("initiating message is nil")
		}
		return validateInitiatingMessage(initMsg)

	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		successMsg := pdu.SuccessfulOutcome
		if successMsg == nil {
			return fmt.Errorf("successful outcome is nil")
		}
		return validateSuccessfulOutcome(successMsg)

	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		// We can add validation for unsuccessful outcomes if needed
		return nil

	default:
		return fmt.Errorf("unknown NGAP PDU type: %d", pdu.Present)
	}
}

func validateInitiatingMessage(msg *ngapType.InitiatingMessage) error {
	procCode := msg.ProcedureCode.Value
	value := msg.Value

	switch procCode {
	case ngapType.ProcedureCodeNGSetup:
		req := value.NGSetupRequest
		if req == nil {
			return fmt.Errorf("NGSetupRequest value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDGlobalRANNodeID,
			ngapType.ProtocolIEIDSupportedTAList,
		}
		return verifyIEsPresent("NGSetupRequest", getIEIds(req.ProtocolIEs.List), mandatory)

	case ngapType.ProcedureCodeInitialUEMessage:
		req := value.InitialUEMessage
		if req == nil {
			return fmt.Errorf("InitialUEMessage value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDRANUENGAPID,
			ngapType.ProtocolIEIDNASPDU,
			ngapType.ProtocolIEIDUserLocationInformation,
			ngapType.ProtocolIEIDRRCEstablishmentCause,
		}
		return verifyIEsPresent("InitialUEMessage", getIEIds(req.ProtocolIEs.List), mandatory)

	case ngapType.ProcedureCodeUplinkNASTransport:
		req := value.UplinkNASTransport
		if req == nil {
			return fmt.Errorf("UplinkNASTransport value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDAMFUENGAPID,
			ngapType.ProtocolIEIDRANUENGAPID,
			ngapType.ProtocolIEIDNASPDU,
			ngapType.ProtocolIEIDUserLocationInformation,
		}
		return verifyIEsPresent("UplinkNASTransport", getIEIds(req.ProtocolIEs.List), mandatory)

	case ngapType.ProcedureCodeUEContextReleaseRequest:
		req := value.UEContextReleaseRequest
		if req == nil {
			return fmt.Errorf("UEContextReleaseRequest value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDAMFUENGAPID,
			ngapType.ProtocolIEIDRANUENGAPID,
			ngapType.ProtocolIEIDCause,
		}
		return verifyIEsPresent("UEContextReleaseRequest", getIEIds(req.ProtocolIEs.List), mandatory)

	case ngapType.ProcedureCodeHandoverPreparation:
		req := value.HandoverRequired
		if req == nil {
			return fmt.Errorf("HandoverRequired value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDAMFUENGAPID,
			ngapType.ProtocolIEIDRANUENGAPID,
			ngapType.ProtocolIEIDHandoverType,
			ngapType.ProtocolIEIDCause,
			ngapType.ProtocolIEIDTargetID,
			ngapType.ProtocolIEIDSourceToTargetTransparentContainer,
		}
		return verifyIEsPresent("HandoverRequired", getIEIds(req.ProtocolIEs.List), mandatory)

	case ngapType.ProcedureCodeHandoverNotification:
		req := value.HandoverNotify
		if req == nil {
			return fmt.Errorf("HandoverNotify value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDAMFUENGAPID,
			ngapType.ProtocolIEIDRANUENGAPID,
			ngapType.ProtocolIEIDUserLocationInformation,
		}
		return verifyIEsPresent("HandoverNotify", getIEIds(req.ProtocolIEs.List), mandatory)

	case ngapType.ProcedureCodePathSwitchRequest:
		req := value.PathSwitchRequest
		if req == nil {
			return fmt.Errorf("PathSwitchRequest value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDRANUENGAPID,
			ngapType.ProtocolIEIDSourceAMFUENGAPID,
			ngapType.ProtocolIEIDUserLocationInformation,
			ngapType.ProtocolIEIDUESecurityCapabilities,
			ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList,
		}
		return verifyIEsPresent("PathSwitchRequest", getIEIds(req.ProtocolIEs.List), mandatory)

	default:
		// Other procedures can be added here as needed
		return nil
	}
}

func validateSuccessfulOutcome(msg *ngapType.SuccessfulOutcome) error {
	procCode := msg.ProcedureCode.Value
	value := msg.Value

	switch procCode {
	case ngapType.ProcedureCodeInitialContextSetup:
		res := value.InitialContextSetupResponse
		if res == nil {
			return fmt.Errorf("InitialContextSetupResponse value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDAMFUENGAPID,
			ngapType.ProtocolIEIDRANUENGAPID,
		}
		return verifyIEsPresent("InitialContextSetupResponse", getIEIds(res.ProtocolIEs.List), mandatory)

	case ngapType.ProcedureCodePDUSessionResourceSetup:
		res := value.PDUSessionResourceSetupResponse
		if res == nil {
			return fmt.Errorf("PDUSessionResourceSetupResponse value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDAMFUENGAPID,
			ngapType.ProtocolIEIDRANUENGAPID,
		}
		return verifyIEsPresent("PDUSessionResourceSetupResponse", getIEIds(res.ProtocolIEs.List), mandatory)

	case ngapType.ProcedureCodePDUSessionResourceRelease:
		res := value.PDUSessionResourceReleaseResponse
		if res == nil {
			return fmt.Errorf("PDUSessionResourceReleaseResponse value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDAMFUENGAPID,
			ngapType.ProtocolIEIDRANUENGAPID,
		}
		return verifyIEsPresent("PDUSessionResourceReleaseResponse", getIEIds(res.ProtocolIEs.List), mandatory)

	case ngapType.ProcedureCodePDUSessionResourceModify:
		res := value.PDUSessionResourceModifyResponse
		if res == nil {
			return fmt.Errorf("PDUSessionResourceModifyResponse value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDAMFUENGAPID,
			ngapType.ProtocolIEIDRANUENGAPID,
		}
		return verifyIEsPresent("PDUSessionResourceModifyResponse", getIEIds(res.ProtocolIEs.List), mandatory)

	case ngapType.ProcedureCodeUEContextRelease:
		res := value.UEContextReleaseComplete
		if res == nil {
			return fmt.Errorf("UEContextReleaseComplete value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDAMFUENGAPID,
			ngapType.ProtocolIEIDRANUENGAPID,
		}
		return verifyIEsPresent("UEContextReleaseComplete", getIEIds(res.ProtocolIEs.List), mandatory)

	case ngapType.ProcedureCodeHandoverResourceAllocation:
		res := value.HandoverRequestAcknowledge
		if res == nil {
			return fmt.Errorf("HandoverRequestAcknowledge value is nil")
		}
		mandatory := []int64{
			ngapType.ProtocolIEIDAMFUENGAPID,
			ngapType.ProtocolIEIDRANUENGAPID,
			ngapType.ProtocolIEIDTargetToSourceTransparentContainer,
		}
		return verifyIEsPresent("HandoverRequestAcknowledge", getIEIds(res.ProtocolIEs.List), mandatory)

	default:
		return nil
	}
}

// getIEIds extracts the list of ProtocolIEIDs from a slice of generic IEs
func getIEIds(list interface{}) []int64 {
	var ids []int64
	switch l := list.(type) {
	case []ngapType.NGSetupRequestIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.InitialUEMessageIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.UplinkNASTransportIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.InitialContextSetupResponseIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.PDUSessionResourceSetupResponseIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.PDUSessionResourceReleaseResponseIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.PDUSessionResourceModifyResponseIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.UEContextReleaseRequestIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.UEContextReleaseCompleteIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.HandoverRequiredIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.HandoverRequestAcknowledgeIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.HandoverNotifyIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	case []ngapType.PathSwitchRequestIEs:
		for _, ie := range l {
			ids = append(ids, ie.Id.Value)
		}
	}
	return ids
}

func verifyIEsPresent(msgName string, present []int64, mandatory []int64) error {
	presentMap := make(map[int64]bool)
	for _, id := range present {
		presentMap[id] = true
	}

	for _, reqId := range mandatory {
		if !presentMap[reqId] {
			return fmt.Errorf("%s missing mandatory Protocol IE ID: %d", msgName, reqId)
		}
	}
	return nil
}
