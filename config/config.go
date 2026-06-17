package config

import (
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"runtime"
)

// Data: Used for access to configuration globally
var Data Config

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
		LinkType string `yaml:"link_type"`
		LinkPort int    `yaml:"link_port"`
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
