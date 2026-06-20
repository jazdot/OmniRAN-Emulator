package webserver

import (
	"encoding/binary"
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
	"syscall"
	"time"

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
)

func init() {
	_ = os.MkdirAll(capturesDir, 0755)
	_ = os.MkdirAll("log", 0755)
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

func wrapInEthernet(data []byte) []byte {
	// If it starts directly with IPv4 (0x45) or IPv6 (0x60), prepend a dummy Ethernet header.
	// This makes it instantly parsable in Wireshark as standard Ethernet link-type.
	if len(data) >= 20 && (data[0] == 0x45 || (data[0]&0xf0 == 0x60)) {
		eth := make([]byte, 14+len(data))
		// Dst MAC: 00:00:00:00:00:02
		eth[0] = 0x00; eth[1] = 0x00; eth[2] = 0x00; eth[3] = 0x00; eth[4] = 0x00; eth[5] = 0x02
		// Src MAC: 00:00:00:00:00:01
		eth[6] = 0x00; eth[7] = 0x00; eth[8] = 0x00; eth[9] = 0x00; eth[10] = 0x00; eth[11] = 0x01
		
		if data[0] == 0x45 {
			eth[12] = 0x08; eth[13] = 0x00 // IPv4 type
		} else {
			eth[12] = 0x86; eth[13] = 0xdd // IPv6 type
		}
		copy(eth[14:], data)
		return eth
	}
	return data
}

func getIpProtocol(data []byte, isRawIp bool) uint8 {
	var ipStart = 0
	if !isRawIp {
		if len(data) < 14 {
			return 0
		}
		etherType := binary.BigEndian.Uint16(data[12:14])
		if etherType != 0x0800 && etherType != 0x86dd {
			return 0
		}
		ipStart = 14
	}

	if len(data) < ipStart+20 {
		return 0
	}

	version := data[ipStart] >> 4
	if version == 4 {
		return data[ipStart+9] // IPv4 Protocol
	} else if version == 6 {
		return data[ipStart+6] // IPv6 Next Header
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

			// Extract IP protocol and filter
			proto := getIpProtocol(packet, isRawIp)
			if !matchesProtocol(proto, filter) {
				continue
			}

			// Wrap IP packet in dummy Ethernet if interface is raw IP
			var writeData []byte
			if isRawIp {
				writeData = wrapInEthernet(packet)
			} else {
				writeData = packet
			}

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
	scanner := bufioNewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Cap at last 500 lines to avoid sending huge JSON responses
	if len(lines) > 500 {
		lines = lines[len(lines)-500:]
	}

	_ = json.NewEncoder(w).Encode(LogHistoryResponse{Logs: lines})
}

// A simple dummy structure because bufio was not imported, let's make sure we import or implement scanner
type bufioScanner struct {
	r io.Reader
	b []byte
	h int
	t int
}

func bufioNewScanner(r io.Reader) *bufioScanner {
	return &bufioScanner{r: r, b: make([]byte, 4096)}
}

func (s *bufioScanner) Scan() bool {
	s.h = s.t
	for {
		// Look for new line
		for i := s.h; i < s.t; i++ {
			if s.b[i] == '\n' {
				s.t = i + 1
				return true
			}
		}
		// Shift buffer
		if s.h > 0 {
			copy(s.b, s.b[s.h:s.t])
			s.t -= s.h
			s.h = 0
		}
		// Read more
		if s.t == len(s.b) {
			nb := make([]byte, len(s.b)*2)
			copy(nb, s.b[:s.t])
			s.b = nb
		}
		n, err := s.r.Read(s.b[s.t:])
		if n > 0 {
			s.t += n
		}
		if err != nil {
			if s.t > s.h {
				// Return last line if not ending with \n
				s.b = append(s.b[:s.t], '\n')
				s.t++
				continue
			}
			return false
		}
	}
}

func (s *bufioScanner) Text() string {
	end := s.t - 1
	if end > s.h && s.b[end] == '\n' {
		if end-1 >= s.h && s.b[end-1] == '\r' {
			end--
		}
	}
	return string(s.b[s.h:end])
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
