package service

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/nas"
	"net"
)

func InitServer(gnb *context.GNBContext) error {
	var ln net.Listener
	var err error

	if gnb.GetLinkType() == "tcp" {
		addr := fmt.Sprintf("%s:%d", gnb.GetGnbIp(), gnb.GetLinkPort())
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("TCP Listen error on %s: %v", addr, err)
		}
		log.Infof("[GNB] TCP/NAS service is running on %s", addr)
	} else {
		// initiated GNB server with unix sockets.
		socketPath := gnb.GetSocketPath()
		ln, err = net.Listen("unix", socketPath)
		if err != nil {
			return fmt.Errorf("UNIX Listen error on %s: %v", socketPath, err)
		}
		log.Infof("[GNB] UNIX/NAS service is running on %s", socketPath)
	}

	gnb.SetListener(ln)

	go gnbListen(gnb)

	return nil
}

func gnbListen(gnb *context.GNBContext) {

	ln := gnb.GetListener()

	for {

		fd, err := ln.Accept()
		if err != nil {
			log.Info("[GNB][UE] Accept error: ", err)
			break
		}

		// TODO this region of the code may induces race condition.

		// new instance GNB UE context
		// store UE in UE Pool
		// store UE connection
		// select AMF and get sctp association
		// make a tun interface
		ue := gnb.NewGnBUe(fd)
		if ue == nil {
			break
		}

		// accept and handle connection.
		go processingConn(ue, gnb)
	}

}

func processingConn(ue *context.GNBUe, gnb *context.GNBContext) {

	buf := make([]byte, 65535)
	conn := ue.GetUnixSocket()

	for {

		n, err := conn.Read(buf[:])
		if err != nil {
			return
		}

		forwardData := make([]byte, n)
		copy(forwardData, buf[:n])

		// Find the active UE context in GNodeB that is currently associated with this connection.
		// If the connection was transferred (e.g. during target cell access in handover),
		// we should use the updated real UE context.
		activeUe := ue
		gnb.RangeUePool(func(ranUeId int64, temp *context.GNBUe) bool {
			if temp.GetUnixSocket() == conn {
				activeUe = temp
				return false // stop iteration
			}
			return true
		})

		// send to dispatch.
		go nas.Dispatch(activeUe, forwardData, gnb)
	}
}
