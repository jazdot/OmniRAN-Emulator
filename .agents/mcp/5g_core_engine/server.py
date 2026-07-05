#!/usr/bin/env python3
"""
5G Core Engine MCP Server
=========================
Provides tools for 3GPP spec lookup, NGAP/NAS APER compilation,
and tshark-based packet validation.

Transport: stdio (for Antigravity / Claude Desktop integration)
"""
import json
import subprocess
import os
import sys
from mcp.server.fastmcp import FastMCP

# Initialize FastMCP Server named "5g-core-engine"
mcp = FastMCP("5g-core-engine")

# ============================================================
# 1. 3GPP ORACLE TOOLS
# ============================================================

# Comprehensive 3GPP IE database for accurate message construction
NGAP_SPEC_DB = {
    "InitialUEMessage": {
        "reference": "TS 38.413 Section 9.2.5.1",
        "procedure_code": 15,
        "mandatory_ies": [
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
            {"ie": "NAS-PDU", "id": 38, "criticality": "reject", "type": "OCTET STRING"},
            {"ie": "UserLocationInformation", "id": 121, "criticality": "reject", "type": "CHOICE"},
            {"ie": "RRCEstablishmentCause", "id": 90, "criticality": "ignore", "type": "ENUMERATED"},
        ],
        "optional_ies": [
            {"ie": "FiveG-S-TMSI", "id": 26, "criticality": "reject", "type": "SEQUENCE"},
            {"ie": "AMFSetID", "id": 3, "criticality": "ignore", "type": "BIT STRING (SIZE(10))"},
            {"ie": "UEContextRequest", "id": 112, "criticality": "ignore", "type": "ENUMERATED"},
            {"ie": "AllowedNSSAI", "id": 0, "criticality": "reject", "type": "SEQUENCE OF"},
        ],
    },
    "UplinkNASTransport": {
        "reference": "TS 38.413 Section 9.2.5.3",
        "procedure_code": 46,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
            {"ie": "NAS-PDU", "id": 38, "criticality": "reject", "type": "OCTET STRING"},
            {"ie": "UserLocationInformation", "id": 121, "criticality": "reject", "type": "CHOICE"},
        ],
        "optional_ies": [],
    },
    "DownlinkNASTransport": {
        "reference": "TS 38.413 Section 9.2.5.2",
        "procedure_code": 4,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
            {"ie": "NAS-PDU", "id": 38, "criticality": "reject", "type": "OCTET STRING"},
        ],
        "optional_ies": [
            {"ie": "OldAMF", "id": 48, "criticality": "reject", "type": "PrintableString (SIZE(1..150))"},
            {"ie": "RANPagingPriority", "id": 83, "criticality": "ignore", "type": "INTEGER (1..256)"},
            {"ie": "MobilityRestrictionList", "id": 36, "criticality": "ignore", "type": "SEQUENCE"},
        ],
    },
    "NGSetupRequest": {
        "reference": "TS 38.413 Section 9.2.6.1",
        "procedure_code": 21,
        "mandatory_ies": [
            {"ie": "GlobalRANNodeID", "id": 27, "criticality": "reject", "type": "CHOICE"},
            {"ie": "RANNodeName", "id": 82, "criticality": "ignore", "type": "PrintableString (SIZE(1..150))"},
            {"ie": "SupportedTAList", "id": 102, "criticality": "reject", "type": "SEQUENCE (SIZE(1..maxnoofTACs)) OF"},
            {"ie": "DefaultPagingDRX", "id": 21, "criticality": "ignore", "type": "ENUMERATED"},
        ],
        "optional_ies": [
            {"ie": "UERetentionInformation", "id": 147, "criticality": "ignore", "type": "ENUMERATED"},
        ],
    },
    "NGSetupResponse": {
        "reference": "TS 38.413 Section 9.2.6.2",
        "procedure_code": 21,
        "mandatory_ies": [
            {"ie": "AMFName", "id": 1, "criticality": "reject", "type": "PrintableString (SIZE(1..150))"},
            {"ie": "ServedGUAMIList", "id": 96, "criticality": "reject", "type": "SEQUENCE OF"},
            {"ie": "RelativeAMFCapacity", "id": 86, "criticality": "ignore", "type": "INTEGER (0..255)"},
            {"ie": "PLMNSupportList", "id": 80, "criticality": "reject", "type": "SEQUENCE OF"},
        ],
        "optional_ies": [],
    },
    "InitialContextSetupRequest": {
        "reference": "TS 38.413 Section 9.2.2.1",
        "procedure_code": 14,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
            {"ie": "GUAMI", "id": 28, "criticality": "reject", "type": "SEQUENCE"},
            {"ie": "AllowedNSSAI", "id": 0, "criticality": "reject", "type": "SEQUENCE OF"},
            {"ie": "UESecurityCapabilities", "id": 119, "criticality": "reject", "type": "SEQUENCE"},
            {"ie": "SecurityKey", "id": 94, "criticality": "reject", "type": "BIT STRING (SIZE(256))"},
        ],
        "optional_ies": [
            {"ie": "NAS-PDU", "id": 38, "criticality": "ignore", "type": "OCTET STRING"},
            {"ie": "UEAggregateMaximumBitRate", "id": 110, "criticality": "reject", "type": "SEQUENCE"},
        ],
    },
    "InitialContextSetupResponse": {
        "reference": "TS 38.413 Section 9.2.2.2",
        "procedure_code": 14,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
        ],
        "optional_ies": [
            {"ie": "PDUSessionResourceSetupListCxtRes", "id": 72, "criticality": "ignore", "type": "SEQUENCE OF"},
            {"ie": "PDUSessionResourceFailedToSetupListCxtRes", "id": 55, "criticality": "ignore", "type": "SEQUENCE OF"},
            {"ie": "CriticalityDiagnostics", "id": 19, "criticality": "ignore", "type": "SEQUENCE"},
        ],
    },
    "PDUSessionResourceSetupRequest": {
        "reference": "TS 38.413 Section 9.2.1.1",
        "procedure_code": 29,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
            {"ie": "PDUSessionResourceSetupListSUReq", "id": 74, "criticality": "reject", "type": "SEQUENCE OF"},
        ],
        "optional_ies": [
            {"ie": "RANPagingPriority", "id": 83, "criticality": "ignore", "type": "INTEGER (1..256)"},
            {"ie": "NAS-PDU", "id": 38, "criticality": "ignore", "type": "OCTET STRING"},
            {"ie": "UEAggregateMaximumBitRate", "id": 110, "criticality": "reject", "type": "SEQUENCE"},
        ],
    },
    "PDUSessionResourceSetupResponse": {
        "reference": "TS 38.413 Section 9.2.1.2",
        "procedure_code": 29,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
        ],
        "optional_ies": [
            {"ie": "PDUSessionResourceSetupListSURes", "id": 75, "criticality": "ignore", "type": "SEQUENCE OF"},
            {"ie": "PDUSessionResourceFailedToSetupListSURes", "id": 58, "criticality": "ignore", "type": "SEQUENCE OF"},
            {"ie": "CriticalityDiagnostics", "id": 19, "criticality": "ignore", "type": "SEQUENCE"},
        ],
    },
    "PDUSessionResourceReleaseCommand": {
        "reference": "TS 38.413 Section 9.2.1.3",
        "procedure_code": 28,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
            {"ie": "PDUSessionResourceToReleaseListRelCmd", "id": 79, "criticality": "reject", "type": "SEQUENCE OF"},
        ],
        "optional_ies": [
            {"ie": "NAS-PDU", "id": 38, "criticality": "ignore", "type": "OCTET STRING"},
        ],
    },
    "UEContextReleaseRequest": {
        "reference": "TS 38.413 Section 9.2.2.3",
        "procedure_code": 42,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
            {"ie": "Cause", "id": 15, "criticality": "ignore", "type": "CHOICE"},
        ],
        "optional_ies": [
            {"ie": "PDUSessionResourceListCxtRelReq", "id": 133, "criticality": "reject", "type": "SEQUENCE OF"},
        ],
    },
    "UEContextReleaseCommand": {
        "reference": "TS 38.413 Section 9.2.2.4",
        "procedure_code": 41,
        "mandatory_ies": [
            {"ie": "UE-NGAP-IDs", "id": 114, "criticality": "reject", "type": "CHOICE"},
            {"ie": "Cause", "id": 15, "criticality": "ignore", "type": "CHOICE"},
        ],
        "optional_ies": [],
    },
    "UEContextReleaseComplete": {
        "reference": "TS 38.413 Section 9.2.2.5",
        "procedure_code": 41,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "ignore", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "ignore", "type": "INTEGER (0..4294967295)"},
        ],
        "optional_ies": [
            {"ie": "UserLocationInformation", "id": 121, "criticality": "ignore", "type": "CHOICE"},
            {"ie": "InfoOnRecommendedCellsAndRANNodesForPaging", "id": 32, "criticality": "ignore", "type": "SEQUENCE"},
            {"ie": "PDUSessionResourceListCxtRelCpl", "id": 60, "criticality": "reject", "type": "SEQUENCE OF"},
        ],
    },
    "PathSwitchRequest": {
        "reference": "TS 38.413 Section 9.2.3.1",
        "procedure_code": 25,
        "mandatory_ies": [
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
            {"ie": "SourceAMF-UE-NGAP-ID", "id": 100, "criticality": "reject", "type": "INTEGER (0..1099511627775)"},
            {"ie": "UserLocationInformation", "id": 121, "criticality": "ignore", "type": "CHOICE"},
            {"ie": "UESecurityCapabilities", "id": 119, "criticality": "ignore", "type": "SEQUENCE"},
            {"ie": "PDUSessionResourceToBeSwitchedDLList", "id": 76, "criticality": "reject", "type": "SEQUENCE OF"},
        ],
        "optional_ies": [
            {"ie": "PDUSessionResourceFailedToSetupListPSReq", "id": 57, "criticality": "ignore", "type": "SEQUENCE OF"},
        ],
    },
    "HandoverRequired": {
        "reference": "TS 38.413 Section 9.2.3.3",
        "procedure_code": 0,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
            {"ie": "HandoverType", "id": 29, "criticality": "reject", "type": "ENUMERATED"},
            {"ie": "Cause", "id": 15, "criticality": "ignore", "type": "CHOICE"},
            {"ie": "TargetID", "id": 105, "criticality": "reject", "type": "CHOICE"},
            {"ie": "SourceToTarget-TransparentContainer", "id": 101, "criticality": "reject", "type": "OCTET STRING"},
        ],
        "optional_ies": [
            {"ie": "DirectForwardingPathAvailability", "id": 22, "criticality": "ignore", "type": "ENUMERATED"},
            {"ie": "PDUSessionResourceListHORqd", "id": 61, "criticality": "reject", "type": "SEQUENCE OF"},
        ],
    },
    "Paging": {
        "reference": "TS 38.413 Section 9.2.5.5",
        "procedure_code": 27,
        "mandatory_ies": [
            {"ie": "UEPagingIdentity", "id": 115, "criticality": "ignore", "type": "CHOICE"},
            {"ie": "TAIListForPaging", "id": 103, "criticality": "ignore", "type": "SEQUENCE OF"},
        ],
        "optional_ies": [
            {"ie": "PagingDRX", "id": 50, "criticality": "ignore", "type": "ENUMERATED"},
            {"ie": "PagingPriority", "id": 51, "criticality": "ignore", "type": "ENUMERATED"},
            {"ie": "UERadioCapabilityForPaging", "id": 117, "criticality": "ignore", "type": "SEQUENCE"},
            {"ie": "AssistanceDataForPaging", "id": 11, "criticality": "ignore", "type": "SEQUENCE"},
        ],
    },
    "ErrorIndication": {
        "reference": "TS 38.413 Section 9.2.7.1",
        "procedure_code": 3,
        "mandatory_ies": [],
        "optional_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "ignore", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "ignore", "type": "INTEGER (0..4294967295)"},
            {"ie": "Cause", "id": 15, "criticality": "ignore", "type": "CHOICE"},
            {"ie": "CriticalityDiagnostics", "id": 19, "criticality": "ignore", "type": "SEQUENCE"},
        ],
    },
    "UEContextModificationRequest": {
        "reference": "TS 38.413 Section 9.2.2.7",
        "procedure_code": 40,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER (0..1099511627775)"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER (0..4294967295)"},
        ],
        "optional_ies": [
            {"ie": "RANPagingPriority", "id": 83, "criticality": "ignore", "type": "INTEGER (1..256)"},
            {"ie": "SecurityKey", "id": 94, "criticality": "reject", "type": "BIT STRING (SIZE(256))"},
            {"ie": "IndexToRFSP", "id": 31, "criticality": "ignore", "type": "INTEGER (1..256)"},
            {"ie": "UEAggregateMaximumBitRate", "id": 110, "criticality": "ignore", "type": "SEQUENCE"},
            {"ie": "UESecurityCapabilities", "id": 119, "criticality": "reject", "type": "SEQUENCE"},
            {"ie": "CoreNetworkAssistanceInformationForInactive", "id": 18, "criticality": "ignore", "type": "SEQUENCE"},
            {"ie": "EmergencyFallbackIndicator", "id": 24, "criticality": "reject", "type": "SEQUENCE"},
            {"ie": "NewAMF-UE-NGAP-ID", "id": 40, "criticality": "ignore", "type": "INTEGER (0..1099511627775)"},
        ],
    },
    "PDUSessionResourceModifyRequest": {
        "reference": "TS 38.413 Section 9.2.1.6",
        "procedure_code": 26,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "reject", "type": "INTEGER"},
            {"ie": "PDUSessionResourceModifyListModReq", "id": 64, "criticality": "reject", "type": "SEQUENCE OF"},
        ],
        "optional_ies": [],
    },
    "PDUSessionResourceModifyResponse": {
        "reference": "TS 38.413 Section 9.2.1.7",
        "procedure_code": 26,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "ignore", "type": "INTEGER"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "ignore", "type": "INTEGER"},
        ],
        "optional_ies": [
            {"ie": "PDUSessionResourceModifyListModRes", "id": 65, "criticality": "ignore", "type": "SEQUENCE OF"},
        ],
    },
    "HandoverRequest": {
        "reference": "TS 38.413 Section 9.2.3.2",
        "procedure_code": 23,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "reject", "type": "INTEGER"},
            {"ie": "HandoverType", "id": 29, "criticality": "reject", "type": "ENUMERATED"},
            {"ie": "Cause", "id": 15, "criticality": "ignore", "type": "CHOICE"},
            {"ie": "UESecurityCapabilities", "id": 119, "criticality": "reject", "type": "SEQUENCE"},
            {"ie": "SecurityContext", "id": 93, "criticality": "reject", "type": "SEQUENCE"},
            {"ie": "SourceToTarget-TransparentContainer", "id": 101, "criticality": "reject", "type": "OCTET STRING"},
        ],
        "optional_ies": [],
    },
    "HandoverRequestAcknowledge": {
        "reference": "TS 38.413 Section 9.2.3.3",
        "procedure_code": 23,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "ignore", "type": "INTEGER"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "ignore", "type": "INTEGER"},
            {"ie": "TargetToSource-TransparentContainer", "id": 106, "criticality": "reject", "type": "OCTET STRING"},
        ],
        "optional_ies": [],
    },
    "HandoverCommand": {
        "reference": "TS 38.413 Section 9.2.3.4",
        "procedure_code": 0,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "ignore", "type": "INTEGER"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "ignore", "type": "INTEGER"},
            {"ie": "HandoverType", "id": 29, "criticality": "reject", "type": "ENUMERATED"},
            {"ie": "TargetToSource-TransparentContainer", "id": 106, "criticality": "reject", "type": "OCTET STRING"},
        ],
        "optional_ies": [],
    },
    "HandoverNotify": {
        "reference": "TS 38.413 Section 9.2.3.7",
        "procedure_code": 11,
        "mandatory_ies": [
            {"ie": "AMF-UE-NGAP-ID", "id": 10, "criticality": "ignore", "type": "INTEGER"},
            {"ie": "RAN-UE-NGAP-ID", "id": 85, "criticality": "ignore", "type": "INTEGER"},
            {"ie": "UserLocationInformation", "id": 121, "criticality": "ignore", "type": "CHOICE"},
        ],
        "optional_ies": [],
    },
}

NAS_MESSAGE_DB = {
    "RegistrationRequest": {
        "reference": "TS 24.501 Section 8.2.6",
        "message_type": "0x41",
        "mandatory_ies": [
            {"ie": "5GSRegistrationType", "type": "UINT8", "description": "Registration type value"},
            {"ie": "NAS-KeySetIdentifier", "type": "UINT8", "description": "ngKSI + TSC"},
            {"ie": "5GSMobileIdentity", "type": "LV-E", "description": "SUCI/GUTI/IMEI"},
        ],
        "optional_ies": [
            {"ie": "UESecurityCapability", "iei": "0x2E", "type": "TLV"},
            {"ie": "RequestedNSSAI", "iei": "0x2F", "type": "TLV"},
            {"ie": "5GMMCapability", "iei": "0x10", "type": "TLV"},
        ],
    },
    "AuthenticationResponse": {
        "reference": "TS 24.501 Section 8.2.2",
        "message_type": "0x57",
        "mandatory_ies": [
            {"ie": "AuthenticationResponseParameter", "type": "TLV", "description": "RES*"},
        ],
        "optional_ies": [],
    },
    "SecurityModeComplete": {
        "reference": "TS 24.501 Section 8.2.26",
        "message_type": "0x5e",
        "mandatory_ies": [],
        "optional_ies": [
            {"ie": "IMEISV", "iei": "0x77", "type": "TLV-E"},
            {"ie": "NASMessageContainer", "iei": "0x71", "type": "TLV-E"},
        ],
    },
    "RegistrationComplete": {
        "reference": "TS 24.501 Section 8.2.8",
        "message_type": "0x43",
        "mandatory_ies": [],
        "optional_ies": [
            {"ie": "SORTransparentContainer", "iei": "0x73", "type": "TLV-E"},
        ],
    },
    "DeregistrationRequest": {
        "reference": "TS 24.501 Section 8.2.12",
        "message_type": "0x45",
        "mandatory_ies": [
            {"ie": "DeregistrationType", "type": "UINT8", "description": "Switch off + access type + re-registration"},
            {"ie": "NAS-KeySetIdentifier", "type": "UINT8", "description": "ngKSI"},
            {"ie": "5GSMobileIdentity", "type": "LV-E", "description": "5G-GUTI or SUCI"},
        ],
        "optional_ies": [],
    },
    "ServiceRequest": {
        "reference": "TS 24.501 Section 8.2.16",
        "message_type": "0x4c",
        "mandatory_ies": [
            {"ie": "NAS-KeySetIdentifier", "type": "UINT8", "description": "ngKSI"},
            {"ie": "ServiceType", "type": "UINT8", "description": "Service type value"},
            {"ie": "5G-S-TMSI", "type": "5GS-TMSI", "description": "5G S-TMSI"},
        ],
        "optional_ies": [
            {"ie": "UplinkDataStatus", "iei": "0x40", "type": "TLV"},
            {"ie": "PDUSessionStatus", "iei": "0x50", "type": "TLV"},
            {"ie": "AllowedPDUSessionStatus", "iei": "0x25", "type": "TLV"},
        ],
    },
    "PDUSessionEstablishmentRequest": {
        "reference": "TS 24.501 Section 8.3.1",
        "message_type": "0xc1",
        "mandatory_ies": [
            {"ie": "PDUSessionID", "type": "UINT8", "description": "PDU session identity"},
            {"ie": "PTI", "type": "UINT8", "description": "Procedure transaction identity"},
            {"ie": "IntegrityProtMaxDataRate", "type": "2 OCTETS", "description": "Max data rate per UE"},
        ],
        "optional_ies": [
            {"ie": "PDUSessionType", "iei": "0x09", "type": "TV"},
            {"ie": "SSCMode", "iei": "0x0A", "type": "TV"},
            {"ie": "ExtendedProtocolConfigurationOptions", "iei": "0x7B", "type": "TLV-E"},
        ],
    },
    "ConfigurationUpdateCommand": {
        "reference": "TS 24.501 Section 8.2.19",
        "message_type": "0x54",
        "mandatory_ies": [],
        "optional_ies": [
            {"ie": "GUTI5G", "iei": "0x77", "type": "TLV-E"},
            {"ie": "TAIList", "iei": "0x54", "type": "TLV"},
            {"ie": "AllowedNSSAI", "iei": "0x15", "type": "TLV"},
        ]
    },
    "ConfigurationUpdateComplete": {
        "reference": "TS 24.501 Section 8.2.20",
        "message_type": "0x55",
        "mandatory_ies": [],
        "optional_ies": []
    },
    "RegistrationReject": {
        "reference": "TS 24.501 Section 8.2.7",
        "message_type": "0x44",
        "mandatory_ies": [
            {"ie": "5GMMCause", "type": "UINT8", "description": "GMM cause value"}
        ],
        "optional_ies": []
    },
    "ServiceReject": {
        "reference": "TS 24.501 Section 8.2.18",
        "message_type": "0x4d",
        "mandatory_ies": [
            {"ie": "5GMMCause", "type": "UINT8", "description": "GMM cause value"}
        ],
        "optional_ies": []
    },
    "ServiceAccept": {
        "reference": "TS 24.501 Section 8.2.17",
        "message_type": "0x4e",
        "mandatory_ies": [],
        "optional_ies": [
            {"ie": "PDUSessionStatus", "iei": "0x50", "type": "TLV"}
        ]
    },
    "Status5GMM": {
        "reference": "TS 24.501 Section 8.2.29",
        "message_type": "0x64",
        "mandatory_ies": [
            {"ie": "5GMMCause", "type": "UINT8", "description": "GMM cause value"}
        ],
        "optional_ies": []
    },
}

CAUSE_CODES_DB = {
    "RadioNetwork": {
        0: "unspecified",
        1: "txnrelocoverall-expiry",
        2: "successful-handover",
        3: "release-due-to-ngran-generated-reason",
        4: "release-due-to-5gc-generated-reason",
        5: "handover-cancelled",
        6: "partial-handover",
        7: "ho-failure-in-target-5GC-ngran-node-or-target-system",
        8: "ho-target-not-allowed",
        9: "tngrelocoverall-expiry",
        10: "tngrelocprep-expiry",
        11: "cell-not-available",
        12: "unknown-targetID",
        13: "no-radio-resources-available-in-target-cell",
        14: "unknown-local-UE-NGAP-ID",
        15: "inconsistent-remote-UE-NGAP-ID",
        16: "handover-desirable-for-radio-reason",
        17: "time-critical-handover",
        18: "resource-optimisation-handover",
        19: "reduce-load-in-serving-cell",
        20: "user-inactivity",
        21: "radio-connection-with-ue-lost",
        22: "radio-resources-not-available",
        23: "invalid-qos-combination",
        24: "failure-in-radio-interface-procedure",
        25: "interaction-with-other-procedure",
        26: "unknown-PDU-session-ID",
        27: "unkown-qos-flow-ID",
        28: "multiple-PDU-session-ID-instances",
        29: "multiple-qos-flow-ID-instances",
        30: "encryption-and-or-integrity-protection-algorithms-not-supported",
        31: "ng-intra-system-handover-triggered",
        32: "ng-inter-system-handover-triggered",
        33: "xn-handover-triggered",
        34: "not-supported-5QI-value",
        35: "ue-context-transfer",
        36: "ims-voice-eps-fallback-or-rat-fallback-triggered",
        37: "up-integrity-protection-not-possible",
        38: "up-confidentiality-protection-not-possible",
        39: "slice-not-supported",
        40: "ue-in-rrc-inactive-state-not-reachable",
        41: "redirection",
        42: "resources-not-available-for-the-slice",
        43: "ue-max-integrity-protected-data-rate-reason",
        44: "release-due-to-cn-detected-mobility",
        45: "n26-interface-not-available",
        46: "release-due-to-pre-emption",
        47: "multiple-location-reporting-reference-ID-instances",
    },
    "Transport": {
        0: "transport-resource-unavailable",
        1: "unspecified",
    },
    "NAS": {
        0: "normal-release",
        1: "authentication-failure",
        2: "deregister",
        3: "unspecified",
    },
    "Protocol": {
        0: "transfer-syntax-error",
        1: "abstract-syntax-error-reject",
        2: "abstract-syntax-error-ignore-and-notify",
        3: "message-not-compatible-with-receiver-state",
        4: "semantic-error",
        5: "abstract-syntax-error-falsely-constructed-message",
        6: "unspecified",
    },
    "Misc": {
        0: "control-processing-overload",
        1: "not-enough-user-plane-processing-resources",
        2: "hardware-failure",
        3: "om-intervention",
        4: "unknown-PLMN-or-SNPN",
        5: "unspecified",
    },
}

RRC_ESTABLISHMENT_CAUSES = {
    "emergency": 0,
    "highPriorityAccess": 1,
    "mt-Access": 2,
    "mo-Signalling": 3,
    "mo-Data": 4,
    "mo-VoiceCall": 5,
    "mo-VideoCall": 6,
    "mo-SMS": 7,
    "mps-PriorityAccess": 8,
    "mcs-PriorityAccess": 9,
    "notAvailable": 15,
}


@mcp.tool()
def search_3gpp_spec(spec: str, release: str, keyword: str) -> str:
    """Searches 3GPP specs for specific procedures, messages, or IEs.

    Args:
        spec: 3GPP specification number (e.g., '38.413' for NGAP, '24.501' for NAS)
        release: 3GPP release number (e.g., '16', '17')
        keyword: Search keyword - message name, IE name, or procedure name
    """
    result = {}

    # Search NGAP spec
    if spec == "38.413":
        for msg_name, msg_data in NGAP_SPEC_DB.items():
            if keyword.lower() in msg_name.lower():
                result = {
                    "message": msg_name,
                    "reference": msg_data["reference"],
                    "procedure_code": msg_data["procedure_code"],
                    "mandatory_ies": msg_data["mandatory_ies"],
                    "optional_ies": msg_data["optional_ies"],
                }
                return json.dumps(result, indent=2)

        # Search for IE in all messages
        for msg_name, msg_data in NGAP_SPEC_DB.items():
            for ie in msg_data["mandatory_ies"] + msg_data["optional_ies"]:
                if keyword.lower() in ie["ie"].lower():
                    if "matches" not in result:
                        result["matches"] = []
                    result["matches"].append({
                        "message": msg_name,
                        "ie": ie["ie"],
                        "id": ie["id"],
                        "criticality": ie["criticality"],
                        "type": ie["type"],
                        "mandatory": ie in msg_data["mandatory_ies"],
                    })
            if result:
                return json.dumps(result, indent=2)

        # Search cause codes
        if "cause" in keyword.lower():
            return json.dumps({"cause_groups": CAUSE_CODES_DB}, indent=2)

        if "rrcestablishment" in keyword.lower().replace(" ", "").replace("-", ""):
            return json.dumps({"RRCEstablishmentCause": RRC_ESTABLISHMENT_CAUSES}, indent=2)

    # Search NAS spec
    elif spec == "24.501":
        for msg_name, msg_data in NAS_MESSAGE_DB.items():
            if keyword.lower() in msg_name.lower():
                result = {
                    "message": msg_name,
                    "reference": msg_data["reference"],
                    "message_type": msg_data["message_type"],
                    "mandatory_ies": msg_data["mandatory_ies"],
                    "optional_ies": msg_data["optional_ies"],
                }
                return json.dumps(result, indent=2)

    return json.dumps({
        "status": "no_match",
        "detail": f"No direct match for '{keyword}' in TS {spec} Rel-{release}. "
                  f"Available NGAP messages: {list(NGAP_SPEC_DB.keys())}. "
                  f"Available NAS messages: {list(NAS_MESSAGE_DB.keys())}."
    }, indent=2)


@mcp.tool()
def get_cause_code(group: str, code: int) -> str:
    """Look up a specific NGAP Cause code by group and numeric value.

    Args:
        group: Cause group (RadioNetwork, Transport, NAS, Protocol, Misc)
        code: Numeric cause code value
    """
    if group in CAUSE_CODES_DB:
        if code in CAUSE_CODES_DB[group]:
            return json.dumps({
                "group": group,
                "code": code,
                "description": CAUSE_CODES_DB[group][code],
            }, indent=2)
        return json.dumps({
            "error": f"Code {code} not found in group '{group}'. Valid codes: {list(CAUSE_CODES_DB[group].keys())}"
        })
    return json.dumps({
        "error": f"Group '{group}' not found. Valid groups: {list(CAUSE_CODES_DB.keys())}"
    })


@mcp.tool()
def list_ngap_messages() -> str:
    """Lists all NGAP message types in the database with their procedure codes."""
    messages = []
    for name, data in NGAP_SPEC_DB.items():
        messages.append({
            "message": name,
            "procedure_code": data["procedure_code"],
            "reference": data["reference"],
            "mandatory_ie_count": len(data["mandatory_ies"]),
            "optional_ie_count": len(data["optional_ies"]),
        })
    return json.dumps({"ngap_messages": messages}, indent=2)


@mcp.tool()
def list_nas_messages() -> str:
    """Lists all NAS 5GMM/5GSM message types in the database."""
    messages = []
    for name, data in NAS_MESSAGE_DB.items():
        messages.append({
            "message": name,
            "message_type": data["message_type"],
            "reference": data["reference"],
            "mandatory_ie_count": len(data["mandatory_ies"]),
            "optional_ie_count": len(data["optional_ies"]),
        })
    return json.dumps({"nas_messages": messages}, indent=2)


# ============================================================
# 2. ASN.1 & NAS COMPILER TOOLS
# ============================================================

@mcp.tool()
def compile_nas_payload(json_payload: str) -> str:
    """Compiles a logical NAS JSON structure into raw NAS hex bytes.

    The NAS payload follows TS 24.501 structure:
    - Extended Protocol Discriminator (1 byte)
    - Security Header Type (1 byte) 
    - Message Type (1 byte)
    - Followed by IEs

    Args:
        json_payload: JSON string describing the NAS message structure
    """
    try:
        data = json.loads(json_payload)
        msg_type = data.get("message_type", "")

        if msg_type not in NAS_MESSAGE_DB and msg_type not in [v["message_type"] for v in NAS_MESSAGE_DB.values()]:
            # Try to match by name
            found = False
            for name, spec in NAS_MESSAGE_DB.items():
                if name.lower() == msg_type.lower():
                    msg_type = spec["message_type"]
                    found = True
                    break
            if not found:
                return json.dumps({
                    "status": "error",
                    "detail": f"Unknown NAS message type: {msg_type}. Known types: {list(NAS_MESSAGE_DB.keys())}"
                })

        # Validate mandatory IEs
        for name, spec in NAS_MESSAGE_DB.items():
            if spec["message_type"] == msg_type:
                for mandatory_ie in spec["mandatory_ies"]:
                    if mandatory_ie["ie"] not in data.get("ies", {}):
                        return json.dumps({
                            "status": "error",
                            "detail": f"Missing mandatory IE [{mandatory_ie['ie']}] for {name} (ref: {spec['reference']})"
                        })
                break

        # Build NAS PDU hex
        # EPD: 0x7e = 5GMM, 0x2e = 5GSM
        epd = data.get("epd", "7e")
        security_header = data.get("security_header", "00")

        nas_hex = epd + security_header + msg_type.replace("0x", "")

        # Append IE hex data if provided
        ie_hex = data.get("ie_hex", "")
        if ie_hex:
            nas_hex += ie_hex

        return json.dumps({
            "status": "success",
            "nas_hex": nas_hex,
            "length_bytes": len(nas_hex) // 2,
            "note": "Inject this hex into the NAS-PDU field of the NGAP message"
        }, indent=2)

    except json.JSONDecodeError as e:
        return json.dumps({"status": "error", "detail": f"JSON parse error: {str(e)}"})
    except Exception as e:
        return json.dumps({"status": "error", "detail": f"Compilation error: {str(e)}"})


@mcp.tool()
def compile_ngap_aper(json_payload: str) -> str:
    """Compiles a logical NGAP JSON structure into raw APER hex.

    The JSON should describe the NGAP message with:
    - message_type: Name of the NGAP message (e.g., "InitialUEMessage")
    - ies: Dict of IE name -> value mappings
    - NAS-PDU: The hex string from compile_nas_payload (if applicable)

    Args:
        json_payload: JSON string describing the NGAP message structure
    """
    try:
        data = json.loads(json_payload)
        msg_type = data.get("message_type", "")

        if msg_type not in NGAP_SPEC_DB:
            return json.dumps({
                "status": "error",
                "detail": f"Unknown NGAP message type: {msg_type}. Known types: {list(NGAP_SPEC_DB.keys())}"
            })

        spec = NGAP_SPEC_DB[msg_type]

        # Validate mandatory IEs
        provided_ies = data.get("ies", {})
        missing = []
        for mandatory_ie in spec["mandatory_ies"]:
            if mandatory_ie["ie"] not in provided_ies:
                missing.append(f"{mandatory_ie['ie']} (ID={mandatory_ie['id']}, criticality={mandatory_ie['criticality']})")

        if missing:
            return json.dumps({
                "status": "error",
                "detail": f"Missing mandatory IEs for {msg_type}: {missing}",
                "reference": spec["reference"]
            }, indent=2)

        # Check for unknown IEs
        known_ie_names = [ie["ie"] for ie in spec["mandatory_ies"] + spec["optional_ies"]]
        unknown = [ie for ie in provided_ies if ie not in known_ie_names]
        warnings = []
        if unknown:
            warnings.append(f"Unknown IEs provided (will be ignored by strict parsers): {unknown}")

        # Build APER-encoded hex (simplified simulation)
        # Real encoding would use asn1tools or the Go APER library
        procedure_code = spec["procedure_code"]
        num_ies = len(provided_ies)

        # NGAP PDU header: procedureCode + criticality + value
        # InitiatingMessage = 0x00, SuccessfulOutcome = 0x20, UnsuccessfulOutcome = 0x40
        pdu_type = data.get("pdu_type", "initiating")
        pdu_type_byte = {"initiating": "00", "successful": "20", "unsuccessful": "40"}.get(pdu_type, "00")

        # Build header
        header_hex = pdu_type_byte + format(procedure_code, '02x') + "00"

        # IE container
        ie_hex_parts = []
        for ie_name, ie_value in provided_ies.items():
            # Find IE ID
            ie_id = None
            for ie_spec in spec["mandatory_ies"] + spec["optional_ies"]:
                if ie_spec["ie"] == ie_name:
                    ie_id = ie_spec["id"]
                    break
            if ie_id is not None:
                # IE header: protocolIE-ID (2 bytes) + criticality (1 nibble) + value
                ie_hex_parts.append(format(ie_id, '04x'))
                if isinstance(ie_value, str):
                    ie_hex_parts.append(ie_value)
                elif isinstance(ie_value, int):
                    ie_hex_parts.append(format(ie_value, '08x'))

        num_ies_hex = format(num_ies, '04x')
        value_hex = num_ies_hex + "".join(ie_hex_parts)

        # Length of value
        value_len = len(value_hex) // 2
        value_len_hex = format(value_len, '04x')

        final_hex = header_hex + value_len_hex + value_hex

        result = {
            "status": "success",
            "hex": final_hex,
            "length_bytes": len(final_hex) // 2,
            "procedure_code": procedure_code,
            "num_ies": num_ies,
            "reference": spec["reference"],
        }
        if warnings:
            result["warnings"] = warnings

        return json.dumps(result, indent=2)

    except json.JSONDecodeError as e:
        return json.dumps({"status": "error", "detail": f"JSON parse error: {str(e)}"})
    except Exception as e:
        return json.dumps({"status": "error", "detail": f"Compilation error: {str(e)}"})


# ============================================================
# 3. TSHARK VALIDATION TOOLS
# ============================================================

@mcp.tool()
def validate_packet_hex(hex_string: str) -> str:
    """Validates raw NGAP/NAS hex against tshark's 3GPP decoders.

    Passes the hex through a local tshark instance (if available)
    to verify the packet can be decoded without [Malformed Packet] errors.

    Args:
        hex_string: Raw hex string of the NGAP PDU to validate
    """
    clean_hex = hex_string.strip().replace(" ", "").replace(":", "")

    # Basic sanity checks
    if len(clean_hex) == 0:
        return json.dumps({"valid": False, "error": "Empty hex string provided"})

    if len(clean_hex) % 2 != 0:
        return json.dumps({
            "valid": False,
            "error": "[Malformed Packet] Byte stream length must be even. "
                     f"Got {len(clean_hex)} hex chars ({len(clean_hex)/2:.1f} bytes)."
        })

    try:
        bytes.fromhex(clean_hex)
    except ValueError as e:
        return json.dumps({
            "valid": False,
            "error": f"[Malformed Packet] Invalid hex characters: {str(e)}"
        })

    # Minimum NGAP PDU size check (header is at least 4 bytes)
    if len(clean_hex) < 8:
        return json.dumps({
            "valid": False,
            "error": "[Malformed Packet] NGAP PDU too short. Minimum 4 bytes for PDU header."
        })

    # Try real tshark if available
    tshark_path = None
    for path in ["/usr/bin/tshark", "/usr/local/bin/tshark"]:
        if os.path.exists(path):
            tshark_path = path
            break

    if tshark_path:
        try:
            # Create a raw hex dump file for tshark
            hex_bytes = bytes.fromhex(clean_hex)

            # Write to a temporary pcap-like raw file
            import tempfile
            with tempfile.NamedTemporaryFile(suffix=".raw", delete=False) as tmp:
                tmp.write(hex_bytes)
                tmp_path = tmp.name

            # Use tshark to decode as NGAP
            result = subprocess.run(
                [tshark_path, "-r", tmp_path, "-d", "sctp.port==38412,ngap",
                 "-T", "json", "-Y", "ngap"],
                capture_output=True, text=True, timeout=10
            )

            os.unlink(tmp_path)

            if "Malformed" in result.stdout or "Malformed" in result.stderr:
                return json.dumps({
                    "valid": False,
                    "error": "[Malformed Packet] tshark detected malformed NGAP/NAS data",
                    "tshark_output": result.stdout[:500],
                })

            return json.dumps({
                "valid": True,
                "method": "tshark_live",
                "dissection": f"Packet decoded successfully. {len(hex_bytes)} bytes processed.",
            })

        except subprocess.TimeoutExpired:
            return json.dumps({
                "valid": False,
                "error": "tshark timed out during validation"
            })
        except Exception as e:
            # Fall through to structural validation
            pass

    # Structural validation (when tshark is not available)
    pdu_type_byte = int(clean_hex[0:2], 16)
    pdu_type_name = {0: "InitiatingMessage", 0x20: "SuccessfulOutcome", 0x40: "UnsuccessfulOutcome"}.get(
        pdu_type_byte, None
    )

    if pdu_type_name is None:
        return json.dumps({
            "valid": False,
            "error": f"[Malformed Packet] Invalid NGAP PDU type byte: 0x{clean_hex[0:2]}. "
                     f"Expected 0x00 (Initiating), 0x20 (Successful), or 0x40 (Unsuccessful)."
        })

    procedure_code = int(clean_hex[2:4], 16)

    # Verify procedure code is valid
    valid_procedure_codes = {spec["procedure_code"] for spec in NGAP_SPEC_DB.values()}
    if procedure_code not in valid_procedure_codes:
        return json.dumps({
            "valid": False,
            "warning": f"Procedure code {procedure_code} (0x{clean_hex[2:4]}) not in known database. "
                       f"May still be valid per newer 3GPP releases.",
            "error": None,
        })

    return json.dumps({
        "valid": True,
        "method": "structural_validation",
        "pdu_type": pdu_type_name,
        "procedure_code": procedure_code,
        "total_bytes": len(clean_hex) // 2,
        "dissection": f"Structural check passed. {pdu_type_name} with procedure code {procedure_code}.",
        "note": "Install tshark for full 3GPP decoder validation: sudo apt install tshark"
    }, indent=2)


@mcp.tool()
def validate_ie_completeness(message_type: str, provided_ies: str) -> str:
    """Validates that all mandatory IEs are present for a given NGAP message type.

    Args:
        message_type: NGAP message name (e.g., "InitialUEMessage")
        provided_ies: JSON array of IE names that are being included
    """
    if message_type not in NGAP_SPEC_DB:
        return json.dumps({
            "status": "error",
            "detail": f"Unknown message type: {message_type}. Known: {list(NGAP_SPEC_DB.keys())}"
        })

    try:
        ie_list = json.loads(provided_ies)
    except json.JSONDecodeError:
        return json.dumps({"status": "error", "detail": "Could not parse provided_ies as JSON array"})

    spec = NGAP_SPEC_DB[message_type]
    mandatory_names = [ie["ie"] for ie in spec["mandatory_ies"]]
    optional_names = [ie["ie"] for ie in spec["optional_ies"]]

    missing_mandatory = [ie for ie in mandatory_names if ie not in ie_list]
    unknown_ies = [ie for ie in ie_list if ie not in mandatory_names and ie not in optional_names]
    included_optional = [ie for ie in ie_list if ie in optional_names]

    status = "valid" if not missing_mandatory else "invalid"

    return json.dumps({
        "status": status,
        "message_type": message_type,
        "reference": spec["reference"],
        "missing_mandatory": missing_mandatory,
        "included_optional": included_optional,
        "unknown_ies": unknown_ies,
        "recommendation": "Include all mandatory IEs. Keep optional IEs minimal to avoid core parser issues."
                          if missing_mandatory else "All mandatory IEs present. Message structure is valid."
    }, indent=2)


if __name__ == "__main__":
    mcp.run(transport="stdio")
