package sender

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/ishidawataru/sctp"
	"OmniRAN-Emulator/internal/chaos"
	gnbContext "OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/message/ngap_control"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
	"OmniRAN-Emulator/lib/ngap/ngapSctp"
)

func findGnbId(conn *sctp.SCTPConn) string {
	gnbContext.ActiveGNBsMu.RLock()
	defer gnbContext.ActiveGNBsMu.RUnlock()
	for id, g := range gnbContext.ActiveGNBs {
		if g.GetN2() == conn {
			return id
		}
	}
	return ""
}

func getNgapMsgType(message []byte) string {
	pdu, err := ngap.Decoder(message)
	if err != nil || pdu == nil {
		return "Unknown"
	}
	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		if pdu.InitiatingMessage == nil {
			return "Unknown"
		}
		switch pdu.InitiatingMessage.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGSetup:
			return "NGSetupRequest"
		case ngapType.ProcedureCodeInitialUEMessage:
			return "InitialUEMessage"
		case ngapType.ProcedureCodeUplinkNASTransport:
			return "UplinkNASTransport"
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			return "PDUSessionResourceSetupResponse"
		case ngapType.ProcedureCodeInitialContextSetup:
			return "InitialContextSetupResponse"
		case ngapType.ProcedureCodeHandoverPreparation:
			return "HandoverRequired"
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			return "HandoverRequest"
		case ngapType.ProcedureCodePathSwitchRequest:
			return "PathSwitchRequest"
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		if pdu.SuccessfulOutcome == nil {
			return "Unknown"
		}
		switch pdu.SuccessfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGSetup:
			return "NGSetupResponse"
		case ngapType.ProcedureCodeInitialContextSetup:
			return "InitialContextSetupResponse"
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			return "PDUSessionResourceSetupResponse"
		case ngapType.ProcedureCodePathSwitchRequest:
			return "PathSwitchRequestAcknowledge"
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		if pdu.UnsuccessfulOutcome == nil {
			return "Unknown"
		}
		switch pdu.UnsuccessfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGSetup:
			return "NGSetupFailure"
		case ngapType.ProcedureCodeInitialContextSetup:
			return "InitialContextSetupFailure"
		}
	}
	return "NGAP-Message"
}

// isUeAssociatedMessage returns true if the NGAP message type is UE-associated
// signalling, which per TS 38.412 §7 must be sent on SCTP stream 1+.
// Non-UE-associated signalling (NGSetup) uses stream 0.
func isUeAssociatedMessage(msgType string) bool {
	switch msgType {
	case "NGSetupRequest", "NGSetupResponse", "NGSetupFailure":
		return false
	case "Unknown", "NGAP-Message":
		// Conservative: treat unknown messages as non-UE-associated
		return false
	default:
		// All other known messages (InitialUEMessage, UplinkNASTransport,
		// InitialContextSetupResponse, PDUSessionResourceSetupResponse,
		// HandoverRequired, PathSwitchRequest, HandoverRequest,
		// PathSwitchRequestAcknowledge, etc.) are UE-associated.
		return true
	}
}

// sendToAmfOnStream sends an NGAP message on the specified SCTP stream.
func sendToAmfOnStream(message []byte, conn *sctp.SCTPConn, stream uint16) error {
	if conn == nil {
		return fmt.Errorf("SCTPConn is nil")
	}

	gnbId := findGnbId(conn)
	msgType := getNgapMsgType(message)

	_, message = chaos.GlobalChaosManager.EvalFuzz(msgType, message)
	shouldDrop, delay := chaos.GlobalChaosManager.EvalNgap(gnbId, msgType)
	if shouldDrop {
		return nil // Drop the packet silently
	}

	if delay > 0 {
		time.Sleep(delay)
	}

	info := &sctp.SndRcvInfo{
		Stream: stream,
		PPID:   ngapSctp.NGAP_PPID,
	}

	_, err := conn.SCTPWrite(message, info)
	if err != nil {
		return fmt.Errorf("Error sending NGAP message: %v", err)
	}

	return nil
}

// SendToAmF sends an NGAP message to the AMF, automatically selecting the
// correct SCTP stream per TS 38.412: stream 0 for non-UE-associated signalling
// (NGSetup), stream 1 for UE-associated signalling (all other messages).
func SendToAmF(message []byte, conn *sctp.SCTPConn) error {
	msgType := getNgapMsgType(message)
	pdu, err := ngap.Decoder(message)
	if err == nil && pdu != nil {
		_ = ngap_control.ValidateNGAPMessage(pdu)
	}
	stream := uint16(0)
	if isUeAssociatedMessage(msgType) {
		stream = uint16(1)
		log.Debugf("[GNB][SCTP] Sending %s on UE-associated stream %d", msgType, stream)
	} else {
		log.Debugf("[GNB][SCTP] Sending %s on non-UE-associated stream %d", msgType, stream)
	}
	return sendToAmfOnStream(message, conn, stream)
}

// SendToAmfUeAssociated sends a UE-associated NGAP message on SCTP stream 1.
// Use this when the caller explicitly knows the message is UE-associated.
func SendToAmfUeAssociated(message []byte, conn *sctp.SCTPConn) error {
	return sendToAmfOnStream(message, conn, uint16(1))
}
