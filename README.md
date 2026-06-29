# OmniRAN Emulator

<p align="center">
  <img width="400" src="docs/media/img/omniran_dark.png#gh-dark-mode-only" alt="OmniRAN Emulator Logo"/>
  <img width="400" src="docs/media/img/omniran_light.png#gh-light-mode-only" alt="OmniRAN Emulator Logo"/>
</p>

OmniRAN Emulator is a unified, high-performance 5G network emulation solution that simulates real-world User Equipment (UE) and next-generation NodeB (gNB) behavior. It streamlines 5G core validation by delivering an affordable, scalable, and highly accurate virtual RAN environment for next-generation telecommunication testing.

*Note: OmniRAN Emulator borrows libraries and data structures from the [free5gc project](https://github.com/free5gc/free5gc) and builds upon previous open-source solutions.*

---

## ⚡ Quick Start

### 1. Build React Web Assets & Compile Binary
You do not need Go pre-installed. The build script automatically downloads a workspace-local Go SDK:
```bash
# Build React Web UI production assets
npm --prefix web run build

# Compile standalone Go executable
make build
```

### 2. Launch the Web Dashboard
Since the emulator configures Linux virtual network interfaces (e.g. `uetun1`), you must start the web server with root privileges:
```bash
sudo ./app web --port 8080 --host 127.0.0.1
```
Open **`http://localhost:8080`** in your browser to access the premium interactive dashboard.

---

## 📖 Documentation & References

All deep technical details, guides, and troubleshooting steps have been migrated out of this file:

* **In-App Dashboard**: Open the **Documentation** tab directly inside the Web UI to view the live parsed reference guide.
* **Markdown File**: You can read the raw Markdown documentation at [docs/technical_reference.md](file:///home/richu/OmniRAN-Emulator/docs/technical_reference.md).

### What is covered in the documentation:
1. **Host Prerequisites**: Loading SCTP & IPIP Linux kernel modules.
2. **Subscriber Registration**: Core subscriber setup examples (e.g., Open5GS profile provisioning).
3. **CLI Command Reference**: Command arguments for standalone execution, load testing, latency profiling, and AMF availability checking.
4. **Manual Scenario Testing Guide**: Built-in 3GPP testing templates (Periodic registration, TAU mobility, Emergency registration, Handover/Path Switch, CM-IDLE to CM-CONNECTED Service Request flow).
5. **Advanced Features**: Dynamic 3GPP Release overrides, Custom Scripting Engine steps, and Real-Traffic VoNR RTP voice echo loop details.
6. **Troubleshooting FAQs**: SCTP binding issues, virtual interface netlink permission faults, stale UNIX socket cleanups, and user-plane routing issues.
