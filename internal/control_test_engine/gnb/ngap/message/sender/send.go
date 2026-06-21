package sender

import (
	"fmt"
	"time"
	"github.com/ishidawataru/sctp"
	"OmniRAN-Emulator/internal/chaos"
	gnbContext "OmniRAN-Emulator/internal/control_test_engine/gnb/context"
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

func SendToAmF(message []byte, conn *sctp.SCTPConn) error {
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
		Stream: uint16(0),
		PPID:   ngapSctp.NGAP_PPID,
	}

	_, err := conn.SCTPWrite(message, info)
	if err != nil {
		return fmt.Errorf("Error sending NGAP message: %v", err)
	}

	return nil
}
