package service

import (
	"fmt"
	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/state"
	"net"
)

func CloseConn(ue *context.UEContext) {
	conn := ue.GetUnixConn()
	conn.Close()
}

func InitConn(ue *context.UEContext) error {

	var conn net.Conn
	var err error

	if ue.GetGnbLinkType() == "tcp" {
		addr := fmt.Sprintf("%s:%d", ue.GetGnbControlIp(), ue.GetGnbLinkPort())
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			return fmt.Errorf("[UE] Error on Dial TCP with server %s: %v", addr, err)
		}
	} else {
		socketPath := ue.GetGnbSocketPath()
		dialer := net.Dialer{
			LocalAddr: &net.UnixAddr{
				Name: fmt.Sprintf("@ue_%d", ue.GetUeId()),
				Net:  "unix",
			},
		}
		conn, err = dialer.Dial("unix", socketPath)
		if err != nil {
			return fmt.Errorf("[UE] Error on Dial UNIX with server on %s: %v", socketPath, err)
		}
	}

	// store unix socket connection in the UE.
	ue.SetUnixConn(conn)
	ue.OnRedirection = UeListen

	// listen NAS.
	go UeListen(ue)

	return nil
}

// ue listen unix sockets.
func UeListen(ue *context.UEContext) {

	buf := make([]byte, 65535)
	conn := ue.GetUnixConn()

	/*
		defer func() {
			err := conn.Close()
			if err != nil {
				fmt.Printf("Error in closing unix sockets for %s ue\n", ue.GetSupi())
			}
		}()
	*/

	for {

		// read message.
		n, err := conn.Read(buf[:])
		if err != nil {
			break
		}

		forwardData := make([]byte, n)
		copy(forwardData, buf[:n])

		// handling NAS message.
		go state.DispatchState(ue, forwardData)

	}
}
