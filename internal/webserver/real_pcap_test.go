package webserver

import (
	"os"
	"testing"
)

func TestParseRealPcaps(t *testing.T) {
	files := []string{
		"../../log/captures/reg_cap.pcap",
		"../../log/captures/r18_cap.pcap",
	}

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			t.Logf("Could not open file %s: %v", file, err)
			continue
		}
		events, err := parsePcapEvents(f)
		f.Close()
		if err != nil {
			t.Errorf("Error parsing %s: %v", file, err)
			continue
		}

		t.Logf("File: %s - Extracted %d events:", file, len(events))
		ngapCount := 0
		for idx, ev := range events {
			if ev.Protocol == "NGAP" {
				ngapCount++
			}
			t.Logf("  [%d] Time: %s, Proto: %s, Msg: %s, Src: %s -> Dst: %s, Summary: %s",
				idx, ev.Timestamp, ev.Protocol, ev.MessageName, ev.SrcRole, ev.DstRole, ev.Summary)
		}

		if ngapCount == 0 {
			t.Errorf("Expected at least one NGAP event in %s, got 0", file)
		}
	}
}
