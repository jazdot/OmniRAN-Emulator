package main

import (
	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/templates"
	"OmniRAN-Emulator/internal/webserver"

	// "fmt"
	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
)

const version = "1.0.1"

func init() {

	cfg, err := config.GetConfig()
	if err != nil {
		//return nil
		log.Fatal("Error in get configuration")
	}

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	if cfg.Logs.Level == 0 {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(log.Level(cfg.Logs.Level))
	}

	spew.Config.Indent = "\t"

	log.Info("OmniRAN-Emulator version " + version)
}

func main() {

	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "config/config.yml",
				Usage:   "Path to the configuration file",
			},
		},
		Before: func(c *cli.Context) error {
			if c.IsSet("config") {
				configPath := c.String("config")
				err := config.LoadConfig(configPath)
				if err != nil {
					log.Fatalf("Failed to load configuration file at %s: %v", configPath, err)
				}
				// Re-apply log level from loaded config
				cfg := config.Data
				if cfg.Logs.Level == 0 {
					log.SetLevel(log.InfoLevel)
				} else {
					log.SetLevel(log.Level(cfg.Logs.Level))
				}
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:    "ue",
				Aliases: []string{"ue"},
				Usage:   "Testing an ue attached with configuration",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "ue-only", Usage: "Run only the UE (do not start gNodeB in this process)", Value: false},
				},
				Action: func(c *cli.Context) error {
					name := "Testing an ue attached with configuration"
					cfg := config.Data
					ueOnly := c.Bool("ue-only")

					log.Info("---------------------------------------")
					log.Info("[TESTER] Starting test function: ", name)
					log.Info("[TESTER][UE] Number of UEs: ", 1)
					log.Info("[TESTER][GNB] Control interface IP/Port: ", cfg.GNodeB.ControlIF.Ip, "/", cfg.GNodeB.ControlIF.Port)
					log.Info("[TESTER][GNB] Data interface IP/Port: ", cfg.GNodeB.DataIF.Ip, "/", cfg.GNodeB.DataIF.Port)
					log.Info("[TESTER][AMF] AMF IP/Port: ", cfg.AMF.Ip, "/", cfg.AMF.Port)
					log.Info("---------------------------------------")
					templates.TestAttachUeWithConfiguration(ueOnly)
					return nil
				},
			},
			{
				Name:    "gnb",
				Aliases: []string{"gnb"},
				Usage:   "Testing an gnb attached with configuration",
				Action: func(c *cli.Context) error {
					name := "Testing an gnb attached with configuration"
					cfg := config.Data

					log.Info("---------------------------------------")
					log.Info("[TESTER] Starting test function: ", name)
					log.Info("[TESTER][GNB] Number of GNBs: ", 1)
					log.Info("[TESTER][GNB] Control interface IP/Port: ", cfg.GNodeB.ControlIF.Ip, "/", cfg.GNodeB.ControlIF.Port)
					log.Info("[TESTER][GNB] Data interface IP/Port: ", cfg.GNodeB.DataIF.Ip, "/", cfg.GNodeB.DataIF.Port)
					log.Info("[TESTER][AMF] AMF IP/Port: ", cfg.AMF.Ip, "/", cfg.AMF.Port)
					log.Info("---------------------------------------")
					templates.TestAttachGnbWithConfiguration()
					return nil
				},
			},
			{
				Name:    "load-test",
				Aliases: []string{"load-test"},
				Usage: "\nLoad endurance stress tests.\n" +
					"Example for testing multiple UEs: load-test -n 5 \n",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "number-of-ues", Value: 1, Aliases: []string{"n"}},
					&cli.BoolFlag{Name: "ue-only", Usage: "Run only the UEs (do not start gNodeB in this process)", Value: false},
				},
				Action: func(c *cli.Context) error {
					var numUes int
					name := "Testing registration of multiple UEs"
					cfg := config.Data
					ueOnly := c.Bool("ue-only")

					if c.IsSet("number-of-ues") {
						numUes = c.Int("number-of-ues")
					} else {
						log.Info(c.Command.Usage)
						return nil
					}

					log.Info("---------------------------------------")
					log.Info("[TESTER] Starting test function: ", name)
					log.Info("[TESTER][UE] Number of UEs: ", numUes)
					log.Info("[TESTER][GNB] gNodeB control interface IP/Port: ", cfg.GNodeB.ControlIF.Ip, "/", cfg.GNodeB.ControlIF.Port)
					log.Info("[TESTER][GNB] gNodeB data interface IP/Port: ", cfg.GNodeB.DataIF.Ip, "/", cfg.GNodeB.DataIF.Port)
					log.Info("[TESTER][AMF] AMF IP/Port: ", cfg.AMF.Ip, "/", cfg.AMF.Port)
					log.Info("---------------------------------------")
					templates.TestMultiUesInQueue(numUes, ueOnly)

					return nil
				},
			},
			{
				Name:    "amf-load-loop",
				Aliases: []string{"amf-load-loop"},
				Usage: "\nTest AMF responses in interval\n" +
					"Example for generating 20 requests to AMF per second in interval of 20 seconds: amf-load-loop -n 20 -t 20\n",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "number-of-requests", Value: 1, Aliases: []string{"n"}},
					&cli.IntFlag{Name: "time", Value: 1, Aliases: []string{"t"}},
				},
				Action: func(c *cli.Context) error {
					var time int
					var numRqs int

					name := "Test AMF responses in interval"
					cfg := config.Data

					numRqs = c.Int("number-of-requests")
					time = c.Int("time")

					log.Info("---------------------------------------")
					log.Warn("[TESTER] Starting test function: ", name)
					log.Warn("[TESTER][UE] Number of Requests per second: ", numRqs)
					log.Info("[TESTER][GNB] gNodeB control interface IP/Port: ", cfg.GNodeB.ControlIF.Ip, "/", cfg.GNodeB.ControlIF.Port)
					log.Info("[TESTER][GNB] gNodeB data interface IP/Port: ", cfg.GNodeB.DataIF.Ip, "/", cfg.GNodeB.DataIF.Port)
					log.Info("[TESTER][AMF] AMF IP/Port: ", cfg.AMF.Ip, "/", cfg.AMF.Port)
					log.Info("---------------------------------------")
					log.Warn("[TESTER][GNB] Total of AMF Responses in the interval:", templates.TestRqsLoop(numRqs, time))
					return nil
				},
			},
			{
				Name:    "ue-latency-interval",
				Aliases: []string{"ue-latency-interval"},
				Usage: "\nTesting UE latency in registration\n" +
					"Testing UE latency for 20 requests: ue-latency-interval -n 20\n",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "number-of-requests", Value: 1, Aliases: []string{"n"}},
				},
				Action: func(c *cli.Context) error {
					var requests int

					name := "Testing UE latency in registration"
					cfg := config.Data

					requests = c.Int("number-of-requests")

					log.Info("---------------------------------------")
					log.Warn("[TESTER] Starting test function: ", name)
					log.Warn("[TESTER][UE] Number of requests: ", requests)
					log.Info("[TESTER][GNB] Control interface IP/Port: ", cfg.GNodeB.ControlIF.Ip, "/", cfg.GNodeB.ControlIF.Port)
					log.Info("[TESTER][GNB] Data interface IP/Port: ", cfg.GNodeB.DataIF.Ip, "/", cfg.GNodeB.DataIF.Port)
					log.Info("[TESTER][AMF] AMF IP/Port: ", cfg.AMF.Ip, "/", cfg.AMF.Port)
					log.Info("---------------------------------------")
					log.Warn("[TESTER][UE] Average of the latency for a queue of requests: ", templates.TestUesLatencyInInterval(requests)/int64(requests), "ms")
					return nil
				},
			},
			{
				Name:    "Test availability of AMF",
				Aliases: []string{"amf-availability"},
				Usage: "\nTest availability of AMF in interval\n" +
					"Test availability of AMF in 20 seconds: amf-availability -t 20\n",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "time", Value: 1, Aliases: []string{"t"}},
				},
				Action: func(c *cli.Context) error {
					var time int

					name := "Test availability of AMF"
					cfg := config.Data

					time = c.Int("time")

					log.Info("---------------------------------------")
					log.Warn("[TESTER] Starting test function: ", name)
					log.Warn("[TESTER][UE] Interval of test: ", time, " seconds")
					log.Info("[TESTER][GNB] Control interface IP/Port: ", cfg.GNodeB.ControlIF.Ip, "/", cfg.GNodeB.ControlIF.Port)
					log.Info("[TESTER][GNB] Data interface IP/Port: ", cfg.GNodeB.DataIF.Ip, "/", cfg.GNodeB.DataIF.Port)
					log.Info("[TESTER][AMF] AMF IP/Port: ", cfg.AMF.Ip, "/", cfg.AMF.Port)
					log.Info("---------------------------------------")
					templates.TestAvailability(time)
					return nil
				},
			},
		{
				Name:  "scenario",
				Usage: "Run real-world 5G scenario tests",
				Subcommands: []*cli.Command{
					{
						Name:  "periodic-reg",
						Usage: "Periodic Registration Update (T3512 expiry simulation)",
						Action: func(c *cli.Context) error {
							log.Info("---------------------------------------")
							log.Info("[SCENARIO] Periodic Registration Update")
							log.Info("---------------------------------------")
							templates.ScenarioPeriodicRegistration(nil)
							return nil
						},
					},
					{
						Name:  "mobility-reg",
						Usage: "Mobility Registration Update (TAU after cell change)",
						Action: func(c *cli.Context) error {
							log.Info("---------------------------------------")
							log.Info("[SCENARIO] Mobility Registration Update")
							log.Info("---------------------------------------")
							templates.ScenarioMobilityRegistration(nil)
							return nil
						},
					},
					{
						Name:  "emergency-reg",
						Usage: "Emergency Registration (unauthenticated emergency services)",
						Action: func(c *cli.Context) error {
							log.Info("---------------------------------------")
							log.Info("[SCENARIO] Emergency Registration")
							log.Info("---------------------------------------")
							templates.ScenarioEmergencyRegistration(nil)
							return nil
						},
					},
					{
						Name:  "handover",
						Usage: "N2 Handover (Path Switch Request to simulate inter-gNB mobility)",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "target-gnb-ip", Value: "127.0.0.1", Usage: "Target gNB IP for handover"},
							&cli.IntFlag{Name: "target-gnb-port", Value: 9489, Usage: "Target gNB port for handover"},
							&cli.IntFlag{Name: "delay", Value: 5, Usage: "Seconds to wait after registration before triggering handover"},
						},
						Action: func(c *cli.Context) error {
							targetIp := c.String("target-gnb-ip")
							targetPort := c.Int("target-gnb-port")
							delay := c.Int("delay")
							log.Info("---------------------------------------")
							log.Infof("[SCENARIO] N2 Handover → target gNB %s:%d (delay: %ds)", targetIp, targetPort, delay)
							log.Info("---------------------------------------")
							templates.ScenarioHandover(targetIp, targetPort, delay, nil)
							return nil
						},
					},
					{
						Name:  "xn-handover",
						Usage: "Xn Handover (Direct peer-to-peer inter-gNB handover simulation)",
						Action: func(c *cli.Context) error {
							log.Info("---------------------------------------")
							log.Info("[SCENARIO] Xn Handover (Inter-gNB)")
							log.Info("---------------------------------------")
							templates.ScenarioXnHandover(nil)
							return nil
						},
					},
					{
						Name:  "pdu-lifecycle",
						Usage: "PDU Session Lifecycle (Establishment and clean release simulation)",
						Action: func(c *cli.Context) error {
							log.Info("---------------------------------------")
							log.Info("[SCENARIO] PDU Session Lifecycle")
							log.Info("---------------------------------------")
							templates.ScenarioPduLifecycle(nil)
							return nil
						},
					},
					{
						Name:  "full-lifecycle",
						Usage: "Full UE lifecycle: register → PDU session → idle → service request → deregister",
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "idle-seconds", Value: 5, Usage: "Seconds to stay in idle/CM-IDLE before service request"},
						},
						Action: func(c *cli.Context) error {
							idle := c.Int("idle-seconds")
							log.Info("---------------------------------------")
							log.Infof("[SCENARIO] Full UE Lifecycle (idle: %ds)", idle)
							log.Info("---------------------------------------")
							templates.ScenarioFullLifecycle(idle, nil)
							return nil
						},
					},
					{
						Name:  "deregister",
						Usage: "UE-initiated Deregistration (normal power-off)",
						Action: func(c *cli.Context) error {
							log.Info("---------------------------------------")
							log.Info("[SCENARIO] UE-initiated Deregistration")
							log.Info("---------------------------------------")
							templates.ScenarioDeregistration(nil)
							return nil
						},
					},
				},
			},
			{
				Name:    "web",
				Aliases: []string{"gui", "dashboard"},
				Usage:   "Start the Web UI Dashboard and API Server",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "host", Value: "127.0.0.1", Usage: "Host IP to bind the web server"},
					&cli.IntFlag{Name: "port", Value: 8080, Aliases: []string{"p"}, Usage: "Port to bind the web server"},
				},
				Action: func(c *cli.Context) error {
					host := c.String("host")
					port := c.Int("port")
					log.Info("---------------------------------------")
					log.Info("[TESTER] Starting Web UI Dashboard Server")
					log.Info("[TESTER] Listening on: http://", host, ":", port)
					log.Info("---------------------------------------")
					err := webserver.StartServer(host, port)
					if err != nil {
						log.Fatal("Failed to start web server: ", err)
					}
					return nil
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
