package gnb

import (
	stdctx "context"
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/config"
	gnbContext "OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	serviceNas "OmniRAN-Emulator/internal/control_test_engine/gnb/nas/service"
	serviceNgap "OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/service"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/ngap/trigger"
	"OmniRAN-Emulator/internal/monitoring"
	"os"
	"os/signal"
	"sync"
	"time"
)

func InitGnb(conf config.Config, wg *sync.WaitGroup) {

	// instance new gnb.
	gnb := &gnbContext.GNBContext{}

	// new gnb context.
	gnb.NewRanGnbContext(
		conf.GNodeB.PlmnList.GnbId,
		conf.GNodeB.PlmnList.Mcc,
		conf.GNodeB.PlmnList.Mnc,
		conf.GNodeB.PlmnList.Tac,
		conf.GNodeB.SliceSupportList.Sst,
		conf.GNodeB.SliceSupportList.Sd,
		conf.GNodeB.ControlIF.Ip,
		conf.GNodeB.DataIF.Ip,
		conf.GNodeB.ControlIF.Port,
		conf.GNodeB.DataIF.Port)
	gnb.SetLinkType(conf.GNodeB.LinkType)
	gnb.SetLinkPort(conf.GNodeB.LinkPort)

	// start communication with AMF (server SCTP).

	// new AMF context.
	amf := gnb.NewGnBAmf(conf.AMF.Ip, conf.AMF.Port)

	// start communication with AMF(SCTP).
	if err := serviceNgap.InitConn(amf, gnb); err != nil {
		log.Fatal("Error in", err)
	} else {
		log.Info("[GNB] SCTP/NGAP service is running")
		// wg.Add(1)
	}

	// start communication with UE (server UNIX sockets).
	if err := serviceNas.InitServer(gnb); err != nil {
		log.Fatal("Error in ", err)
	} else {
		log.Info("[GNB] UNIX/NAS service is running")
	}

	trigger.SendNgSetupRequest(gnb, amf)

	// control the signals
	sigGnb := make(chan os.Signal, 1)
	signal.Notify(sigGnb, os.Interrupt)

	// Block until a signal is received.
	<-sigGnb
	gnb.Terminate()
	wg.Done()
	// os.Exit(0)

}

func InitGnbForUeLatency(conf config.Config, sigGnb chan bool, synch chan bool) {

	// instance new gnb.
	gnb := &gnbContext.GNBContext{}

	// new gnb context.
	gnb.NewRanGnbContext(
		conf.GNodeB.PlmnList.GnbId,
		conf.GNodeB.PlmnList.Mcc,
		conf.GNodeB.PlmnList.Mnc,
		conf.GNodeB.PlmnList.Tac,
		conf.GNodeB.SliceSupportList.Sst,
		conf.GNodeB.SliceSupportList.Sd,
		conf.GNodeB.ControlIF.Ip,
		conf.GNodeB.DataIF.Ip,
		conf.GNodeB.ControlIF.Port,
		conf.GNodeB.DataIF.Port)
	gnb.SetLinkType(conf.GNodeB.LinkType)
	gnb.SetLinkPort(conf.GNodeB.LinkPort)

	// start communication with AMF (server SCTP).

	// new AMF context.
	amf := gnb.NewGnBAmf(conf.AMF.Ip, conf.AMF.Port)

	// start communication with AMF(SCTP).
	if err := serviceNgap.InitConn(amf, gnb); err != nil {
		log.Info("Error in", err)

		synch <- false

		return
	} else {
		log.Info("[GNB] SCTP/NGAP service is running")
		// wg.Add(1)
	}

	// start communication with UE (server UNIX sockets).
	if err := serviceNas.InitServer(gnb); err != nil {
		log.Info("Error in", err)

		synch <- false
	} else {
		log.Info("[GNB] UNIX/NAS service is running")

	}

	trigger.SendNgSetupRequest(gnb, amf)

	synch <- true

	// Block until a signal is received.
	<-sigGnb
	gnb.Terminate()
}

func InitGnbForLoadSeconds(conf config.Config, wg *sync.WaitGroup,
	monitor *monitoring.Monitor) {

	// instance new gnb.
	gnb := &gnbContext.GNBContext{}

	// new gnb context.
	gnb.NewRanGnbContext(
		conf.GNodeB.PlmnList.GnbId,
		conf.GNodeB.PlmnList.Mcc,
		conf.GNodeB.PlmnList.Mnc,
		conf.GNodeB.PlmnList.Tac,
		conf.GNodeB.SliceSupportList.Sst,
		conf.GNodeB.SliceSupportList.Sd,
		conf.GNodeB.ControlIF.Ip,
		conf.GNodeB.DataIF.Ip,
		conf.GNodeB.ControlIF.Port,
		conf.GNodeB.DataIF.Port)

	// start communication with AMF (server SCTP).

	// new AMF context.
	amf := gnb.NewGnBAmf(conf.AMF.Ip, conf.AMF.Port)

	// start communication with AMF(SCTP).
	if err := serviceNgap.InitConn(amf, gnb); err != nil {
		log.Info("Error in ", err)

		time.Sleep(1000 * time.Millisecond)

		wg.Done()

		return
	} else {
		log.Info("[GNB] SCTP/NGAP service is running")
		// wg.Add(1)
	}

	trigger.SendNgSetupRequest(gnb, amf)

	// timeout is 1 second for receive NG Setup Response
	time.Sleep(1000 * time.Millisecond)

	// AMF responds message sends by Tester
	// means AMF is available
	if amf.GetState() == 0x01 {
		monitor.IncRqs()
	}

	gnb.Terminate()
	wg.Done()
	// os.Exit(0)
}

func InitGnbForAvaibility(conf config.Config,
	monitor *monitoring.Monitor) {

	// instance new gnb.
	gnb := &gnbContext.GNBContext{}

	// new gnb context.
	gnb.NewRanGnbContext(
		conf.GNodeB.PlmnList.GnbId,
		conf.GNodeB.PlmnList.Mcc,
		conf.GNodeB.PlmnList.Mnc,
		conf.GNodeB.PlmnList.Tac,
		conf.GNodeB.SliceSupportList.Sst,
		conf.GNodeB.SliceSupportList.Sd,
		conf.GNodeB.ControlIF.Ip,
		conf.GNodeB.DataIF.Ip,
		conf.GNodeB.ControlIF.Port,
		conf.GNodeB.DataIF.Port)

	// start communication with AMF (server SCTP).

	// new AMF context.
	amf := gnb.NewGnBAmf(conf.AMF.Ip, conf.AMF.Port)

	// start communication with AMF(SCTP).
	if err := serviceNgap.InitConn(amf, gnb); err != nil {
		log.Info("Error in ", err)

		return

	} else {
		log.Info("[GNB] SCTP/NGAP service is running")

	}

	trigger.SendNgSetupRequest(gnb, amf)

	// timeout is 1 second for receive NG Setup Response
	time.Sleep(1000 * time.Millisecond)

	// AMF responds message sends by Tester
	// means AMF is available
	if amf.GetState() == 0x01 {
		monitor.IncAvaibility()

	}

	gnb.Terminate()
	// os.Exit(0)
}

// InitGnbFleet starts a gNB in fleet mode with context-based lifecycle management.
// The gnbSocketPath allows specifying a unique socket to avoid conflicts between
// multiple gNBs running simultaneously (e.g. /tmp/gnb_<gnbId>.sock).
// The returned channel will receive an error (or nil) when the gNB exits.
func InitGnbFleet(conf config.Config, ctx stdctx.Context, gnbSocketPath string) <-chan error {
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)

		// Override socket path if provided and using unix link type
		if gnbSocketPath != "" && conf.GNodeB.LinkType == "unix" {
			// Clean up old socket
			_ = os.Remove(gnbSocketPath)
		}

		// instance new gnb
		g := &gnbContext.GNBContext{}
		if gnbSocketPath != "" {
			g.SetSocketPath(gnbSocketPath)
		}

		g.NewRanGnbContext(
			conf.GNodeB.PlmnList.GnbId,
			conf.GNodeB.PlmnList.Mcc,
			conf.GNodeB.PlmnList.Mnc,
			conf.GNodeB.PlmnList.Tac,
			conf.GNodeB.SliceSupportList.Sst,
			conf.GNodeB.SliceSupportList.Sd,
			conf.GNodeB.ControlIF.Ip,
			conf.GNodeB.DataIF.Ip,
			conf.GNodeB.ControlIF.Port,
			conf.GNodeB.DataIF.Port)
		g.SetLinkType(conf.GNodeB.LinkType)
		g.SetLinkPort(conf.GNodeB.LinkPort)

		// Connect to AMF
		amf := g.NewGnBAmf(conf.AMF.Ip, conf.AMF.Port)

		if err := serviceNgap.InitConn(amf, g); err != nil {
			log.Errorf("[GNB-FLEET] NGAP connection failed: %v", err)
			errCh <- err
			return
		}
		log.Infof("[GNB-FLEET] %s SCTP/NGAP connected", conf.GNodeB.PlmnList.GnbId)

		if err := serviceNas.InitServer(g); err != nil {
			log.Errorf("[GNB-FLEET] NAS server start failed: %v", err)
			errCh <- err
			return
		}
		log.Infof("[GNB-FLEET] %s NAS service running", conf.GNodeB.PlmnList.GnbId)

		trigger.SendNgSetupRequest(g, amf)
		log.Infof("[GNB-FLEET] %s NG Setup Request sent", conf.GNodeB.PlmnList.GnbId)

		// Block until context cancelled (stop signal from fleet runner)
		<-ctx.Done()
		log.Infof("[GNB-FLEET] %s stopping (context cancelled)", conf.GNodeB.PlmnList.GnbId)
		g.Terminate()
		if gnbSocketPath != "" {
			_ = os.Remove(gnbSocketPath)
		}
		errCh <- nil
	}()

	return errCh
}
