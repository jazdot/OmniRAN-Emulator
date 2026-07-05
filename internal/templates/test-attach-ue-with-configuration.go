package templates

import (
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/control_test_engine/gnb"
	"OmniRAN-Emulator/internal/control_test_engine/ue"
	"sync"
	"time"
)

func TestAttachUeWithConfiguration(ueOnly bool) {

	wg := sync.WaitGroup{}

	cfg, err := config.GetConfig()
	if err != nil {
		//return nil
		log.Fatal("Error in get configuration")
	}

	if !ueOnly {
		go gnb.InitGnb(cfg, &wg)
		wg.Add(1)
		time.Sleep(1 * time.Second)
	}

	go ue.RegistrationUe(cfg, 1, &wg)

	wg.Add(1)

	wg.Wait()
}
