package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// UEProfile is a named, persisted UE configuration profile.
type UEProfile struct {
	Name             string            `json:"name"`
	Msin             string            `json:"msin"`
	Key              string            `json:"key"`
	Opc              string            `json:"opc"`
	Amf              string            `json:"amf"`
	Sqn              string            `json:"sqn"`
	Dnn              string            `json:"dnn"`
	PduSessionType   string            `json:"pduSessionType"`
	RegistrationType string            `json:"registrationType"`
	Hplmn            HplmnConfig       `json:"hplmn"`
	Snssai           SnssaiConfig      `json:"snssai"`
	PduSessions      []PDUSessionConfig `json:"pduSessions,omitempty"`
}

// HplmnConfig holds PLMN identity for a UE profile.
type HplmnConfig struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

// SnssaiConfig holds S-NSSAI for a UE profile.
type SnssaiConfig struct {
	Sst int    `json:"sst"`
	Sd  string `json:"sd"`
}

// GNBProfile is a named, persisted gNB configuration profile.
type GNBProfile struct {
	Name            string `json:"name"`
	GnbId           string `json:"gnbId"`
	Mcc             string `json:"mcc"`
	Mnc             string `json:"mnc"`
	Tac             string `json:"tac"`
	SliceSst        string `json:"sliceSst"`
	SliceSd         string `json:"sliceSd"`
	ControlIp       string `json:"controlIp"`
	ControlPort     int    `json:"controlPort"`
	DataIp          string `json:"dataIp"`
	DataPort        int    `json:"dataPort"`
	LinkType        string `json:"linkType"`
	LinkPort        int    `json:"linkPort"`
	AmfIp           string `json:"amfIp"`
	AmfPort         int    `json:"amfPort"`
}

// FleetConfig holds all persisted profiles.
type FleetConfig struct {
	UEProfiles  []UEProfile  `json:"ueProfiles"`
	GNBProfiles []GNBProfile `json:"gnbProfiles"`
}

const fleetConfigPath = "config/fleet.json"

var (
	fleetMu   sync.RWMutex
	fleetData FleetConfig
)

// LoadFleet reads fleet.json from disk. Creates empty config if absent.
func LoadFleet() error {
	fleetMu.Lock()
	defer fleetMu.Unlock()

	data, err := os.ReadFile(fleetConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Initialize empty fleet
			fleetData = FleetConfig{
				UEProfiles:  []UEProfile{},
				GNBProfiles: []GNBProfile{},
			}
			return saveFleetLocked()
		}
		return fmt.Errorf("failed to read fleet config: %w", err)
	}

	var fc FleetConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("failed to parse fleet config: %w", err)
	}
	if fc.UEProfiles == nil {
		fc.UEProfiles = []UEProfile{}
	}
	if fc.GNBProfiles == nil {
		fc.GNBProfiles = []GNBProfile{}
	}
	fleetData = fc
	return nil
}

// GetFleet returns a copy of the full fleet config.
func GetFleet() FleetConfig {
	fleetMu.RLock()
	defer fleetMu.RUnlock()
	// Deep copy
	fc := FleetConfig{
		UEProfiles:  make([]UEProfile, len(fleetData.UEProfiles)),
		GNBProfiles: make([]GNBProfile, len(fleetData.GNBProfiles)),
	}
	copy(fc.UEProfiles, fleetData.UEProfiles)
	copy(fc.GNBProfiles, fleetData.GNBProfiles)
	return fc
}

// UpsertUEProfile creates or updates a UE profile by name.
func UpsertUEProfile(p UEProfile) error {
	if p.Name == "" {
		return fmt.Errorf("UE profile name cannot be empty")
	}
	fleetMu.Lock()
	defer fleetMu.Unlock()
	for i, existing := range fleetData.UEProfiles {
		if existing.Name == p.Name {
			fleetData.UEProfiles[i] = p
			return saveFleetLocked()
		}
	}
	fleetData.UEProfiles = append(fleetData.UEProfiles, p)
	return saveFleetLocked()
}

// DeleteUEProfile removes a UE profile by name.
func DeleteUEProfile(name string) error {
	fleetMu.Lock()
	defer fleetMu.Unlock()
	for i, p := range fleetData.UEProfiles {
		if p.Name == name {
			fleetData.UEProfiles = append(fleetData.UEProfiles[:i], fleetData.UEProfiles[i+1:]...)
			return saveFleetLocked()
		}
	}
	return fmt.Errorf("UE profile '%s' not found", name)
}

// GetUEProfile retrieves a UE profile by name.
func GetUEProfile(name string) (UEProfile, bool) {
	fleetMu.RLock()
	defer fleetMu.RUnlock()
	for _, p := range fleetData.UEProfiles {
		if p.Name == name {
			return p, true
		}
	}
	return UEProfile{}, false
}

// UpsertGNBProfile creates or updates a gNB profile by name.
func UpsertGNBProfile(p GNBProfile) error {
	if p.Name == "" {
		return fmt.Errorf("gNB profile name cannot be empty")
	}
	fleetMu.Lock()
	defer fleetMu.Unlock()
	for i, existing := range fleetData.GNBProfiles {
		if existing.Name == p.Name {
			fleetData.GNBProfiles[i] = p
			return saveFleetLocked()
		}
	}
	fleetData.GNBProfiles = append(fleetData.GNBProfiles, p)
	return saveFleetLocked()
}

// DeleteGNBProfile removes a gNB profile by name.
func DeleteGNBProfile(name string) error {
	fleetMu.Lock()
	defer fleetMu.Unlock()
	for i, p := range fleetData.GNBProfiles {
		if p.Name == name {
			fleetData.GNBProfiles = append(fleetData.GNBProfiles[:i], fleetData.GNBProfiles[i+1:]...)
			return saveFleetLocked()
		}
	}
	return fmt.Errorf("gNB profile '%s' not found", name)
}

// GetGNBProfile retrieves a gNB profile by name.
func GetGNBProfile(name string) (GNBProfile, bool) {
	fleetMu.RLock()
	defer fleetMu.RUnlock()
	for _, p := range fleetData.GNBProfiles {
		if p.Name == name {
			return p, true
		}
	}
	return GNBProfile{}, false
}

// saveFleetLocked writes fleet data to disk. Caller must hold the write lock.
func saveFleetLocked() error {
	data, err := json.MarshalIndent(fleetData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal fleet config: %w", err)
	}
	if err := os.WriteFile(fleetConfigPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write fleet config: %w", err)
	}
	return nil
}

// BuildConfigFromGNBProfile converts a GNBProfile into a full Config struct
// by overlaying the profile's gNB settings on top of the current global config.
func BuildConfigFromGNBProfile(p GNBProfile) Config {
	cfg := Data // copy base config (AMF, logs, etc.)
	cfg.GNodeB.PlmnList.GnbId = p.GnbId
	cfg.GNodeB.PlmnList.Mcc = p.Mcc
	cfg.GNodeB.PlmnList.Mnc = p.Mnc
	cfg.GNodeB.PlmnList.Tac = p.Tac
	cfg.GNodeB.SliceSupportList.Sst = p.SliceSst
	cfg.GNodeB.SliceSupportList.Sd = p.SliceSd
	cfg.GNodeB.ControlIF.Ip = p.ControlIp
	cfg.GNodeB.ControlIF.Port = p.ControlPort
	cfg.GNodeB.DataIF.Ip = p.DataIp
	cfg.GNodeB.DataIF.Port = p.DataPort
	cfg.GNodeB.LinkType = p.LinkType
	cfg.GNodeB.LinkPort = p.LinkPort
	if p.AmfIp != "" {
		cfg.AMF.Ip = p.AmfIp
	}
	if p.AmfPort != 0 {
		cfg.AMF.Port = p.AmfPort
	}
	return cfg
}

// BuildConfigFromUEProfile converts a UEProfile into a full Config struct
// by overlaying the profile's UE settings on top of the current global config.
func BuildConfigFromUEProfile(p UEProfile) Config {
	cfg := Data // copy base config
	cfg.Ue.Msin = p.Msin
	cfg.Ue.Key = p.Key
	cfg.Ue.Opc = p.Opc
	cfg.Ue.Amf = p.Amf
	cfg.Ue.Sqn = p.Sqn
	cfg.Ue.Dnn = p.Dnn
	if p.PduSessionType != "" {
		cfg.Ue.PduSessionType = p.PduSessionType
	}
	if p.RegistrationType != "" {
		cfg.Ue.RegistrationType = p.RegistrationType
	}
	cfg.Ue.Hplmn.Mcc = p.Hplmn.Mcc
	cfg.Ue.Hplmn.Mnc = p.Hplmn.Mnc
	cfg.Ue.Snssai.Sst = p.Snssai.Sst
	cfg.Ue.Snssai.Sd = p.Snssai.Sd
	if p.PduSessions != nil {
		cfg.Ue.PduSessions = p.PduSessions
	}
	return cfg
}

// ValidateUEProfile validates the fields of a UEProfile.
func ValidateUEProfile(p UEProfile) error {
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	// Build a temporary Config and run Validate() on it
	cfg := BuildConfigFromUEProfile(p)
	return cfg.Validate()
}

// ValidateGNBProfile validates the fields of a GNBProfile.
func ValidateGNBProfile(p GNBProfile) error {
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if p.GnbId == "" {
		return fmt.Errorf("gNB ID is required")
	}
	if p.ControlIp == "" {
		return fmt.Errorf("control IP is required")
	}
	if p.ControlPort <= 0 || p.ControlPort > 65535 {
		return fmt.Errorf("control port must be 1-65535")
	}
	if p.LinkPort <= 0 || p.LinkPort > 65535 {
		return fmt.Errorf("link port must be 1-65535")
	}
	if p.AmfIp == "" {
		return fmt.Errorf("AMF IP is required")
	}
	if p.AmfPort <= 0 || p.AmfPort > 65535 {
		return fmt.Errorf("AMF port must be 1-65535")
	}
	return nil
}
