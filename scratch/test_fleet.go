package main

import (
	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/webserver"
	"fmt"
	"time"
)

func main() {
	// Initialize config
	if err := config.LoadConfig("config/config.yml"); err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}
	if err := config.LoadFleet(); err != nil {
		fmt.Printf("Failed to load fleet: %v\n", err)
		return
	}

	fmt.Println("Launching gNB profile 'gNB-West'...")
	err := webserver.LaunchGNBProfile("gNB-West")
	if err != nil {
		fmt.Printf("Failed to launch gNB: %v\n", err)
		return
	}

	time.Sleep(2 * time.Second)

	fmt.Println("Launching UE profile 'UE-test'...")
	ueId, err := webserver.LaunchUEFromProfile("UE-test", "gNB-West")
	if err != nil {
		fmt.Printf("Failed to launch UE: %v\n", err)
		webserver.CleanUpAll()
		return
	}
	fmt.Printf("UE launched successfully with ID: %d\n", ueId)

	fmt.Println("Waiting 20 seconds for registration to complete...")
	for i := 0; i < 20; i++ {
		time.Sleep(1 * time.Second)
		summary := webserver.GetFleetRunningSummary()
		if len(summary.RunningUEs) > 0 {
			ue := summary.RunningUEs[0]
			fmt.Printf("[%d s] UE State MM: %d (%s), SM: %d (%s)\n", i+1, ue.StateMM, ue.StateMMDesc, ue.StateSM, ue.StateSMDesc)
		} else {
			fmt.Printf("[%d s] No active UEs found in summary\n", i+1)
		}
	}

	fmt.Println("Cleaning up...")
	webserver.CleanUpAll()
	fmt.Println("Done!")
}
