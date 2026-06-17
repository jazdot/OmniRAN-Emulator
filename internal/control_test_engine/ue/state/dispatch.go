package state

import (
	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
	data "OmniRAN-Emulator/internal/control_test_engine/ue/data/service"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control/mm_5gs"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/sender"
	"OmniRAN-Emulator/lib/nas/nasMessage"
	log "github.com/sirupsen/logrus"
)

func DispatchState(ue *context.UEContext, message []byte) {
	if len(message) == 0 {
		return
	}

	firstByte := message[0]
	if firstByte == 0x7e || firstByte == 0x2e {
		nas.DispatchNas(ue, message)
	} else {
		// It's an IP plane configuration message from GNodeB.
		// Format: [PDU Session ID (1 byte)] [GNodeB UE IP (remaining bytes)]
		if len(message) > 1 {
			pduSessionId := uint8(message[0])
			gnbIp := message[1:]
			data.InitDataPlane(ue, pduSessionId, gnbIp)

			// After the data plane of this session is initialized, check if there are other
			// PDU sessions that are not yet active and trigger the next one.
			triggerNextPduSession(ue, pduSessionId)
		}
	}
}

func triggerNextPduSession(ue *context.UEContext, currentSessionId uint8) {
	// Look for the next inactive session in the map
	var nextSessionId uint8 = 0
	for id, s := range ue.PduSessions {
		if s.State == context.SM5G_PDU_SESSION_INACTIVE {
			nextSessionId = id
			break
		}
	}

	if nextSessionId != 0 {
		// We found an inactive session! Trigger it.
		log.Infof("[UE][NAS] Triggering establishment of next configured PDU session: ID %d", nextSessionId)
		// Generate the request message
		ulNasTransport, err := mm_5gs.UlNasTransport(ue, nextSessionId, nasMessage.ULNASTransportRequestTypeInitialRequest)
		if err != nil {
			log.Errorf("[UE][NAS] Error triggering next PDU session: %v", err)
			return
		}

		// Update the session state to pending
		ue.GetPduSession(nextSessionId).State = context.SM5G_PDU_SESSION_ACTIVE_PENDING

		// Send to GNodeB
		sender.SendToGnb(ue, ulNasTransport)
	}
}
