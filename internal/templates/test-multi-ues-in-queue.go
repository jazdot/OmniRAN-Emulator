package templates

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/control_test_engine/gnb"
	ueContext "OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/service"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/trigger"
	"OmniRAN-Emulator/lib/nas/nasMessage"
	"OmniRAN-Emulator/lib/nas/security"

	log "github.com/sirupsen/logrus"
)

func TestMultiUesInQueue(numUes int, ueOnly bool) {
	wg := sync.WaitGroup{}

	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal("Error in get configuration")
	}

	if !ueOnly {
		go gnb.InitGnb(cfg, &wg)
		wg.Add(1)
		time.Sleep(1 * time.Second)
	}

	msin := cfg.Ue.Msin
	for i := 1; i <= numUes; i++ {
		imsi := imsiGenerator(i, msin)
		log.Info("[TESTER] TESTING REGISTRATION USING IMSI ", imsi, " UE")
		
		// Copy config and set IMSI
		ueCfg := cfg
		ueCfg.Ue.Msin = imsi

		wg.Add(1)
		go func(id uint8, conf config.Config) {
			defer wg.Done()

			// Build and register UE
			u := &ueContext.UEContext{}
			u.SetGnbLinkType(conf.GNodeB.LinkType)
			u.SetGnbLinkPort(conf.GNodeB.LinkPort)
			u.SetGnbControlIp(conf.GNodeB.ControlIF.Ip)

			u.NewRanUeContext(
				conf.Ue.Msin,
				security.AlgCiphering128NEA0,
				security.AlgIntegrity128NIA2,
				conf.Ue.Key,
				conf.Ue.Opc,
				"c9e8763286b5b9ffbdf56e1297d0887b",
				conf.Ue.Amf,
				conf.Ue.Sqn,
				conf.Ue.Hplmn.Mcc,
				conf.Ue.Hplmn.Mnc,
				conf.Ue.Dnn,
				conf.Ue.PduSessionType,
				int32(conf.Ue.Snssai.Sst),
				conf.Ue.Snssai.Sd,
				id,
				conf.Ue.PduSessions,
			)
			u.SetRegistrationType(nasMessage.RegistrationType5GSInitialRegistration)

			// Defer termination immediately to guarantee cleanup on return
			defer u.Terminate()

			if err := service.InitConn(u); err != nil {
				log.Errorf("[TESTER][UE %d] Connection failed: %v", id, err)
				return
			}

			trigger.InitRegistration(u)

			// Wait up to 15 seconds for registration success
			deadline := time.Now().Add(15 * time.Second)
			registered := false
			for time.Now().Before(deadline) {
				if u.GetStateMM() == ueContext.MM5G_REGISTERED {
					registered = true
					break
				}
				time.Sleep(200 * time.Millisecond)
			}

			if registered {
				log.Infof("[TESTER][UE %d] Registered successfully", id)
				// Keep UE registered for 5 seconds to simulate active hold time
				time.Sleep(5 * time.Second)
			} else {
				log.Warnf("[TESTER][UE %d] Registration timed out", id)
			}
		}(uint8(i), ueCfg)

		// Delay before launching the next UE
		time.Sleep(2 * time.Second)
	}

	wg.Wait()
}

func imsiGenerator(i int, msin string) string {
	msin_int, err := strconv.Atoi(msin)
	if err != nil {
		log.Fatal("Error in get configuration")
	}
	base := msin_int + (i - 1)

	imsi := fmt.Sprintf("%010d", base)
	return imsi
}