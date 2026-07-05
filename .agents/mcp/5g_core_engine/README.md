# 5G Core Engine MCP Server

A Model Context Protocol server providing 3GPP-compliant tools for building and validating NGAP/NAS messages.

## Tools Provided

| Tool | Description |
|------|-------------|
| `search_3gpp_spec` | Search TS 38.413 (NGAP) or TS 24.501 (NAS) for message/IE definitions |
| `get_cause_code` | Look up NGAP Cause codes by group and numeric value |
| `list_ngap_messages` | List all supported NGAP message types with procedure codes |
| `list_nas_messages` | List all supported NAS message types |
| `compile_nas_payload` | Compile NAS JSON to raw hex bytes (inner Matryoshka layer) |
| `compile_ngap_aper` | Compile NGAP JSON to APER hex (outer Matryoshka layer) |
| `validate_packet_hex` | Validate hex against tshark decoders (or structural fallback) |
| `validate_ie_completeness` | Check mandatory IE presence for a message type |

## Setup

```bash
cd .agents/mcp/5g_core_engine
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

## Antigravity MCP Configuration

Add to `~/.gemini/config/settings.json`:

```json
{
  "mcpServers": {
    "5g-core-engine": {
      "command": "/home/richu/OmniRAN-Emulator/.agents/mcp/5g_core_engine/.venv/bin/python3",
      "args": ["/home/richu/OmniRAN-Emulator/.agents/mcp/5g_core_engine/server.py"],
      "transportType": "stdio"
    }
  }
}
```

## Workflow (Two-Step Matryoshka Protocol)

1. **Draft NAS JSON** → `compile_nas_payload` → get `nas_hex`
2. **Draft NGAP JSON** with `NAS-PDU: nas_hex` → `compile_ngap_aper` → get `ngap_hex`
3. **Validate** → `validate_packet_hex` with `ngap_hex`
4. **Fix** any `[Malformed Packet]` errors and recompile
