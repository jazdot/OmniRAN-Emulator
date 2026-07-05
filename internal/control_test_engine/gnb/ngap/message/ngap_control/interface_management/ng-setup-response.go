package interface_management

import (
	"fmt"
	"github.com/ishidawataru/sctp"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
)

func NgSetupResponse(connN2 *sctp.SCTPConn) (*ngapType.NGAPPDU, error) {
	var recvMsg = make([]byte, 2048)
	var n int

	// receive NGAP message from AMF.
	n, err := connN2.Read(recvMsg)
	if err != nil {
		return nil, fmt.Errorf("Error receiving NG-SETUP-RESPONSE: %w", err)
	}

	ngapMsg, err := ngap.Decoder(recvMsg[:n])
	if err != nil {
		return nil, fmt.Errorf("Error decoding NG-SETUP-RESPONSE: %w", err)
	}

	return ngapMsg, nil
}
