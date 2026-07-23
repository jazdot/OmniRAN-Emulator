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

// AMFProfile holds saved 5G Core AMF endpoint configuration.
type AMFProfile struct {
	Name        string `json:"name"`
	Ip          string `json:"ip"`
	Port        int    `json:"port"`
	Mcc         string `json:"mcc,omitempty"`
	Mnc         string `json:"mnc,omitempty"`
	Description string `json:"description,omitempty"`
}

// SliceProfile holds saved S-NSSAI slice configuration.
type SliceProfile struct {
	Name        string `json:"name"`
	Sst         int    `json:"sst"`
	Sd          string `json:"sd"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
}

// SecurityProfile holds saved 5G UE authentication credentials.
type SecurityProfile struct {
	Name        string `json:"name"`
	Key         string `json:"key"`
	Opc         string `json:"opc"`
	Amf         string `json:"amf"`
	Sqn         string `json:"sqn"`
	Description string `json:"description,omitempty"`
}

// FleetConfig holds all persisted profiles.
type FleetConfig struct {
	UEProfiles       []UEProfile       `json:"ueProfiles"`
	GNBProfiles      []GNBProfile      `json:"gnbProfiles"`
	AMFProfiles      []AMFProfile      `json:"amfProfiles"`
	SliceProfiles    []SliceProfile    `json:"sliceProfiles"`
	SecurityProfiles []SecurityProfile `json:"securityProfiles"`
}

const fleetConfigPath = "config/fleet.json"

var (
	fleetMu   sync.RWMutex
	fleetData FleetConfig
)

func getDefaultAMFProfiles() []AMFProfile {
	return []AMFProfile{
		{Name: "Local Open5GS Core", Ip: "127.0.0.18", Port: 38412, Mcc: "208", Mnc: "93", Description: "Default local Open5GS 5G Core AMF"},
		{Name: "Loopback AMF (127.0.0.1)", Ip: "127.0.0.1", Port: 38412, Mcc: "208", Mnc: "93", Description: "Standard loopback 5GC AMF"},
	}
}

func getDefaultSliceProfiles() []SliceProfile {
	return []SliceProfile{
		{Name: "eMBB General (SST:1, SD:010203)", Sst: 1, Sd: "010203", Category: "eMBB", Description: "Enhanced Mobile Broadband standard slice"},
		{Name: "URLLC Ultra-Low Latency (SST:2, SD:112233)", Sst: 2, Sd: "112233", Category: "URLLC", Description: "Ultra-reliable low-latency communications"},
		{Name: "MIoT Smart Metering (SST:3, SD:FFFFFF)", Sst: 3, Sd: "FFFFFF", Category: "MIoT", Description: "Massive Internet of Things slice"},
		{Name: "IMS VoNR Voice (SST:1, SD:111111)", Sst: 1, Sd: "111111", Category: "VoNR", Description: "Dedicated Voice over NR IMS slice"},
	}
}

func getDefaultSecurityProfiles() []SecurityProfile {
	return []SecurityProfile{
		{Name: "Standard 3GPP Test Keys", Key: "465B5CE8B2E8863F638D4F72EC869C96", Opc: "E8ED289DEBA952E4283B54E88E6183CA", Amf: "8000", Sqn: "000000000020", Description: "Default 5G AKA authentication credentials"},
	}
}

// LoadFleet reads fleet.json from disk. Creates empty config if absent.
func LoadFleet() error {
	fleetMu.Lock()
	defer fleetMu.Unlock()

	data, err := os.ReadFile(fleetConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Initialize empty fleet with defaults
			fleetData = FleetConfig{
				UEProfiles:       []UEProfile{},
				GNBProfiles:      []GNBProfile{},
				AMFProfiles:      getDefaultAMFProfiles(),
				SliceProfiles:    getDefaultSliceProfiles(),
				SecurityProfiles: getDefaultSecurityProfiles(),
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
	if fc.AMFProfiles == nil || len(fc.AMFProfiles) == 0 {
		fc.AMFProfiles = getDefaultAMFProfiles()
	}
	if fc.SliceProfiles == nil || len(fc.SliceProfiles) == 0 {
		fc.SliceProfiles = getDefaultSliceProfiles()
	}
	if fc.SecurityProfiles == nil || len(fc.SecurityProfiles) == 0 {
		fc.SecurityProfiles = getDefaultSecurityProfiles()
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
		UEProfiles:       make([]UEProfile, len(fleetData.UEProfiles)),
		GNBProfiles:      make([]GNBProfile, len(fleetData.GNBProfiles)),
		AMFProfiles:      make([]AMFProfile, len(fleetData.AMFProfiles)),
		SliceProfiles:    make([]SliceProfile, len(fleetData.SliceProfiles)),
		SecurityProfiles: make([]SecurityProfile, len(fleetData.SecurityProfiles)),
	}
	copy(fc.UEProfiles, fleetData.UEProfiles)
	copy(fc.GNBProfiles, fleetData.GNBProfiles)
	copy(fc.AMFProfiles, fleetData.AMFProfiles)
	copy(fc.SliceProfiles, fleetData.SliceProfiles)
	copy(fc.SecurityProfiles, fleetData.SecurityProfiles)
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

// UpsertAMFProfile creates or updates an AMF profile by name.
func UpsertAMFProfile(p AMFProfile) error {
	if p.Name == "" {
		return fmt.Errorf("AMF profile name cannot be empty")
	}
	if p.Ip == "" || p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("invalid AMF IP or Port")
	}
	fleetMu.Lock()
	defer fleetMu.Unlock()
	for i, existing := range fleetData.AMFProfiles {
		if existing.Name == p.Name {
			fleetData.AMFProfiles[i] = p
			return saveFleetLocked()
		}
	}
	fleetData.AMFProfiles = append(fleetData.AMFProfiles, p)
	return saveFleetLocked()
}

// DeleteAMFProfile removes an AMF profile by name.
func DeleteAMFProfile(name string) error {
	fleetMu.Lock()
	defer fleetMu.Unlock()
	for i, p := range fleetData.AMFProfiles {
		if p.Name == name {
			fleetData.AMFProfiles = append(fleetData.AMFProfiles[:i], fleetData.AMFProfiles[i+1:]...)
			return saveFleetLocked()
		}
	}
	return fmt.Errorf("AMF profile '%s' not found", name)
}

// UpsertSliceProfile creates or updates a Slice profile by name.
func UpsertSliceProfile(p SliceProfile) error {
	if p.Name == "" {
		return fmt.Errorf("Slice profile name cannot be empty")
	}
	fleetMu.Lock()
	defer fleetMu.Unlock()
	for i, existing := range fleetData.SliceProfiles {
		if existing.Name == p.Name {
			fleetData.SliceProfiles[i] = p
			return saveFleetLocked()
		}
	}
	fleetData.SliceProfiles = append(fleetData.SliceProfiles, p)
	return saveFleetLocked()
}

// DeleteSliceProfile removes a Slice profile by name.
func DeleteSliceProfile(name string) error {
	fleetMu.Lock()
	defer fleetMu.Unlock()
	for i, p := range fleetData.SliceProfiles {
		if p.Name == name {
			fleetData.SliceProfiles = append(fleetData.SliceProfiles[:i], fleetData.SliceProfiles[i+1:]...)
			return saveFleetLocked()
		}
	}
	return fmt.Errorf("Slice profile '%s' not found", name)
}

// UpsertSecurityProfile creates or updates a Security profile by name.
func UpsertSecurityProfile(p SecurityProfile) error {
	if p.Name == "" {
		return fmt.Errorf("Security profile name cannot be empty")
	}
	fleetMu.Lock()
	defer fleetMu.Unlock()
	for i, existing := range fleetData.SecurityProfiles {
		if existing.Name == p.Name {
			fleetData.SecurityProfiles[i] = p
			return saveFleetLocked()
		}
	}
	fleetData.SecurityProfiles = append(fleetData.SecurityProfiles, p)
	return saveFleetLocked()
}

// DeleteSecurityProfile removes a Security profile by name.
func DeleteSecurityProfile(name string) error {
	fleetMu.Lock()
	defer fleetMu.Unlock()
	for i, p := range fleetData.SecurityProfiles {
		if p.Name == name {
			fleetData.SecurityProfiles = append(fleetData.SecurityProfiles[:i], fleetData.SecurityProfiles[i+1:]...)
			return saveFleetLocked()
		}
	}
	return fmt.Errorf("Security profile '%s' not found", name)
}

// ImportFleet replaces or merges full fleet configuration.
func ImportFleet(fc FleetConfig) error {
	fleetMu.Lock()
	defer fleetMu.Unlock()
	if fc.UEProfiles != nil {
		fleetData.UEProfiles = fc.UEProfiles
	}
	if fc.GNBProfiles != nil {
		fleetData.GNBProfiles = fc.GNBProfiles
	}
	if fc.AMFProfiles != nil {
		fleetData.AMFProfiles = fc.AMFProfiles
	}
	if fc.SliceProfiles != nil {
		fleetData.SliceProfiles = fc.SliceProfiles
	}
	if fc.SecurityProfiles != nil {
		fleetData.SecurityProfiles = fc.SecurityProfiles
	}
	return saveFleetLocked()
}
