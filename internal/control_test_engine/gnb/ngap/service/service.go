package service

import (
	"encoding/binary"
	"fmt"
	"github.com/ishidawataru/sctp"
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap"
	"net"
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
			break
		}

		log.Info("[GNB][SCTP] Receive message in ", info.Stream, " stream\n")

		forwardData := make([]byte, n)
		copy(forwardData, buf[:n])

		// handling NGAP message.
		go ngap.Dispatch(amf, gnb, forwardData)

	}

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
		// Respond with XN HANDOVER REQUEST ACKNOWLEDGE
		ackMsg := []byte{0x58, 0x4e, 0x02}
		_, _ = gnb.GetXnConn().WriteToUDP(ackMsg, remoteAddr)
		log.Infof("[GNB-Target][XnAP] Sent XN HANDOVER REQUEST ACKNOWLEDGE to %v", remoteAddr)
	case 0x02: // XN HANDOVER REQUEST ACKNOWLEDGE
		log.Infof("[GNB-Source][XnAP] Received XN HANDOVER REQUEST ACKNOWLEDGE from %v", remoteAddr)
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
