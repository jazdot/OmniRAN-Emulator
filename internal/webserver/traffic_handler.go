package webserver

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	ueContext "OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"github.com/sirupsen/logrus"
)

// TrafficStats represents kernel network stats
type InterfaceStats struct {
	RxBytes   int64 `json:"rxBytes"`
	RxPackets int64 `json:"rxPackets"`
	TxBytes   int64 `json:"txBytes"`
	TxPackets int64 `json:"txPackets"`
}

// ActiveStream tracks video streaming simulation state
type ActiveStream struct {
	UeID       uint8              `json:"ueId"`
	Quality    string             `json:"quality"`
	BytesTrans int64              `json:"bytesTrans"`
	SpeedMbps  float64            `json:"speedMbps"`
	BufferSec  int                `json:"bufferSec"`
	Status     string             `json:"status"` // "buffering", "streaming", "stopped"
	cancel     context.CancelFunc
}

// ActiveCall tracks VoNR SIP and UDP RTP call simulation state
type ActiveCall struct {
	CallerID      uint8     `json:"callerId"`
	CalleeID      string    `json:"calleeId"` // UE ID or "echo"
	Status        string    `json:"status"`   // "dialing", "ringing", "connected", "disconnected"
	CallDuration  int       `json:"callDuration"`
	PacketsSent   int64     `json:"packetsSent"`
	PacketsRecv   int64     `json:"packetsRecv"`
	JitterMs      float64   `json:"jitterMs"`
	LatencyMs     float64   `json:"latencyMs"`
	PacketLossPct float64   `json:"packetLossPct"`
	MosScore      float64   `json:"mosScore"`
	SipLogs       []string  `json:"sipLogs"`
	StartedAt     time.Time
	cancel        context.CancelFunc
	mu            sync.Mutex
}

var (
	streamsMu     sync.RWMutex
	activeStreams = make(map[uint8]*ActiveStream)

	callsMu     sync.RWMutex
	activeCalls = make(map[uint8]*ActiveCall) // keyed by caller ID
)

// parseProcNetDev parses /proc/net/dev to get raw throughput of uetun interfaces
func parseProcNetDev() (map[string]*InterfaceStats, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := make(map[string]*InterfaceStats)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		ifName := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(ifName, "uetun") {
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		rxBytes, _ := strconv.ParseInt(fields[0], 10, 64)
		rxPackets, _ := strconv.ParseInt(fields[1], 10, 64)
		txBytes, _ := strconv.ParseInt(fields[8], 10, 64)
		txPackets, _ := strconv.ParseInt(fields[9], 10, 64)

		stats[ifName] = &InterfaceStats{
			RxBytes:   rxBytes,
			RxPackets: rxPackets,
			TxBytes:   txBytes,
			TxPackets: txPackets,
		}
	}
	return stats, nil
}

// ─── API Handlers ─────────────────────────────────────────────────────────────

type UEPingRequest struct {
	UeID uint8  `json:"ueId"`
	Host string `json:"host"`
}

type UEPingResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Mode    string `json:"mode"` // "real" or "simulated"
}

func handleUEPing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UEPingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	u := ueContext.GetActiveUE(req.UeID)
	if u == nil {
		http.Error(w, fmt.Sprintf("UE %d is not active", req.UeID), http.StatusNotFound)
		return
	}

	host := req.Host
	if host == "" {
		host = "8.8.8.8"
	}

	ifName := fmt.Sprintf("uetun%d", req.UeID)
	logrus.Infof("[WEB][TRAFFIC] Ping requested on UE %d (%s) to host %s", req.UeID, ifName, host)

	// Try running a real ping bound to the interface
	cmd := exec.Command("ping", "-I", ifName, "-c", "4", "-W", "2", host)
	out, err := cmd.CombinedOutput()

	resp := UEPingResponse{}
	if err == nil {
		resp.Success = true
		resp.Output = string(out)
		resp.Mode = "real"
	} else {
		// Fallback to simulated ping to make it failproof in mock/disconnected setups
		resp.Success = true
		resp.Mode = "simulated"
		resp.Output = fmt.Sprintf("PING %s (%s) from uetun%d: 56(84) bytes of data.\n"+
			"64 bytes from %s: icmp_seq=1 ttl=64 time=14.5 ms\n"+
			"64 bytes from %s: icmp_seq=2 ttl=64 time=16.1 ms\n"+
			"64 bytes from %s: icmp_seq=3 ttl=64 time=15.0 ms\n"+
			"64 bytes from %s: icmp_seq=4 ttl=64 time=15.4 ms\n\n"+
			"--- %s ping statistics ---\n"+
			"4 packets transmitted, 4 received, 0%% packet loss, time 3004ms\n"+
			"rtt min/avg/max/mdev = 14.502/15.250/16.120/0.590 ms\n"+
			"(Core simulation mode fallback: Real ping execution failed)",
			host, host, req.UeID, host, host, host, host, host)
	}

	_ = json.NewEncoder(w).Encode(resp)
}

type UEHttpFetchRequest struct {
	UeID uint8  `json:"ueId"`
	URL  string `json:"url"`
}

type UEHttpFetchResponse struct {
	Success    bool   `json:"success"`
	StatusCode int    `json:"statusCode"`
	Headers    string `json:"headers"`
	Body       string `json:"body"`
	TimeMs     int64  `json:"timeMs"`
	Mode       string `json:"mode"`
}

func handleUEHttp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UEHttpFetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	u := ueContext.GetActiveUE(req.UeID)
	if u == nil {
		http.Error(w, fmt.Sprintf("UE %d is not active", req.UeID), http.StatusNotFound)
		return
	}

	url := req.URL
	if url == "" {
		url = "http://example.com"
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}

	// Retrieve UE IP to bind local address
	rawIP := u.GetIp(req.UeID)
	ueIp := strings.Split(rawIP, ",")[0]

	logrus.Infof("[WEB][TRAFFIC] HTTP Fetch on UE %d (IP: %s) to URL: %s", req.UeID, ueIp, url)

	start := time.Now()
	resp := UEHttpFetchResponse{}

	// Setup Dialer bound to the UE's IP
	localAddr := &net.TCPAddr{
		IP: net.ParseIP(ueIp),
	}
	dialer := &net.Dialer{
		LocalAddr: localAddr,
		Timeout:   4 * time.Second,
	}
	transport := &http.Transport{
		DialContext: dialer.DialContext,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   6 * time.Second,
	}

	httpResp, err := client.Get(url)
	if err == nil {
		defer httpResp.Body.Close()
		elapsed := time.Since(start).Milliseconds()

		// Read some of the body
		bodyBuf := make([]byte, 1020)
		n, _ := httpResp.Body.Read(bodyBuf)
		bodyStr := string(bodyBuf[:n])
		if n == 1020 {
			bodyStr += "\n...[truncated]"
		}

		// Read headers
		var headerBuilder strings.Builder
		for key, vals := range httpResp.Header {
			headerBuilder.WriteString(fmt.Sprintf("%s: %s\n", key, strings.Join(vals, ", ")))
		}

		resp.Success = true
		resp.StatusCode = httpResp.StatusCode
		resp.Headers = headerBuilder.String()
		resp.Body = bodyStr
		resp.TimeMs = elapsed
		resp.Mode = "real"
	} else {
		// Fallback to simulated HTTP fetch
		elapsed := rand.Int63n(150) + 80
		resp.Success = true
		resp.StatusCode = 200
		resp.Headers = "Date: " + time.Now().Format(time.RFC1123) + "\n" +
			"Server: Omni5G-Edge-Gateway/v1.0.1\n" +
			"Content-Type: text/html; charset=UTF-8\n" +
			"Content-Length: 354\n" +
			"Connection: close"
		resp.Body = fmt.Sprintf("<!DOCTYPE html>\n<html>\n<head>\n  <title>Omni5G Core Edge Simulator</title>\n</head>\n<body>\n  <h1>Welcome to example website!</h1>\n  <p>This page was fetched cleanly via the simulated user-plane network data path of <strong>UE-%d</strong> (Interface: <em>uetun%d</em>).</p>\n  <p>Core network simulation mode: fallback mock enabled.</p>\n</body>\n</html>", req.UeID, req.UeID)
		resp.TimeMs = elapsed
		resp.Mode = "simulated"
	}

	_ = json.NewEncoder(w).Encode(resp)
}

type UEStreamRequest struct {
	UeID    uint8  `json:"ueId"`
	Action  string `json:"action"`  // "start" or "stop"
	Quality string `json:"quality"` // "720p", "1080p", "4k"
}

func handleUEStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UEStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	u := ueContext.GetActiveUE(req.UeID)
	if u == nil {
		http.Error(w, fmt.Sprintf("UE %d is not active", req.UeID), http.StatusNotFound)
		return
	}

	streamsMu.Lock()
	defer streamsMu.Unlock()

	if req.Action == "stop" {
		if stream, ok := activeStreams[req.UeID]; ok {
			stream.cancel()
			delete(activeStreams, req.UeID)
			logrus.Infof("[WEB][TRAFFIC] Video stream stopped on UE %d", req.UeID)
		}
		_, _ = w.Write([]byte(`{"status":"stopped"}`))
		return
	}

	// Start video stream
	if old, ok := activeStreams[req.UeID]; ok {
		old.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	stream := &ActiveStream{
		UeID:      req.UeID,
		Quality:   req.Quality,
		Status:    "buffering",
		BufferSec: 0,
		cancel:    cancel,
	}
	activeStreams[req.UeID] = stream

	go runStreamSimulation(ctx, stream)

	logrus.Infof("[WEB][TRAFFIC] Video stream (%s) started on UE %d", req.Quality, req.UeID)
	_, _ = w.Write([]byte(`{"status":"started"}`))
}

func runStreamSimulation(ctx context.Context, s *ActiveStream) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Speed ranges based on quality
	var minSpeed, maxSpeed float64
	switch s.Quality {
	case "720p":
		minSpeed, maxSpeed = 1.5, 3.5
	case "1080p":
		minSpeed, maxSpeed = 4.5, 8.5
	case "4k":
		minSpeed, maxSpeed = 16.0, 26.0
	default:
		minSpeed, maxSpeed = 4.0, 8.0
	}

	bufCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			streamsMu.Lock()
			if s.Status == "buffering" {
				bufCount++
				s.BufferSec = bufCount * 4 // simulate buffer filling up quickly
				// Peak download speed during buffering
				s.SpeedMbps = maxSpeed * 1.5
				s.BytesTrans += int64((s.SpeedMbps * 1024 * 1024) / 8) // convert to bytes

				if bufCount >= 2 {
					s.Status = "streaming"
					s.BufferSec = 15
				}
			} else {
				// Streaming state
				s.SpeedMbps = minSpeed + rand.Float64()*(maxSpeed-minSpeed)
				s.BytesTrans += int64((s.SpeedMbps * 1024 * 1024) / 8)

				// Fluctuating buffer size (maintaining 12-25s)
				s.BufferSec += rand.Intn(3) - 1
				if s.BufferSec < 10 {
					s.BufferSec = 10
				} else if s.BufferSec > 25 {
					s.BufferSec = 25
				}
			}
			streamsMu.Unlock()
		}
	}
}

type UEVonrDialRequest struct {
	CallerID uint8  `json:"callerId"`
	CalleeID string `json:"calleeId"` // "echo" or another UE ID (e.g. "102")
}

var (
	echoServerOnce sync.Once
	echoServerAddr = "127.0.0.2:5005"
)

func startVoiceEchoServer() {
	echoServerOnce.Do(func() {
		addr, err := net.ResolveUDPAddr("udp", echoServerAddr)
		if err != nil {
			logrus.Errorf("[VoNR-ECHO] Resolve UDP address error: %v", err)
			return
		}
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			logrus.Errorf("[VoNR-ECHO] Listen UDP error on %s: %v", echoServerAddr, err)
			return
		}
		logrus.Infof("[VoNR-ECHO] Real VoNR Voice Echo Server listening on UDP %s", echoServerAddr)

		go func() {
			defer conn.Close()
			buf := make([]byte, 2048)
			for {
				n, raddr, err := conn.ReadFromUDP(buf)
				if err != nil {
					logrus.Warnf("[VoNR-ECHO] Read error: %v", err)
					return
				}
				// Echo the packet back to the sender
				_, err = conn.WriteToUDP(buf[:n], raddr)
				if err != nil {
					logrus.Warnf("[VoNR-ECHO] Write error: %v", err)
				}
			}
		}()
	})
}

func startRealRtpLoop(ctx context.Context, c *ActiveCall, localIP, remoteIP string, localPort, remotePort int, appendLog func(string)) {
	lAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", localIP, localPort))
	if err != nil {
		appendLog(fmt.Sprintf("RTP UDP local address resolve error: %v", err))
		return
	}
	rAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", remoteIP, remotePort))
	if err != nil {
		appendLog(fmt.Sprintf("RTP UDP remote address resolve error: %v", err))
		return
	}

	conn, err := net.ListenUDP("udp", lAddr)
	if err != nil {
		appendLog(fmt.Sprintf("RTP UDP bind error on %s:%d: %v", localIP, localPort, err))
		return
	}
	defer conn.Close()

	appendLog(fmt.Sprintf("RTP UDP socket bound to %s:%d. Remote Peer: %s:%d", localIP, localPort, remoteIP, remotePort))

	// Start packet receiver
	go func() {
		buf := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				n, _, err := conn.ReadFrom(buf)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					return // Socket closed or error, exit receiver
				}
				if n >= 20 {
					c.mu.Lock()
					c.PacketsRecv++
					
					// Parse timestamp from payload (bytes 12-20)
					sentNano := int64(binary.BigEndian.Uint64(buf[12:20]))
					if sentNano > 0 {
						rttMs := float64(time.Now().UnixNano()-sentNano) / 1e6
						if rttMs > 0 && rttMs < 5000 {
							// Update Jitter (RFC 3550 style estimate)
							if c.LatencyMs > 0 {
								diff := rttMs - c.LatencyMs
								if diff < 0 {
									diff = -diff
								}
								c.JitterMs = c.JitterMs + (diff-c.JitterMs)/16.0
							}
							c.LatencyMs = rttMs
						}
					}
					c.mu.Unlock()
				}
			}
		}
	}()

	// Start packet sender
	ticker := time.NewTicker(20 * time.Millisecond) // 50 packets/sec
	defer ticker.Stop()

	rtpHeader := make([]byte, 40)
	rtpHeader[0] = 0x80 // RFC 1889 Version 2
	rtpHeader[1] = 0x60 // AMR-WB payload type

	var seq uint16 = 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			c.PacketsSent++
			c.mu.Unlock()

			// Put sequence number (bytes 2-3)
			binary.BigEndian.PutUint16(rtpHeader[2:4], seq)
			seq++

			// Put timestamp in payload (bytes 12-20)
			nowNano := time.Now().UnixNano()
			binary.BigEndian.PutUint64(rtpHeader[12:20], uint64(nowNano))

			_, _ = conn.WriteTo(rtpHeader, rAddr)
		}
	}
}

func handleUEVonrDial(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UEVonrDialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	uCaller := ueContext.GetActiveUE(req.CallerID)
	if uCaller == nil {
		http.Error(w, fmt.Sprintf("Caller UE %d is not active", req.CallerID), http.StatusNotFound)
		return
	}

	callsMu.Lock()
	defer callsMu.Unlock()

	// If already in a call, hang up first
	if old, ok := activeCalls[req.CallerID]; ok {
		old.cancel()
		delete(activeCalls, req.CallerID)
	}

	if req.CalleeID == "echo" {
		startVoiceEchoServer()
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &ActiveCall{
		CallerID:  req.CallerID,
		CalleeID:  req.CalleeID,
		Status:    "dialing",
		SipLogs:   []string{"[SIP] Sending SIP INVITE to Voice Core..."},
		StartedAt: time.Now(),
		cancel:    cancel,
	}
	activeCalls[req.CallerID] = c

	go runVonrCallSimulation(ctx, c, uCaller)

	logrus.Infof("[WEB][TRAFFIC] VoNR Call initiated by UE %d to %s", req.CallerID, req.CalleeID)
	_ = json.NewEncoder(w).Encode(c)
}

func runVonrCallSimulation(ctx context.Context, c *ActiveCall, uCaller *ueContext.UEContext) {
	appendLog := func(log string) {
		c.mu.Lock()
		c.SipLogs = append(c.SipLogs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05.000"), log))
		c.mu.Unlock()
	}

	// 1. Dialing Handshake
	time.Sleep(500 * time.Millisecond)
	appendLog("SIP/2.0 100 Trying")
	time.Sleep(500 * time.Millisecond)
	c.Status = "ringing"
	appendLog("SIP/2.0 180 Ringing (Target Alerted)")

	time.Sleep(1000 * time.Millisecond)
	c.Status = "connected"
	appendLog("SIP/2.0 200 OK (Session Established)")
	appendLog("Content-Type: application/sdp")
	appendLog("SDP: m=audio 5004 RTP/AVP 96 (AMR-WB voice codec)")
	appendLog("SIP ACK sent")

	// Determine if we can do real UDP connection
	callerIP := strings.Split(uCaller.GetIp(c.CallerID), ",")[0]
	isRealUdpExchange := false

	var localIP, remoteIP string
	var localPort, remotePort int

	if callerIP != "" {
		if c.CalleeID == "echo" {
			localIP = callerIP
			remoteIP = "127.0.0.2"
			localPort = 5004
			remotePort = 5005
			isRealUdpExchange = true
			appendLog(fmt.Sprintf("Established real user-plane VoNR voice echo loop: %s:5004 <-> %s:5005", localIP, remoteIP))
		} else {
			calleeIdInt, err := strconv.Atoi(c.CalleeID)
			if err == nil {
				uCallee := ueContext.GetActiveUE(uint8(calleeIdInt))
				if uCallee != nil {
					calleeIP := strings.Split(uCallee.GetIp(uint8(calleeIdInt)), ",")[0]
					if calleeIP != "" {
						localIP = callerIP
						remoteIP = calleeIP
						localPort = 5004
						remotePort = 5004
						isRealUdpExchange = true
						appendLog(fmt.Sprintf("Established real peer-to-peer VoNR exchange: %s:5004 <-> %s:5004", localIP, remoteIP))

						// Set callee status in activeCalls map
						calleeIDUint := uint8(calleeIdInt)
						calleeCall := &ActiveCall{
							CallerID:  calleeIDUint,
							CalleeID:  fmt.Sprintf("%d", c.CallerID),
							Status:    "connected",
							SipLogs: []string{
								fmt.Sprintf("[%s] Incoming call from UE-%d...", time.Now().Format("15:04:05.000"), c.CallerID),
								fmt.Sprintf("[%s] SIP/2.0 200 OK (Call Accepted)", time.Now().Format("15:04:05.000")),
							},
							StartedAt: c.StartedAt,
							cancel:    c.cancel, // share cancel
						}

						callsMu.Lock()
						activeCalls[calleeIDUint] = calleeCall
						callsMu.Unlock()

						// Start peer loop for callee in background
						go startRealRtpLoop(ctx, calleeCall, calleeIP, callerIP, 5004, 5004, func(l string) {
							calleeCall.mu.Lock()
							calleeCall.SipLogs = append(calleeCall.SipLogs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05.000"), l))
							calleeCall.mu.Unlock()
						})
					}
				}
			}
		}
	}

	if isRealUdpExchange {
		go startRealRtpLoop(ctx, c, localIP, remoteIP, localPort, remotePort, appendLog)
	} else {
		appendLog("Voice Core NAT Fallback: running RTP loop in simulated media mode")
	}

	// 2. Call Active Loop (updates durations, packet loss, and MOS scores)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer func() {
		c.Status = "disconnected"
		appendLog("SIP BYE sent")
		appendLog("SIP/2.0 200 OK (Call Terminated)")

		// Clean up callee call if peer-to-peer
		if c.CalleeID != "echo" {
			calleeIdInt, err := strconv.Atoi(c.CalleeID)
			if err == nil {
				callsMu.Lock()
				delete(activeCalls, uint8(calleeIdInt))
				callsMu.Unlock()
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			c.CallDuration = int(time.Since(c.StartedAt).Seconds())

			if !isRealUdpExchange {
				// Sim mode: increment simulated packets
				c.PacketsSent += 50
				c.PacketsRecv += 50
				c.LatencyMs = 12.0 + rand.Float64()*18.0
				c.JitterMs = 1.2 + rand.Float64()*4.0
				c.PacketLossPct = 0.0

				// Occasional simulated network drops to show MOS score changes
				if rand.Intn(20) == 0 {
					c.PacketLossPct = 0.5 + rand.Float64()*2.0
					c.JitterMs += 10
					c.LatencyMs += 30
				}
			} else {
				// Real mode: calculate packet loss
				if c.PacketsSent > 0 {
					loss := 100.0 * float64(c.PacketsSent-c.PacketsRecv) / float64(c.PacketsSent)
					if loss < 0 {
						loss = 0
					}
					c.PacketLossPct = loss
				}
			}

			// MOS Score Calculation
			rFactor := 94.2 - (c.LatencyMs * 0.024) - (c.PacketLossPct * 2.5) - (c.JitterMs * 0.4)
			if rFactor < 0 {
				rFactor = 0
			}
			c.MosScore = 1.0 + (0.035 * rFactor) + (0.000007 * rFactor * (rFactor - 60) * (100 - rFactor))
			if c.MosScore > 4.5 {
				c.MosScore = 4.5
			} else if c.MosScore < 1.0 {
				c.MosScore = 1.0
			}

			// Update Callee call duration and stats in peer-to-peer call
			if c.CalleeID != "echo" {
				calleeIdInt, err := strconv.Atoi(c.CalleeID)
				if err == nil {
					callsMu.Lock()
					if calleeCall, ok := activeCalls[uint8(calleeIdInt)]; ok {
						calleeCall.mu.Lock()
						calleeCall.CallDuration = c.CallDuration
						calleeCall.MosScore = c.MosScore
						calleeCall.LatencyMs = c.LatencyMs
						calleeCall.JitterMs = c.JitterMs
						calleeCall.PacketLossPct = c.PacketLossPct
						calleeCall.mu.Unlock()
					}
					callsMu.Unlock()
				}
			}

			c.mu.Unlock()
		}
	}
}

func handleUEVonrHangup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type HangupRequest struct {
		CallerID uint8 `json:"callerId"`
	}

	var req HangupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	callsMu.Lock()
	defer callsMu.Unlock()

	if c, ok := activeCalls[req.CallerID]; ok {
		c.cancel()
		delete(activeCalls, req.CallerID)
		logrus.Infof("[WEB][TRAFFIC] VoNR Call hung up by UE %d", req.CallerID)
	}

	_, _ = w.Write([]byte(`{"status":"disconnected"}`))
}

// handleUETrafficStats returns live traffic statistics for all active UEs
func handleUETrafficStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Get raw kernel stats
	devStats, err := parseProcNetDev()
	if err != nil {
		logrus.Warnf("[WEB][TRAFFIC] Failed to parse /proc/net/dev: %v", err)
	}

	// 2. Build live response map
	type UETrafficInfo struct {
		UeID          uint8           `json:"ueId"`
		ActiveAction  string          `json:"activeAction"` // "idle", "web", "streaming", "vonr"
		SpeedMbps     float64         `json:"speedMbps"`
		BufferSec     int             `json:"bufferSec"`
		BytesTrans    int64           `json:"bytesTrans"`
		InterfaceName string          `json:"interfaceName"`
		KernelStats   *InterfaceStats `json:"kernelStats,omitempty"`
		VonrCall      *ActiveCall     `json:"vonrCall,omitempty"`
	}

	response := make(map[uint8]*UETrafficInfo)

	// Fetch all active UEs
	ues := ueContext.GetAllActiveUEs()
	for _, u := range ues {
		id := u.GetUeId()
		ifName := fmt.Sprintf("uetun%d", id)

		info := &UETrafficInfo{
			UeID:          id,
			ActiveAction:  "idle",
			InterfaceName: ifName,
		}

		// Attach kernel stats
		if devStats != nil {
			if stats, ok := devStats[ifName]; ok {
				info.KernelStats = stats
				info.BytesTrans = stats.RxBytes + stats.TxBytes
			}
		}

		// Check video stream activity
		streamsMu.RLock()
		if stream, active := activeStreams[id]; active {
			info.ActiveAction = "streaming"
			info.SpeedMbps = stream.SpeedMbps
			info.BufferSec = stream.BufferSec
			// Add stream bytes if kernel stats not showing anything
			if info.BytesTrans == 0 {
				info.BytesTrans = stream.BytesTrans
			}
		}
		streamsMu.RUnlock()

		// Check VoNR call activity
		callsMu.RLock()
		if call, active := activeCalls[id]; active {
			info.ActiveAction = "vonr"
			info.VonrCall = call
			// VoNR code bitrate is low (approx 24 kbps)
			info.SpeedMbps = 0.024
			if info.BytesTrans == 0 {
				info.BytesTrans = call.PacketsSent * 40
			}
		}
		callsMu.RUnlock()

		response[id] = info
	}

	_ = json.NewEncoder(w).Encode(response)
}
