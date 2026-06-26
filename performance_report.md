# 5G Core Performance & Capability Measuring Report

## Executive Summary
This report presents the capability metrics, latency statistics, and 3GPP TS 38.413 IE compliance status of the 5G Core network under load.

- **Benchmark Mode**: Validation & Capability Mode
- **3GPP Release Version**: Release 15
- **Total UEs Tested**: 10
- **Execution Duration**: 5.00 seconds

---

## Key Performance Indicators (KPIs)

| Metric | Measured Value | Target SLA | Status |
| :--- | :--- | :--- | :--- |
| **Peak Registration Rate** | 455.17 RPS | > 100 RPS | ✅ PASSED |
| **Mean Registration Latency** | 19.85 ms | < 50 ms | ✅ PASSED |
| **Mean Session Setup Latency** | 8.69 ms | < 30 ms | ✅ PASSED |
| **Handover Success Rate** | 100.00% | > 99.5% | ✅ PASSED |
| **Mean Handover Latency** | 19.41 ms | < 80 ms | ✅ PASSED |

---

## 3GPP Schema Compliance Verification
Every message sent and received during load testing is verified against structural mandatory IE requirements defined in **3GPP TS 38.413**.

- **Total Messages Checked**: 90
- **Validation Failures**: 0
- **Compliance Rating**: 100.00%

### Validation Summary:
- ✅ **100% compliant** with no structural or mandatory IE omissions.

---
*Report generated automatically by OmniRAN-Emulator Performance Suite.*
