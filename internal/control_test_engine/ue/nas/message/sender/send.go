package sender

import (
	"fmt"
	"time"
	"OmniRAN-Emulator/internal/chaos"
	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
)

func SendToGnb(ue *context.UEContext, message []byte) {
	conn := ue.GetUnixConn()
	if conn == nil {
		return
	}

	msgType := chaos.GetNasMsgNameFromBytes(message, ue.GetUeId())
	_, message = chaos.GlobalChaosManager.EvalFuzz(msgType, message)
	shouldDrop, delay := chaos.GlobalChaosManager.EvalNas(ue.GetUeId(), msgType)
	if shouldDrop {
		return
	}

	if delay > 0 {
		time.Sleep(delay)
	}

	_, err := conn.Write(message)
	if err != nil {
		fmt.Println("Tratar o erro:", err)
	}
}
