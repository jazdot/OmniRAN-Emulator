#!/bin/bash
# OmniRAN-Emulator Quick Test Script
# Runs gNB+UE together via: ./app ue  (which internally starts both)
# For gNB-only test: ./app gnb
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP="$SCRIPT_DIR/app"
LOG_DIR="$SCRIPT_DIR/test-logs"
mkdir -p "$LOG_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${BLUE}[TEST]${NC} $*"; }
pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }
info() { echo -e "${CYAN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }

PROC_PID=""

cleanup() {
    echo ""
    warn "Cleaning up..."
    [ -n "$PROC_PID" ] && kill "$PROC_PID" 2>/dev/null && info "Process stopped"
    # Clean up any stale sockets
    rm -f /tmp/gnb.sock /tmp/ue*.sock 2>/dev/null
    wait 2>/dev/null
    echo ""
    info "Logs saved to: $LOG_DIR/"
}
trap cleanup EXIT INT TERM

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║       OmniRAN-Emulator Test Suite v1.0           ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════╝${NC}"
echo ""

# ── Pre-flight checks ─────────────────────────────────────────────────────────
log "Pre-flight checks..."

# Clean stale sockets first
rm -f /tmp/gnb.sock /tmp/ue*.sock 2>/dev/null

if ! ss -alnp 2>/dev/null | grep -q '38412'; then
    fail "Open5GS AMF is NOT listening on port 38412."
    fail "Please run: sudo systemctl restart open5gs-amfd open5gs-smfd open5gs-upfd"
    exit 1
fi
AMF_ADDR=$(ss -alnp 2>/dev/null | grep '38412' | awk '{print $5}')
pass "Open5GS AMF is listening: $AMF_ADDR"

AMF_IP=$(grep -A1 '^amfif:' "$SCRIPT_DIR/config/config.yml" | grep 'ip:' | awk '{print $2}' | tr -d '"')
AMF_PORT=$(grep -A2 '^amfif:' "$SCRIPT_DIR/config/config.yml" | grep 'port:' | awk '{print $2}')
info "Config AMF target: ${AMF_IP}:${AMF_PORT}"

if ! [ -x "$APP" ]; then
    warn "Binary not found, building..."
    make -C "$SCRIPT_DIR" build
fi
pass "Binary ready"

MCC=$(grep 'mcc:' "$SCRIPT_DIR/config/config.yml" | head -1 | awk '{print $2}' | tr -d '"')
MNC=$(grep 'mnc:' "$SCRIPT_DIR/config/config.yml" | head -1 | awk '{print $2}' | tr -d '"')
MSIN=$(grep 'msin:' "$SCRIPT_DIR/config/config.yml" | awk '{print $2}' | tr -d '"')
REG_TYPE=$(grep 'registration_type:' "$SCRIPT_DIR/config/config.yml" | awk '{print $2}' | tr -d '"')
info "PLMN: MCC=${MCC} MNC=${MNC} | IMSI: ${MCC}${MNC}${MSIN} | Reg: ${REG_TYPE:-initial}"
echo ""

# ── Test 1: gNB-only (NG-Setup) ───────────────────────────────────────────────
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}  Test 1: gNB NG-Setup (standalone gNB → AMF)       ${NC}"
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

log "Starting gNB standalone..."
"$APP" gnb > "$LOG_DIR/gnb_only.log" 2>&1 &
PROC_PID=$!

GNB_OK=false
for i in $(seq 1 10); do
    if grep -qE 'Ng Setup Response|NG-Setup Response|ngSetupResponse|AMF.*Active|State of AMF: Active|PLMNs Identities' "$LOG_DIR/gnb_only.log" 2>/dev/null; then
        GNB_OK=true
        pass "gNB NG-Setup ACCEPTED by AMF ✓"
        break
    fi
    if grep -qE 'NG-Setup [Ff]ailure|AMF is inactive|Setup.*[Ff]ail' "$LOG_DIR/gnb_only.log" 2>/dev/null; then
        fail "gNB NG-Setup REJECTED by AMF"
        echo ""
        cat "$LOG_DIR/gnb_only.log"
        echo ""
        fail "Check: sudo journalctl -u open5gs-amfd -f"
        break
    fi
    sleep 1
done

if [ "$GNB_OK" = false ] && ! grep -qE 'Failure|failure|fatal' "$LOG_DIR/gnb_only.log" 2>/dev/null; then
    warn "NG-Setup status unclear after 10s — showing log:"
    cat "$LOG_DIR/gnb_only.log"
fi

kill "$PROC_PID" 2>/dev/null; PROC_PID=""
rm -f /tmp/gnb.sock /tmp/ue*.sock 2>/dev/null
sleep 1

echo ""
# ── Test 2: Full UE Registration (gNB+UE combined) ────────────────────────────
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}  Test 2: Full UE Registration (gNB+UE combined)    ${NC}"
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

log "Starting full UE test (gNB+UE in one process)..."
"$APP" ue > "$LOG_DIR/ue_full.log" 2>&1 &
PROC_PID=$!

REGISTERED=false
PDU_ACTIVE=false

for i in $(seq 1 30); do
    LOG="$LOG_DIR/ue_full.log"
    if grep -qE 'Registration.*[Cc]omplete|RegistrationAccept|registered|REGISTERED|MM5G_REGISTERED|Registration.*[Aa]ccept' "$LOG" 2>/dev/null; then
        REGISTERED=true
    fi
    if grep -qE 'PDU.*[Ss]ession.*[Ee]stablish|PDUSession.*Active|SM5G_PDU_SESSION_ACTIVE|PDU Session.*[Ss]uccess|tun.*created|uesimtun' "$LOG" 2>/dev/null; then
        PDU_ACTIVE=true
    fi
    [ "$REGISTERED" = true ] && pass "UE Registration ACCEPTED ✓"
    [ "$PDU_ACTIVE"  = true ] && pass "PDU Session ESTABLISHED ✓"
    [ "$REGISTERED" = true ] && [ "$PDU_ACTIVE" = true ] && break

    if ! kill -0 "$PROC_PID" 2>/dev/null; then
        info "Process exited"
        break
    fi
    sleep 1
done

if [ "$REGISTERED" = false ]; then
    fail "UE Registration did NOT complete within 30s"
    echo ""
    info "=== Full log ==="
    cat "$LOG_DIR/ue_full.log"
fi

kill "$PROC_PID" 2>/dev/null; PROC_PID=""
rm -f /tmp/gnb.sock /tmp/ue*.sock 2>/dev/null

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}  Test Summary                                      ${NC}"
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
[ "$GNB_OK"      = true ] && pass "✅ gNB NG-Setup:    PASS" || fail "❌ gNB NG-Setup:    FAIL"
[ "$REGISTERED"  = true ] && pass "✅ UE Registration: PASS" || fail "❌ UE Registration: FAIL"
[ "$PDU_ACTIVE"  = true ] && pass "✅ PDU Session:     PASS" || warn "⚠️  PDU Session:     NOT CONFIRMED"
echo ""
info "Logs:"
info "  gNB only: $LOG_DIR/gnb_only.log"
info "  UE full:  $LOG_DIR/ue_full.log"
echo ""

if [ "$GNB_OK" = true ] && [ "$REGISTERED" = true ]; then
    echo -e "${GREEN}╔══════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  🎉 Core tests PASSED! OmniRAN-Emulator works.  ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Next: try other scenarios:"
    echo "  Mobility registration: edit config.yml → registration_type: mobility"
    echo "  Load test (5 UEs):     ./app load-test -n 5"
    echo "  Monitor AMF:           sudo journalctl -u open5gs-amfd -f"
fi
