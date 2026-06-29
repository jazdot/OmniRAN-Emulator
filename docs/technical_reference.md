# OmniRAN Technical Reference Manual & Documentation

This document contains deep technical details, configuration guidelines, host prerequisites, troubleshooting steps, and execution command references for the OmniRAN 5G Emulator.

---

## 📑 Table of Contents
1. [🛠️ Prerequisites & Host Requirements](#️-prerequisites--host-requirements)
   - [Load Linux Kernel Modules](#1-load-linux-kernel-modules)
   - [5G Core Subscriber Registration (Open5GS example)](#2-5g-core-subscriber-registration-open5gs-example)
2. [📦 How to Build](#-how-to-build)
   - [Local Compilation](#1-local-compilation)
   - [Docker Image Build](#2-docker-image-build)
3. [📖 CLI Usage & Command Reference](#-cli-usage--command-reference)
   - [Standalone Modes](#standalone-modes)
   - [Performance & Load Testing](#performance--load-testing)
4. [📋 Manual Scenario Testing Guide](#-manual-scenario-testing-guide)
   - [1. Periodic Registration Update](#1-periodic-registration-update)
   - [2. Mobility Registration Update](#2-mobility-registration-update)
   - [3. Emergency Registration](#3-emergency-registration)
   - [4. N2 Handover](#4-n2-handover-path-switch-request)
   - [5. Full UE Lifecycle](#5-full-ue-lifecycle)
   - [6. UE-initiated Deregistration](#6-ue-initiated-deregistration)
5. [🧬 3GPP Protocol Spec References & Release Compliance](#-3gpp-protocol-spec-references--release-compliance)
   - [Control Plane Protocols (TS 38.413 & TS 24.501)](#1-control-plane-protocols-ts-38413--ts-24501)
   - [Release-Specific Nuances](#2-release-specific-nuances)
6. [✍️ 3GPP Custom Scripting Engine Reference](#️-3gpp-custom-scripting-engine-reference)
   - [ue_nas_trigger command parameters](#1-ue_nas_trigger)
   - [gnb_ngap_trigger command parameters](#2-gnb_ngap_trigger)
7. [⚡ Performance Testing & Capability Measurement](#-performance-testing--capability-measurement)
   - [Execution KPIs](#1-execution-kpis)
   - [Graceful Fallback Mode](#2-graceful-fallback-mode)
8. [🔒 Secure Access Control & Credentials Administration](#-secure-access-control--credentials-administration)
   - [Security Hashing Implementation](#1-security-hashing-implementation)
   - [Session Management](#2-session-management)
9. [🔧 Troubleshooting & FAQs](#-troubleshooting--faqs)
   - [AMF Connection Refused](#1-amf-connection-refused--sctp-dial-failed)
   - [Operation Not Permitted](#2-error-in-setting-virtual-interface-operation-not-permitted)
   - [Address Already in Use](#3-address-already-in-use-or-socket-locked)
   - [Ping Routing Issues](#4-pdu-session-active-but-ping-doesnt-route-traffic)

---

## 🛠️ Prerequisites & Host Requirements

Because the emulator interacts directly with the Linux kernel network stack to create virtual interfaces and policy routing tables, you must configure your host machine as follows:

### 1. Load Linux Kernel Modules
Ensure the SCTP (control plane) and IPIP (user plane) kernel modules are loaded:
```bash
# Load SCTP module for NGAP control plane
sudo modprobe sctp

# Load IPIP module for virtual user-plane tunneling
sudo modprobe ipip

# Verify modules are active
lsmod | grep -E "sctp|ip"
```

### 2. 5G Core Subscriber Registration (Open5GS example)
Before running the emulator, you must provision a matching subscriber profile in your 5G Core:
1. Open the Open5GS WebUI (typically at `http://localhost:3000` or port `3000` on your core server).
2. Click **Add Subscriber** and enter the following values matching the default emulator profile (`config/config.yml`):
   - **IMSI**: `001010000000001` (or match configured PLMN `001`/`01` + MSIN `0000000001`)
   - **Subscriber Key (K)**: `465B5CE8B199B49FAA5F0A2EE238A6BC`
   - **OPc**: `E8ED9B87E14101FAFD283A41341B70A0`
   - **AMF**: `8000`
   - **SST (Slice Service Type)**: `1`
   - **SD (Slice Differentiator)**: `010203` (or match configured slice)
3. Save the subscriber profile.

---

## 📦 How to Build

### 1. Local Compilation
You do not need Go pre-installed. The Makefile will automatically download a local Go SDK inside the workspace if `go` is missing from your PATH:
```bash
# Build the React Web UI assets (Requires Node.js installed once on compilation machine)
npm --prefix web run build

# Compile the standalone Go binary
make build
```
This generates the standalone compiled binary `app` at the root of the project.

### 2. Docker Image Build
To build the optimized Docker container:
```bash
make docker-build
```

---

## 📖 CLI Usage & Command Reference

You can also run all tests and emulations directly from the command line:

### Standalone Modes
* **`./app ue`**: Runs a single UE registration, authentication, security mode, and PDU session attachment procedure (creates `uetun1`). Add `--ue-only` to run without initializing the gNodeB.
* **`./app gnb`**: Initializes the SCTP connection (NGAP) and GTP-U tunnel (N3) with the AMF/UPF, simulating a running gNodeB cell.

### Performance & Load Testing
* **`./app load-test -n 50`**: Stress tests the AMF by firing concurrent/queued registration requests for multiple simulated UEs sequentially in a queue. Add `--ue-only` to run UEs in decoupled mode.
* **`./app amf-load-loop -n 20 -t 30`**: Generates `20` requests per second over `30` seconds to stress test AMF responsiveness over time.
* **`./app ue-latency-interval -n 10`**: Evaluates the registration latency of `10` UEs and logs the average latency in milliseconds.
* **`./app amf-availability -t 60`**: Performs uptime and availability reachability tests on the AMF over a `60` second time window.

---

## 📋 Manual Scenario Testing Guide

The emulator provides 6 built-in scenario templates representing real-world 5G operations:

### 1. Periodic Registration Update
* **Command**: `sudo ./app scenario periodic-reg`
* **Flow**: Registers a UE ➔ waits 5 seconds to simulate T3512 expiration ➔ sends a Periodic Registration Update.
* **Verify Core Log**: `sudo journalctl -u open5gs-amfd -f | grep -i "periodic"`

### 2. Mobility Registration Update (Tracking Area Update)
* **Command**: `sudo ./app scenario mobility-reg`
* **Flow**: Registers a UE ➔ simulates cell transition (3s delay) ➔ triggers a Mobility Registration Update (TAU).
* **Verify Core Log**: `sudo journalctl -u open5gs-amfd -f | grep -E "Mobility|registration"`

### 3. Emergency Registration
* **Command**: `sudo ./app scenario emergency-reg`
* **Flow**: Triggers unauthenticated Emergency Registration. Note: Depending on your 5G core configuration, it may either accept or reject the emergency request.
* **Verify Core Log**: `sudo journalctl -u open5gs-amfd -f | grep -i "emergency"`

### 4. N2 Handover (Path Switch Request)
* **Command**: `sudo ./app scenario handover --target-gnb-ip 127.0.0.1 --target-gnb-port 9489 --delay 5`
* **Flow**: Registers a UE on Source gNodeB ➔ waits 5 seconds ➔ target gNodeB sends a Path Switch Request NGAP message to notify core of target transport parameters.
* **Verify Core Log**: `sudo journalctl -u open5gs-amfd -f | grep -i "Path Switch"`

### 5. Full UE Lifecycle
* **Command**: `sudo ./app scenario full-lifecycle --idle-seconds 5`
* **Flow**: Registers UE ➔ establishes PDU session ➔ enters CM-IDLE (5s wait) ➔ sends Service Request to wake up ➔ cleanly detaches via UE-initiated Deregistration.
* **Verify Core Log**: `sudo journalctl -u open5gs-amfd -f | grep -E "Service Request|Deregistration|PDU"`

### 6. UE-initiated Deregistration
* **Command**: `sudo ./app scenario deregister`
* **Flow**: Registers UE ➔ waits 3 seconds ➔ sends a Deregistration Request (Access Type: 3GPP, Switch off: false) ➔ receives Deregistration Accept.
* **Verify Core Log**: `sudo journalctl -u open5gs-amfd -f | grep -i "Deregistration"`

---

## 🧬 3GPP Protocol Spec References & Release Compliance

### 1. Control Plane Protocols (TS 38.413 & TS 24.501)
The emulator conforms exactly to the specifications laid out in:
* **TS 38.413**: NG Application Protocol (NGAP). Defines control plane messaging between gNodeB and AMF (e.g. `NGSetupRequest`, `InitialUEMessage`, `InitialContextSetupResponse`).
* **TS 24.501**: Non-Access-Stratum (NAS) protocol for 5G System (5GS). Defines mobility management (5GMM) and session management (5GSM) signaling (e.g. `RegistrationRequest`, `PDUSessionEstablishmentRequest`).

### 2. Release-Specific Nuances
You can switch active 3GPP release profiles in the UI or custom scripting parameters. The emulator dynamically adapts signaling to avoid core parsing failures:
* **Release 15 & 16 (Standard 5G SA)**:
  * Uses `mo-Signalling` as the default `RRCEstablishmentCause` inside `InitialUEMessage`.
  * Adheres to standard cell identity length and location layouts.
* **Release 17 (RedCap & NTN Satellite)**:
  * Configures `mt-Access` (Mobile-Terminated Access) cause value to bypass high congestion thresholds on satellite connections.
  * Adds NTN transparent location markers to bypass terrestrially locked validation routines.
* **Release 18 (UAV & Mission-Critical Slicing)**:
  * Uses `highPriorityAccess` cause values to request higher QoS precedence on core elements.
  * Supports slicing updates and custom security indicators.
* **Release 19 (AI-enhanced Sensing & Ambient IoT)**:
  * Configures `mo-VoiceCall` cause values representing ambient device wake-up or ISAC sensing telemetry registration.

---

## ✍️ 3GPP Custom Scripting Engine Reference

The custom scripting engine allows you to chain multiple scenario actions using a JSON file.

### 1. `ue_nas_trigger`
Forces a simulated UE to compile and send specific NAS payloads.
```json
{
  "type": "ue_nas_trigger",
  "ueId": 1,
  "action": "registration_request",
  "params": {
    "release": "17",
    "registrationType": "periodic",
    "pduSessionId": 1,
    "dnn": "internet",
    "sst": 1,
    "sd": "010203"
  }
}
```
* **Supported actions**: `registration_request`, `service_request`, `deregistration_request`, `pdu_establishment`, `pdu_modification`, `pdu_release`.
* **Dynamic Connection Switching**: You can specify `"switchGnbSocket": "/tmp/gnb2.sock"` inside `params` to simulate a physical cell change.

### 2. `gnb_ngap_trigger`
Forces the gNodeB socket connection to dispatch a direct NGAP control frame.
```json
{
  "type": "gnb_ngap_trigger",
  "gnbId": 1,
  "action": "error_indication",
  "params": {
    "ranUeId": 1,
    "amfUeId": 4096,
    "causeGroup": "radioNetwork",
    "causeValue": "release-due-to-nth-order-interference"
  }
}
```
* **Supported actions**: `ng_setup_request`, `ue_context_release_request`, `error_indication`, `pdu_session_resource_modify_response`.
* **Cause Groups**: `radioNetwork`, `nas`, `protocol`, `transport`, `misc`. Correct cause values are dynamically resolved against the 3GPP Spec DB.

---

## ⚡ Performance Testing & Capability Measurement

The performance suite executes multiple parallel attachments to measure the true capabilities of your 5G Core under stress.

### 1. Execution KPIs
* **Peak RPS (Requests Per Second)**: Measures the maximum registration rate sustained by the AMF.
* **Mean Registration Latency**: Measures average time from `InitialUEMessage` to `RegistrationAccept` in milliseconds.
* **Mean Session Setup Latency**: Measures average time to allocate GTP endpoints and establish data planes.
* **Handover Success Rate**: Tracks Path Switch Request/Response success rates.

### 2. Graceful Fallback Mode
If the target AMF is offline or unreachable, the performance suite enters **Offline 3GPP Schema Validation & Capability Mode**. Instead of socket drops, the engine loops through 3GPP TS 38.413 rules inside `compliance_validator.go`, validating the completeness of every constructed IE block to confirm 100% compliance under high simulated load.

---

## 🔒 Secure Access Control & Credentials Administration

The web dashboard is protected by a zero-dependency authorization framework:

### 1. Security Hashing Implementation
Passwords are hashed using Go's standard `crypto/sha256` library. They are mixed with a unique 16-byte random cryptographically secure salt generated via `crypto/rand`, and stretched across **10,000 rounds** of SHA-256. Verification uses `crypto/subtle.ConstantTimeCompare` to defend against timing side-channel attacks.

### 2. Session Management
Upon login, the server issues a 32-byte cryptographic session token returned inside responses and registered as a cookie. Active session IDs are kept in a thread-safe server memory map (`activeSessions`) and expire after 2 hours. If a request is made without a token, the backend rejects it with `401 Unauthorized`.

---

## 🔧 Troubleshooting & FAQs

### 1. AMF Connection Refused / "SCTP dial failed"
* **Cause**: The 5G Core AMF is either not running, firewall is blocking port 38412, or the SCTP module is missing.
* **Troubleshoot**:
  * Check AMF service state: `sudo systemctl status open5gs-amfd`
  * Verify AMF is listening on SCTP: `sudo ss -sln | grep 38412`
  * Check local firewall: `sudo ufw status` (ensure SCTP port 38412 is allowed)
  * Re-verify kernel module is loaded: `sudo modprobe sctp`

### 2. "Error in setting virtual interface: operation not permitted"
* **Cause**: The emulator must configure virtual interfaces (`uetun1`) using netlink, which requires root permissions.
* **Troubleshoot**: Always prefix execution commands with `sudo` (e.g. `sudo ./app ue` or `sudo ./app web`).

### 3. "Address already in use" or "Socket locked"
* **Cause**: Stale UNIX sockets or ports left over by previously interrupted runs.
* **Troubleshoot**: Clean up standard socket files and check for orphaned processes:
  ```bash
  sudo rm -f /tmp/gnb.sock /tmp/ue*.sock
  sudo killall app
  ```

### 4. PDU session active, but "ping" doesn't route traffic
* **Cause**: The `ipip` tunnel module is missing or core UPF does not route traffic from the allocated UE IP pool.
* **Troubleshoot**:
  * Load the tunnel kernel module: `sudo modprobe ipip`
  * Verify routing tables created by the emulator: `ip rule show` and `ip route show table all`
  * Inspect core UPF routing and check `ogstun` interface settings on the core.
