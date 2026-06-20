package state

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

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
	} else if firstByte == 0x00 {
		// Control message from GNodeB
		if len(message) == 2 && message[1] == 0x01 {
			log.Info("[UE] Received Paging trigger from GNodeB. Initiating Service Request...")
			pdu, err := mm_5gs.ServiceRequest(ue, nasMessage.ServiceTypeMobileTerminatedServices)
			if err != nil {
				log.Errorf("[UE] Error creating Service Request: %v", err)
				return
			}
			sender.SendToGnb(ue, pdu)
		} else if len(message) == 10 && message[1] == 0x03 {
			amfUeId := int64(binary.BigEndian.Uint64(message[2:10]))
			ue.SetAmfUeId(amfUeId)
			log.Infof("[UE] AMF UE NGAP ID updated from GNodeB: %d", amfUeId)
		} else if len(message) == 2 && message[1] == 0x05 {
			log.Info("[UE] Received Handover Command from Source GNodeB. Closing old connection and accessing Target GNodeB...")
			
			oldConn := ue.GetUnixConn()
			if oldConn != nil {
				_ = oldConn.Close()
			}

			var conn net.Conn
			var err error
			targetGnbLinkType := ue.GetGnbLinkType()

			if targetGnbLinkType == "tcp" {
				addr := fmt.Sprintf("%s:%d", ue.GetGnbControlIp(), ue.GetGnbLinkPort())
				conn, err = net.Dial("tcp", addr)
				if err != nil {
					log.Errorf("[UE] Error connecting to Target GNodeB via TCP: %v", err)
					return
				}
			} else {
				socketPath := ue.GetGnbSocketPath()
				if socketPath == "" {
					socketPath = "/tmp/gnb.sock"
				}
				dialer := net.Dialer{
					LocalAddr: &net.UnixAddr{
						Name: fmt.Sprintf("@ue_%d", ue.GetUeId()),
						Net:  "unix",
					},
				}
				for i := 0; i < 10; i++ {
					conn, err = dialer.Dial("unix", socketPath)
					if err == nil {
						break
					}
					if i < 9 {
						log.Warnf("[UE] Dial Target GNodeB UNIX socket failed: %v. Retrying in 100ms...", err)
						time.Sleep(100 * time.Millisecond)
					}
				}
				if err != nil {
					log.Errorf("[UE] Error connecting to Target GNodeB via UNIX socket %s after retries: %v", socketPath, err)
					return
				}
			}

			ue.SetUnixConn(conn)
			if ue.OnRedirection != nil {
				go ue.OnRedirection(ue)
			}

			amfUeId := ue.GetAmfUeId()
			accessMsg := make([]byte, 10)
			accessMsg[0] = 0x00
			accessMsg[1] = 0x04
			binary.BigEndian.PutUint64(accessMsg[2:10], uint64(amfUeId))

			_, err = conn.Write(accessMsg)
			if err != nil {
				log.Errorf("[UE] Error sending target cell access trigger: %v", err)
			} else {
				log.Info("[UE] Cell switch completed. Target cell access trigger sent successfully.")
			}
		}
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
