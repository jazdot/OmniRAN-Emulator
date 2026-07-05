package config

import (
	"encoding/hex"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"runtime"
	"os"
	"sync"

	"gopkg.in/yaml.v2"
)

// Data: Used for access to configuration globally
var Data Config

var (
	ActiveRelease string = "15" // "15" (R15/R16), "17", "18", "19"
	releaseMu     sync.RWMutex
)

func GetActiveRelease() string {
	releaseMu.RLock()
	defer releaseMu.RUnlock()
	return ActiveRelease
}

func SetActiveRelease(rel string) {
	releaseMu.Lock()
	defer releaseMu.Unlock()
	ActiveRelease = rel
}

func init() {
	_ = DefaultInit()
}

type Config struct {
	GNodeB struct {
		ControlIF struct {
			Ip   string `yaml:"ip"`
			Port int    `yaml:"port"`
		} `yaml:"controlif"`
		DataIF struct {
			Ip   string `yaml:"ip"`
			Port int    `yaml:"port"`
		} `yaml:"dataif"`
		PlmnList struct {
			Mcc   string `yaml:"mcc"` // mcc typo in original, keep compatibility
			Mnc   string `yaml:"mnc"`
			Tac   string `yaml:"tac"`
			GnbId string `yaml:"gnbid"`
		} `yaml:"plmnlist"`
		SliceSupportList struct {
			Sst string `yaml:"sst"`
			Sd  string `yaml:"sd"`
		} `yaml:"slicesupportlist"`
		LinkType  string `yaml:"link_type"`
		LinkPort  int    `yaml:"link_port"`
		PagingDRX string `yaml:"paging_drx"`
		CellId    int64  `yaml:"cell_id"`
	} `yaml:"gnodeb"`
	Ue struct {
		Msin  string `yaml:"msin"`
		Key   string `yaml:"key"`
		Opc   string `yaml:"opc"`
		Amf   string `yaml:"amf"`
		Sqn   string `yaml:"sqn"`
		Dnn            string `yaml:"dnn"`
		PduSessionType string `yaml:"pdusessiontype"`
		RegistrationType string `yaml:"registration_type"`
		Hplmn struct {
			Mcc string `yaml:"mcc"`
			Mnc string `yaml:"mnc"`
		} `yaml:"hplmn"`
		Snssai struct {
			Sst int    `yaml:"sst"`
			Sd  string `yaml:"sd"`
		} `yaml:"snssai"`
		PduSessions []PDUSessionConfig `yaml:"pdusessions"`
	} `yaml:"ue"`
	AMF struct {
		Ip   string `yaml:"ip"`
		Port int    `yaml:"port"`
	} `yaml:"amfif"`
	Logs struct {
		Level int `yaml:"level"`
	} `yaml:"logs"`
}

type PDUSessionConfig struct {
	Id             uint8  `yaml:"id"`
	Dnn            string `yaml:"dnn"`
	PduSessionType string `yaml:"pdusessiontype"`
	Sst            int    `yaml:"sst"`
	Sd             string `yaml:"sd"`
}

func RootDir() string {
	_, b, _, _ := runtime.Caller(0)
	d := filepath.Dir(b)
	return filepath.Dir(d)
}

// LoadConfig reads and unmarshals configuration from a specific file path
func LoadConfig(configPath string) error {
	file, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var cfg = Config{}
	err = yaml.Unmarshal(file, &cfg)
	if err != nil {
		return err
	}
	Data = cfg
	return nil
}

// DefaultInit attempts to discover and load the configuration from default paths
func DefaultInit() error {
	// Try loading from ./config/config.yml first (standard deployment)
	err := LoadConfig("config/config.yml")
	if err == nil {
		return nil
	}

	// Try loading from config.yml in current directory
	err = LoadConfig("config.yml")
	if err == nil {
		return nil
	}

	// Fallback to runtime caller source directory
	_, b, _, ok := runtime.Caller(0)
	if ok {
		fallbackPath := filepath.Join(filepath.Dir(filepath.Dir(b)), "config", "config.yml")
		err = LoadConfig(fallbackPath)
		if err == nil {
			return nil
		}
	}

	return err
}

func GetConfig() (Config, error) {
	return Data, nil
}

func (c *Config) Validate() error {
	// 1. Validate PLMN mcc/mnc (should be numeric digits)
	mccRegex := regexp.MustCompile(`^[0-9]{3}$`)
	mncRegex := regexp.MustCompile(`^[0-9]{2,3}$`)
	if !mccRegex.MatchString(c.GNodeB.PlmnList.Mcc) {
		return fmt.Errorf("GNodeB PLMN MCC must be exactly 3 digits")
	}
	if !mncRegex.MatchString(c.GNodeB.PlmnList.Mnc) {
		return fmt.Errorf("GNodeB PLMN MNC must be 2 or 3 digits")
	}
	if !mccRegex.MatchString(c.Ue.Hplmn.Mcc) {
		return fmt.Errorf("UE HPLMN MCC must be exactly 3 digits")
	}
	if !mncRegex.MatchString(c.Ue.Hplmn.Mnc) {
		return fmt.Errorf("UE HPLMN MNC must be 2 or 3 digits")
	}

	// 2. Validate UE Keys
	if len(c.Ue.Key) != 32 {
		return fmt.Errorf("UE Key must be exactly 32 hex characters (128-bit)")
	}
	if _, err := hex.DecodeString(c.Ue.Key); err != nil {
		return fmt.Errorf("UE Key must be a valid hex string")
	}
	if len(c.Ue.Opc) != 32 {
		return fmt.Errorf("UE OPc must be exactly 32 hex characters (128-bit)")
	}
	if _, err := hex.DecodeString(c.Ue.Opc); err != nil {
		return fmt.Errorf("UE OPc must be a valid hex string")
	}
	if len(c.Ue.Amf) != 4 {
		return fmt.Errorf("UE AMF field must be exactly 4 hex characters")
	}
	if _, err := hex.DecodeString(c.Ue.Amf); err != nil {
		return fmt.Errorf("UE AMF must be a valid hex string")
	}
	if len(c.Ue.Sqn) != 12 {
		return fmt.Errorf("UE SQN field must be exactly 12 hex characters")
	}
	if _, err := hex.DecodeString(c.Ue.Sqn); err != nil {
		return fmt.Errorf("UE SQN must be a valid hex string")
	}

	// 3. Validate MSIN
	msinRegex := regexp.MustCompile(`^[0-9]{8,10}$`)
	if !msinRegex.MatchString(c.Ue.Msin) {
		return fmt.Errorf("UE MSIN must be between 8 and 10 numeric digits")
	}

	// 4. Validate IP addresses
	if net.ParseIP(c.GNodeB.ControlIF.Ip) == nil {
		return fmt.Errorf("GNodeB Control IP '%s' is invalid", c.GNodeB.ControlIF.Ip)
	}
	if net.ParseIP(c.GNodeB.DataIF.Ip) == nil {
		return fmt.Errorf("GNodeB Data IP '%s' is invalid", c.GNodeB.DataIF.Ip)
	}
	if net.ParseIP(c.AMF.Ip) == nil {
		return fmt.Errorf("AMF IP '%s' is invalid", c.AMF.Ip)
	}

	// 5. Validate Ports
	if c.GNodeB.ControlIF.Port <= 0 || c.GNodeB.ControlIF.Port > 65535 {
		return fmt.Errorf("GNodeB Control Port must be between 1 and 65535")
	}
	if c.GNodeB.DataIF.Port <= 0 || c.GNodeB.DataIF.Port > 65535 {
		return fmt.Errorf("GNodeB Data Port must be between 1 and 65535")
	}
	if c.GNodeB.LinkPort <= 0 || c.GNodeB.LinkPort > 65535 {
		return fmt.Errorf("GNodeB Link Port must be between 1 and 65535")
	}
	if c.AMF.Port <= 0 || c.AMF.Port > 65535 {
		return fmt.Errorf("AMF Port must be between 1 and 65535")
	}

	// 6. Validate slice configuration
	if c.Ue.Snssai.Sst < 0 || c.Ue.Snssai.Sst > 255 {
		return fmt.Errorf("UE Slice SST must be between 0 and 255")
	}
	if c.Ue.Snssai.Sd != "" {
		if len(c.Ue.Snssai.Sd) != 6 {
			return fmt.Errorf("UE Slice SD must be exactly 6 hex characters (or empty)")
		}
		if _, err := hex.DecodeString(c.Ue.Snssai.Sd); err != nil {
			return fmt.Errorf("UE Slice SD must be a valid hex string")
		}
	}

	// 7. Validate PDU Sessions slice config
	for _, sess := range c.Ue.PduSessions {
		if sess.Id < 2 || sess.Id > 15 {
			return fmt.Errorf("Secondary PDU Session ID %d must be between 2 and 15", sess.Id)
		}
		if sess.Sst < 0 || sess.Sst > 255 {
			return fmt.Errorf("PDU Session %d SST must be between 0 and 255", sess.Id)
		}
		if sess.Sd != "" {
			if len(sess.Sd) != 6 {
				return fmt.Errorf("PDU Session %d SD must be exactly 6 hex characters (or empty)", sess.Id)
			}
			if _, err := hex.DecodeString(sess.Sd); err != nil {
				return fmt.Errorf("PDU Session %d SD must be a valid hex string", sess.Id)
			}
		}
	}

	return nil
}

// PcapHook is a global callback for injecting simulated control plane packets into active PCAP recordings.
var PcapHook func(srcIp, dstIp string, srcPort, dstPort uint16, proto uint8, payload []byte)

