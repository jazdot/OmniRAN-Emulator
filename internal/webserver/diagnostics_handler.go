package webserver

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"strconv"
	"syscall"
	"time"

	"OmniRAN-Emulator/config"
	"OmniRAN-Emulator/internal/control_test_engine/gnb/context"
	ueContext "OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/lib/ngap"
	"OmniRAN-Emulator/lib/ngap/ngapType"
	"github.com/sirupsen/logrus"
)

// PCAP Global Header link type
const linkTypeEthernet = 1

// PcapCaptureManager handles active PCAP capture state
type PcapCaptureManager struct {
	mu          sync.Mutex
	isCapturing bool
	ifName      string
	protoFilter string
	fileName    string
	file        *os.File
	socketFd    int
	packetCount int64
	bytesCount  int64
	startTime   time.Time
	stopChan    chan struct{}
}

var (
	pcapMgr       = &PcapCaptureManager{}
	capturesDir   = "log/captures"
	logFileMutex  sync.Mutex
	logFilePath   = "log/emulator.log"

	gnbPortHistoryMu sync.RWMutex
	gnbPortHistory   = make(map[uint16]string)
	ueIpHistoryMu   sync.RWMutex
	ueIpHistory     = make(map[string]string)
)

func init() {
	_ = os.MkdirAll(capturesDir, 0755)
	_ = os.MkdirAll("log", 0755)

	config.PcapHook = WriteSimulatedPacket

	// Start background mapping updates
	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			updateLifelineHistory()
		}
	}()
}

// WriteSimulatedPacket builds an Ethernet/IP/UDP (or TCP/SCTP) header and writes it directly to the active PCAP file, if capture is enabled.
func WriteSimulatedPacket(srcIp, dstIp string, srcPort, dstPort uint16, proto uint8, payload []byte) {
	pcapMgr.mu.Lock()
	defer pcapMgr.mu.Unlock()

	if !pcapMgr.isCapturing || pcapMgr.file == nil {
		return
	}

	// 1. Build IP header
	ipHeader := make([]byte, 20)
	ipHeader[0] = 0x45 // Version 4, IHL 5
	ipHeader[1] = 0x00 // TOS
	binary.BigEndian.PutUint16(ipHeader[2:4], uint16(20+8+len(payload))) // Total length
	ipHeader[8] = 64 // TTL
	ipHeader[9] = proto // Protocol
	
	src := net.ParseIP(srcIp).To4()
	if src == nil { src = []byte{127,0,0,1} }
	dst := net.ParseIP(dstIp).To4()
	if dst == nil { dst = []byte{127,0,0,1} }
	copy(ipHeader[12:16], src)
	copy(ipHeader[16:20], dst)

	// 2. Build Transport header
	var transportHeader []byte
	if proto == 17 { // UDP
		transportHeader = make([]byte, 8)
		binary.BigEndian.PutUint16(transportHeader[0:2], srcPort)
		binary.BigEndian.PutUint16(transportHeader[2:4], dstPort)
		binary.BigEndian.PutUint16(transportHeader[4:6], uint16(8+len(payload)))
	} else if proto == 6 { // TCP
		transportHeader = make([]byte, 20)
		binary.BigEndian.PutUint16(transportHeader[0:2], srcPort)
		binary.BigEndian.PutUint16(transportHeader[2:4], dstPort)
		transportHeader[12] = 0x50 // Data offset 5
	} else if proto == 132 { // SCTP DATA Chunk
		// SCTP common header (12 bytes) + DATA chunk header (16 bytes)
		transportHeader = make([]byte, 28)
		binary.BigEndian.PutUint16(transportHeader[0:2], srcPort)
		binary.BigEndian.PutUint16(transportHeader[2:4], dstPort)
		// chunk type = 0 (DATA), chunk flags = 0x07 (BEU), chunk length = 16 + len(payload)
		transportHeader[12] = 0
		transportHeader[13] = 0x07
		binary.BigEndian.PutUint16(transportHeader[14:16], uint16(16+len(payload)))
		// PPID = 60 (NGAP)
		binary.BigEndian.PutUint32(transportHeader[24:28], 60)
	}

	packetData := append(ipHeader, transportHeader...)
	packetData = append(packetData, payload...)

	ethData := wrapInEthernet(packetData, 0x0800)

	writeErr := writePcapPacketHeader(pcapMgr.file, len(ethData), time.Now())
	if writeErr == nil {
		_, _ = pcapMgr.file.Write(ethData)
		pcapMgr.packetCount++
		pcapMgr.bytesCount += int64(len(ethData))
	}
}

func updateLifelineHistory() {
	// 1. Update GNB Port History
	context.ActiveGNBsMu.RLock()
	for _, g := range context.ActiveGNBs {
		n2 := g.GetN2()
		if n2 != nil {
			localAddr := n2.LocalAddr()
			if localAddr != nil {
				_, portStr, err := net.SplitHostPort(localAddr.String())
				if err == nil {
					if pVal, err := strconv.ParseUint(portStr, 10, 16); err == nil {
						gnbIdStr := g.GetGnbId()
						var roleStr string
						if val, err := strconv.ParseInt(gnbIdStr, 16, 64); err == nil {
							roleStr = fmt.Sprintf("gNB (%d)", val)
						} else {
							roleStr = fmt.Sprintf("gNB (%s)", gnbIdStr)
						}
						gnbPortHistoryMu.Lock()
						gnbPortHistory[uint16(pVal)] = roleStr
						gnbPortHistoryMu.Unlock()
					}
				}
			}
		}
	}
	context.ActiveGNBsMu.RUnlock()

	// 2. Update UE IP History
	for _, u := range ueContext.GetAllActiveUEs() {
		for _, sess := range u.PduSessions {
			ueIp := u.GetIp(sess.Id)
			if ueIp != "" {
				roleStr := fmt.Sprintf("UE (%d)", u.GetUeId())
				ueIpHistoryMu.Lock()
				ueIpHistory[ueIp] = roleStr
				ueIpHistoryMu.Unlock()
			}
		}
	}
}

// ─── PCAP Helper functions ────────────────────────────────────────────────────

func writePcapGlobalHeader(w io.Writer) error {
	header := make([]byte, 24)
	binary.LittleEndian.PutUint32(header[0:4], 0xa1b2c3d4)  // Magic number
	binary.LittleEndian.PutUint16(header[4:6], 2)           // Major version
	binary.LittleEndian.PutUint16(header[6:8], 4)           // Minor version
	binary.LittleEndian.PutUint32(header[8:12], 0)          // Timezone correction
	binary.LittleEndian.PutUint32(header[12:16], 0)         // Timestamp accuracy
	binary.LittleEndian.PutUint32(header[16:20], 65535)     // Max snaplen
	binary.LittleEndian.PutUint32(header[20:24], linkTypeEthernet) // Link type (Ethernet)
	_, err := w.Write(header)
	return err
}

func writePcapPacketHeader(w io.Writer, length int, timestamp time.Time) error {
	header := make([]byte, 16)
	sec := timestamp.Unix()
	usec := timestamp.UnixNano() / 1000 % 1000000

	binary.LittleEndian.PutUint32(header[0:4], uint32(sec))
	binary.LittleEndian.PutUint32(header[4:8], uint32(usec))
	binary.LittleEndian.PutUint32(header[8:12], uint32(length))
	binary.LittleEndian.PutUint32(header[12:16], uint32(length))

	_, err := w.Write(header)
	return err
}

func wrapInEthernet(data []byte, ethType uint16) []byte {
	eth := make([]byte, 14+len(data))
	// Dst MAC: 00:00:00:00:00:02
	eth[0] = 0x00; eth[1] = 0x00; eth[2] = 0x00; eth[3] = 0x00; eth[4] = 0x00; eth[5] = 0x02
	// Src MAC: 00:00:00:00:00:01
	eth[6] = 0x00; eth[7] = 0x00; eth[8] = 0x00; eth[9] = 0x00; eth[10] = 0x00; eth[11] = 0x01
	
	binary.BigEndian.PutUint16(eth[12:14], ethType)
	copy(eth[14:], data)
	return eth
}

func parsePacket(data []byte) (ipStart int, ethType uint16) {
	// 1. Try Null/Loopback (4-byte header, BSD Null/Loopback)
	if len(data) >= 24 { // 4 bytes + 20 bytes IPv4
		// check family (can be big or little endian)
		familyLE := binary.LittleEndian.Uint32(data[0:4])
		familyBE := binary.BigEndian.Uint32(data[0:4])
		
		ipVersion := data[4] >> 4
		if (familyLE == 2 || familyBE == 2) && ipVersion == 4 {
			return 4, 0x0800
		}
		if (familyLE == 24 || familyLE == 28 || familyLE == 30 || familyBE == 24 || familyBE == 28 || familyBE == 30) && ipVersion == 6 {
			return 4, 0x86dd
		}
	}

	// 2. Try SLL (Linux Cooked Capture v1, 16-byte header)
	if len(data) >= 16 {
		proto := binary.BigEndian.Uint16(data[14:16])
		if proto == 0x0800 || proto == 0x86dd {
			version := data[16] >> 4
			if (proto == 0x0800 && version == 4) || (proto == 0x86dd && version == 6) {
				return 16, proto
			}
		}
	}

	// 3. Try Ethernet (14-byte header)
	if len(data) >= 14 {
		proto := binary.BigEndian.Uint16(data[12:14])
		if proto == 0x0800 || proto == 0x86dd {
			version := data[14] >> 4
			if (proto == 0x0800 && version == 4) || (proto == 0x86dd && version == 6) {
				return 14, proto
			}
		}
	}

	// 4. Try SLL2 (Linux Cooked Capture v2, 20-byte header)
	if len(data) >= 40 {
		proto := binary.BigEndian.Uint16(data[0:2])
		if proto == 0x0800 || proto == 0x86dd {
			version := data[20] >> 4
			if (proto == 0x0800 && version == 4) || (proto == 0x86dd && version == 6) {
				return 20, proto
			}
		}
	}

	// 5. Try Raw IP
	if len(data) >= 20 {
		version := data[0] >> 4
		if version == 4 {
			return 0, 0x0800
		} else if version == 6 {
			return 0, 0x86dd
		}
	}

	return -1, 0
}

func getIpProtocol(ipPayload []byte, ethType uint16) uint8 {
	if ethType == 0x0800 && len(ipPayload) >= 20 {
		return ipPayload[9]
	} else if ethType == 0x86dd && len(ipPayload) >= 40 {
		return ipPayload[6]
	}
	return 0
}

func matchesProtocol(protoNum uint8, filter string) bool {
	if filter == "" || filter == "all" {
		return true
	}
	switch strings.ToLower(filter) {
	case "icmp":
		return protoNum == 1
	case "tcp":
		return protoNum == 6
	case "udp":
		return protoNum == 17
	case "sctp":
		return protoNum == 132
	}
	return false
}

func htons(val uint16) uint16 {
	return (val << 8) | (val >> 8)
}

// ─── API Handlers ─────────────────────────────────────────────────────────────

type NetworkInterfaceInfo struct {
	Index        int      `json:"index"`
	Name         string   `json:"name"`
	Flags        string   `json:"flags"`
	IPAddresses  []string `json:"ipAddresses"`
}

func handleGetInterfaces(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list interfaces: %v", err), http.StatusInternalServerError)
		return
	}

	result := make([]NetworkInterfaceInfo, 0, len(ifaces))
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		ips := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			ips = append(ips, addr.String())
		}
		result = append(result, NetworkInterfaceInfo{
			Index:        iface.Index,
			Name:         iface.Name,
			Flags:        iface.Flags.String(),
			IPAddresses:  ips,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	_ = json.NewEncoder(w).Encode(result)
}

type PcapStartRequest struct {
	Interface   string `json:"interface"` // Interface name or "any"
	Protocol    string `json:"protocol"`  // "all", "icmp", "tcp", "udp", "sctp"
	FileName    string `json:"fileName"`  // Destination filename
}

func handleStartPcap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PcapStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.FileName == "" {
		req.FileName = fmt.Sprintf("capture_%d.pcap", time.Now().Unix())
	}
	if !strings.HasSuffix(req.FileName, ".pcap") {
		req.FileName += ".pcap"
	}
	// Sanitize fileName to prevent directory traversal
	req.FileName = filepath.Base(req.FileName)

	pcapMgr.mu.Lock()
	defer pcapMgr.mu.Unlock()

	if pcapMgr.isCapturing {
		http.Error(w, "A packet capture session is already running", http.StatusConflict)
		return
	}

	// Create output file
	filePath := filepath.Join(capturesDir, req.FileName)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create pcap file: %v", err), http.StatusInternalServerError)
		return
	}

	if err := writePcapGlobalHeader(file); err != nil {
		file.Close()
		_ = os.Remove(filePath)
		http.Error(w, fmt.Sprintf("Failed to write global pcap header: %v", err), http.StatusInternalServerError)
		return
	}

	// Open raw packet socket
	// AF_PACKET, SOCK_RAW allows capturing link-layer packets. 
	// Protocol ETH_P_ALL captures all ether types.
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(syscall.ETH_P_ALL)))
	if err != nil {
		file.Close()
		_ = os.Remove(filePath)
		http.Error(w, fmt.Sprintf("Failed to open raw packet socket (requires sudo/root): %v", err), http.StatusInternalServerError)
		return
	}

	// Bind to interface if requested
	isRawIp := false
	if req.Interface != "" && req.Interface != "any" {
		iface, err := net.InterfaceByName(req.Interface)
		if err != nil {
			syscall.Close(fd)
			file.Close()
			_ = os.Remove(filePath)
			http.Error(w, fmt.Sprintf("Interface '%s' not found: %v", req.Interface, err), http.StatusBadRequest)
			return
		}

		sll := &syscall.SockaddrLinklayer{
			Protocol: htons(syscall.ETH_P_ALL),
			Ifindex:  iface.Index,
		}
		if err := syscall.Bind(fd, sll); err != nil {
			syscall.Close(fd)
			file.Close()
			_ = os.Remove(filePath)
			http.Error(w, fmt.Sprintf("Failed to bind socket to interface %s: %v", req.Interface, err), http.StatusInternalServerError)
			return
		}
		// uetun interfaces are IPIP tunnels (no ethernet layer)
		if strings.HasPrefix(req.Interface, "uetun") {
			isRawIp = true
		}
	}

	// Initialize manager state
	pcapMgr.isCapturing = true
	pcapMgr.ifName = req.Interface
	pcapMgr.protoFilter = req.Protocol
	pcapMgr.fileName = req.FileName
	pcapMgr.file = file
	pcapMgr.socketFd = fd
	pcapMgr.packetCount = 0
	pcapMgr.bytesCount = 0
	pcapMgr.startTime = time.Now()
	pcapMgr.stopChan = make(chan struct{})

	// Start reading loop in background
	go captureLoop(fd, file, req.Protocol, isRawIp, pcapMgr.stopChan)

	logrus.Infof("[DIAGNOSTICS] Started packet capture on interface '%s' (filter: %s) -> file %s", req.Interface, req.Protocol, req.FileName)
	_, _ = w.Write([]byte(`{"status":"started","fileName":"` + req.FileName + `"}`))
}

func captureLoop(fd int, file *os.File, filter string, isRawIp bool, stopChan chan struct{}) {
	defer syscall.Close(fd)
	defer file.Close()

	buf := make([]byte, 65535)

	for {
		select {
		case <-stopChan:
			return
		default:
			// Read packet from raw socket
			n, _, err := syscall.Recvfrom(fd, buf, 0)
			if err != nil {
				// Socket closed or error, exit loop
				return
			}
			if n <= 0 {
				continue
			}

			packet := make([]byte, n)
			copy(packet, buf[:n])

			// Parse packet format and extract IP payload
			ipStart, ethType := parsePacket(packet)
			if ipStart < 0 {
				continue
			}

			ipPayload := packet[ipStart:]
			proto := getIpProtocol(ipPayload, ethType)
			if !matchesProtocol(proto, filter) {
				continue
			}

			// Re-encapsulate raw IP payload in standard 14-byte Ethernet header
			writeData := wrapInEthernet(ipPayload, ethType)

			// Write PCAP packet header and data
			pcapMgr.mu.Lock()
			writeErr := writePcapPacketHeader(file, len(writeData), time.Now())
			if writeErr == nil {
				_, writeErr = file.Write(writeData)
				if writeErr == nil {
					pcapMgr.packetCount++
					pcapMgr.bytesCount += int64(len(writeData))
				}
			}
			pcapMgr.mu.Unlock()
		}
	}
}

func handleStopPcap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pcapMgr.mu.Lock()
	defer pcapMgr.mu.Unlock()

	if !pcapMgr.isCapturing {
		http.Error(w, "No active packet capture session to stop", http.StatusBadRequest)
		return
	}

	// Trigger goroutine exit and close sockets/files
	close(pcapMgr.stopChan)
	syscall.Close(pcapMgr.socketFd)
	pcapMgr.file.Sync()
	pcapMgr.file.Close()

	pcapMgr.isCapturing = false

	logrus.Infof("[DIAGNOSTICS] Stopped packet capture session for file %s. Captured %d packets.", pcapMgr.fileName, pcapMgr.packetCount)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"stopped","packets":%d,"bytes":%d}`, pcapMgr.packetCount, pcapMgr.bytesCount)))
}

type PcapStatusResponse struct {
	IsCapturing bool    `json:"isCapturing"`
	Interface   string  `json:"interface"`
	Protocol    string  `json:"protocol"`
	FileName    string  `json:"fileName"`
	PacketCount int64   `json:"packetCount"`
	BytesCount  int64   `json:"bytesCount"`
	ElapsedSec  int     `json:"elapsedSec"`
}

func handleGetPcapStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pcapMgr.mu.Lock()
	defer pcapMgr.mu.Unlock()

	resp := PcapStatusResponse{
		IsCapturing: pcapMgr.isCapturing,
	}

	if pcapMgr.isCapturing {
		resp.Interface = pcapMgr.ifName
		resp.Protocol = pcapMgr.protoFilter
		resp.FileName = pcapMgr.fileName
		resp.PacketCount = pcapMgr.packetCount
		resp.BytesCount = pcapMgr.bytesCount
		resp.ElapsedSec = int(time.Since(pcapMgr.startTime).Seconds())
	}

	_ = json.NewEncoder(w).Encode(resp)
}

type SavedPcapFileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

func handleListPcaps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	files, err := os.ReadDir(capturesDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read captures directory: %v", err), http.StatusInternalServerError)
		return
	}

	pcaps := make([]SavedPcapFileInfo, 0)
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".pcap") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		pcaps = append(pcaps, SavedPcapFileInfo{
			Name:    file.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}

	sort.Slice(pcaps, func(i, j int) bool {
		return pcaps[i].ModTime > pcaps[j].ModTime // Show newest first
	})

	_ = json.NewEncoder(w).Encode(pcaps)
}

func handleDownloadPcap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		http.Error(w, "Missing 'file' parameter", http.StatusBadRequest)
		return
	}

	fileName = filepath.Base(fileName) // Prevent directory traversal
	filePath := filepath.Join(capturesDir, fileName)

	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)
}

func handleDeletePcap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		http.Error(w, "Missing 'file' parameter", http.StatusBadRequest)
		return
	}

	fileName = filepath.Base(fileName)
	filePath := filepath.Join(capturesDir, fileName)

	pcapMgr.mu.Lock()
	if pcapMgr.isCapturing && pcapMgr.fileName == fileName {
		pcapMgr.mu.Unlock()
		http.Error(w, "Cannot delete an active capture file", http.StatusConflict)
		return
	}
	pcapMgr.mu.Unlock()

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to delete file: %v", err), http.StatusInternalServerError)
		}
		return
	}

	logrus.Infof("[DIAGNOSTICS] Deleted capture file: %s", fileName)
	_, _ = w.Write([]byte(`{"status":"success"}`))
}

func handleDownloadLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logFileMutex.Lock()
	defer logFileMutex.Unlock()

	if _, err := os.Stat(logFilePath); err != nil {
		http.Error(w, "No log file available", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=emulator.log")
	w.Header().Set("Content-Type", "text/plain")
	http.ServeFile(w, r, logFilePath)
}

func handleClearLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logFileMutex.Lock()
	defer logFileMutex.Unlock()

	// Truncate the file to 0 bytes
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to truncate log file: %v", err), http.StatusInternalServerError)
		return
	}
	f.Close()

	logrus.Info("[DIAGNOSTICS] System logs cleared by request")
	_, _ = w.Write([]byte(`{"status":"success"}`))
}

type LogHistoryResponse struct {
	Logs []string `json:"logs"`
}

func handleGetLogsHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logFileMutex.Lock()
	defer logFileMutex.Unlock()

	file, err := os.Open(logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			_ = json.NewEncoder(w).Encode(LogHistoryResponse{Logs: []string{}})
		} else {
			http.Error(w, fmt.Sprintf("Failed to read log file: %v", err), http.StatusInternalServerError)
		}
		return
	}
	defer file.Close()

	var lines []string
	// Scan log history as plain text
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Cap at last 500 lines to avoid sending huge JSON responses
	if len(lines) > 500 {
		lines = lines[len(lines)-500:]
	}

	_ = json.NewEncoder(w).Encode(LogHistoryResponse{Logs: lines})
}

// AppendLogToFile writes a single log line to emulator.log
func AppendLogToFile(msg string) {
	logFileMutex.Lock()
	defer logFileMutex.Unlock()

	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = f.WriteString(msg)
}

type PcapEvent struct {
	Timestamp   string                 `json:"timestamp"`
	Protocol    string                 `json:"protocol"`
	SrcIp       string                 `json:"srcIp"`
	SrcPort     int                    `json:"srcPort"`
	DstIp       string                 `json:"dstIp"`
	DstPort     int                    `json:"dstPort"`
	SrcRole     string                 `json:"srcRole"`
	DstRole     string                 `json:"dstRole"`
	MessageName string                 `json:"messageName"`
	Summary     string                 `json:"summary"`
	Details     map[string]interface{} `json:"details"`
	RawHex      string                 `json:"rawHex"`
}

func getLifelineRole(ip string, port uint16) string {
	sbiPorts := map[uint16]string{
		7777:  "Open5GS-SBI",
		29502: "AMF-SBI",
		29518: "UDM-SBI",
		29509: "AUSF-SBI",
		29505: "SMF-SBI",
		29503: "UDR-SBI",
		29504: "NSSF-SBI",
		29507: "PCF-SBI",
		29510: "NRF-SBI",
	}
	if svc, ok := sbiPorts[port]; ok {
		return svc
	}

	if port == 38412 {
		return "AMF"
	}
	if port == 5005 {
		return "UPF"
	}
	if port == 5004 {
		return "UE (1)"
	}

	// Check ActiveGNBs
	context.ActiveGNBsMu.RLock()
	defer context.ActiveGNBsMu.RUnlock()

	// 1. Try exact port match first (control port, link port, xn/udp etc)
	for _, g := range context.ActiveGNBs {
		if g.GetGnbIp() == ip {
			if uint16(g.GetGnbPort()) == port || uint16(g.GetLinkPort()) == port || uint16(g.GetLinkPort()+1) == port {
				gnbIdStr := g.GetGnbId()
				if val, err := strconv.ParseInt(gnbIdStr, 16, 64); err == nil {
					return fmt.Sprintf("gNB (%d)", val)
				}
				return fmt.Sprintf("gNB (%s)", gnbIdStr)
			}
		}
	}

	// 2. Try matching GNB by active SCTP local ephemeral client port
	for _, g := range context.ActiveGNBs {
		n2 := g.GetN2()
		if n2 != nil {
			localAddr := n2.LocalAddr()
			if localAddr != nil {
				host, portStr, err := net.SplitHostPort(localAddr.String())
				if err == nil {
					if pVal, err := strconv.ParseUint(portStr, 10, 16); err == nil && uint16(pVal) == port {
						if host == ip || ip == "127.0.0.1" || host == "0.0.0.0" {
							gnbIdStr := g.GetGnbId()
							if val, err := strconv.ParseInt(gnbIdStr, 16, 64); err == nil {
								return fmt.Sprintf("gNB (%d)", val)
							}
							return fmt.Sprintf("gNB (%s)", gnbIdStr)
						}
					}
				}
			}
		}
	}

	// 3. Try IP match as fallback (if the IP is uniquely assigned to a gNB)
	var matchedGnb *context.GNBContext
	matchCount := 0
	for _, g := range context.ActiveGNBs {
		if g.GetGnbIp() == ip {
			matchedGnb = g
			matchCount++
		}
	}
	if matchCount == 1 && matchedGnb != nil {
		gnbIdStr := matchedGnb.GetGnbId()
		if val, err := strconv.ParseInt(gnbIdStr, 16, 64); err == nil {
			return fmt.Sprintf("gNB (%d)", val)
		}
		return fmt.Sprintf("gNB (%s)", gnbIdStr)
	}

	// 4. Check Active UEs by checking dynamic PDU session IP allocation
	for _, u := range ueContext.GetAllActiveUEs() {
		for _, sess := range u.PduSessions {
			if u.GetIp(sess.Id) == ip {
				return fmt.Sprintf("UE (%d)", u.GetUeId())
			}
		}
	}

	// 5. Look up in historical registries (for post-hoc review of closed entities)
	gnbPortHistoryMu.RLock()
	gnbRole, gnbFound := gnbPortHistory[port]
	gnbPortHistoryMu.RUnlock()
	if gnbFound {
		return gnbRole
	}

	ueIpHistoryMu.RLock()
	ueRole, ueFound := ueIpHistory[ip]
	ueIpHistoryMu.RUnlock()
	if ueFound {
		return ueRole
	}

	if strings.HasPrefix(ip, "10.200.200.") {
		ueIdStr := ip[len("10.200.200."):]
		return "UE (" + ueIdStr + ")"
	}

	// Fallback mappings based on Profiles
	if port == 9487 || port == 9488 || port == 9489 {
		return "gNB (1)"
	}
	if port == 9490 || port == 9491 || port == 9492 {
		return "gNB (3)"
	}
	if port == 9525 || port == 9526 || port == 9527 {
		return "gNB (4)"
	}
	if port == 9497 || port == 9498 || port == 9499 {
		return "gNB (2)"
	}

	return "External"
}

func parseNasHeader(nasBytes []byte) (string, string) {
	if len(nasBytes) < 3 {
		return "NAS Message", ""
	}
	secHeader := nasBytes[1] & 0x0f
	var payload []byte
	if secHeader != 0 {
		if len(nasBytes) < 7 {
			return "Secure NAS Message", ""
		}
		payload = nasBytes[7:]
	} else {
		payload = nasBytes
	}

	if len(payload) < 3 {
		return "Plain NAS Message", ""
	}

	epd := payload[0]
	msgType := payload[2]

	if epd == 0x7e { // 5GMM
		switch msgType {
		case 0x41:
			return "Registration Request", "Initial Registration"
		case 0x42:
			return "Registration Accept", "Slicing & TAI allocated"
		case 0x43:
			return "Registration Complete", "UE confirms registration accept"
		case 0x44:
			return "Registration Reject", "Registration rejected by AMF"
		case 0x4b:
			return "Authentication Request", "RAND & AUTN challenge"
		case 0x4c:
			return "Authentication Response", "RES* response token"
		case 0x5d:
			return "Security Mode Command", "Integrity & Ciphering active"
		case 0x5e:
			return "Security Mode Complete", "Security complete"
		case 0xae:
			return "Deregistration Request", "UE detach request"
		}
	} else if epd == 0x2e { // 5GSM
		switch msgType {
		case 0xc1:
			return "PDU Session Est. Request", "PDU session setup request"
		case 0xc2:
			return "PDU Session Est. Accept", "PDU IP & QoS allocated"
		case 0xc3:
			return "PDU Session Est. Reject", "PDU setup failed"
		}
	}

	return fmt.Sprintf("NAS Type 0x%02x", msgType), fmt.Sprintf("EPD: 0x%02x", epd)
}

func decodeNgapMessage(pdu *ngapType.NGAPPDU) (string, string, map[string]interface{}) {
	details := make(map[string]interface{})
	var msgName = "NGAP Message"
	var summary = ""

	if pdu == nil {
		return msgName, "Nil PDU", details
	}

	defer func() {
		if r := recover(); r != nil {
			summary = fmt.Sprintf("Decoded partially (recovered from: %v)", r)
		}
	}()

	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		initVal := pdu.InitiatingMessage
		if initVal == nil {
			return "InitiatingMessage", "Empty value", details
		}

		switch initVal.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGSetup:
			msgName = "NGSetupRequest"
			summary = "GNodeB setup request"
			if initVal.Value.NGSetupRequest != nil {
				for _, ie := range initVal.Value.NGSetupRequest.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDGlobalRANNodeID {
						details["GlobalRANNodeID"] = "Present"
					}
					if ie.Id.Value == ngapType.ProtocolIEIDRANNodeName {
						if ie.Value.RANNodeName != nil {
							details["RANNodeName"] = ie.Value.RANNodeName.Value
							summary = fmt.Sprintf("gNB Setup Request: %s", ie.Value.RANNodeName.Value)
						}
					}
				}
			}
		case ngapType.ProcedureCodeInitialUEMessage:
			msgName = "InitialUEMessage"
			summary = "Initial UE connection trigger"
			if initVal.Value.InitialUEMessage != nil {
				for _, ie := range initVal.Value.InitialUEMessage.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
						details["RANUENGAPID"] = ie.Value.RANUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDUserLocationInformation {
						details["UserLocationInformation"] = "Present"
					}
					if ie.Id.Value == ngapType.ProtocolIEIDNASPDU {
						if ie.Value.NASPDU != nil {
							nasName, nasSummary := parseNasHeader(ie.Value.NASPDU.Value)
							details["NASPDU"] = nasName
							summary = fmt.Sprintf("Initial UE Msg: %s", nasSummary)
							msgName = fmt.Sprintf("InitialUEMessage (%s)", nasName)
						}
					}
				}
			}
		case ngapType.ProcedureCodeDownlinkNASTransport:
			msgName = "DownlinkNASTransport"
			summary = "Downlink NAS message transfer"
			if initVal.Value.DownlinkNASTransport != nil {
				for _, ie := range initVal.Value.DownlinkNASTransport.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
						details["AMFUENGAPID"] = ie.Value.AMFUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
						details["RANUENGAPID"] = ie.Value.RANUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDNASPDU {
						if ie.Value.NASPDU != nil {
							nasName, nasSummary := parseNasHeader(ie.Value.NASPDU.Value)
							details["NASPDU"] = nasName
							summary = fmt.Sprintf("DL NAS: %s (%s)", nasName, nasSummary)
							msgName = fmt.Sprintf("DownlinkNASTransport (%s)", nasName)
						}
					}
				}
			}
		case ngapType.ProcedureCodeUplinkNASTransport:
			msgName = "UplinkNASTransport"
			summary = "Uplink NAS message transfer"
			if initVal.Value.UplinkNASTransport != nil {
				for _, ie := range initVal.Value.UplinkNASTransport.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
						details["AMFUENGAPID"] = ie.Value.AMFUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
						details["RANUENGAPID"] = ie.Value.RANUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDNASPDU {
						if ie.Value.NASPDU != nil {
							nasName, nasSummary := parseNasHeader(ie.Value.NASPDU.Value)
							details["NASPDU"] = nasName
							summary = fmt.Sprintf("UL NAS: %s (%s)", nasName, nasSummary)
							msgName = fmt.Sprintf("UplinkNASTransport (%s)", nasName)
						}
					}
				}
			}
		case ngapType.ProcedureCodeInitialContextSetup:
			msgName = "InitialContextSetupRequest"
			summary = "Setup UE context"
			if initVal.Value.InitialContextSetupRequest != nil {
				for _, ie := range initVal.Value.InitialContextSetupRequest.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
						details["AMFUENGAPID"] = ie.Value.AMFUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
						details["RANUENGAPID"] = ie.Value.RANUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDNASPDU {
						if ie.Value.NASPDU != nil {
							nasName, nasSummary := parseNasHeader(ie.Value.NASPDU.Value)
							details["NASPDU"] = nasName
							summary = fmt.Sprintf("Initial Context Setup: %s", nasSummary)
							msgName = fmt.Sprintf("InitialContextSetupRequest (%s)", nasName)
						}
					}
				}
			}
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			msgName = "PDUSessionResourceSetupRequest"
			summary = "PDU session setup request"
			if initVal.Value.PDUSessionResourceSetupRequest != nil {
				for _, ie := range initVal.Value.PDUSessionResourceSetupRequest.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
						details["AMFUENGAPID"] = ie.Value.AMFUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
						details["RANUENGAPID"] = ie.Value.RANUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDNASPDU {
						if ie.Value.NASPDU != nil {
							nasName, nasSummary := parseNasHeader(ie.Value.NASPDU.Value)
							details["NASPDU"] = nasName
							summary = fmt.Sprintf("PDU Session Setup: %s", nasSummary)
							msgName = fmt.Sprintf("PDUSessionResourceSetupRequest (%s)", nasName)
						}
					}
				}
			}
		case ngapType.ProcedureCodeHandoverPreparation:
			msgName = "HandoverRequired"
			summary = "Source gNB triggers N2 handover"
			if initVal.Value.HandoverRequired != nil {
				for _, ie := range initVal.Value.HandoverRequired.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
						details["AMFUENGAPID"] = ie.Value.AMFUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
						details["RANUENGAPID"] = ie.Value.RANUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDHandoverType {
						details["HandoverType"] = "Intra5GS-N2"
					}
				}
			}
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			msgName = "HandoverRequest"
			summary = "AMF requests Target gNB resource allocation"
			if initVal.Value.HandoverRequest != nil {
				for _, ie := range initVal.Value.HandoverRequest.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
						details["AMFUENGAPID"] = ie.Value.AMFUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDHandoverType {
						details["HandoverType"] = "Intra5GS-N2"
					}
				}
			}
		case ngapType.ProcedureCodeHandoverNotification:
			msgName = "HandoverNotify"
			summary = "Target gNB notifies attachment complete"
			if initVal.Value.HandoverNotify != nil {
				for _, ie := range initVal.Value.HandoverNotify.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
						details["AMFUENGAPID"] = ie.Value.AMFUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
						details["RANUENGAPID"] = ie.Value.RANUENGAPID.Value
					}
				}
			}
		case ngapType.ProcedureCodePathSwitchRequest:
			msgName = "PathSwitchRequest"
			summary = "Target gNB requests path switch"
			if initVal.Value.PathSwitchRequest != nil {
				for _, ie := range initVal.Value.PathSwitchRequest.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDSourceAMFUENGAPID {
						if ie.Value.SourceAMFUENGAPID != nil {
							details["SourceAMFUENGAPID"] = ie.Value.SourceAMFUENGAPID.Value
						}
					}
				}
			}
		case ngapType.ProcedureCodeUEContextRelease:
			msgName = "UEContextReleaseCommand"
			summary = "AMF commands source gNB to release UE context"
			if initVal.Value.UEContextReleaseCommand != nil {
				for _, ie := range initVal.Value.UEContextReleaseCommand.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDUENGAPIDs {
						if ie.Value.UENGAPIDs != nil {
							uengapids := ie.Value.UENGAPIDs
							if uengapids.Present == ngapType.UENGAPIDsPresentUENGAPIDPair && uengapids.UENGAPIDPair != nil {
								details["AMFUENGAPID"] = uengapids.UENGAPIDPair.AMFUENGAPID.Value
								details["RANUENGAPID"] = uengapids.UENGAPIDPair.RANUENGAPID.Value
							} else if uengapids.Present == ngapType.UENGAPIDsPresentAMFUENGAPID && uengapids.AMFUENGAPID != nil {
								details["AMFUENGAPID"] = uengapids.AMFUENGAPID.Value
							}
						}
					}
				}
			}
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		succVal := pdu.SuccessfulOutcome
		if succVal == nil {
			return "SuccessfulOutcome", "Empty value", details
		}
		switch succVal.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGSetup:
			msgName = "NGSetupResponse"
			summary = "AMF setup accept"
		case ngapType.ProcedureCodeInitialContextSetup:
			msgName = "InitialContextSetupResponse"
			summary = "gNB confirms UE context setup"
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			msgName = "PDUSessionResourceSetupResponse"
			summary = "gNB session setup completed"
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			msgName = "HandoverRequestAcknowledge"
			summary = "Target gNB resource allocated"
			if succVal.Value.HandoverRequestAcknowledge != nil {
				for _, ie := range succVal.Value.HandoverRequestAcknowledge.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
						details["AMFUENGAPID"] = ie.Value.AMFUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
						details["RANUENGAPID"] = ie.Value.RANUENGAPID.Value
					}
				}
			}
		case ngapType.ProcedureCodeHandoverPreparation:
			msgName = "HandoverCommand"
			summary = "AMF commands Source gNB to handover UE"
			if succVal.Value.HandoverCommand != nil {
				for _, ie := range succVal.Value.HandoverCommand.ProtocolIEs.List {
					if ie.Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
						details["AMFUENGAPID"] = ie.Value.AMFUENGAPID.Value
					}
					if ie.Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
						details["RANUENGAPID"] = ie.Value.RANUENGAPID.Value
					}
				}
			}
		case ngapType.ProcedureCodePathSwitchRequest:
			msgName = "PathSwitchRequestAcknowledge"
			summary = "AMF acknowledges path switch"
		case ngapType.ProcedureCodeUEContextRelease:
			msgName = "UEContextReleaseComplete"
			summary = "Source gNB confirms UE context release"
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		failVal := pdu.UnsuccessfulOutcome
		if failVal == nil {
			return "UnsuccessfulOutcome", "Empty value", details
		}
		summary = "Procedure failed"
		switch failVal.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGSetup:
			msgName = "NGSetupFailure"
			summary = "AMF rejected setup request"
		}
	}

	return msgName, summary, details
}

func parseSbiPayload(tcpPayload []byte) (method string, path string, statusCode string, jsonBody map[string]interface{}) {
	if len(tcpPayload) == 0 {
		return
	}
	payloadStr := string(tcpPayload)
	headerEnd := strings.Index(payloadStr, "\r\n\r\n")
	var body string
	if headerEnd != -1 {
		body = payloadStr[headerEnd+4:]
		headersPart := payloadStr[:headerEnd]
		lines := strings.Split(headersPart, "\r\n")
		if len(lines) > 0 {
			reqLine := lines[0]
			parts := strings.Split(reqLine, " ")
			if len(parts) >= 3 {
				if strings.HasPrefix(parts[2], "HTTP/") {
					method = parts[0]
					path = parts[1]
				}
			} else if len(parts) >= 2 && strings.HasPrefix(parts[0], "HTTP/") {
				statusCode = parts[1]
				if len(parts) > 2 {
					statusCode += " " + strings.Join(parts[2:], " ")
				}
			}
		}
	} else {
		body = payloadStr
	}

	// Attempt to extract JSON from body
	startIdx := strings.Index(body, "{")
	endIdx := strings.LastIndex(body, "}")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		jsonStr := body[startIdx : endIdx+1]
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			jsonBody = parsed
		}
	}
	return
}

func parsePcapEvents(r io.Reader) ([]PcapEvent, error) {
	globalHeader := make([]byte, 24)
	if _, err := io.ReadFull(r, globalHeader); err != nil {
		return nil, fmt.Errorf("failed to read global header: %w", err)
	}

	magic := binary.LittleEndian.Uint32(globalHeader[0:4])
	var isLittleEndian = true
	if magic == 0xd4c3b2a1 || magic == 0x4d3cb2a1 {
		isLittleEndian = false
	} else if magic != 0xa1b2c3d4 && magic != 0xa1b23c4d {
		return nil, fmt.Errorf("invalid pcap magic number: 0x%x", magic)
	}

	var events []PcapEvent
	headerBuf := make([]byte, 16)

	for {
		if _, err := io.ReadFull(r, headerBuf); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, err
		}

		var inclLen uint32
		var sec uint32
		var usec uint32
		if isLittleEndian {
			sec = binary.LittleEndian.Uint32(headerBuf[0:4])
			usec = binary.LittleEndian.Uint32(headerBuf[4:8])
			inclLen = binary.LittleEndian.Uint32(headerBuf[8:12])
		} else {
			sec = binary.BigEndian.Uint32(headerBuf[0:4])
			usec = binary.BigEndian.Uint32(headerBuf[4:8])
			inclLen = binary.BigEndian.Uint32(headerBuf[8:12])
		}

		packetData := make([]byte, inclLen)
		if _, err := io.ReadFull(r, packetData); err != nil {
			break
		}

		ipStart, ethType := parsePacket(packetData)
		if ipStart < 0 {
			continue
		}

		ipPayload := packetData[ipStart:]

		var srcIp, dstIp string
		var proto uint8
		var transportPayload []byte

		if ethType == 0x0800 && len(ipPayload) >= 20 {
			proto = ipPayload[9]
			srcIp = net.IP(ipPayload[12:16]).String()
			dstIp = net.IP(ipPayload[16:20]).String()

			ipHeaderLen := int(ipPayload[0]&0x0f) * 4
			if len(ipPayload) >= ipHeaderLen {
				transportPayload = ipPayload[ipHeaderLen:]
			}
		} else if ethType == 0x86dd && len(ipPayload) >= 40 {
			proto = ipPayload[6]
			srcIp = net.IP(ipPayload[8:24]).String()
			dstIp = net.IP(ipPayload[24:40]).String()
			transportPayload = ipPayload[40:]
		} else {
			continue
		}

		var srcPort, dstPort uint16
		var protocolName = "IP"
		var messageName = "Data Packet"
		var summary = ""
		var details = make(map[string]interface{})

		switch proto {
		case 1: // ICMP
			protocolName = "ICMP"
			messageName = "Ping"
			summary = "ICMP Echo Request/Reply"
		case 6: // TCP
			protocolName = "TCP"
			if len(transportPayload) >= 4 {
				srcPort = binary.BigEndian.Uint16(transportPayload[0:2])
				dstPort = binary.BigEndian.Uint16(transportPayload[2:4])
				
				// Calculate TCP header length to find payload start
				if len(transportPayload) >= 13 {
					tcpHeaderLen := int((transportPayload[12] >> 4) & 0x0f) * 4
					if len(transportPayload) >= tcpHeaderLen {
						tcpPayload := transportPayload[tcpHeaderLen:]
						if len(tcpPayload) > 0 {
							payloadStr := string(tcpPayload)
							// Check for HTTP/1.x signatures
							if strings.HasPrefix(payloadStr, "GET ") ||
								strings.HasPrefix(payloadStr, "POST ") ||
								strings.HasPrefix(payloadStr, "PUT ") ||
								strings.HasPrefix(payloadStr, "DELETE ") ||
								strings.HasPrefix(payloadStr, "PATCH ") {
								protocolName = "HTTP"
								firstLine := payloadStr
								if idx := strings.Index(payloadStr, "\r\n"); idx != -1 {
									firstLine = payloadStr[:idx]
								}
								messageName = firstLine
								summary = "HTTP SBI Request"
							} else if strings.HasPrefix(payloadStr, "HTTP/1.") {
								protocolName = "HTTP"
								firstLine := payloadStr
								if idx := strings.Index(payloadStr, "\r\n"); idx != -1 {
									firstLine = payloadStr[:idx]
								}
								messageName = firstLine
								summary = "HTTP SBI Response"
							} else if strings.HasPrefix(payloadStr, "PRI * HTTP/2.0") {
								protocolName = "HTTP"
								messageName = "HTTP/2 Connection Preface"
								summary = "HTTP/2 SBI connection init"
							}
						}
					}
				}
				
				// Fallback to port checking for 5G SBI control plane
				if protocolName == "TCP" {
					sbiPorts := map[uint16]string{
						7777:  "Open5GS SBI",
						29502: "AMF SBI",
						29518: "UDM SBI",
						29509: "AUSF SBI",
						29505: "SMF SBI",
						29503: "UDR SBI",
						29504: "NSSF SBI",
						29507: "PCF SBI",
						29510: "NRF SBI",
					}
					if svc, ok := sbiPorts[srcPort]; ok {
						protocolName = "HTTP"
						messageName = fmt.Sprintf("SBI Message (%s)", svc)
						summary = fmt.Sprintf("SBI outbound on port %d", srcPort)
					} else if svc, ok := sbiPorts[dstPort]; ok {
						protocolName = "HTTP"
						messageName = fmt.Sprintf("SBI Message (%s)", svc)
						summary = fmt.Sprintf("SBI inbound on port %d", dstPort)
					}
				}

				// Extract HTTP/SBI payloads
				if protocolName == "HTTP" && len(transportPayload) >= 13 {
					tcpHeaderLen := int((transportPayload[12] >> 4) & 0x0f) * 4
					if len(transportPayload) >= tcpHeaderLen {
						tcpPayload := transportPayload[tcpHeaderLen:]
						method, path, statusCode, jsonBody := parseSbiPayload(tcpPayload)
						if method != "" {
							details["method"] = method
							details["path"] = path
							messageName = fmt.Sprintf("%s %s", method, path)
						}
						if statusCode != "" {
							details["statusCode"] = statusCode
							messageName = fmt.Sprintf("HTTP %s", statusCode)
						}
						if jsonBody != nil {
							details["payload"] = jsonBody
						}
					}
				}
			}
			if protocolName == "TCP" {
				messageName = "TCP Packet"
				summary = fmt.Sprintf("TCP communication: port %d -> %d", srcPort, dstPort)
			}
		case 17: // UDP
			protocolName = "UDP"
			if len(transportPayload) >= 8 {
				srcPort = binary.BigEndian.Uint16(transportPayload[0:2])
				dstPort = binary.BigEndian.Uint16(transportPayload[2:4])
				udpPayload := transportPayload[8:]

				if srcPort == 5004 || dstPort == 5004 || srcPort == 5005 || dstPort == 5005 {
					protocolName = "RTP"
					messageName = "VoNR Audio"
					summary = "Bidirectional voice RTP UDP stream"
				} else if len(udpPayload) >= 3 && udpPayload[0] == 0x58 && udpPayload[1] == 0x4e {
					protocolName = "XnAP"
					xnType := udpPayload[2]
					switch xnType {
					case 0x01:
						messageName = "XN HANDOVER REQUEST"
						summary = "Source gNB requests handover to Target gNB"
						if len(udpPayload) >= 11 {
							details["AMFUENGAPID"] = int64(binary.BigEndian.Uint64(udpPayload[3:11]))
						}
					case 0x02:
						messageName = "XN HANDOVER REQUEST ACKNOWLEDGE"
						summary = "Target gNB resource allocated"
					case 0x03:
						messageName = "XN UE CONTEXT RELEASE"
						summary = "Target gNB releases context"
						if len(udpPayload) >= 11 {
							details["AMFUENGAPID"] = int64(binary.BigEndian.Uint64(udpPayload[3:11]))
						}
					case 0x04:
						messageName = "XN SN STATUS TRANSFER"
						summary = "Source gNB transfers sequence numbers"
					}
				} else if len(udpPayload) >= 4 && udpPayload[0] == 0x52 && udpPayload[1] == 0x52 && udpPayload[2] == 0x43 {
					protocolName = "RRC"
					rrcType := udpPayload[3]
					switch rrcType {
					case 0x01:
						messageName = "RRCSetupRequest"
						summary = "UE requests RRC connection setup"
					case 0x02:
						messageName = "RRCSetup"
						summary = "gNB sends RRC Setup"
					case 0x03:
						messageName = "RRCSetupComplete"
						summary = "UE RRC connection setup complete"
					case 0x05:
						messageName = "RRCReconfiguration"
						summary = "gNB RRC reconfiguration (PDU session establishment)"
					case 0x06:
						messageName = "RRCReconfigurationComplete"
						summary = "UE RRC reconfiguration complete"
					case 0x07:
						messageName = "MeasurementReport"
						summary = "UE sends radio link measurements to source gNB"
					case 0x08:
						messageName = "RRCReconfiguration (Handover Command)"
						summary = "Source gNB commands handover"
					case 0x09:
						messageName = "RRCReconfigurationComplete (Handover)"
						summary = "UE arrives at target gNB cell"
					}
				}
			}
		case 132: // SCTP
			protocolName = "SCTP"
			if len(transportPayload) >= 12 {
				srcPort = binary.BigEndian.Uint16(transportPayload[0:2])
				dstPort = binary.BigEndian.Uint16(transportPayload[2:4])

				chunks := transportPayload[12:]
				for len(chunks) >= 4 {
					chunkType := chunks[0]
					chunkLen := binary.BigEndian.Uint16(chunks[2:4])
					if chunkLen < 4 {
						break
					}

					if chunkType == 0 { // DATA Chunk
						if len(chunks) >= 16 {
							ppid := binary.BigEndian.Uint32(chunks[12:16])
							if ppid == 60 { // PPID NGAP
								protocolName = "NGAP"
								ngapPayload := chunks[16:chunkLen]

								pdu, err := ngap.Decoder(ngapPayload)
								if err == nil {
									messageName, summary, details = decodeNgapMessage(pdu)
								} else {
									messageName = "NGAP Decode Error"
									summary = fmt.Sprintf("Error: %v", err)
								}
							}
						}
					}

					alignedLen := (chunkLen + 3) &^ 3
					if int(alignedLen) > len(chunks) {
						break
					}
					chunks = chunks[alignedLen:]
				}
			}
		}

		timestamp := time.Unix(int64(sec), int64(usec)*1000).Format(time.RFC3339Nano)
		events = append(events, PcapEvent{
			Timestamp:   timestamp,
			Protocol:    protocolName,
			SrcIp:       srcIp,
			SrcPort:     int(srcPort),
			DstIp:       dstIp,
			DstPort:     int(dstPort),
			SrcRole:     getLifelineRole(srcIp, srcPort),
			DstRole:     getLifelineRole(dstIp, dstPort),
			MessageName: messageName,
			Summary:     summary,
			Details:     details,
			RawHex:      hex.EncodeToString(packetData),
		})
	}

	var filtered []PcapEvent
	for _, ev := range events {
		if ev.Protocol == "NGAP" || ev.Protocol == "HTTP" || ev.Protocol == "XnAP" || ev.Protocol == "RRC" {
			filtered = append(filtered, ev)
		}
	}
	return filtered, nil
}

func parseLogEvents(r io.Reader) []PcapEvent {
	extractUeRole := func(msg string) string {
		for _, tag := range []string{"UE ID ", "UE ", "AMF UE ID: ", "ranUeId: ", "RAN UE ID: "} {
			if idx := strings.Index(msg, tag); idx != -1 {
				sub := msg[idx+len(tag):]
				var digits []rune
				for _, r := range sub {
					if r >= '0' && r <= '9' {
						digits = append(digits, r)
					} else {
						break
					}
				}
				if len(digits) > 0 {
					return fmt.Sprintf("UE (%s)", string(digits))
				}
			}
		}
		return "UE (101)"
	}

	extractGnbRole := func(msg string, defaultRole string) string {
		if strings.Contains(msg, "gNB-West") {
			return "gNB (2)"
		}
		if strings.Contains(msg, "gNB-East") {
			return "gNB (4)"
		}
		if strings.Contains(msg, "gNB-Default") {
			return "gNB (1)"
		}

		for _, tag := range []string{"gNB-ID: ", "GNB-ID: ", "gNB-ID ", "GNB-ID ", "GNB[ID:", "gNB-FLEET] ", "gNB ", "GNB "} {
			if idx := strings.Index(msg, tag); idx != -1 {
				sub := msg[idx+len(tag):]
				var digits []rune
				for _, r := range sub {
					if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
						digits = append(digits, r)
					} else {
						break
					}
				}
				if len(digits) > 0 {
					val, err := strconv.ParseInt(string(digits), 16, 64)
					if err == nil {
						return fmt.Sprintf("gNB (%d)", val)
					}
				}
			}
		}

		// Search for any 6-digit hex/decimal GNB ID prefix like "000001" or "000002"
		for i := 0; i <= len(msg)-6; i++ {
			sub := msg[i : i+6]
			if strings.HasPrefix(sub, "000") {
				isHex := true
				for _, r := range sub {
					if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
						isHex = false
						break
					}
				}
				if isHex {
					val, err := strconv.ParseInt(sub, 16, 64)
					if err == nil {
						return fmt.Sprintf("gNB (%d)", val)
					}
				}
			}
		}

		if defaultRole == "gNB-Source" {
			return "gNB (2)"
		}
		if defaultRole == "gNB-Target" {
			return "gNB (4)"
		}
		return defaultRole
	}

	var events []PcapEvent
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		timestamp := time.Now().Format(time.RFC3339)
		if idx := strings.Index(line, "time=\""); idx != -1 {
			tsPart := line[idx+6:]
			if endIdx := strings.Index(tsPart, "\""); endIdx != -1 {
				timestamp = tsPart[:endIdx]
			}
		}

		msg := ""
		if idx := strings.Index(line, "msg=\""); idx != -1 {
			msgPart := line[idx+5:]
			if endIdx := strings.Index(msgPart, "\""); endIdx != -1 {
				msg = msgPart[:endIdx]
			}
		} else {
			msg = line
		}

		if msg == "" {
			continue
		}

		var proto = "Logs"
		var msgName = ""
		var srcRole = "External"
		var dstRole = "External"
		var details = make(map[string]interface{})
		details["logLine"] = msg

		if strings.Contains(msg, "NG Setup Request sent") || strings.Contains(msg, "Send NG Setup Request") {
			proto = "NGAP"
			msgName = "NGSetupRequest"
			srcRole = "gNB-Source"
			dstRole = "AMF"
		} else if strings.Contains(msg, "Receive Ng Setup Response") || strings.Contains(msg, "NG Setup Response") {
			proto = "NGAP"
			msgName = "NGSetupResponse"
			srcRole = "AMF"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "Registration Request") {
			proto = "NGAP"
			msgName = "InitialUEMessage (Registration Request)"
			srcRole = "gNB-Source"
			dstRole = "AMF"
		} else if strings.Contains(msg, "Registration Accept") {
			proto = "NGAP"
			msgName = "InitialContextSetupRequest (Registration Accept)"
			srcRole = "AMF"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "Registration Complete") {
			proto = "NGAP"
			msgName = "UplinkNASTransport (Registration Complete)"
			srcRole = "gNB-Source"
			dstRole = "AMF"
		} else if strings.Contains(msg, "Authentication Request") {
			proto = "NGAP"
			msgName = "DownlinkNASTransport (Authentication Request)"
			srcRole = "AMF"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "Authentication Response") {
			proto = "NGAP"
			msgName = "UplinkNASTransport (Authentication Response)"
			srcRole = "gNB-Source"
			dstRole = "AMF"
		} else if strings.Contains(msg, "Security Mode Command") {
			proto = "NGAP"
			msgName = "DownlinkNASTransport (Security Mode Command)"
			srcRole = "AMF"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "Security Mode Complete") {
			proto = "NGAP"
			msgName = "UplinkNASTransport (Security Mode Complete)"
			srcRole = "gNB-Source"
			dstRole = "AMF"
		} else if strings.Contains(msg, "PDU Session Establishment Request") || strings.Contains(msg, "PDU Session Est. Request") {
			proto = "NGAP"
			msgName = "UplinkNASTransport (PDU Session Est. Request)"
			srcRole = "gNB-Source"
			dstRole = "AMF"
		} else if strings.Contains(msg, "PDU Session Establishment Accept") || strings.Contains(msg, "PDU Session Est. Accept") {
			proto = "NGAP"
			msgName = "PDUSessionResourceSetupRequest (PDU Session Est. Accept)"
			srcRole = "AMF"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "initial UE message") || strings.Contains(msg, "Initial UE Message") {
			proto = "NGAP"
			msgName = "InitialUEMessage"
			srcRole = "gNB-Source"
			dstRole = "AMF"
		} else if strings.Contains(msg, "Downlink NAS Transport") || strings.Contains(msg, "DL NAS") {
			proto = "NGAP"
			msgName = "DownlinkNASTransport"
			srcRole = "AMF"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "Uplink Nas Transport") || strings.Contains(msg, "UL NAS") {
			proto = "NGAP"
			msgName = "UplinkNASTransport"
			srcRole = "gNB-Source"
			dstRole = "AMF"
		} else if strings.Contains(msg, "Handover Required sent") || strings.Contains(msg, "Error sending Handover Required") {
			proto = "NGAP"
			msgName = "HandoverRequired"
			srcRole = "gNB-Source"
			dstRole = "AMF"
		} else if strings.Contains(msg, "Received HANDOVER REQUEST from AMF") {
			proto = "NGAP"
			msgName = "HandoverRequest"
			srcRole = "AMF"
			dstRole = "gNB-Target"
		} else if strings.Contains(msg, "Handover Request Acknowledge sent") {
			proto = "NGAP"
			msgName = "HandoverRequestAcknowledge"
			srcRole = "gNB-Target"
			dstRole = "AMF"
		} else if strings.Contains(msg, "Handover Command received") {
			proto = "NGAP"
			msgName = "HandoverCommand"
			srcRole = "AMF"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "Handover Notify sent") {
			proto = "NGAP"
			msgName = "HandoverNotify"
			srcRole = "gNB-Target"
			dstRole = "AMF"
		} else if strings.Contains(msg, "Path Switch Request sent") {
			proto = "NGAP"
			msgName = "PathSwitchRequest"
			srcRole = "gNB-Target"
			dstRole = "AMF"
		} else if strings.Contains(msg, "Receive Path Switch Request Acknowledge") {
			proto = "NGAP"
			msgName = "PathSwitchRequestAcknowledge"
			srcRole = "AMF"
			dstRole = "gNB-Target"
		} else if strings.Contains(msg, "Received Handover Command from Source GNodeB") {
			proto = "RRC"
			msgName = "RRCReconfiguration (Handover)"
			srcRole = "gNB-Source"
			dstRole = "UE"
		} else if strings.Contains(msg, "Cell switch completed") || strings.Contains(msg, "UE accessing target cell") {
			proto = "RRC"
			msgName = "RRCReconfigurationComplete"
			srcRole = "UE"
			dstRole = "gNB-Target"
		} else if strings.Contains(msg, "Initiating Handover to Target GNodeB") {
			proto = "RRC"
			msgName = "HandoverTrigger"
			srcRole = "UE"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "Received XN HANDOVER REQUEST") {
			proto = "XnAP"
			msgName = "XN HANDOVER REQUEST"
			srcRole = "gNB-Source"
			dstRole = "gNB-Target"
		} else if strings.Contains(msg, "Sent XN HANDOVER REQUEST ACKNOWLEDGE") || strings.Contains(msg, "Sending XN HANDOVER REQUEST ACKNOWLEDGE") {
			proto = "XnAP"
			msgName = "XN HANDOVER REQUEST ACKNOWLEDGE"
			srcRole = "gNB-Target"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "Sending XN UE CONTEXT RELEASE") || strings.Contains(msg, "Received XN UE CONTEXT RELEASE") {
			proto = "XnAP"
			msgName = "XN UE CONTEXT RELEASE"
			srcRole = "gNB-Target"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "UE Context Release Command received") {
			proto = "NGAP"
			msgName = "UEContextReleaseCommand"
			srcRole = "AMF"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "UE Context Release Complete sent") {
			proto = "NGAP"
			msgName = "UEContextReleaseComplete"
			srcRole = "gNB-Source"
			dstRole = "AMF"
		} else if strings.Contains(msg, "Handover Command trigger sent to UE") {
			proto = "RRC"
			msgName = "RRCReconfiguration (Handover)"
			srcRole = "gNB-Source"
			dstRole = "UE"
		} else if strings.Contains(msg, "Processing N2 Handover Trigger") {
			proto = "RRC"
			msgName = "HandoverTrigger"
			srcRole = "UE"
			dstRole = "gNB-Source"
		} else if strings.Contains(msg, "Ping") || strings.Contains(msg, "ping") {
			proto = "ICMP"
			msgName = "Ping"
			srcRole = "UE"
			dstRole = "UPF"
		} else if strings.Contains(msg, "Voice Echo") || strings.Contains(msg, "VoNR") {
			proto = "RTP"
			msgName = "VoNR Audio"
			srcRole = "UE"
			dstRole = "UPF"
		}

		if msgName != "" {
			finalSrc := srcRole
			finalDst := dstRole

			if srcRole == "UE" {
				finalSrc = extractUeRole(msg)
			} else if strings.HasPrefix(srcRole, "gNB-") {
				finalSrc = extractGnbRole(msg, srcRole)
			}

			if dstRole == "UE" {
				finalDst = extractUeRole(msg)
			} else if strings.HasPrefix(dstRole, "gNB-") {
				finalDst = extractGnbRole(msg, dstRole)
			}

			events = append(events, PcapEvent{
				Timestamp:   timestamp,
				Protocol:    proto,
				SrcIp:       "Logs",
				DstIp:       "Logs",
				SrcRole:     finalSrc,
				DstRole:     finalDst,
				MessageName: msgName,
				Summary:     msg,
				Details:     details,
			})
		}
	}

	var filtered []PcapEvent
	for _, ev := range events {
		if ev.Protocol == "NGAP" || ev.Protocol == "HTTP" || ev.Protocol == "XnAP" || ev.Protocol == "RRC" {
			filtered = append(filtered, ev)
		}
	}
	return filtered
}

func handleParsePcap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		http.Error(w, "Missing 'file' parameter", http.StatusBadRequest)
		return
	}

	fileName = filepath.Base(fileName)
	filePath := filepath.Join(capturesDir, fileName)

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusNotFound)
		return
	}
	defer file.Close()

	events, err := parsePcapEvents(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse PCAP: %v", err), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(events)
}

func handleParseLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logFileMutex.Lock()
	file, err := os.Open(logFilePath)
	if err != nil {
		logFileMutex.Unlock()
		if os.IsNotExist(err) {
			_ = json.NewEncoder(w).Encode([]PcapEvent{})
		} else {
			http.Error(w, fmt.Sprintf("Failed to open log file: %v", err), http.StatusInternalServerError)
		}
		return
	}
	defer file.Close()
	logFileMutex.Unlock()

	events := parseLogEvents(file)
	_ = json.NewEncoder(w).Encode(events)
}

