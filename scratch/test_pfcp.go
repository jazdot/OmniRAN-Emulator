package main

import (
	"encoding/hex"
	"fmt"
	"github.com/free5gc/pfcp"
	"github.com/free5gc/pfcp/pfcpType"
)

func main() {
	// Let's build a CreatePDR
	createPDR := pfcp.CreatePDR{
		PDRID: &pfcpType.PacketDetectionRuleID{
			RuleId: 1,
		},
		Precedence: &pfcpType.Precedence{
			PrecedenceValue: 100,
		},
		PDI: &pfcp.PDI{
			SourceInterface: &pfcpType.SourceInterface{
				InterfaceValue: pfcpType.SourceInterfaceAccess,
			},
		},
		FARID: &pfcpType.FARID{
			FarIdValue: 1,
		},
	}

	pdrBuf, err := createPDR.Marshal()
	if err != nil {
		fmt.Printf("Error marshaling PDR: %v\n", err)
	} else {
		fmt.Printf("CreatePDR Hex: %s\n", hex.EncodeToString(pdrBuf))
	}

	// Let's build a CreateQER
	createQER := pfcp.CreateQER{
		QERID: &pfcpType.QERID{
			QERID: 1,
		},
		GateStatus: &pfcpType.GateStatus{
			Uplink:   pfcpType.GateStatusOpen,
			Downlink: pfcpType.GateStatusOpen,
		},
	}

	qerBuf, err := createQER.Marshal()
	if err != nil {
		fmt.Printf("Error marshaling QER: %v\n", err)
	} else {
		fmt.Printf("CreateQER Hex: %s\n", hex.EncodeToString(qerBuf))
	}
}
