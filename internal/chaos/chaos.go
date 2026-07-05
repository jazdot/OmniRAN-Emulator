package chaos

import (
	"math/rand"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type ChaosConfig struct {
	DropProbability float64       `json:"dropProbability"` // 0.0 to 1.0
	DelayDuration   time.Duration `json:"delayDuration"`   // Delay duration in ms
	TargetMsgType   string        `json:"targetMsgType"`   // Empty for all, or specific message name
	Enabled         bool          `json:"enabled"`
}

type FuzzConfig struct {
	TargetMsg   string  `json:"targetMsg"`   // Message type to fuzz
	FuzzType    string  `json:"fuzzType"`    // "bit_flip", "truncate", "overflow", "zero_out"
	Probability float64 `json:"probability"` // Fuzz probability (0.0 to 1.0)
	Enabled     bool    `json:"enabled"`
}

type ChaosManager struct {
	mu          sync.RWMutex
	NasRules    map[uint8]ChaosConfig   // Key: UE ID
	NgapRules   map[string]ChaosConfig  // Key: GNB ID (decimal string)
	DroppedNas  int64
	DelayedNas  int64
	DroppedNgap int64
	DelayedNgap int64
	
	FuzzRule    FuzzConfig
	FuzzedMsgs  int64
}

var GlobalChaosManager = &ChaosManager{
	NasRules:  make(map[uint8]ChaosConfig),
	NgapRules: make(map[string]ChaosConfig),
}

var LastNasMsgTypes = struct {
	sync.RWMutex
	m map[uint8]string
}{
	m: make(map[uint8]string),
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func (m *ChaosManager) ConfigureNas(ueId uint8, config ChaosConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.NasRules[ueId] = config
	log.Infof("[CHAOS] Configured NAS rule for UE %d: drop=%f, delay=%v, target=%s, enabled=%t",
		ueId, config.DropProbability, config.DelayDuration, config.TargetMsgType, config.Enabled)
}

func (m *ChaosManager) ConfigureNgap(gnbId string, config ChaosConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.NgapRules[gnbId] = config
	log.Infof("[CHAOS] Configured NGAP rule for gNB %s: drop=%f, delay=%v, target=%s, enabled=%t",
		gnbId, config.DropProbability, config.DelayDuration, config.TargetMsgType, config.Enabled)
}

func (m *ChaosManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.NasRules = make(map[uint8]ChaosConfig)
	m.NgapRules = make(map[string]ChaosConfig)
	m.DroppedNas = 0
	m.DelayedNas = 0
	m.DroppedNgap = 0
	m.DelayedNgap = 0
	log.Info("[CHAOS] Reset all rules and counters.")
}

func (m *ChaosManager) GetStats() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]int64{
		"droppedNas":  m.DroppedNas,
		"delayedNas":  m.DelayedNas,
		"droppedNgap": m.DroppedNgap,
		"delayedNgap": m.DelayedNgap,
	}
}

// SetLastNasMsgType records the message type of the plaintext NAS packet being encrypted.
func SetLastNasMsgType(ueId uint8, msgType string) {
	LastNasMsgTypes.Lock()
	defer LastNasMsgTypes.Unlock()
	LastNasMsgTypes.m[ueId] = msgType
}

// GetLastNasMsgType retrieves the message type of the last ciphered NAS packet for a UE.
func GetLastNasMsgType(ueId uint8) string {
	LastNasMsgTypes.RLock()
	defer LastNasMsgTypes.RUnlock()
	if val, ok := LastNasMsgTypes.m[ueId]; ok {
		return val
	}
	return "CipheredNAS"
}

// GetNasMsgNameFromBytes resolves a plain NAS message type or returns default fallback.
func GetNasMsgNameFromBytes(msg []byte, ueId uint8) string {
	if len(msg) < 3 {
		return "Unknown"
	}
	// Plain GMM message
	if msg[0] == 0x7e && msg[1] == 0x00 {
		msgType := msg[2]
		switch msgType {
		case 0x41:
			return "RegistrationRequest"
		case 0x42:
			return "RegistrationAccept"
		case 0x43:
			return "RegistrationComplete"
		case 0x44:
			return "RegistrationReject"
		case 0x45:
			return "DeregistrationRequest"
		case 0x46:
			return "DeregistrationAccept"
		case 0x54:
			return "ServiceRequest"
		case 0x56:
			return "ServiceReject"
		case 0x57:
			return "ServiceAccept"
		case 0x64:
			return "AuthenticationRequest"
		case 0x65:
			return "AuthenticationResponse"
		case 0x67:
			return "AuthenticationFailure"
		case 0x68:
			return "SecurityModeCommand"
		case 0x69:
			return "SecurityModeComplete"
		case 0x6a:
			return "SecurityModeReject"
		case 0x6e:
			return "IdentityRequest"
		case 0x6f:
			return "IdentityResponse"
		}
	}
	// Plain GSM message
	if msg[0] == 0x2e && len(msg) >= 4 {
		msgType := msg[3]
		switch msgType {
		case 0xc1:
			return "PduSessionEstablishmentRequest"
		case 0xc2:
			return "PduSessionEstablishmentAccept"
		case 0xc3:
			return "PduSessionEstablishmentReject"
		case 0xc5:
			return "PduSessionModificationRequest"
		case 0xc6:
			return "PduSessionModificationCommand"
		case 0xc7:
			return "PduSessionModificationComplete"
		case 0xca:
			return "PduSessionReleaseRequest"
		case 0xcb:
			return "PduSessionReleaseCommand"
		case 0xcc:
			return "PduSessionReleaseComplete"
		}
	}
	// Ciphered message
	if msg[0] == 0x7e && msg[1] != 0x00 {
		return GetLastNasMsgType(ueId)
	}
	return "NAS-Message"
}

// EvalNas evaluates if a NAS message should be dropped or delayed.
// Returns (shouldDrop, delayDuration).
func (m *ChaosManager) EvalNas(ueId uint8, messageType string) (bool, time.Duration) {
	m.mu.RLock()
	config, ok := m.NasRules[ueId]
	m.mu.RUnlock()

	if !ok || !config.Enabled {
		return false, 0
	}

	// Match target message if specified
	if config.TargetMsgType != "" && config.TargetMsgType != messageType {
		return false, 0
	}

	var drop bool
	if config.DropProbability > 0 {
		roll := rand.Float64()
		if roll < config.DropProbability {
			drop = true
			m.mu.Lock()
			m.DroppedNas++
			m.mu.Unlock()
			log.Warnf("[CHAOS][NAS] DROPPED NAS message type %s for UE %d", messageType, ueId)
		}
	}

	var delay time.Duration
	if !drop && config.DelayDuration > 0 {
		delay = config.DelayDuration
		m.mu.Lock()
		m.DelayedNas++
		m.mu.Unlock()
		log.Warnf("[CHAOS][NAS] DELAYING NAS message type %s for UE %d by %v", messageType, ueId, delay)
	}

	return drop, delay
}

// EvalNgap evaluates if an NGAP message should be dropped or delayed.
// Returns (shouldDrop, delayDuration).
func (m *ChaosManager) EvalNgap(gnbId string, messageType string) (bool, time.Duration) {
	m.mu.RLock()
	config, ok := m.NgapRules[gnbId]
	m.mu.RUnlock()

	if !ok || !config.Enabled {
		return false, 0
	}

	// Match target message if specified
	if config.TargetMsgType != "" && config.TargetMsgType != messageType {
		return false, 0
	}

	var drop bool
	if config.DropProbability > 0 {
		roll := rand.Float64()
		if roll < config.DropProbability {
			drop = true
			m.mu.Lock()
			m.DroppedNgap++
			m.mu.Unlock()
			log.Warnf("[CHAOS][NGAP] DROPPED NGAP message type %s for gNB %s", messageType, gnbId)
		}
	}

	var delay time.Duration
	if !drop && config.DelayDuration > 0 {
		delay = config.DelayDuration
		m.mu.Lock()
		m.DelayedNgap++
		m.mu.Unlock()
		log.Warnf("[CHAOS][NGAP] DELAYING NGAP message type %s for gNB %s by %v", messageType, gnbId, delay)
	}

	return drop, delay
}

// GetRules thread-safely returns copies of active rules to prevent data races.
func (m *ChaosManager) GetRules() (map[uint8]ChaosConfig, map[string]ChaosConfig) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nasCopy := make(map[uint8]ChaosConfig)
	for k, v := range m.NasRules {
		nasCopy[k] = v
	}

	ngapCopy := make(map[string]ChaosConfig)
	for k, v := range m.NgapRules {
		ngapCopy[k] = v
	}

	return nasCopy, ngapCopy
}

func (m *ChaosManager) ConfigureFuzz(config FuzzConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FuzzRule = config
	log.Infof("[CHAOS] Configured Fuzz rule: target=%s, type=%s, prob=%f, enabled=%t",
		config.TargetMsg, config.FuzzType, config.Probability, config.Enabled)
}

func (m *ChaosManager) GetFuzz() FuzzConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.FuzzRule
}

func (m *ChaosManager) EvalFuzz(messageType string, msg []byte) (bool, []byte) {
	m.mu.RLock()
	rule := m.FuzzRule
	m.mu.RUnlock()

	if !rule.Enabled {
		return false, msg
	}

	if rule.TargetMsg != "" && rule.TargetMsg != messageType {
		return false, msg
	}

	roll := rand.Float64()
	if roll >= rule.Probability {
		return false, msg
	}

	m.mu.Lock()
	m.FuzzedMsgs++
	m.mu.Unlock()

	fuzzed := make([]byte, len(msg))
	copy(fuzzed, msg)

	log.Warnf("[CHAOS][FUZZ] Fuzzing message type %s using mutation %s", messageType, rule.FuzzType)

	switch rule.FuzzType {
	case "bit_flip":
		if len(fuzzed) > 0 {
			// Flip 1-3 random bits
			flips := rand.Intn(3) + 1
			for idx := 0; idx < flips; idx++ {
				byteIdx := rand.Intn(len(fuzzed))
				bitIdx := rand.Intn(8)
				fuzzed[byteIdx] ^= (1 << bitIdx)
			}
		}
	case "truncate":
		if len(fuzzed) > 4 {
			newLen := rand.Intn(len(fuzzed)-4) + 4
			fuzzed = fuzzed[:newLen]
		}
	case "overflow":
		junk := make([]byte, 256)
		rand.Read(junk)
		fuzzed = append(fuzzed, junk...)
	case "zero_out":
		if len(fuzzed) > 4 {
			start := rand.Intn(len(fuzzed) - 2)
			length := rand.Intn(len(fuzzed)-start) + 1
			if start+length > len(fuzzed) {
				length = len(fuzzed) - start
			}
			for i := start; i < start+length; i++ {
				fuzzed[i] = 0x00
			}
		}
	}

	return true, fuzzed
}
