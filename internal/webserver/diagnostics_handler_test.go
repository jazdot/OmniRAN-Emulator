package webserver

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestParsePacket(t *testing.T) {
	// 1. Mock IPv4 Ethernet Packet
	// 14 bytes Ethernet header + 20 bytes IPv4 header
	ethIpv4 := make([]byte, 14+20)
	ethIpv4[12] = 0x08; ethIpv4[13] = 0x00 // EtherType IPv4
	ethIpv4[14] = 0x45                     // Version 4, IHL 5
	ethIpv4[14+9] = 132                    // Protocol SCTP (132)

	ipStart, ethType := parsePacket(ethIpv4)
	if ipStart != 14 || ethType != 0x0800 {
		t.Errorf("Expected ipStart=14, ethType=0x0800; got ipStart=%d, ethType=0x%04x", ipStart, ethType)
	}
	proto := getIpProtocol(ethIpv4[ipStart:], ethType)
	if proto != 132 {
		t.Errorf("Expected proto=132; got %d", proto)
	}

	// 2. Mock IPv6 Ethernet Packet
	ethIpv6 := make([]byte, 14+40)
	ethIpv6[12] = 0x86; ethIpv6[13] = 0xdd // EtherType IPv6
	ethIpv6[14] = 0x60                     // Version 6
	ethIpv6[14+6] = 6                      // Next Header TCP (6)

	ipStart, ethType = parsePacket(ethIpv6)
	if ipStart != 14 || ethType != 0x86dd {
		t.Errorf("Expected ipStart=14, ethType=0x86dd; got ipStart=%d, ethType=0x%04x", ipStart, ethType)
	}
	proto = getIpProtocol(ethIpv6[ipStart:], ethType)
	if proto != 6 {
		t.Errorf("Expected proto=6; got %d", proto)
	}

	// 3. Mock IPv4 Linux Cooked Capture (SLL) Packet
	// 16 bytes SLL header + 20 bytes IPv4 header
	sllIpv4 := make([]byte, 16+20)
	sllIpv4[14] = 0x08; sllIpv4[15] = 0x00 // Protocol type IPv4
	sllIpv4[16] = 0x45                     // Version 4
	sllIpv4[16+9] = 17                     // Protocol UDP (17)

	ipStart, ethType = parsePacket(sllIpv4)
	if ipStart != 16 || ethType != 0x0800 {
		t.Errorf("Expected ipStart=16, ethType=0x0800; got ipStart=%d, ethType=0x%04x", ipStart, ethType)
	}
	proto = getIpProtocol(sllIpv4[ipStart:], ethType)
	if proto != 17 {
		t.Errorf("Expected proto=17; got %d", proto)
	}

	// 4. Mock IPv6 Linux Cooked Capture (SLL) Packet
	sllIpv6 := make([]byte, 16+40)
	sllIpv6[14] = 0x86; sllIpv6[15] = 0xdd // Protocol type IPv6
	sllIpv6[16] = 0x60                     // Version 6
	sllIpv6[16+6] = 132                    // Next Header SCTP (132)

	ipStart, ethType = parsePacket(sllIpv6)
	if ipStart != 16 || ethType != 0x86dd {
		t.Errorf("Expected ipStart=16, ethType=0x86dd; got ipStart=%d, ethType=0x%04x", ipStart, ethType)
	}
	proto = getIpProtocol(sllIpv6[ipStart:], ethType)
	if proto != 132 {
		t.Errorf("Expected proto=132; got %d", proto)
	}

	// 5. Mock Raw IPv4 Packet
	rawIpv4 := make([]byte, 20)
	rawIpv4[0] = 0x45    // Version 4
	rawIpv4[9] = 1       // Protocol ICMP (1)

	ipStart, ethType = parsePacket(rawIpv4)
	if ipStart != 0 || ethType != 0x0800 {
		t.Errorf("Expected ipStart=0, ethType=0x0800; got ipStart=%d, ethType=0x%04x", ipStart, ethType)
	}
	proto = getIpProtocol(rawIpv4[ipStart:], ethType)
	if proto != 1 {
		t.Errorf("Expected proto=1; got %d", proto)
	}
}

func TestWrapInEthernet(t *testing.T) {
	ipPayload := []byte{0x45, 0x00, 0x00, 0x14, 0x00, 0x00, 0x00, 0x00, 0x40, 0x84, 0x00, 0x00, 0x7f, 0x00, 0x00, 0x01, 0x7f, 0x00, 0x00, 0x01} // 20 bytes
	wrapped := wrapInEthernet(ipPayload, 0x0800)

	if len(wrapped) != 14+len(ipPayload) {
		t.Errorf("Expected wrapped length %d; got %d", 14+len(ipPayload), len(wrapped))
	}
	etherType := binary.BigEndian.Uint16(wrapped[12:14])
	if etherType != 0x0800 {
		t.Errorf("Expected EtherType 0x0800; got 0x%04x", etherType)
	}
	if wrapped[0] != 0x00 || wrapped[5] != 0x02 {
		t.Errorf("Expected Dst MAC to end with 0x02; got dst MAC %v", wrapped[0:6])
	}
	if wrapped[6] != 0x00 || wrapped[11] != 0x01 {
		t.Errorf("Expected Src MAC to end with 0x01; got src MAC %v", wrapped[6:12])
	}
}

func TestPcapLogParsers(t *testing.T) {
	// Test getLifelineRole
	if role := getLifelineRole("127.0.0.1", 38412); role != "AMF" {
		t.Errorf("Expected role 'AMF'; got '%s'", role)
	}
	if role := getLifelineRole("127.0.0.1", 5005); role != "UPF" {
		t.Errorf("Expected role 'UPF'; got '%s'", role)
	}
	if role := getLifelineRole("127.0.0.1", 5004); role != "UE" {
		t.Errorf("Expected role 'UE'; got '%s'", role)
	}
	if role := getLifelineRole("127.0.0.1", 9487); role != "gNB-Source" {
		t.Errorf("Expected role 'gNB-Source'; got '%s'", role)
	}
	if role := getLifelineRole("127.0.0.1", 9497); role != "gNB-Target" {
		t.Errorf("Expected role 'gNB-Target'; got '%s'", role)
	}
	if role := getLifelineRole("1.2.3.4", 12345); role != "External" {
		t.Errorf("Expected role 'External'; got '%s'", role)
	}

	// Test parseNasHeader
	// Plain Registration Request (5GMM) -> epd = 0x7e, msgType = 0x41
	nasReq := []byte{0x7e, 0x00, 0x41, 0x01, 0x02}
	name, _ := parseNasHeader(nasReq)
	if name != "Registration Request" {
		t.Errorf("Expected 'Registration Request'; got '%s'", name)
	}

	// Plain PDU Session Est. Request (5GSM) -> epd = 0x2e, msgType = 0xc1
	nasPduReq := []byte{0x2e, 0x00, 0xc1, 0x01, 0x02}
	name, _ = parseNasHeader(nasPduReq)
	if name != "PDU Session Est. Request" {
		t.Errorf("Expected 'PDU Session Est. Request'; got '%s'", name)
	}

	// Test decodeNgapMessage with nil input
	name, summary, _ := decodeNgapMessage(nil)
	if name != "NGAP Message" || summary != "Nil PDU" {
		t.Errorf("Expected decode of nil PDU to return 'Nil PDU' summary; got name='%s', summary='%s'", name, summary)
	}

	// Test parseLogEvents
	logData := `time="2026-06-21T11:00:00Z" level=info msg="Send NG Setup Request to AMF"
time="2026-06-21T11:00:01Z" level=info msg="Received Handover Command from Source GNodeB"
time="2026-06-21T11:00:02Z" level=info msg="Cell switch completed"
time="2026-06-21T11:00:03Z" level=info msg="Ping response received"`

	events := parseLogEvents(strings.NewReader(logData))
	if len(events) != 4 {
		t.Fatalf("Expected 4 events from log parsing; got %d", len(events))
	}

	if events[0].MessageName != "NGSetupRequest" || events[0].SrcRole != "gNB-Source" || events[0].DstRole != "AMF" {
		t.Errorf("Event 0 parsed incorrectly: %+v", events[0])
	}
	if events[1].MessageName != "RRCReconfiguration (Handover)" || events[1].SrcRole != "gNB-Source" || events[1].DstRole != "UE" {
		t.Errorf("Event 1 parsed incorrectly: %+v", events[1])
	}
	if events[2].MessageName != "RRCReconfigurationComplete" || events[2].SrcRole != "UE" || events[2].DstRole != "gNB-Target" {
		t.Errorf("Event 2 parsed incorrectly: %+v", events[2])
	}
	if events[3].MessageName != "Ping" || events[3].SrcRole != "UE" || events[3].DstRole != "UPF" {
		t.Errorf("Event 3 parsed incorrectly: %+v", events[3])
	}
}
