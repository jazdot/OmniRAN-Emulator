package context

import (
	"testing"
)

func TestGlobalRegistryAndFindUe(t *testing.T) {
	// Clear registry
	ActiveGNBsMu.Lock()
	ActiveGNBs = make(map[string]*GNBContext)
	ActiveGNBsMu.Unlock()

	// 1. Create Source GNodeB
	gnb1 := &GNBContext{}
	gnb1.NewRanGnbContext("000001", "999", "70", "0001", "01", "010203", "127.0.0.1", "127.0.0.2", 9487, 2152)
	amf1 := gnb1.NewGnBAmf("127.0.0.1", 38412)
	amf1.SetStateActive()

	// Create Target GNodeB
	gnb2 := &GNBContext{}
	gnb2.NewRanGnbContext("000002", "999", "70", "0002", "01", "010203", "127.0.0.1", "127.0.0.2", 9497, 2162)
	amf2 := gnb2.NewGnBAmf("127.0.0.1", 38412)
	amf2.SetStateActive()

	// Check they are in registry
	ActiveGNBsMu.RLock()
	if len(ActiveGNBs) != 2 {
		t.Errorf("Expected 2 registered GNodeBs, got %d", len(ActiveGNBs))
	}
	ActiveGNBsMu.RUnlock()

	// Add UE to gnb1
	ue1 := gnb1.NewGnBUe(nil)
	ue1.SetAmfUeId(42)

	// Lookup UE from the perspective of gnb2 (exclude gnb2)
	foundGnb, foundUe := FindUeInOtherGnb(gnb2.GetGnbId(), 42)
	if foundGnb == nil || foundUe == nil {
		t.Errorf("Expected to find UE 42 in Source GNodeB")
	} else if foundGnb.GetGnbId() != "000001" {
		t.Errorf("Expected UE to be in GNodeB 000001, got %s", foundGnb.GetGnbId())
	}

	// Terminate gnb1
	gnb1.Terminate()

	// Check it is deleted from registry
	ActiveGNBsMu.RLock()
	if len(ActiveGNBs) != 1 {
		t.Errorf("Expected 1 registered GNodeB after termination, got %d", len(ActiveGNBs))
	}
	ActiveGNBsMu.RUnlock()
}
