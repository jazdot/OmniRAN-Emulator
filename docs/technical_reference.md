# OmniRAN Technical Reference Manual & Documentation

This document contains deep technical details, configuration guidelines, host prerequisites, troubleshooting steps, and execution command references for the OmniRAN 5G Emulator.

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

---

## 🔬 Advanced Real-Traffic Loop & Scripting

### 1. Real VoNR (Voice over New Radio) RTP loop
The emulator features a real user-plane voice media exchange mechanism. When a VoNR call is initiated to target `"echo"`:
- The emulator spins up a UDP voice echo server at `127.0.0.2:5005`.
- It initiates an RFC 3550 compliant RTP audio loop bound to the UE's PDU session IP.
- RTP packets are encapsulated and sent down the real GTP-U user-plane path, measuring active jitter, latency, packet loss, and calculating the Mean Opinion Score (MOS) in real-time.

### 2. 3GPP Custom Scripting Engine
You can compose JSON scenario scripts with the following advanced step commands:
- **`ue_nas_trigger`**: Dispatches specific GMM/5GSM messages (`registration_request`, `service_request`, `deregistration_request`, `pdu_establishment`, `pdu_modification`, `pdu_release`). Allows setting slice fields (`sst`, `sd`), active release version, and socket path switches.
- **`gnb_ngap_trigger`**: Dispatches NGAP messages (`ng_setup_request`, `ue_context_release_request`, `error_indication`, `pdu_session_resource_modify_response`) with custom 3GPP cause values.
