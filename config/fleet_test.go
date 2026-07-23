package config

import (
	"os"
	"testing"
)

func TestFleetConfigLibraryProfiles(t *testing.T) {
	// Create config dir if running within config package test dir
	_ = os.MkdirAll("config", 0755)
	defer os.RemoveAll("config/fleet.json")

	if err := LoadFleet(); err != nil {
		t.Fatalf("LoadFleet failed: %v", err)
	}

	// 1. Add AMF Profile
	amf := AMFProfile{
		Name:        "Test-AMF-Core",
		Ip:          "192.168.1.100",
		Port:        38412,
		Mcc:         "999",
		Mnc:         "70",
		Description: "Test Core Endpoint",
	}
	if err := UpsertAMFProfile(amf); err != nil {
		t.Fatalf("UpsertAMFProfile failed: %v", err)
	}

	// 2. Add Slice Profile
	slice := SliceProfile{
		Name:        "Test-Slice-eMBB",
		Sst:         1,
		Sd:          "010203",
		Category:    "eMBB",
		Description: "High Throughput Test Slice",
	}
	if err := UpsertSliceProfile(slice); err != nil {
		t.Fatalf("UpsertSliceProfile failed: %v", err)
	}

	// 3. Add Security Profile
	sec := SecurityProfile{
		Name:        "Test-Sec-Keys",
		Key:         "465b5ce8b199b49faa5f0a2ee238a6bc",
		Opc:         "e8ed289deba952e4283b54e88e6183ca",
		Amf:         "8000",
		Sqn:         "000000000000",
		Description: "Test AKA Keys",
	}
	if err := UpsertSecurityProfile(sec); err != nil {
		t.Fatalf("UpsertSecurityProfile failed: %v", err)
	}

	// 4. Verify via GetFleet
	fc := GetFleet()

	foundAMF := false
	for _, a := range fc.AMFProfiles {
		if a.Name == "Test-AMF-Core" && a.Ip == "192.168.1.100" {
			foundAMF = true
			break
		}
	}
	if !foundAMF {
		t.Errorf("Upserted AMF profile not found in GetFleet()")
	}

	foundSlice := false
	for _, s := range fc.SliceProfiles {
		if s.Name == "Test-Slice-eMBB" && s.Sst == 1 && s.Sd == "010203" {
			foundSlice = true
			break
		}
	}
	if !foundSlice {
		t.Errorf("Upserted Slice profile not found in GetFleet()")
	}

	foundSec := false
	for _, s := range fc.SecurityProfiles {
		if s.Name == "Test-Sec-Keys" && s.Key == "465b5ce8b199b49faa5f0a2ee238a6bc" {
			foundSec = true
			break
		}
	}
	if !foundSec {
		t.Errorf("Upserted Security profile not found in GetFleet()")
	}

	// Clean up test AMF, Slice, Security profiles
	_ = DeleteAMFProfile("Test-AMF-Core")
	_ = DeleteSliceProfile("Test-Slice-eMBB")
	_ = DeleteSecurityProfile("Test-Sec-Keys")
}
