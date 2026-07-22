package service

import (
	"fmt"
	"net"
	"golang.org/x/net/ipv4"
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
)

func InitGatewayGnb(gnb *context.GNBContext) error {

	// get ip for GNB gateway for data plane.
	ipGateway := gnb.GetGatewayGnbIp()

	conn, err := net.ListenPacket("ip4:4", ipGateway)
	if err != nil {
		return fmt.Errorf("[GNB][DATA] Error setting listen gateway GNB: %v", err)
	}

	dataPlaneConn, err := ipv4.NewRawConn(conn)
	if err != nil {
		return fmt.Errorf("[GNB][DATA] Error setting data plane communication with UEs: %v", err)
	}

	// successful established GNB/UE tunnel.
	gnb.SetUePlane(dataPlaneConn)

	go gatewayListen(gnb)

	return nil
}

func gatewayListen(gnb *context.GNBContext) {

	buffer := make([]byte, 65535)
	conn := gnb.GetUePlane()

	defer func() {
		err := conn.Close()
		if err != nil {
			log.Info("[GNB][DATA] Error in closing GNB/UE tunnel\n")
		}
	}()

	for {

		ipHeader, payload, _, err := conn.ReadFrom(buffer)
		// log.Info(" [GNB][DATA] Read %d bytes in GNB/UE tunnel", len(payload))
		if err != nil {
			log.Infof("[GNB][DATA] Error in reading from GNB/UE tunnel: %+v", err)
			return
		}

		forwardData := make([]byte, len(payload[:]))
		copy(forwardData, payload[:])

		// find owner of  the Data Plane.
		ue, err := gnb.GetGnbUeByIp(ipHeader.Src.String())
		if err != nil || ue == nil {
			log.Info("[GNB][DATA] Invalid GNB UE IP. UE is not found in GNB UE IP Pool")
			return
		}

		go processingData(ue, gnb, ipHeader.Src.String(), forwardData)
	}
}

func processingData(ue *context.GNBUe, gnb *context.GNBContext, srcIp string, packet []byte) {

	// get GTP/UDP connection.
	conn := gnb.GetN3Plane()
	if conn == nil {
		errMsg := fmt.Sprintf("[GNB][GTP] N3 GTP/UDP user-plane socket is not active on gNodeB %s. User-plane data forwarding to UPF (2152) requires N3 tunnel setup.", gnb.GetGnbId())
		log.Warn(errMsg)
		if config.ProtocolErrorHook != nil {
			config.ProtocolErrorHook("N3GTP", errMsg)
		}
		return
	}

	// send Data plane with GTP header.
	var teidUplink uint32
	sess := ue.FindPduSessionByIp(srcIp)
	if sess != nil {
		teidUplink = sess.GetUplinkTeid()
	} else {
		teidUplink = ue.GetTeidUplink()
	}

	remote := fmt.Sprintf("%s:%d", gnb.GetUpfIp(), gnb.GetUpfPort())
	upfAddr, err := net.ResolveUDPAddr("udp", remote)
	if err != nil {
		log.Info("[GNB][GTP] Error resolving UPF address for GTP/UDP tunnel", err)
		return
	}

	// send Data plane with GTP header.
	_, err = conn.WriteToGTP(teidUplink, packet, upfAddr)
	if err != nil {
		log.Info("[GNB][GTP] Error sending data plane in GTP/UDP tunnel")
	}

	//log.Info("[GNB][GTP] Send %d bytes in GNB->UPF tunnel\n", n)
}
