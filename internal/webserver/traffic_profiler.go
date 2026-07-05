package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"OmniRAN-Emulator/internal/chaos"
	ueContext "OmniRAN-Emulator/internal/control_test_engine/ue/context"
)

type TrafficMetricPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	Throughput    float64   `json:"throughput"`    // Mbps
	Latency       float64   `json:"latency"`       // ms
	Jitter        float64   `json:"jitter"`        // ms
	PacketLossPct float64   `json:"packetLossPct"` // %
}

type UETelemetryHistory struct {
	UeID    uint8                `json:"ueId"`
	History []TrafficMetricPoint `json:"history"`
}

type GTPUPacketDecode struct {
	Timestamp   time.Time `json:"timestamp"`
	TEID        uint32    `json:"teid"`
	SeqNumber   uint16    `json:"seqNumber"`
	Length      uint16    `json:"length"`
	PayloadType string    `json:"payloadType"` // "ping", "http", "vonr", "dns", or "data"
}

var (
	telemetryMu        sync.RWMutex
	telemetryHistories = make(map[uint8]*UETelemetryHistory) // Key: UE ID
	maxHistoryPoints   = 30

	packetDecodesMu sync.RWMutex
	packetDecodes   = make(map[uint8][]GTPUPacketDecode) // Key: UE ID
	seqNumbers      = make(map[uint8]uint16)
)

type SliceSlaProfile struct {
	SST             int32   `json:"sst"`
	SD              string  `json:"sd"`
	MaxThroughput   float64 `json:"maxThroughput"`   // Mbps
	BaselineLatency float64 `json:"baselineLatency"` // ms
	BaselineLoss    float64 `json:"baselineLoss"`    // %
	Congested       bool    `json:"congested"`       // Congestion injection status
}

var (
	slicesSlaMu   sync.RWMutex
	slicesSlaFile = "config/slices_sla.json"
	slicesSlas    = map[string]SliceSlaProfile{
		"1-": {
			SST:             1,
			SD:              "",
			MaxThroughput:   100.0,
			BaselineLatency: 10.0,
			BaselineLoss:    0.0,
			Congested:       false,
		},
	}
)

func getSliceSlaKey(sst int32, sd string) string {
	return fmt.Sprintf("%d-%s", sst, sd)
}

func loadSlicesSla() error {
	slicesSlaMu.Lock()
	defer slicesSlaMu.Unlock()

	data, err := os.ReadFile(slicesSlaFile)
	if err != nil {
		if os.IsNotExist(err) {
			return saveSlicesSlaLocked()
		}
		return err
	}

	var list []SliceSlaProfile
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	slicesSlas = make(map[string]SliceSlaProfile)
	for _, s := range list {
		key := getSliceSlaKey(s.SST, s.SD)
		slicesSlas[key] = s
	}
	return nil
}

func saveSlicesSlaLocked() error {
	list := make([]SliceSlaProfile, 0, len(slicesSlas))
	for _, s := range slicesSlas {
		list = append(list, s)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(slicesSlaFile, data, 0644)
}

func init() {
	_ = loadSlicesSla()
	// Start background metric collector
	go startTelemetryCollector()
}

func startTelemetryCollector() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		collectTelemetry()
	}
}

func generatePackets(ueId uint8, activeAction string) {
	packetDecodesMu.Lock()
	defer packetDecodesMu.Unlock()

	decodes, ok := packetDecodes[ueId]
	if !ok {
		decodes = make([]GTPUPacketDecode, 0)
	}

	seq := seqNumbers[ueId]
	now := time.Now()
	
	// Format dynamic TEID
	teid := uint32(0x10000000) + uint32(ueId)*256

	// Determine payload types and sizes
	var pType string
	var baseLen uint16
	switch activeAction {
	case "streaming":
		pType = "http"
		baseLen = 1200
	case "vonr":
		pType = "vonr"
		baseLen = 64
	case "ping":
		pType = "ping"
		baseLen = 84
	default:
		// Alternate between data and dns to look extremely high fidelity
		if seq%3 == 0 {
			pType = "dns"
			baseLen = 72
		} else {
			pType = "data"
			baseLen = 150
		}
	}

	// Generate 3 packets
	for i := 0; i < 3; i++ {
		seq++
		// Add small random noise to length
		pLen := baseLen
		if baseLen > 100 {
			pLen += uint16(now.UnixNano() % 200)
		} else {
			pLen += uint16(now.UnixNano() % 16)
		}

		decodes = append(decodes, GTPUPacketDecode{
			Timestamp:   now.Add(-time.Duration(3-i) * 300 * time.Millisecond),
			TEID:        teid,
			SeqNumber:   seq,
			Length:      pLen,
			PayloadType: pType,
		})
	}

	// Cap at 20
	if len(decodes) > 20 {
		decodes = decodes[len(decodes)-20:]
	}

	packetDecodes[ueId] = decodes
	seqNumbers[ueId] = seq
}

func collectTelemetry() {
	telemetryMu.Lock()
	defer telemetryMu.Unlock()

	// Get raw kernel stats
	devStats, err := parseProcNetDev()
	_ = err // Ignore or debug log if needed

	ues := ueContext.GetAllActiveUEs()
	now := time.Now()

	// Keep track of active UEs so we can clean up stale ones
	activeUEIds := make(map[uint8]bool)

	for _, u := range ues {
		id := u.GetUeId()
		activeUEIds[id] = true
		ifName := fmt.Sprintf("uetun%d", id)

		// 1. Throughput calculation
		var throughput float64

		// Check video stream activity
		streamsMu.RLock()
		if stream, active := activeStreams[id]; active && stream.Status == "streaming" {
			throughput = stream.SpeedMbps
		}
		streamsMu.RUnlock()

		// Check VoNR call activity
		callsMu.RLock()
		var vonrCall *ActiveCall
		if call, active := activeCalls[id]; active && call.Status == "connected" {
			throughput = 0.024 // 24 kbps
			vonrCall = call
		}
		callsMu.RUnlock()

		// Fallback/addition if interface is transmitting data but actions are idle
		if throughput == 0 && devStats != nil {
			if stats, ok := devStats[ifName]; ok {
				throughput = 0.05 + 0.1*(float64(stats.RxBytes%10)/10.0)
			}
		}

		// 2. Latency, Jitter, Packet Loss
		var latency, jitter, loss float64
		if vonrCall != nil {
			latency = vonrCall.LatencyMs
			jitter = vonrCall.JitterMs
			loss = vonrCall.PacketLossPct
		} else {
			// Ambient simulated stats for active sessions
			if u.GetStateMM() == ueContext.MM5G_REGISTERED_INITIATED || u.GetStateMM() == ueContext.MM5G_REGISTERED {
				latency = 12.0 + 5.0*(float64(now.UnixNano()%7)/7.0) // 12-17 ms base 5G latency
				jitter = 1.0 + 2.0*(float64(now.UnixNano()%5)/5.0)   // 1-3 ms jitter
				loss = 0.0

				// Apply Slice SLA constraints if configured
				sst := u.PduSession.Snssai.Sst
				sd := u.PduSession.Snssai.Sd
				key := getSliceSlaKey(sst, sd)

				slicesSlaMu.RLock()
				sla, exists := slicesSlas[key]
				if !exists {
					keySstOnly := getSliceSlaKey(sst, "")
					sla, exists = slicesSlas[keySstOnly]
				}
				slicesSlaMu.RUnlock()

				if exists {
					if throughput > sla.MaxThroughput {
						throughput = sla.MaxThroughput
					} else if throughput == 0 {
						throughput = 0.5 + 2.0*(float64(now.UnixNano()%10)/10.0)
						if throughput > sla.MaxThroughput {
							throughput = sla.MaxThroughput
						}
					}
					latency = sla.BaselineLatency + 3.0*(float64(now.UnixNano()%5)/5.0)
					loss = sla.BaselineLoss

					if sla.Congested {
						throughput = throughput * 0.1
						latency += 180.0
						loss += 15.0
					}
				}

				// If fuzzer/chaos is actively corrupting, spike the latency or packet loss!
				fuzz := chaos.GlobalChaosManager.GetFuzz()
				if fuzz.Enabled && fuzz.Probability > 0 {
					loss += fuzz.Probability * 100.0
					latency += fuzz.Probability * 150.0
					jitter += fuzz.Probability * 30.0
				}
			}
		}

		// If no session exists or not registered, stats are zero
		if u.GetStateMM() != ueContext.MM5G_REGISTERED_INITIATED && u.GetStateMM() != ueContext.MM5G_REGISTERED {
			throughput = 0
			latency = 0
			jitter = 0
			loss = 0
		}

		// Get or create history for this UE
		history, ok := telemetryHistories[id]
		if !ok {
			history = &UETelemetryHistory{
				UeID:    id,
				History: make([]TrafficMetricPoint, 0),
			}
			telemetryHistories[id] = history
		}

		// Add new data point
		history.History = append(history.History, TrafficMetricPoint{
			Timestamp:     now,
			Throughput:    throughput,
			Latency:       latency,
			Jitter:        jitter,
			PacketLossPct: loss,
		})

		// Limit history size
		if len(history.History) > maxHistoryPoints {
			history.History = history.History[1:]
		}

		// Generate GTP-U dissected packets
		act := "idle"
		if vonrCall != nil {
			act = "vonr"
		} else {
			streamsMu.RLock()
			if stream, active := activeStreams[id]; active && stream.Status == "streaming" {
				act = "streaming"
			}
			streamsMu.RUnlock()
		}
		if u.GetStateMM() == ueContext.MM5G_REGISTERED || u.GetStateMM() == ueContext.MM5G_REGISTERED_INITIATED {
			generatePackets(id, act)
		}
	}

	// Clean up stale histories and packets for UEs that are no longer active
	for id := range telemetryHistories {
		if !activeUEIds[id] {
			delete(telemetryHistories, id)
			packetDecodesMu.Lock()
			delete(packetDecodes, id)
			delete(seqNumbers, id)
			packetDecodesMu.Unlock()
		}
	}
}

func handleUETrafficPerformance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	telemetryMu.RLock()
	defer telemetryMu.RUnlock()

	_ = json.NewEncoder(w).Encode(telemetryHistories)
}

func handleUETrafficPackets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ueIdStr := r.URL.Query().Get("ueId")
	packetDecodesMu.RLock()
	defer packetDecodesMu.RUnlock()

	if ueIdStr != "" {
		var ueId uint8
		_, err := fmt.Sscanf(ueIdStr, "%d", &ueId)
		if err != nil {
			http.Error(w, "Invalid ueId parameter", http.StatusBadRequest)
			return
		}
		decodes, ok := packetDecodes[ueId]
		if !ok {
			decodes = []GTPUPacketDecode{}
		}
		_ = json.NewEncoder(w).Encode(decodes)
		return
	}

	_ = json.NewEncoder(w).Encode(packetDecodes)
}

func handleSlicesSla(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Allow-Methods", "*")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodGet {
		slicesSlaMu.RLock()
		defer slicesSlaMu.RUnlock()

		list := make([]SliceSlaProfile, 0, len(slicesSlas))
		for _, s := range slicesSlas {
			list = append(list, s)
		}
		_ = json.NewEncoder(w).Encode(list)
		return
	}

	if r.Method == http.MethodPost {
		var req SliceSlaProfile
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}

		key := getSliceSlaKey(req.SST, req.SD)

		slicesSlaMu.Lock()
		slicesSlas[key] = req
		_ = saveSlicesSlaLocked()
		slicesSlaMu.Unlock()

		_ = json.NewEncoder(w).Encode(req)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

