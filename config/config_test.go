package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidFile(t *testing.T) {
	// Write a minimal config YAML to a temp file
	content := `
gnodeb:
  controlif:
    ip: "127.0.0.1"
    port: 9487
  dataif:
    ip: "127.0.0.1"
    port: 2152
  plmnlist:
    mcc: "208"
    mnc: "93"
    tac: "000001"
    gnbid: "000008"
  slicesupportlist:
    sst: "01"
    sd: "010203"
  link_type: "tcp"
  link_port: 9487
ue:
  msin: "0000000120"
  key: "5122250214c33e723a5dd523fc145fc0"
  opc: "981d464c7c52eb6e5036234984ad0bcf"
  amf: "8000"
  sqn: "000000000780"
  dnn: "internet"
  pdusessiontype: "ipv4"
  registration_type: "initial"
  hplmn:
    mcc: "208"
    mnc: "93"
  snssai:
    sst: 1
    sd: "010203"
amfif:
  ip: "127.0.0.1"
  port: 38412
logs:
  level: 4
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	var cfg Config
	err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	cfg = Data

	// Assertions
	if cfg.GNodeB.ControlIF.Ip != "127.0.0.1" {
		t.Errorf("GNodeB ControlIF IP: expected 127.0.0.1, got %s", cfg.GNodeB.ControlIF.Ip)
	}
	if cfg.GNodeB.ControlIF.Port != 9487 {
		t.Errorf("GNodeB ControlIF Port: expected 9487, got %d", cfg.GNodeB.ControlIF.Port)
	}
	if cfg.Ue.Msin != "0000000120" {
		t.Errorf("UE MSIN: expected 0000000120, got %s", cfg.Ue.Msin)
	}
	if cfg.Ue.RegistrationType != "initial" {
		t.Errorf("RegistrationType: expected 'initial', got %s", cfg.Ue.RegistrationType)
	}
	if cfg.AMF.Ip != "127.0.0.1" {
		t.Errorf("AMF IP: expected 127.0.0.1, got %s", cfg.AMF.Ip)
	}
	if cfg.Logs.Level != 4 {
		t.Errorf("Logs level: expected 4, got %d", cfg.Logs.Level)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	err := LoadConfig("/nonexistent/path/config.yml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadConfig_RegistrationTypes(t *testing.T) {
	types := []string{"initial", "mobility", "periodic", "emergency"}
	for _, rt := range types {
		t.Run(rt, func(t *testing.T) {
			content := `ue:
  registration_type: "` + rt + `"
`
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yml")
			if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
				t.Fatalf("write failed: %v", err)
			}
			if err := LoadConfig(configPath); err != nil {
				t.Fatalf("LoadConfig error: %v", err)
			}
			if Data.Ue.RegistrationType != rt {
				t.Errorf("expected %q, got %q", rt, Data.Ue.RegistrationType)
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	// Ensure GetConfig returns the current Data without error
	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}
	// Just validate it's a valid Config struct (zero value is fine in test context)
	_ = cfg
}
