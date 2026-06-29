package service

import (
	"encoding/binary"
	"fmt"
	"github.com/ishidawataru/sctp"
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap"
	"net"
	"strconv"
	"time"
)

func InitConn(amf *context.GNBAmf, gnb *context.GNBContext) error {

	// check AMF IP and AMF port.
	remote := fmt.Sprintf("%s:%d", amf.GetAmfIp(), amf.GetAmfPort())
	local := fmt.Sprintf("%s:%d", gnb.GetGnbIp(), gnb.GetGnbPort())

	rem, err := sctp.ResolveSCTPAddr("sctp", remote)
	if err != nil {
		return err
	}
	loc, err := sctp.ResolveSCTPAddr("sctp", local)
	if err != nil {
		return err
	}

	// streams := amf.GetTNLAStreams()

	conn, err := sctp.DialSCTPExt(
		"sctp",
		loc,
		rem,
		sctp.InitMsg{NumOstreams: 2, MaxInstreams: 2})
	if err != nil {
		amf.SetSCTPConn(nil)
		return err
	}

	// set streams and other information about TNLA

	// successful established SCTP (TNLA - N2)
	amf.SetSCTPConn(conn)
	gnb.SetN2(conn)

	conn.SubscribeEvents(sctp.SCTP_EVENT_DATA_IO)

	go GnbListen(amf, gnb)

	return nil
}

func GnbListen(amf *context.GNBAmf, gnb *context.GNBContext) {

	buf := make([]byte, 65535)
	conn := amf.GetSCTPConn()

	/*
		defer func() {
			err := conn.Close()
			if err != nil {
				log.Info("[GNB][SCTP] Error in closing SCTP association for %d AMF\n", amf.GetAmfId())
			}
		}()
	*/

	for {

		n, info, err := conn.SCTPRead(buf[:])
		if err != nil {
			log.Warnf("[GNB][SCTP] SCTP socket read error or disconnect: %v", err)
			break
		}

		log.Info("[GNB][SCTP] Receive message in ", info.Stream, " stream\n")

		forwardData := make([]byte, n)
		copy(forwardData, buf[:n])

		// handling NGAP message.
		go ngap.Dispatch(amf, gnb, forwardData)

	}

	gnb.SignalConnLoss()
}

func StartXnListener(gnb *context.GNBContext) error {
	addrStr := fmt.Sprintf("%s:%d", gnb.GetGnbIp(), gnb.GetLinkPort()+1)
	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	gnb.SetXnConn(conn)
	go xnListen(gnb, conn)
	return nil
}

func xnListen(gnb *context.GNBContext, conn *net.UDPConn) {
	buf := make([]byte, 2048)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		processXnMessage(gnb, buf[:n], remoteAddr)
	}
}

func processXnMessage(gnb *context.GNBContext, data []byte, remoteAddr *net.UDPAddr) {
	if len(data) < 3 || data[0] != 0x58 || data[1] != 0x4e {
		return
	}
	msgType := data[2]
	switch msgType {
	case 0x01: // XN HANDOVER REQUEST
		log.Infof("[GNB-Target][XnAP] Received XN HANDOVER REQUEST from %v", remoteAddr)
		var amfUeId int64 = 0
		if len(data) >= 11 {
			amfUeId = int64(binary.BigEndian.Uint64(data[3:11]))
		}
		// Respond with XN HANDOVER REQUEST ACKNOWLEDGE
		ackMsg := make([]byte, 11)
		ackMsg[0] = 0x58; ackMsg[1] = 0x4e; ackMsg[2] = 0x02
		binary.BigEndian.PutUint64(ackMsg[3:11], uint64(amfUeId))

		if config.PcapHook != nil {
			config.PcapHook(gnb.GetGnbIp(), remoteAddr.IP.String(), uint16(gnb.GetLinkPort()+1), uint16(remoteAddr.Port), 17, ackMsg)
			time.Sleep(5 * time.Millisecond)
		}
		_, _ = gnb.GetXnConn().WriteToUDP(ackMsg, remoteAddr)
		log.Infof("[GNB-Target][XnAP] Sent XN HANDOVER REQUEST ACKNOWLEDGE to %v", remoteAddr)

	case 0x02: // XN HANDOVER REQUEST ACKNOWLEDGE
		log.Infof("[GNB-Source][XnAP] Received XN HANDOVER REQUEST ACKNOWLEDGE from %v", remoteAddr)
		var amfUeId int64 = 0
		if len(data) >= 11 {
			amfUeId = int64(binary.BigEndian.Uint64(data[3:11]))
		}
		var targetUe *context.GNBUe
		gnb.RangeUePool(func(id int64, ue *context.GNBUe) bool {
			if ue.GetAmfUeId() == amfUeId {
				targetUe = ue
				return false
			}
			return true
		})
		if targetUe != nil {
			// 1. RRCReconfiguration (HO Command) (0x08)
			if config.PcapHook != nil {
				ueIp := "10.200.200." + strconv.Itoa(targetUe.GetUeId())
				gnbIp := gnb.GetGnbIp()
				gnbPort := uint16(gnb.GetLinkPort())
				config.PcapHook(gnbIp, ueIp, gnbPort, 9999, 17, []byte{0x52, 0x52, 0x43, 0x08})
				time.Sleep(5 * time.Millisecond)
			}
			// 2. XN SN STATUS TRANSFER (0x04)
			snMsg := make([]byte, 11)
			snMsg[0] = 0x58; snMsg[1] = 0x4e; snMsg[2] = 0x04
			binary.BigEndian.PutUint64(snMsg[3:11], uint64(amfUeId))
			if config.PcapHook != nil {
				config.PcapHook(gnb.GetGnbIp(), remoteAddr.IP.String(), uint16(gnb.GetLinkPort()+1), uint16(remoteAddr.Port), 17, snMsg)
				time.Sleep(5 * time.Millisecond)
			}
			_, _ = gnb.GetXnConn().WriteToUDP(snMsg, remoteAddr)
			log.Infof("[GNB-Source][XnAP] Sent XN SN STATUS TRANSFER to %v", remoteAddr)
		}

	case 0x04: // XN SN STATUS TRANSFER
		log.Infof("[GNB-Target][XnAP] Received XN SN STATUS TRANSFER from %v", remoteAddr)

	case 0x03: // XN UE CONTEXT RELEASE
		log.Infof("[GNB-Source][XnAP] Received XN UE CONTEXT RELEASE from %v. Cleaning up UE context...", remoteAddr)
		if len(data) >= 11 {
			amfUeId := int64(binary.BigEndian.Uint64(data[3:11]))
			var ranUeId int64 = -1
			gnb.RangeUePool(func(id int64, ue *context.GNBUe) bool {
				if ue.GetAmfUeId() == amfUeId {
					ranUeId = id
					return false
				}
				return true
			})
			if ranUeId != -1 {
				gnb.DeleteGnBUe(ranUeId)
				log.Infof("[GNB-Source][XnAP] UE context %d deleted successfully", ranUeId)
			}
		}
	}
}
