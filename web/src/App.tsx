import React, { useState, useEffect, useRef } from 'react';
import {
  Activity,
  Play,
  Settings,
  Terminal,
  Network,
  Radio,
  Cpu,
  Server,
  AlertTriangle,
  RefreshCw,
  Globe,
  Layers,
  Trash2,
  Sliders,
  Sun,
  Moon,
  Phone,
  Tv,
  Download,
  Search,
  FileText
} from 'lucide-react';

// API base path (works with relative path when served by Go, or proxied in dev)
const API_BASE = '/api';

interface PDUSession {
  Id: number;
  Dnn: string;
  PduSessionType: string;
  Sst: number;
  Sd: string;
}

interface ConfigData {
  GNodeB: {
    ControlIF: { Ip: string; Port: number };
    DataIF: { Ip: string; Port: number };
    PlmnList: { Mcc: string; Mnc: string; Tac: string; GnbId: string };
    SliceSupportList: { Sst: string; Sd: string };
    LinkType: string;
    LinkPort: number;
  };
  Ue: {
    Msin: string;
    Key: string;
    Opc: string;
    Amf: string;
    Sqn: string;
    Dnn: string;
    PduSessionType: string;
    RegistrationType: string;
    Hplmn: { Mcc: string; Mnc: string };
    Snssai: { Sst: number; Sd: string };
    PduSessions: PDUSession[] | null;
  };
  AMF: { Ip: string; Port: number };
  Logs: { Level: number };
}

interface NetworkInterface {
  name: string;
  ips: string[];
}

interface RunningGNB {
  profileName: string;
  gnbId: string;
  startedAt: string;
  state: string;
  linkType: string;
  linkPort: number;
  controlIp: string;
  socketPath?: string;
  mcc: string;
  mnc: string;
  tac: string;
  connectedUes?: string[];
}

interface StatusData {
  isRunning: boolean;
  runningName: string;
  interfaces: NetworkInterface[];
  gnbLinkState: string;
  configSummary: {
    ueImsi: string;
    ueKey: string;
    ueOpc: string;
    ueSlice: string;
    gnbControl: string;
    amfTarget: string;
  };
  runningGnbs?: RunningGNB[];
  runningUes?: RunningUE[];
}

interface RunningUE {
  id: number;
  supi: string;
  stateMm: number;
  stateMmDesc: string;
  stateSm: number;
  stateSmDesc: string;
  gnbControlIp: string;
  gnbId?: string;
  gnbProfileName?: string;
  amfUeNgapId?: number;
  pduSessions: { id: number; ueIp: string; dnn: string; stateDesc: string }[];
}

interface Scenario {
  id: string;
  name: string;
  description: string;
}

interface LogMsg {
  text: string;
  level: 'info' | 'warn' | 'error' | 'debug' | 'system';
  timestamp: string;
}

interface DecodedIE {
  name: string;
  value?: string;
  indent: number;
  release?: string;
  highlight?: boolean;
  comment?: string;
}

interface DecodedMessageData {
  name: string;
  hex: string;
  ies: DecodedIE[];
  description: string;
  specRef: string;
}

const getDecodedPacketData = (msgId: string, rel: string): DecodedMessageData => {
  const isR17 = rel === '17' || rel === '18' || rel === '19';
  const isR18 = rel === '18' || rel === '19';
  const isR19 = rel === '19';

  switch (msgId) {
    case 'ng-setup': {
      const ies: DecodedIE[] = [
        { name: 'NGAP-PDU: initiatingMessage', indent: 0 },
        { name: 'procedureCode: id-NGSetup (21)', indent: 1 },
        { name: 'criticality: reject (0)', indent: 1 },
        { name: 'value: NGSetupRequest', indent: 1 },
        { name: 'protocolIEs: ProtocolIE-Container', indent: 2 },
        { name: 'IE [id=GlobalRANNodeID (27)]', indent: 3 },
        { name: 'value: GlobalGNB-ID', indent: 4 },
        { name: 'plmnIdentity: 999-70', indent: 5 },
        { name: 'gnb-ID: macroGNB-ID (0x000001)', indent: 5 },
        { name: 'IE [id=RANNodeName (95)]', indent: 3 },
        { name: 'value: "OmniRAN-gNodeB"', indent: 4 },
        { name: 'IE [id=SupportedTAIList (102)]', indent: 3 },
        { name: 'SupportedTAI-Item', indent: 4 },
        { name: 'tai: TAC: 0x000001, PLMN: 999-70', indent: 5 },
        { name: 'sliceSupportList: SupportedSlicesList', indent: 5 },
        { name: 'SliceSupportItem: SST: 1, SD: 010203', indent: 6 }
      ];

      let hex = '0015003f000003001b00050000000001005f000b00084f6d6e6952414e0066001000050050f001000001';

      if (isR17) {
        ies.push(
          { name: 'IE [id=NTN-AccessInformation (145)]', indent: 3, release: '17', highlight: true },
          { name: 'value: satellite-access-allowed (1)', indent: 4, release: '17', highlight: true, comment: 'Allows access from Non-Terrestrial Network (NTN) satellite cells' },
          { name: 'IE [id=RedCap-CellAccessInfo (146)]', indent: 3, release: '17', highlight: true },
          { name: 'value: redcap-access-allowed (1)', indent: 4, release: '17', highlight: true, comment: 'Allows reduced capability (RedCap) UEs to camp on this GNodeB cell' }
        );
        hex += '009100020101009200020101';
      }
      if (isR18) {
        ies.push(
          { name: 'IE [id=PagingEarlyIndicationSupport (163)]', indent: 3, release: '18', highlight: true },
          { name: 'value: supported (0)', indent: 4, release: '18', highlight: true, comment: 'R18 PEI enables power savings for eRedCap and stationary UEs' }
        );
        hex += '00a300020000';
      }
      if (isR19) {
        ies.push(
          { name: 'IE [id=AmbientIoT-GatewayCapabilities (177)]', indent: 3, release: '19', highlight: true },
          { name: 'value: sensor-relay-active (0)', indent: 4, release: '19', highlight: true, comment: 'Allows reading/relaying passive ambient IoT tags (RFID-style battery-free devices)' },
          { name: 'IE [id=RAN-AI-Capabilities (194)]', indent: 3, release: '19', highlight: true },
          { name: 'value: model-inference-supported (1)', indent: 4, release: '19', highlight: true, comment: 'Supports deployment and inference of edge AI/ML models on the GNodeB' }
        );
        hex += '00b10002010000c200020101';
      }

      return {
        name: 'NG Setup Request',
        hex,
        ies,
        specRef: '3GPP TS 38.413, Section 9.1.8.1',
        description: 'Initiated by the GNodeB to configure and establish the N2 control interface association with the AMF core, exchanging GNodeB capability flags and PLMN/TAC mappings.'
      };
    }
    case 'initial-ue': {
      const ies: DecodedIE[] = [
        { name: 'NGAP-PDU: initiatingMessage', indent: 0 },
        { name: 'procedureCode: id-InitialUEMessage (15)', indent: 1 },
        { name: 'criticality: ignore (1)', indent: 1 },
        { name: 'value: InitialUEMessage', indent: 1 },
        { name: 'protocolIEs: ProtocolIE-Container', indent: 2 },
        { name: 'IE [id=RAN-UE-NGAP-ID (85)]', indent: 3 },
        { name: 'value: 1', indent: 4 },
        { name: 'IE [id=NAS-PDU (38)]', indent: 3 },
        { name: 'value: 5GMM Registration Request (0x7e004101...)', indent: 4 },
        { name: 'IE [id=UserLocationInformation (121)]', indent: 3 },
        { name: 'value: UserLocationInformationNR', indent: 4 },
        { name: 'nr-CGI: PLMN: 999-70, NR Cell ID: 0x000001', indent: 5 },
        { name: 'tai: TAC: 0x000001, PLMN: 999-70', indent: 5 },
        { name: 'IE [id=RRCEstablishmentCause (90)]', indent: 3 },
        { name: 'value: mo-Signalling (2)', indent: 4 }
      ];

      let hex = '000f003600000400550004000000010026000780f0010000010079000b000899970f000001005a000102';

      if (isR17) {
        ies.push(
          { name: 'IE [id=RedCapIndication (142)]', indent: 3, release: '17', highlight: true },
          { name: 'value: true (Reduced Capability UE type)', indent: 4, release: '17', highlight: true, comment: 'Flags to AMF that the UE is a low-complexity RedCap client (1-Rx or 2-Rx)' },
          { name: 'IE [id=UserLocationInformation-NTN (155)]', indent: 3, release: '17', highlight: true },
          { name: 'value: TAI: 999-70, OrbitInfo: Sat-ID-42, Coords: Lat 37.7749 / Lon -122.4194', indent: 4, release: '17', highlight: true, comment: 'R17 NTN User location metadata including satellite ephemeris orbital tracking details' }
        );
        hex += '008e00020101009b000b000899970f1a2b3c4d';
      }
      if (isR18) {
        ies.push(
          { name: 'IE [id=AerialUEIndication (173)]', indent: 3, release: '18', highlight: true },
          { name: 'value: UAV-Drone-Flight-99a (Federal flight auth: FA-8827-US)', indent: 4, release: '18', highlight: true, comment: 'R18 aerial drone indication with authorized flight context and trajectory metadata' },
          { name: 'IE [id=SliceGroupID (181)]', indent: 3, release: '18', highlight: true },
          { name: 'value: 0x4f (Slice Group Ultra-Reliable Low-Latency)', indent: 4, release: '18', highlight: true, comment: 'R18 classification grouping for mapping the UE to specific low-latency slice pools' }
        );
        hex += '00ad000c0b5541562d44726f6e6500bc000605414d423031';
      }
      if (isR19) {
        ies.push(
          { name: 'IE [id=AmbientIoTIndication (188)]', indent: 3, release: '19', highlight: true },
          { name: 'value: EPC: AMB01 (Sensor Temp: 24.5C, Battery: Passive/Harvesting)', indent: 4, release: '19', highlight: true, comment: 'R19 Passive Ambient IoT tag sensor payload (battery-free RFID sensor data)' },
          { name: 'IE [id=RanAISensingInfo (207)]', indent: 3, release: '19', highlight: true },
          { name: 'value: ISAC (Integrated Sensing and Communication radar range: 150m, speed: 12m/s)', indent: 4, release: '19', highlight: true, comment: 'R19 ISAC environment sensing targets tracked near the UE location' }
        );
        hex += '00cf000b0008c2a3f0190a42';
      }

      return {
        name: 'Initial UE Message',
        hex,
        ies,
        specRef: '3GPP TS 38.413, Section 9.1.5.1',
        description: 'Sent by the GNodeB to the AMF to initiate a signaling connection for a UE, encapsulating the raw NAS Registration Request payload and location identifiers.'
      };
    }
    case 'pdu-setup': {
      const ies: DecodedIE[] = [
        { name: 'NGAP-PDU: initiatingMessage', indent: 0 },
        { name: 'procedureCode: id-PDUSessionResourceSetup (29)', indent: 1 },
        { name: 'criticality: reject (0)', indent: 1 },
        { name: 'value: PDUSessionResourceSetupRequest', indent: 1 },
        { name: 'protocolIEs: ProtocolIE-Container', indent: 2 },
        { name: 'IE [id=AMF-UE-NGAP-ID (10)]', indent: 3 },
        { name: 'value: 1', indent: 4 },
        { name: 'IE [id=RAN-UE-NGAP-ID (85)]', indent: 3 },
        { name: 'value: 1', indent: 4 },
        { name: 'IE [id=PDUSessionResourceSetupListSUReq (74)]', indent: 3 },
        { name: 'PDUSessionResourceSetupItemSUReq', indent: 4 },
        { name: 'pduSessionID: 1', indent: 5 },
        { name: 'pduSessionResourceSetupRequestTransfer: (UPF IP: 127.0.0.1, TEID: 1)', indent: 5 },
        { name: 'snssai: SST: 1, SD: 010203', indent: 5 }
      ];

      let hex = '001d003b0000030055000400000001004a0004000000010068001e001c00010001000c0101020300100a0000010000000109060504030201';

      if (isR17) {
        ies.push(
          { name: 'IE [id=XR-QoS-Parameters (164)]', indent: 3, release: '17', highlight: true },
          { name: 'value: JitterBudget: 10ms, PacketDelayBudget: 15ms, BurstSize: 1024B', indent: 4, release: '17', highlight: true, comment: 'R17 high-resolution low-latency QoS guidelines optimized for Extended Reality (XR) streams' }
        );
        hex += '00a4000d0c0a0f0505';
      }
      if (isR18) {
        ies.push(
          { name: 'IE [id=SliceGroupID (183)]', indent: 3, release: '18', highlight: true },
          { name: 'value: SG-01-UltraLowLatency', indent: 4, release: '18', highlight: true, comment: 'R18 Slice Group ID classification for aggregated low-latency priority queue scheduling' }
        );
        hex += '00b700040353473031';
      }
      if (isR19) {
        ies.push(
          { name: 'IE [id=AI-Traffic-Pattern (204)]', indent: 3, release: '19', highlight: true },
          { name: 'value: Periodicity: 20ms, PredictionAccuracy: 95%, PacketArrivalWindow: 2ms', indent: 4, release: '19', highlight: true, comment: 'R19 AI-derived packet arrival prediction context for battery-saving DRX micro-sleep alignment' }
        );
        hex += '00cc000a09145f';
      }

      return {
        name: 'PDU Session Resource Setup Request',
        hex,
        ies,
        specRef: '3GPP TS 38.413, Section 9.1.4.1',
        description: 'Sent by the AMF core to command the GNodeB to provision radio resources and create a GTP-U user plane tunnel endpoint for user plane network data transfer.'
      };
    }
    case 'path-switch': {
      const ies: DecodedIE[] = [
        { name: 'NGAP-PDU: initiatingMessage', indent: 0 },
        { name: 'procedureCode: id-PathSwitchRequest (30)', indent: 1 },
        { name: 'criticality: reject (0)', indent: 1 },
        { name: 'value: PathSwitchRequest', indent: 1 },
        { name: 'protocolIEs: ProtocolIE-Container', indent: 2 },
        { name: 'IE [id=AMF-UE-NGAP-ID (10)]', indent: 3 },
        { name: 'value: 1', indent: 4 },
        { name: 'IE [id=RAN-UE-NGAP-ID (85)]', indent: 3 },
        { name: 'value: 2', indent: 4 },
        { name: 'IE [id=UserLocationInformation (121)]', indent: 3 },
        { name: 'value: nr-CGI: 999-70-000001, TAI: 999-70-000001', indent: 4 },
        { name: 'IE [id=SourceToTarget-TransparentContainer (91)]', indent: 3 },
        { name: 'value: 0x0301...', indent: 4 }
      ];

      let hex = '001e002e000004000a00040000000100550004000000020079000b000899970f000001005b00020301';

      if (isR17) {
        ies.push(
          { name: 'IE [id=RedCap-HO-Indication (170)]', indent: 3, release: '17', highlight: true },
          { name: 'value: supported (0)', indent: 4, release: '17', highlight: true, comment: 'R17 Handover support verification flag for RedCap low-complexity parameters' }
        );
        hex += '00aa00020101';
      }
      if (isR18) {
        ies.push(
          { name: 'IE [id=UAV-Flight-Path-Update (195)]', indent: 3, release: '18', highlight: true },
          { name: 'value: UAV-Active-Flight (3D Flight Path verified, altitude ceiling 120m)', indent: 4, release: '18', highlight: true, comment: 'R18 Active flight trajectory sync with core to authorize flying drone airspace handover' }
        );
        hex += '00c3000b0a5541562d416374697665';
      }
      if (isR19) {
        ies.push(
          { name: 'IE [id=ISAC-SensingContextTransfer (212)]', indent: 3, release: '19', highlight: true },
          { name: 'value: active (0)', indent: 4, release: '19', highlight: true, comment: 'R19 context transfer of target radio radar reflection profiles to prevent sensing discontinuity' }
        );
        hex += '00d400020101';
      }

      return {
        name: 'Path Switch Request',
        hex,
        ies,
        specRef: '3GPP TS 38.413, Section 9.1.2.1',
        description: 'Sent by the target GNodeB to request the AMF core to switch the active PDU session user plane GTP-U tunnel endpoints from the source GNodeB during Xn handover.'
      };
    }
    case 'handover-req': {
      const ies: DecodedIE[] = [
        { name: 'NGAP-PDU: initiatingMessage', indent: 0 },
        { name: 'procedureCode: id-HandoverPreparation (27)', indent: 1 },
        { name: 'criticality: reject (0)', indent: 1 },
        { name: 'value: HandoverRequired', indent: 1 },
        { name: 'protocolIEs: ProtocolIE-Container', indent: 2 },
        { name: 'IE [id=AMF-UE-NGAP-ID (10)]', indent: 3 },
        { name: 'value: 1', indent: 4 },
        { name: 'IE [id=RAN-UE-NGAP-ID (85)]', indent: 3 },
        { name: 'value: 2', indent: 4 },
        { name: 'IE [id=HandoverType (28)]', indent: 3 },
        { name: 'value: intra5gs (0)', indent: 4 },
        { name: 'IE [id=Cause (22)]', indent: 3 },
        { name: 'value: radioNetwork (handover-desirable-for-radio-reasons)', indent: 4 },
        { name: 'IE [id=TargetID (122)]', indent: 3 },
        { name: 'value: targetGNB-ID (0x000002)', indent: 4 }
      ];

      let hex = '001b002c000005000a0004000000010055000400000002001c00020000001600020001007a000400000002';

      if (isR17) {
        ies.push(
          { name: 'IE [id=NTN-HO-Offset (171)]', indent: 3, release: '17', highlight: true },
          { name: 'value: OrbitOffset: 120km, DopplerCompensation: active', indent: 4, release: '17', highlight: true, comment: 'R17 Handover compensation metrics adjusting for high-velocity LEO satellite relative movement' }
        );
        hex += '00ab0008070000007801';
      }
      if (isR18) {
        ies.push(
          { name: 'IE [id=AerialUE-SafetyStatus (196)]', indent: 3, release: '18', highlight: true },
          { name: 'value: flight-zone-safety-clear (1)', indent: 4, release: '18', highlight: true, comment: 'R18 Airspace geofence and trajectory safety clearance verification flags' }
        );
        hex += '00c400020101';
      }
      if (isR19) {
        ies.push(
          { name: 'IE [id=RAN-AI-ModelContext (213)]', indent: 3, release: '19', highlight: true },
          { name: 'value: Model-V4 (AI Model context weights transfer active)', indent: 4, release: '19', highlight: true, comment: 'R19 transfers localized reinforcement-learning model weights to target GNodeB for zero-handover-latency tracking' }
        );
        hex += '00d500060541492d5634';
      }

      return {
        name: 'Handover Required',
        hex,
        ies,
        specRef: '3GPP TS 38.413, Section 9.1.3.1',
        description: 'Sent by the source GNodeB to the AMF core to trigger N2-based handover when Xn direct interface is unavailable, prompting target resource allocation.'
      };
    }
    case 'handover-ack': {
      const ies: DecodedIE[] = [
        { name: 'NGAP-PDU: successfulOutcome', indent: 0 },
        { name: 'procedureCode: id-HandoverResourceAllocation (29)', indent: 1 },
        { name: 'criticality: reject (0)', indent: 1 },
        { name: 'value: HandoverRequestAcknowledge', indent: 1 },
        { name: 'protocolIEs: ProtocolIE-Container', indent: 2 },
        { name: 'IE [id=AMF-UE-NGAP-ID (10)]', indent: 3 },
        { name: 'value: 1', indent: 4 },
        { name: 'IE [id=RAN-UE-NGAP-ID (85)]', indent: 3 },
        { name: 'value: 2', indent: 4 },
        { name: 'IE [id=PDUSessionResourceAdmittedList (75)]', indent: 3 },
        { name: 'PDUSessionResourceAdmittedItem', indent: 4 },
        { name: 'pduSessionID: 1', indent: 5 },
        { name: 'handoverRequestAcknowledgeTransfer: (GTP-U TEID: 1)', indent: 5 }
      ];

      let hex = '001d002f000004000a0004000000010055000400000002004b000800000001000000010069000a0901020304';

      if (isR17) {
        ies.push(
          { name: 'IE [id=XR-LowLatencyAdmitted (172)]', indent: 3, release: '17', highlight: true },
          { name: 'value: admitted (1)', indent: 4, release: '17', highlight: true, comment: 'R17 target GNodeB verifies allocation of specialized high-jitter low-latency buffer queues' }
        );
        hex += '00ac00020101';
      }
      if (isR18) {
        ies.push(
          { name: 'IE [id=SliceGroupAdmitted (197)]', indent: 3, release: '18', highlight: true },
          { name: 'value: SG-02-RealTime', indent: 4, release: '18', highlight: true, comment: 'R18 Target cell accepts the user\'s specific Slicing Group scheduling' }
        );
        hex += '00c500040353473032';
      }
      if (isR19) {
        ies.push(
          { name: 'IE [id=RAN-SensingContextAcknowledge (214)]', indent: 3, release: '19', highlight: true },
          { name: 'value: active-sensing-granted (1)', indent: 4, release: '19', highlight: true, comment: 'R19 confirms target radio sensor sweeps will execute continuously post-HO' }
        );
        hex += '00d600060553454e5345';
      }

      return {
        name: 'Handover Request Acknowledge',
        hex,
        ies,
        specRef: '3GPP TS 38.413, Section 9.1.3.3',
        description: 'Sent by the target GNodeB to the AMF core to confirm readiness and allocate user plane tunnel endpoints for incoming UE handover context.'
      };
    }
    case 'ue-release-complete': {
      const ies: DecodedIE[] = [
        { name: 'NGAP-PDU: successfulOutcome', indent: 0 },
        { name: 'procedureCode: id-UEContextRelease (41)', indent: 1 },
        { name: 'criticality: ignore (1)', indent: 1 },
        { name: 'value: UEContextReleaseComplete', indent: 1 },
        { name: 'protocolIEs: ProtocolIE-Container', indent: 2 },
        { name: 'IE [id=AMF-UE-NGAP-ID (10)]', indent: 3 },
        { name: 'value: 1', indent: 4 },
        { name: 'IE [id=RAN-UE-NGAP-ID (85)]', indent: 3 },
        { name: 'value: 2', indent: 4 },
        { name: 'IE [id=UserLocationInformation (123)]', indent: 3 },
        { name: 'value: nr-CGI: 999-70-000001, TAI: 999-70-000001', indent: 4 }
      ];

      let hex = '0029001b000003000a0004000000010055000400000002007b000500000001';

      if (isR17) {
        ies.push(
          { name: 'IE [id=NTN-SessionReleaseStats (173)]', indent: 3, release: '17', highlight: true },
          { name: 'value: SatelliteConnectionTime: 342s', indent: 4, release: '17', highlight: true, comment: 'R17 records satellite connection duration logs to analyze orbital handovers' }
        );
        hex += '00ad00020101';
      }
      if (isR18) {
        ies.push(
          { name: 'IE [id=UAV-FlightReleaseSummary (198)]', indent: 3, release: '18', highlight: true },
          { name: 'value: FlightPathCompleted: true', indent: 4, release: '18', highlight: true, comment: 'R18 transmits final drone flight checklist and geo-tracking audit logs' }
        );
        hex += '00c600020101';
      }
      if (isR19) {
        ies.push(
          { name: 'IE [id=AI-InferenceModelLog (215)]', indent: 3, release: '19', highlight: true },
          { name: 'value: ModelAccPct: 94.8%, InferenceDelay: 4ms', indent: 4, release: '19', highlight: true, comment: 'R19 AI-powered channel prediction feedback metrics logged upon connection lifecycle' }
        );
        hex += '00d700040341495f';
      }

      return {
        name: 'UE Context Release Complete',
        hex,
        ies,
        specRef: '3GPP TS 38.413, Section 9.1.6.2',
        description: 'Sent by the GNodeB to the AMF core to confirm that the UE radio/control context resources have been successfully torn down and released.'
      };
    }
    default: {
      return {
        name: 'Unknown Message',
        hex: '',
        ies: [],
        specRef: '',
        description: ''
      };
    }
  }
};

const formatHexDump = (hex: string): string => {
  const cleanHex = hex.replace(/[^0-9a-fA-F]/g, '');
  let result = '';
  for (let i = 0; i < cleanHex.length; i += 32) {
    const chunk = cleanHex.slice(i, i + 32);
    const offset = (i / 2).toString(16).padStart(4, '0');
    
    let hexPart = '';
    for (let j = 0; j < 32; j += 2) {
      if (j < chunk.length) {
        hexPart += chunk.slice(j, j + 2) + ' ';
      } else {
        hexPart += '   ';
      }
      if (j === 14) hexPart += ' ';
    }
    
    let asciiPart = '';
    for (let j = 0; j < chunk.length; j += 2) {
      const byteHex = chunk.slice(j, j + 2);
      const val = parseInt(byteHex, 16);
      if (val >= 32 && val <= 126) {
        asciiPart += String.fromCharCode(val);
      } else {
        asciiPart += '.';
      }
    }
    
    result += `${offset}  ${hexPart} |${asciiPart}|\n`;
  }
  return result;
};

export default function App() {
  const [theme, setTheme] = useState<'dark' | 'light'>(() => {
    const saved = localStorage.getItem('theme');
    return (saved === 'dark' || saved === 'light') ? saved : 'light';
  });
  const [activeTab, setActiveTab] = useState<'dashboard' | 'scenarios' | 'config' | 'logs' | 'connectivity' | 'fleet' | 'diagnostics'>('dashboard');
  const [selectedNode, setSelectedNode] = useState<'ue' | 'gnb' | 'amf' | 'upf' | 'dn' | 'uu-link' | 'n2-link' | 'n3-link' | 'n6-link' | null>('ue');
  // 3GPP Spec Release State
  const [activeRelease, setActiveRelease] = useState<string>('15');
  const [selectedInspectMsg, setSelectedInspectMsg] = useState<string>('ng-setup');
  const [inspectRelease, setInspectRelease] = useState<string>('15');

  // Sync inspection release with active release when it updates
  useEffect(() => {
    setInspectRelease(activeRelease);
  }, [activeRelease]);

  const updateRelease = async (rel: string) => {
    try {
      const res = await fetch(`${API_BASE}/config/release`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ release: rel })
      });
      if (res.ok) {
        setActiveRelease(rel);
        fetchStatus();
      } else {
        const text = await res.text();
        alert(`Failed to update 3GPP release: ${text}`);
      }
    } catch (err) {
      console.error("Error updating 3GPP release:", err);
    }
  };

  // Diagnostics & PCAP States
  const [pcapInterfaces, setPcapInterfaces] = useState<{ index: number; name: string; flags: string; ipAddresses: string[] }[]>([]);
  const [activeCaptureStatus, setActiveCaptureStatus] = useState<{
    isCapturing: boolean;
    interface?: string;
    protocol?: string;
    fileName?: string;
    packetCount?: number;
    bytesCount?: number;
    elapsedSec?: number;
  } | null>(null);
  const [savedPcapFiles, setSavedPcapFiles] = useState<{ name: string; size: number; modTime: string }[]>([]);
  const [pcapInterface, setPcapInterface] = useState('any');
  const [pcapProtocol, setPcapProtocol] = useState('all');
  const [pcapFileName, setPcapFileName] = useState('');
  const [diagnosticsLogs, setDiagnosticsLogs] = useState<string[]>([]);
  const [diagnosticsLogSearch, setDiagnosticsLogSearch] = useState('');
  const [diagnosticsLogLevel, setDiagnosticsLogLevel] = useState<'all' | 'info' | 'warn' | 'error'>('all');

  const [isStartingPcap, setIsStartingPcap] = useState(false);
  const [isStoppingPcap, setIsStoppingPcap] = useState(false);
  const [isRefreshingPcaps, setIsRefreshingPcaps] = useState(false);
  const [isClearingLogs, setIsClearingLogs] = useState(false);
  const [isRefreshingLogs, setIsRefreshingLogs] = useState(false);

  const [showBanners, setShowBanners] = useState(true);
  const [bannerFade, setBannerFade] = useState(false);

  // Auto-hide warning/tips banners after 5 seconds of tab change
  useEffect(() => {
    setShowBanners(true);
    setBannerFade(false);
    const fadeTimer = setTimeout(() => {
      setBannerFade(true);
    }, 4500);
    const hideTimer = setTimeout(() => {
      setShowBanners(false);
    }, 5000);
    return () => {
      clearTimeout(fadeTimer);
      clearTimeout(hideTimer);
    };
  }, [activeTab]);

  // Toggle theme class on body
  useEffect(() => {
    localStorage.setItem('theme', theme);
    if (theme === 'light') {
      document.body.classList.add('light');
    } else {
      document.body.classList.remove('light');
    }
  }, [theme]);

  const [status, setStatus] = useState<StatusData | null>(null);
  const [configData, setConfigData] = useState<ConfigData | null>(null);
  const [scenarios, setScenarios] = useState<Scenario[]>([]);
  const [logs, setLogs] = useState<LogMsg[]>([]);
  const [logFilter, setLogFilter] = useState<'all' | 'info' | 'warn' | 'error'>('all');
  const [autoscroll, setAutoscroll] = useState(true);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [logSearch, setLogSearch] = useState('');

  // Input states for Scenario Runner
  const [targetGnbIp, setTargetGnbIp] = useState('127.0.0.1');
  const [targetGnbPort, setTargetGnbPort] = useState(9489);
  const [delay, setDelay] = useState(5);
  const [idleSeconds, setIdleSeconds] = useState(5);
  const [ueCount, setUeCount] = useState(5);
  const [requests, setRequests] = useState(20);
  const [duration, setDuration] = useState(10);
  const [ueOnly, setUeOnly] = useState(false);

  // Active UE States
  const [activeUEs, setActiveUEs] = useState<any[]>([]);
  const [controlUeId, setControlUeId] = useState<number | null>(null);

  // User Plane Traffic simulation states
  const [inspectorTab, setInspectorTab] = useState<'details' | 'traffic'>('details');
  const [trafficStats, setTrafficStats] = useState<any>({});
  const [uePingTarget, setUePingTarget] = useState('8.8.8.8');
  const [uePingLog, setUePingLog] = useState('');
  const [uePingRunning, setUePingRunning] = useState(false);
  const [browserUrl, setBrowserUrl] = useState('example.com');
  const [browserResult, setBrowserResult] = useState<any>(null);
  const [browserRunning, setBrowserRunning] = useState(false);
  const [videoQuality, setVideoQuality] = useState('1080p');
  const [vonrCallee, setVonrCallee] = useState('echo');
  const [vonrActiveCall, setVonrActiveCall] = useState<any>(null);

  // Secondary PDU and Handover inputs
  const [newPduId, setNewPduId] = useState(2);
  const [newPduDnn, setNewPduDnn] = useState('internet2');
  const [newPduSst, setNewPduSst] = useState(1);
  const [newPduSd, setNewPduSd] = useState('010203');
  const [newPduType, setNewPduType] = useState('IPv4');
  const [hoTargetIp, setHoTargetIp] = useState('127.0.0.1');
  const [hoTargetPort, setHoTargetPort] = useState(9489);
  const [hoTargetLinkType, setHoTargetLinkType] = useState('unix');
  const [hoTargetSocketPath, setHoTargetSocketPath] = useState('');
  const [selectedTargetGnbName, setSelectedTargetGnbName] = useState('');
  const [selectedUeId, setSelectedUeId] = useState<number | null>(null);
  const [selectedGnbName, setSelectedGnbName] = useState<string | null>(null);



  // Connectivity Test State
  const [pingHost, setPingHost] = useState('');
  const [pingResult, setPingResult] = useState('');
  const [pingRunning, setPingRunning] = useState(false);
  const [checks, setChecks] = useState({
    sctpModule: 'loading',
    ipipModule: 'loading',
    amfReachability: 'loading',
    socketsClean: 'loading'
  });

  // Fleet Manager State
  interface UEProfile {
    name: string;
    msin: string;
    key: string;
    opc: string;
    amf: string;
    sqn: string;
    dnn: string;
    pduSessionType: string;
    registrationType: string;
    hplmn: { mcc: string; mnc: string };
    snssai: { sst: number; sd: string };
  }
  interface GNBProfile {
    name: string;
    gnbId: string;
    mcc: string;
    mnc: string;
    tac: string;
    sliceSst: string;
    sliceSd: string;
    controlIp: string;
    controlPort: number;
    dataIp: string;
    dataPort: number;
    linkType: string;
    linkPort: number;
    amfIp: string;
    amfPort: number;
  }



  const defaultUEProfile: UEProfile = {
    name: '', msin: '', key: '', opc: '', amf: '8000', sqn: '000000000000',
    dnn: 'internet', pduSessionType: 'IPv4', registrationType: 'initial',
    hplmn: { mcc: '999', mnc: '70' }, snssai: { sst: 1, sd: '' }
  };
  const defaultGNBProfile: GNBProfile = {
    name: '', gnbId: '', mcc: '999', mnc: '70', tac: '000001', sliceSst: '01', sliceSd: '',
    controlIp: '127.0.0.1', controlPort: 9487, dataIp: '127.0.0.1', dataPort: 2152,
    linkType: 'unix', linkPort: 9488, amfIp: '127.0.0.1', amfPort: 38412
  };

  const [ueProfiles, setUEProfiles] = useState<UEProfile[]>([]);
  const [gnbProfiles, setGNBProfiles] = useState<GNBProfile[]>([]);
  const [showUEForm, setShowUEForm] = useState(false);
  const [showGNBForm, setShowGNBForm] = useState(false);
  const [editingUE, setEditingUE] = useState<UEProfile>(defaultUEProfile);
  const [editingGNB, setEditingGNB] = useState<GNBProfile>(defaultGNBProfile);
  const [fleetRunning, setFleetRunning] = useState<{ runningUes: RunningUE[]; runningGnbs: RunningGNB[] }>({ runningUes: [], runningGnbs: [] });
  const [fleetMsg, setFleetMsg] = useState<{ text: string; type: 'success' | 'error' } | null>(null);
  const [fleetActiveSection, setFleetActiveSection] = useState<'ue' | 'gnb' | 'live'>('ue');
  const [ueToLaunch, setUeToLaunch] = useState<string | null>(null);
  const [selectedTargetGnb, setSelectedTargetGnb] = useState<string>('');

  const renderSvgLinks = () => {
    const links: React.ReactNode[] = [];
    const gnbs = status?.runningGnbs || [];
    const ues = status?.runningUes || [];
    const totalGnbs = gnbs.length;
    const totalUes = ues.length;

    const defs = (
      <defs key="defs">
        <linearGradient id="uu-grad" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#10b981" />
          <stop offset="100%" stopColor="#6366f1" />
        </linearGradient>
        <linearGradient id="n2-grad" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#6366f1" stopOpacity="0.8" />
          <stop offset="100%" stopColor="#8b5cf6" stopOpacity="0.8" />
        </linearGradient>
        <linearGradient id="n3-grad" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#6366f1" stopOpacity="0.8" />
          <stop offset="100%" stopColor="#3b82f6" stopOpacity="0.8" />
        </linearGradient>
        <linearGradient id="n6-grad" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#3b82f6" />
          <stop offset="100%" stopColor="#06b6d4" />
        </linearGradient>
        <filter id="glow-filter" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="3" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>
      </defs>
    );
    links.push(defs);

    const renderBadge = (key: string, midX: number, midY: number, label: string, type: 'uu-link' | 'n2-link' | 'n3-link' | 'n6-link', color: string, active: boolean, onClick: () => void) => {
      const isSelected = selectedNode === type;
      return (
        <g key={`badge-${key}`} style={{ pointerEvents: 'auto', cursor: 'pointer' }} onClick={(e) => { e.stopPropagation(); onClick(); }}>
          <rect
            x={midX - 45}
            y={midY - 10}
            width="90"
            height="20"
            rx="10"
            fill="rgba(15, 23, 42, 0.95)"
            stroke={isSelected ? '#3b82f6' : active ? color : '#334155'}
            strokeWidth={isSelected ? 2 : 1}
            filter={active ? "url(#glow-filter)" : ""}
            style={{ transition: 'all 0.2s' }}
          />
          <text
            x={midX}
            y={midY + 4}
            textAnchor="middle"
            fill={active ? "#ffffff" : "#94a3b8"}
            fontSize="9"
            fontWeight="bold"
            fontFamily="monospace"
          >
            {label}
          </text>
        </g>
      );
    };

    if (totalGnbs === 0) {
      const isGnbActive = !!(status?.gnbLinkState && status.gnbLinkState !== 'offline');
      const strokeN2 = isGnbActive ? 'url(#n2-grad)' : '#334155';
      links.push(
        <g key="def-n2">
          <path d="M 530 288 C 610 288, 700 200, 788 200" fill="none" stroke={strokeN2} strokeWidth="3" strokeDasharray={isGnbActive ? "none" : "4,4"} opacity={isGnbActive ? 1 : 0.4} />
          {isGnbActive && (
            <circle r="4" fill="#a78bfa" filter="url(#glow-filter)">
              <animateMotion dur="3s" repeatCount="indefinite" path="M 530 288 C 610 288, 700 200, 788 200" />
            </circle>
          )}
        </g>
      );
      links.push(renderBadge('def-n2', 660, 244, 'N2 (NGAP)', 'n2-link', '#8b5cf6', isGnbActive, () => setSelectedNode('n2-link')));

      const hasActiveSession = activeUEs.some(ue => ue.pduSessions && ue.pduSessions.some((s: any) => s.stateDesc?.includes('ACTIVE')));
      const strokeN3 = hasActiveSession ? 'url(#n3-grad)' : '#334155';
      links.push(
        <g key="def-n3">
          <path d="M 530 312 C 610 312, 700 400, 788 400" fill="none" stroke={strokeN3} strokeWidth="3" opacity={hasActiveSession ? 1 : 0.4} />
          {hasActiveSession && (
            <circle r="4" fill="#60a5fa" filter="url(#glow-filter)">
              <animateMotion dur="2.5s" repeatCount="indefinite" path="M 530 312 C 610 312, 700 400, 788 400" />
            </circle>
          )}
        </g>
      );
      links.push(renderBadge('def-n3', 660, 356, 'N3 (GTP-U)', 'n3-link', '#3b82f6', hasActiveSession, () => setSelectedNode('n3-link')));

      const strokeN6 = hasActiveSession ? 'url(#n6-grad)' : '#334155';
      links.push(
        <g key="def-n6">
          <line x1="852" y1="400" x2="908" y2="400" stroke={strokeN6} strokeWidth="3" opacity={hasActiveSession ? 1 : 0.4} />
          {hasActiveSession && (
            <circle r="4" fill="#06b6d4" filter="url(#glow-filter)">
              <animate attributeName="cx" from="852" to="908" dur="2s" repeatCount="indefinite" />
              <animate attributeName="cy" from="400" to="400" dur="2s" repeatCount="indefinite" />
            </circle>
          )}
        </g>
      );
      links.push(renderBadge('def-n6', 880, 400, 'N6 (SGi)', 'n6-link', '#06b6d4', hasActiveSession, () => setSelectedNode('n6-link')));

      const isUeActive = !!status?.isRunning || activeUEs.length > 0;
      const strokeUu = isUeActive ? 'url(#uu-grad)' : '#334155';
      links.push(
        <g key="def-uu">
          <line x1="182" y1="300" x2="468" y2="300" stroke={strokeUu} strokeWidth="3" opacity={isUeActive ? 1 : 0.4} />
          {isUeActive && (
            <circle r="5" fill="#34d399" filter="url(#glow-filter)">
              <animate attributeName="cx" from="182" to="468" dur="2s" repeatCount="indefinite" />
              <animate attributeName="cy" from="300" to="300" dur="2s" repeatCount="indefinite" />
            </circle>
          )}
        </g>
      );
      links.push(renderBadge('def-uu', 325, 300, 'Uu (5G-NR)', 'uu-link', '#10b981', isUeActive, () => setSelectedNode('uu-link')));

    } else {
      gnbs.forEach((g, gIdx) => {
        const gy = (gIdx + 1) * 600 / (totalGnbs + 1);
        
        links.push(
          <g key={`n2-${g.profileName}`}>
            <path d={`M 530 ${gy - 12} C 610 ${gy - 12}, 700 200, 788 200`} fill="none" stroke="url(#n2-grad)" strokeWidth="2.5" strokeDasharray="3,3" />
            <circle r="3.5" fill="#c084fc" filter="url(#glow-filter)">
              <animateMotion dur="3s" repeatCount="indefinite" path={`M 530 ${gy - 12} C 610 ${gy - 12}, 700 200, 788 200`} />
            </circle>
          </g>
        );
        const n2MidY = (gy - 12 + 200) / 2;
        links.push(renderBadge(`n2-${g.profileName}`, 660, n2MidY, `N2 (${g.gnbId})`, 'n2-link', '#8b5cf6', true, () => {
          setSelectedNode('n2-link');
          setSelectedGnbName(g.profileName);
        }));

        const gnbHasActiveSession = ues.some(u => u.gnbProfileName === g.profileName && u.pduSessions && u.pduSessions.some(s => s.stateDesc?.includes('ACTIVE')));
        const strokeN3 = gnbHasActiveSession ? 'url(#n3-grad)' : '#334155';
        links.push(
          <g key={`n3-${g.profileName}`}>
            <path d={`M 530 ${gy + 12} C 610 ${gy + 12}, 700 400, 788 400`} fill="none" stroke={strokeN3} strokeWidth="3" opacity={gnbHasActiveSession ? 1 : 0.4} />
            {gnbHasActiveSession && (
              <circle r="4" fill="#60a5fa" filter="url(#glow-filter)">
                <animateMotion dur="2.2s" repeatCount="indefinite" path={`M 530 ${gy + 12} C 610 ${gy + 12}, 700 400, 788 400`} />
              </circle>
            )}
          </g>
        );
        const n3MidY = (gy + 12 + 400) / 2;
        links.push(renderBadge(`n3-${g.profileName}`, 660, n3MidY, 'N3 (GTP-U)', 'n3-link', '#3b82f6', gnbHasActiveSession, () => {
          setSelectedNode('n3-link');
          setSelectedGnbName(g.profileName);
        }));
      });

      const hasAnyActiveSession = ues.some(u => u.pduSessions && u.pduSessions.some(s => s.stateDesc?.includes('ACTIVE')));
      const strokeN6 = hasAnyActiveSession ? 'url(#n6-grad)' : '#334155';
      links.push(
        <g key="fleet-n6">
          <line x1="852" y1="400" x2="908" y2="400" stroke={strokeN6} strokeWidth="3" opacity={hasAnyActiveSession ? 1 : 0.4} />
          {hasAnyActiveSession && (
            <circle r="4" fill="#06b6d4" filter="url(#glow-filter)">
              <animate attributeName="cx" from="852" to="908" dur="2s" repeatCount="indefinite" />
              <animate attributeName="cy" from="400" to="400" dur="2s" repeatCount="indefinite" />
            </circle>
          )}
        </g>
      );
      links.push(renderBadge('fleet-n6', 880, 400, 'N6 (SGi)', 'n6-link', '#06b6d4', hasAnyActiveSession, () => setSelectedNode('n6-link')));

      ues.forEach((u, uIdx) => {
        const uy = (uIdx + 1) * 600 / (totalUes + 1);
        let targetGy = 300;

        if (totalGnbs > 0) {
          const gIdx = gnbs.findIndex(g => g.profileName === u.gnbProfileName);
          if (gIdx !== -1) {
            targetGy = (gIdx + 1) * 600 / (totalGnbs + 1);
          } else {
            targetGy = 1 * 600 / (totalGnbs + 1);
          }
        }

        const isRegistered = u.stateMmDesc?.includes('REGISTERED');
        
        // Retrieve traffic info for this UE
        const tInfo = trafficStats[u.id];
        const activeAction = tInfo?.activeAction || 'idle';
        
        let dotColor = '#34d399';
        let dotRadius = 4;
        let flowDuration = '2.5s';
        
        if (activeAction === 'web') {
          dotColor = '#3b82f6';
          flowDuration = '0.7s';
        } else if (activeAction === 'streaming') {
          dotColor = '#a855f7';
          dotRadius = 6;
          flowDuration = '0.4s';
        } else if (activeAction === 'vonr') {
          dotColor = '#22c55e';
          flowDuration = '1.2s';
        }

        const strokeColor = isRegistered 
          ? (activeAction !== 'idle' ? dotColor : 'url(#uu-grad)') 
          : '#f59e0b';

        links.push(
          <g key={`uu-${u.id}`}>
            <line x1="182" y1={uy} x2="468" y2={targetGy} stroke={strokeColor} strokeWidth={activeAction !== 'idle' ? 4 : 3} opacity="0.95" />
            {isRegistered && (
              <circle r={dotRadius} fill={dotColor} filter="url(#glow-filter)">
                <animate attributeName="cx" from="182" to="468" dur={flowDuration} repeatCount="indefinite" />
                <animate attributeName="cy" from={uy} to={targetGy} dur={flowDuration} repeatCount="indefinite" />
              </circle>
            )}
          </g>
        );

        const uuMidX = (182 + 468) / 2;
        const uuMidY = (uy + targetGy) / 2;
        links.push(renderBadge(`uu-${u.id}`, uuMidX, uuMidY, `Uu (UE-${u.id})`, 'uu-link', '#10b981', isRegistered, () => {
          setSelectedNode('uu-link');
          setSelectedUeId(u.id);
        }));
      });
    }

    return links;
  };

  const renderSvgNodes = () => {
    const gnbs = status?.runningGnbs || [];
    const ues = status?.runningUes || [];
    const totalGnbs = gnbs.length;
    const totalUes = ues.length;
    const nodes: React.ReactNode[] = [];

    if (totalUes === 0) {
      nodes.push(
        <foreignObject key="def-ue" x={150 - 60} y={300 - 100} width="120" height="200" style={{ pointerEvents: 'none' }}>
          <div 
            className={`topology-node clickable-node active ${selectedNode === 'ue' ? 'selected' : ''}`} 
            onClick={() => {
              setSelectedNode('ue');
              setSelectedUeId(null);
            }}
            style={{ 
              pointerEvents: 'auto',
              height: '100%',
              paddingTop: '68px',
              boxSizing: 'border-box',
              '--node-color': '#10b981',
              '--node-color-glow': 'rgba(16, 185, 129, 0.2)'
            } as React.CSSProperties}
          >
            <div className="node-icon-wrapper">
              <Cpu />
            </div>
            <span className="node-label">User Equipment</span>
            <span className="node-status-text">
              {status?.isRunning || activeUEs.length > 0 ? 'CONNECTED' : 'IDLE'}
            </span>
          </div>
        </foreignObject>
      );
    } else {
      ues.forEach((u, idx) => {
        const uy = (idx + 1) * 600 / (totalUes + 1);
        const isRegistered = u.stateMmDesc?.includes('REGISTERED');
        const isSelected = selectedNode === 'ue' && selectedUeId === u.id;
        nodes.push(
          <foreignObject key={`ue-${u.id}`} x={150 - 60} y={uy - 100} width="120" height="200" style={{ pointerEvents: 'none' }}>
            <div 
              className={`topology-node clickable-node active ${isSelected ? 'selected' : ''}`} 
              onClick={() => {
                setSelectedNode('ue');
                setSelectedUeId(u.id);
              }}
              style={{ 
                pointerEvents: 'auto',
                height: '100%',
                paddingTop: '68px',
                boxSizing: 'border-box',
                '--node-color': isRegistered ? '#10b981' : '#f59e0b',
                '--node-color-glow': isRegistered ? 'rgba(16, 185, 129, 0.2)' : 'rgba(245, 158, 11, 0.2)'
              } as React.CSSProperties}
            >
              <div className="node-icon-wrapper">
                <Cpu />
              </div>
              <span className="node-label">UE-{u.id}</span>
              <span className="node-status-text">
                {isRegistered ? 'REGISTERED' : 'PENDING'}
              </span>
              {isRegistered && trafficStats[u.id] && trafficStats[u.id].activeAction !== 'idle' && (
                <div style={{
                  fontSize: '8px',
                  fontWeight: 'bold',
                  color: '#ffffff',
                  background: 'rgba(15, 23, 42, 0.95)',
                  border: '1px solid ' + (trafficStats[u.id].activeAction === 'web' ? '#3b82f6' : trafficStats[u.id].activeAction === 'streaming' ? '#a855f7' : '#22c55e'),
                  padding: '2px 4px',
                  borderRadius: '4px',
                  marginTop: '4px',
                  fontFamily: 'monospace',
                  textTransform: 'uppercase',
                  boxShadow: '0 2px 6px rgba(0,0,0,0.5)'
                }}>
                  {trafficStats[u.id].activeAction}: {trafficStats[u.id].speedMbps >= 1.0 ? `${trafficStats[u.id].speedMbps.toFixed(1)} Mbps` : `${(trafficStats[u.id].speedMbps * 1000).toFixed(0)} Kbps`}
                </div>
              )}
            </div>
          </foreignObject>
        );
      });
    }

    if (totalGnbs === 0) {
      nodes.push(
        <foreignObject key="def-gnb" x={500 - 60} y={300 - 100} width="120" height="200" style={{ pointerEvents: 'none' }}>
          <div
            className={`topology-node clickable-node ${status?.gnbLinkState !== 'offline' ? 'active' : ''} ${selectedNode === 'gnb' ? 'selected' : ''}`}
            onClick={() => {
              setSelectedNode('gnb');
              setSelectedGnbName(null);
            }}
            style={{ 
              pointerEvents: 'auto',
              height: '100%',
              paddingTop: '68px',
              boxSizing: 'border-box',
              '--node-color': '#6366f1',
              '--node-color-glow': 'rgba(99, 102, 241, 0.2)'
            } as React.CSSProperties}
          >
            <div className="node-icon-wrapper">
              <Radio />
            </div>
            <span className="node-label">gNodeB Cell</span>
            <span className="node-status-text">
              {status?.gnbLinkState !== 'offline' ? 'ESTABLISHED' : 'OFFLINE'}
            </span>
          </div>
        </foreignObject>
      );
    } else {
      gnbs.forEach((g, idx) => {
        const gy = (idx + 1) * 600 / (totalGnbs + 1);
        const isSelected = selectedNode === 'gnb' && selectedGnbName === g.profileName;
        nodes.push(
          <foreignObject key={`gnb-${g.profileName}`} x={500 - 60} y={gy - 100} width="120" height="200" style={{ pointerEvents: 'none' }}>
            <div
              className={`topology-node clickable-node active ${isSelected ? 'selected' : ''}`}
              onClick={() => {
                setSelectedNode('gnb');
                setSelectedGnbName(g.profileName);
              }}
              style={{ 
                pointerEvents: 'auto',
                height: '100%',
                paddingTop: '68px',
                boxSizing: 'border-box',
                '--node-color': '#6366f1',
                '--node-color-glow': 'rgba(99, 102, 241, 0.2)'
              } as React.CSSProperties}
            >
              <div className="node-icon-wrapper">
                <Radio />
              </div>
              <span className="node-label">{g.profileName}</span>
              <span className="node-status-text" style={{ fontSize: '10px' }}>
                ID: {g.gnbId}
              </span>
            </div>
          </foreignObject>
        );
      });
    }

    nodes.push(
      <foreignObject key="amf-node" x={820 - 60} y={200 - 100} width="120" height="200" style={{ pointerEvents: 'none' }}>
        <div
          className={`topology-node clickable-node ${status?.gnbLinkState !== 'offline' || totalGnbs > 0 ? 'active' : ''} ${selectedNode === 'amf' ? 'selected' : ''}`}
          onClick={() => setSelectedNode('amf')}
          style={{ 
            pointerEvents: 'auto',
            height: '100%',
            paddingTop: '68px',
            boxSizing: 'border-box',
            '--node-color': '#8b5cf6',
            '--node-color-glow': 'rgba(139, 92, 246, 0.2)'
          } as React.CSSProperties}
        >
          <div className="node-icon-wrapper">
            <Server />
          </div>
          <span className="node-label">AMF Core</span>
          <span className="node-status-text">
            {status?.gnbLinkState !== 'offline' || totalGnbs > 0 ? 'ACTIVE' : 'OFFLINE'}
          </span>
        </div>
      </foreignObject>
    );

    nodes.push(
      <foreignObject key="upf-node" x={820 - 60} y={400 - 100} width="120" height="200" style={{ pointerEvents: 'none' }}>
        <div
          className={`topology-node clickable-node ${status?.gnbLinkState !== 'offline' || totalGnbs > 0 ? 'active' : ''} ${selectedNode === 'upf' ? 'selected' : ''}`}
          onClick={() => setSelectedNode('upf')}
          style={{ 
            pointerEvents: 'auto',
            height: '100%',
            paddingTop: '68px',
            boxSizing: 'border-box',
            '--node-color': '#3b82f6',
            '--node-color-glow': 'rgba(59, 130, 246, 0.2)'
          } as React.CSSProperties}
        >
          <div className="node-icon-wrapper">
            <Network />
          </div>
          <span className="node-label">UPF Gateway</span>
          <span className="node-status-text">
            {status?.gnbLinkState !== 'offline' || totalGnbs > 0 ? 'ACTIVE' : 'OFFLINE'}
          </span>
        </div>
      </foreignObject>
    );

    nodes.push(
      <foreignObject key="dn-node" x={940 - 60} y={400 - 100} width="120" height="200" style={{ pointerEvents: 'none' }}>
        <div
          className={`topology-node clickable-node active ${selectedNode === 'dn' ? 'selected' : ''}`}
          onClick={() => setSelectedNode('dn')}
          style={{ 
            pointerEvents: 'auto',
            height: '100%',
            paddingTop: '68px',
            boxSizing: 'border-box',
            '--node-color': '#06b6d4',
            '--node-color-glow': 'rgba(6, 182, 212, 0.2)'
          } as React.CSSProperties}
        >
          <div className="node-icon-wrapper">
            <Globe />
          </div>
          <span className="node-label">DN (Internet)</span>
          <span className="node-status-text">
            ONLINE
          </span>
        </div>
      </foreignObject>
    );

    return nodes;
  };

  const getActiveUeToInspect = () => {
    const ues = status?.runningUes || [];
    if (ues.length > 0) {
      if (selectedUeId !== null) {
        const u = ues.find(x => x.id === selectedUeId);
        if (u) return u;
      }
      return ues[0];
    }
    return null;
  };

  const getActiveGnbToInspect = () => {
    const gnbs = status?.runningGnbs || [];
    if (gnbs.length > 0) {
      if (selectedGnbName !== null) {
        const g = gnbs.find(x => x.profileName === selectedGnbName);
        if (g) return g;
      }
      return gnbs[0];
    }
    return null;
  };

  const fetchFleetProfiles = async () => {
    try {
      const [ueRes, gnbRes] = await Promise.all([
        fetch(`${API_BASE}/fleet/ue`),
        fetch(`${API_BASE}/fleet/gnb`)
      ]);
      if (ueRes.ok) setUEProfiles(await ueRes.json());
      if (gnbRes.ok) setGNBProfiles(await gnbRes.json());
    } catch {}
  };

  const fetchFleetRunning = async () => {
    try {
      const res = await fetch(`${API_BASE}/fleet/running`);
      if (res.ok) setFleetRunning(await res.json());
    } catch {}
  };

  useEffect(() => {
    if (activeTab === 'fleet') {
      fetchFleetProfiles();
      fetchFleetRunning();
      const iv = setInterval(fetchFleetRunning, 2000);
      return () => clearInterval(iv);
    }
  }, [activeTab]);

  const showFleetMsg = (text: string, type: 'success' | 'error') => {
    setFleetMsg({ text, type });
    setTimeout(() => setFleetMsg(null), 4000);
  };

  const saveUEProfile = async () => {
    try {
      const res = await fetch(`${API_BASE}/fleet/ue`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(editingUE)
      });
      if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
      showFleetMsg(`UE profile '${editingUE.name}' saved`, 'success');
      setShowUEForm(false);
      setEditingUE(defaultUEProfile);
      fetchFleetProfiles();
    } catch (e) { showFleetMsg(`Error: ${e}`, 'error'); }
  };

  const saveGNBProfile = async () => {
    try {
      const res = await fetch(`${API_BASE}/fleet/gnb`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(editingGNB)
      });
      if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
      showFleetMsg(`gNB profile '${editingGNB.name}' saved`, 'success');
      setShowGNBForm(false);
      setEditingGNB(defaultGNBProfile);
      fetchFleetProfiles();
    } catch (e) { showFleetMsg(`Error: ${e}`, 'error'); }
  };

  const deleteUEProfile = async (name: string) => {
    if (!confirm(`Delete UE profile '${name}'?`)) return;
    try {
      const res = await fetch(`${API_BASE}/fleet/ue/${encodeURIComponent(name)}`, { method: 'DELETE' });
      if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
      showFleetMsg(`Profile '${name}' deleted`, 'success');
      fetchFleetProfiles();
    } catch (e) { showFleetMsg(`Error: ${e}`, 'error'); }
  };

  const deleteGNBProfile = async (name: string) => {
    if (!confirm(`Delete gNB profile '${name}'?`)) return;
    try {
      const res = await fetch(`${API_BASE}/fleet/gnb/${encodeURIComponent(name)}`, { method: 'DELETE' });
      if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
      showFleetMsg(`Profile '${name}' deleted`, 'success');
      fetchFleetProfiles();
    } catch (e) { showFleetMsg(`Error: ${e}`, 'error'); }
  };

  const getDuplicateName = (baseName: string, existingNames: string[]): string => {
    const trailingNumRegex = /(\d+)$/;
    const match = baseName.match(trailingNumRegex);
    let baseWithoutNum = "";
    let startNum = 1;
    if (match) {
      const numStr = match[1];
      baseWithoutNum = baseName.substring(0, baseName.length - numStr.length);
      startNum = parseInt(numStr, 10) + 1;
    } else {
      baseWithoutNum = baseName;
      startNum = 1;
    }
    let currentNum = startNum;
    let candidateName = "";
    do {
      candidateName = `${baseWithoutNum}${currentNum}`;
      currentNum++;
    } while (existingNames.includes(candidateName));
    return candidateName;
  };

  const duplicateUEProfile = async (profile: UEProfile) => {
    try {
      // 1. Generate unique name
      const newName = getDuplicateName(profile.name, ueProfiles.map(p => p.name));

      // 2. Generate unique MSIN
      let msinVal = parseInt(profile.msin, 10);
      if (isNaN(msinVal)) msinVal = 1000000000;
      let newMsin = profile.msin;
      do {
        msinVal++;
        newMsin = msinVal.toString().padStart(profile.msin.length, '0');
      } while (ueProfiles.some(p => p.msin === newMsin));

      // 3. Clone and save
      const cloned: UEProfile = {
        ...profile,
        name: newName,
        msin: newMsin
      };

      const res = await fetch(`${API_BASE}/fleet/ue`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(cloned)
      });
      if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
      showFleetMsg(`Duplicated to profile '${newName}' (MSIN: ${newMsin})`, 'success');
      fetchFleetProfiles();
    } catch (e) { showFleetMsg(`Error: ${e}`, 'error'); }
  };

  const duplicateGNBProfile = async (profile: GNBProfile) => {
    try {
      // 1. Generate unique name
      const newName = getDuplicateName(profile.name, gnbProfiles.map(p => p.name));

      // 2. Generate unique gNB ID
      let gnbIdVal = parseInt(profile.gnbId, 16);
      if (isNaN(gnbIdVal)) gnbIdVal = 1;
      let newGnbId = profile.gnbId;
      do {
        gnbIdVal++;
        newGnbId = gnbIdVal.toString(16).toUpperCase().padStart(profile.gnbId.length, '0');
      } while (gnbProfiles.some(p => p.gnbId === newGnbId));

      // 3. Generate unique ports for controlPort, linkPort, dataPort
      let newControlPort = profile.controlPort;
      while (gnbProfiles.some(p => p.controlPort === newControlPort)) {
        newControlPort++;
      }
      let newLinkPort = profile.linkPort;
      while (gnbProfiles.some(p => p.linkPort === newLinkPort)) {
        newLinkPort++;
      }
      let newDataPort = profile.dataPort;
      while (gnbProfiles.some(p => p.dataPort === newDataPort)) {
        newDataPort++;
      }

      // 4. Clone and save
      const cloned: GNBProfile = {
        ...profile,
        name: newName,
        gnbId: newGnbId,
        controlPort: newControlPort,
        linkPort: newLinkPort,
        dataPort: newDataPort
      };

      const res = await fetch(`${API_BASE}/fleet/gnb`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(cloned)
      });
      if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
      showFleetMsg(`Duplicated to profile '${newName}' (gNB-ID: ${newGnbId})`, 'success');
      fetchFleetProfiles();
    } catch (e) { showFleetMsg(`Error: ${e}`, 'error'); }
  };

  const launchUEProfile = async (name: string, gnbProfileName?: string) => {
    try {
      const res = await fetch(`${API_BASE}/fleet/launch/ue`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ profileName: name, gnbProfileName })
      });
      if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
      const data = await res.json();
      showFleetMsg(`UE launched: ID ${data.ueId} — ${data.message}`, 'success');
      setFleetActiveSection('live');
      fetchFleetRunning();
    } catch (e) { showFleetMsg(`Error: ${e}`, 'error'); }
  };

  const handleLaunchUEClick = (name: string) => {
    const hasFleetGnbs = fleetRunning.runningGnbs && fleetRunning.runningGnbs.length > 0;
    const isDefaultGnbActive = status?.gnbLinkState && status.gnbLinkState !== 'offline';

    if (hasFleetGnbs) {
      setUeToLaunch(name);
      setSelectedTargetGnb(fleetRunning.runningGnbs[0].profileName);
    } else if (isDefaultGnbActive) {
      launchUEProfile(name);
    } else {
      showFleetMsg('Cannot launch UE. No active gNodeB cell detected. Please launch a gNodeB first.', 'error');
    }
  };

  const launchGNBProfile = async (name: string) => {
    try {
      const res = await fetch(`${API_BASE}/fleet/launch/gnb`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ profileName: name })
      });
      if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
      showFleetMsg(`gNB '${name}' launched successfully`, 'success');
      setFleetActiveSection('live');
      fetchFleetRunning();
    } catch (e) { showFleetMsg(`Error: ${e}`, 'error'); }
  };

  const stopFleetUE = async (ueId: number) => {
    try {
      const res = await fetch(`${API_BASE}/fleet/stop/ue`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ueId })
      });
      if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
      showFleetMsg(`UE ${ueId} terminated`, 'success');
      fetchFleetRunning();
    } catch (e) { showFleetMsg(`Error: ${e}`, 'error'); }
  };

  const stopFleetGNB = async (name: string) => {
    try {
      const res = await fetch(`${API_BASE}/fleet/stop/gnb/${encodeURIComponent(name)}`, { method: 'POST' });
      if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
      showFleetMsg(`gNB '${name}' stopping...`, 'success');
      fetchFleetRunning();
    } catch (e) { showFleetMsg(`Error: ${e}`, 'error'); }
  };

  const consoleEndRef = useRef<HTMLDivElement>(null);
  const pingLogEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (pingLogEndRef.current) {
      pingLogEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [uePingLog]);


  // Diagnostics & PCAP Fetchers & Handlers
  const fetchPcapInterfaces = async () => {
    try {
      const res = await fetch(`${API_BASE}/diagnostics/pcap/interfaces`);
      if (res.ok) {
        const data = await res.json();
        setPcapInterfaces(data);
      }
    } catch (err) {
      console.error("Error fetching pcap interfaces:", err);
    }
  };

  const fetchPcapStatus = async () => {
    try {
      const res = await fetch(`${API_BASE}/diagnostics/pcap/status`);
      if (res.ok) {
        const data = await res.json();
        setActiveCaptureStatus(data);
      }
    } catch (err) {
      console.error("Error fetching pcap status:", err);
    }
  };

  const fetchSavedPcapFiles = async () => {
    setIsRefreshingPcaps(true);
    try {
      const res = await fetch(`${API_BASE}/diagnostics/pcap/list`);
      if (res.ok) {
        const data = await res.json();
        setSavedPcapFiles(data);
      }
    } catch (err) {
      console.error("Error fetching saved pcaps:", err);
    } finally {
      setIsRefreshingPcaps(false);
    }
  };

  const fetchDiagnosticsLogs = async () => {
    setIsRefreshingLogs(true);
    try {
      const res = await fetch(`${API_BASE}/diagnostics/logs/history`);
      if (res.ok) {
        const data = await res.json();
        setDiagnosticsLogs(data.logs || []);
      }
    } catch (err) {
      console.error("Error fetching logs history:", err);
    } finally {
      setIsRefreshingLogs(false);
    }
  };

  const startPcapCapture = async () => {
    if (!pcapFileName.trim()) {
      alert("Please provide a filename for the capture.");
      return;
    }
    setIsStartingPcap(true);
    try {
      const res = await fetch(`${API_BASE}/diagnostics/pcap/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          interface: pcapInterface,
          protocol: pcapProtocol,
          fileName: pcapFileName
        })
      });
      if (res.ok) {
        await fetchPcapStatus();
        await fetchSavedPcapFiles();
      } else {
        const errMsg = await res.text();
        alert(`Failed to start capture: ${errMsg}`);
      }
    } catch (err) {
      console.error("Error starting pcap capture:", err);
      alert(`Error starting capture: ${err}`);
    } finally {
      setIsStartingPcap(false);
    }
  };

  const stopPcapCapture = async () => {
    setIsStoppingPcap(true);
    try {
      const res = await fetch(`${API_BASE}/diagnostics/pcap/stop`, {
        method: 'POST'
      });
      if (res.ok) {
        await fetchPcapStatus();
        await fetchSavedPcapFiles();
      } else {
        const errMsg = await res.text();
        alert(`Failed to stop capture: ${errMsg}`);
      }
    } catch (err) {
      console.error("Error stopping pcap capture:", err);
      alert(`Error stopping capture: ${err}`);
    } finally {
      setIsStoppingPcap(false);
    }
  };

  const deletePcapFile = async (name: string) => {
    if (!confirm(`Are you sure you want to delete ${name}?`)) return;
    try {
      const res = await fetch(`${API_BASE}/diagnostics/pcap/delete?file=${encodeURIComponent(name)}`, {
        method: 'DELETE'
      });
      if (res.ok) {
        await fetchSavedPcapFiles();
      } else {
        const errMsg = await res.text();
        alert(`Failed to delete: ${errMsg}`);
      }
    } catch (err) {
      console.error("Error deleting pcap file:", err);
    }
  };

  const clearSystemLogs = async () => {
    if (!confirm("Are you sure you want to clear the emulator system log file? This will empty the backend log file.")) return;
    setIsClearingLogs(true);
    try {
      const res = await fetch(`${API_BASE}/diagnostics/logs/clear`, {
        method: 'POST'
      });
      if (res.ok) {
        setDiagnosticsLogs([]);
        await fetchDiagnosticsLogs();
      } else {
        const errMsg = await res.text();
        alert(`Failed to clear logs: ${errMsg}`);
      }
    } catch (err) {
      console.error("Error clearing logs:", err);
    } finally {
      setIsClearingLogs(false);
    }
  };

  // Effect to handle diagnostics tab load and polling
  useEffect(() => {
    if (activeTab === 'diagnostics') {
      fetchPcapInterfaces();
      fetchPcapStatus();
      fetchSavedPcapFiles();
      fetchDiagnosticsLogs();
      if (!pcapFileName) {
        setPcapFileName(`capture_${Math.floor(Date.now() / 1000)}.pcap`);
      }
    }
  }, [activeTab]);

  // Polling active capture status when capture is running
  useEffect(() => {
    if (activeTab !== 'diagnostics' || !activeCaptureStatus?.isCapturing) return;
    const timer = setInterval(() => {
      fetchPcapStatus();
    }, 1500);
    return () => clearInterval(timer);
  }, [activeTab, activeCaptureStatus?.isCapturing]);

  // Fetch emulator status
  const fetchStatus = async () => {
    try {
      const res = await fetch(`${API_BASE}/status`);
      if (res.ok) {
        const data = await res.json();
        setStatus(data);
        if (data.activeRelease) {
          setActiveRelease(data.activeRelease);
        }
        if (data.configSummary && !pingHost) {
          // Prefill ping host with AMF IP
          const ip = data.configSummary.amfTarget.split(':')[0];
          setPingHost(ip);
        }
      }
    } catch (err) {
      console.error('Error fetching status:', err);
    }
  };

  // Fetch emulator config
  const fetchConfig = async () => {
    try {
      const res = await fetch(`${API_BASE}/config`);
      if (res.ok) {
        const data = await res.json();
        setConfigData(data);
      }
    } catch (err) {
      console.error('Error fetching config:', err);
    }
  };

  // Fetch scenarios list
  const fetchScenarios = async () => {
    try {
      const res = await fetch(`${API_BASE}/scenarios`);
      if (res.ok) {
        const data = await res.json();
        setScenarios(data);
      }
    } catch (err) {
      console.error('Error fetching scenarios:', err);
    }
  };

  // Fetch active UEs
  const fetchActiveUEs = async () => {
    try {
      const res = await fetch(`${API_BASE}/ue/active`);
      if (res.ok) {
        const data = await res.json();
        setActiveUEs(data || []);
      }
    } catch (err) {
      console.error('Error fetching active UEs:', err);
    }
  };

  // Fetch traffic stats
  const fetchTrafficStats = async () => {
    try {
      const res = await fetch(`${API_BASE}/ue/traffic/stats`);
      if (res.ok) {
        const data = await res.json();
        setTrafficStats(data || {});
      }
    } catch (err) {
      console.error('Error fetching traffic stats:', err);
    }
  };

  // User Plane Traffic Actions
  const startUePing = async (ueId: number) => {
    setUePingRunning(true);
    setUePingLog('Pinging in progress...');
    try {
      const res = await fetch(`${API_BASE}/ue/ping`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ueId, host: uePingTarget })
      });
      if (res.ok) {
        const data = await res.json();
        setUePingLog(data.output);
      } else {
        setUePingLog(`Ping failed: ${await res.text()}`);
      }
    } catch (err) {
      setUePingLog(`Ping error: ${err}`);
    } finally {
      setUePingRunning(false);
    }
  };

  const fetchUeHttp = async (ueId: number) => {
    setBrowserRunning(true);
    setBrowserResult(null);
    try {
      const res = await fetch(`${API_BASE}/ue/http`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ueId, url: browserUrl })
      });
      if (res.ok) {
        const data = await res.json();
        setBrowserResult(data);
      } else {
        alert(`HTTP fetch failed: ${await res.text()}`);
      }
    } catch (err) {
      console.error(err);
      alert(`HTTP error: ${err}`);
    } finally {
      setBrowserRunning(false);
    }
  };

  const toggleUeStream = async (ueId: number, action: string) => {
    try {
      const res = await fetch(`${API_BASE}/ue/stream`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ueId, action, quality: videoQuality })
      });
      if (res.ok) {
        fetchTrafficStats();
      } else {
        alert(`Stream action failed: ${await res.text()}`);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const dialVonr = async (callerId: number) => {
    try {
      const res = await fetch(`${API_BASE}/ue/vonr/dial`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ callerId, calleeId: vonrCallee })
      });
      if (res.ok) {
        const data = await res.json();
        setVonrActiveCall(data);
        fetchTrafficStats();
      } else {
        alert(`VoNR dial failed: ${await res.text()}`);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const hangupVonr = async (callerId: number) => {
    try {
      const res = await fetch(`${API_BASE}/ue/vonr/hangup`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ callerId })
      });
      if (res.ok) {
        setVonrActiveCall(null);
        fetchTrafficStats();
      } else {
        alert(`VoNR hangup failed: ${await res.text()}`);
      }
    } catch (err) {
      console.error(err);
    }
  };

  // Trigger UE action
  const triggerUeAction = async (action: string, extra = {}) => {
    if (controlUeId === null) return;
    try {
      const res = await fetch(`${API_BASE}/ue/action`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ueId: controlUeId,
          action,
          ...extra
        })
      });
      if (res.ok) {
        alert(`Successfully triggered ${action} on UE #${controlUeId}`);
        fetchActiveUEs();
      } else {
        const text = await res.text();
        alert(`Failed to trigger action: ${text}`);
      }
    } catch (err) {
      console.error(err);
      alert(`Action error: ${err}`);
    }
  };

  // Stop executing scenario
  const stopScenario = async () => {
    try {
      const res = await fetch(`${API_BASE}/scenarios/stop`, {
        method: 'POST'
      });
      if (res.ok) {
        fetchStatus();
        fetchActiveUEs();
      } else {
        alert('Failed to stop scenario.');
      }
    } catch (err) {
      console.error(err);
    }
  };

  // SSE Real-time log stream
  useEffect(() => {
    const sse = new EventSource(`${API_BASE}/logs/stream`);

    sse.onmessage = (event) => {
      const line = event.data;
      if (line === 'log_stream_started') {
        setLogs([{ text: 'Connected to 5G Emulator Real-time Log Stream...', level: 'system', timestamp: new Date().toLocaleTimeString() }]);
        return;
      }

      // Parse log message style from logrus string format
      let level: 'info' | 'warn' | 'error' | 'debug' | 'system' = 'info';
      if (line.includes('level=warning') || line.includes('WARN') || line.includes('⚠️')) {
        level = 'warn';
      } else if (line.includes('level=error') || line.includes('ERRO') || line.includes('FAIL')) {
        level = 'error';
      } else if (line.includes('level=debug') || line.includes('DEBU')) {
        level = 'debug';
      } else if (line.includes('[SCENARIO]') || line.includes('✅')) {
        level = 'system';
      }

      setLogs((prev) => [
        ...prev.slice(-499), // Cap logs at 500 lines to preserve DOM memory
        { text: line, level, timestamp: new Date().toLocaleTimeString() }
      ]);
    };

    sse.onerror = (err) => {
      console.error('SSE connection error:', err);
      sse.close();
    };

    return () => {
      sse.close();
    };
  }, []);

  // Selected UE sync
  useEffect(() => {
    if (activeUEs.length > 0 && controlUeId === null) {
      setControlUeId(activeUEs[0].id);
    } else if (activeUEs.length === 0) {
      setControlUeId(null);
    }
  }, [activeUEs]);

  // Poll status every 2 seconds if auto-refresh is active
  useEffect(() => {
    fetchStatus();
    fetchConfig();
    fetchScenarios();
    fetchActiveUEs();
    fetchTrafficStats();
  }, []);

  useEffect(() => {
    if (!autoRefresh) return;
    const timer = setInterval(() => {
      fetchStatus();
      fetchActiveUEs();
      fetchTrafficStats();
    }, 2000);
    return () => clearInterval(timer);
  }, [autoRefresh]);

  // Run Connectivity Checks on load or tab change
  useEffect(() => {
    if (activeTab === 'connectivity') {
      runConnectivityChecks();
    }
  }, [activeTab, status]);

  // Scroll terminal logs to bottom
  useEffect(() => {
    if (autoscroll && consoleEndRef.current) {
      consoleEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, autoscroll]);

  const runConnectivityChecks = async () => {
    setChecks({
      sctpModule: 'checking',
      ipipModule: 'checking',
      amfReachability: 'checking',
      socketsClean: 'checking'
    });

    // Determine AMF Reachability
    let amfOk = 'error';
    if (status?.gnbLinkState === 'listening' || status?.gnbLinkState === 'socket_active') {
      amfOk = 'success';
    } else {
      try {
        // Try quick ping test to see if host responds
        const amfIp = status?.configSummary?.amfTarget ? status.configSummary.amfTarget.split(':')[0] : '127.0.0.1';
        const res = await fetch(`${API_BASE}/ping`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ host: amfIp })
        });
        if (res.ok) {
          const result = await res.json();
          amfOk = result.success ? 'success' : 'error';
        }
      } catch {
        amfOk = 'error';
      }
    }

    // Check sockets clean
    const socketsOk = !status?.isRunning ? 'success' : 'warning';

    // Mock kernel modules since they require raw filesystem reading or command run which we evaluate on backend status
    // Standard Linux installs usually have sctp and ipip loaded when running Emulator.
    // If they aren't loaded, they can cause errors. We'll default to success if AMF or local sockets are active.
    const modulesOk = status?.interfaces && status.interfaces.length > 0 ? 'success' : 'warning';

    setChecks({
      sctpModule: modulesOk,
      ipipModule: modulesOk,
      amfReachability: amfOk,
      socketsClean: socketsOk
    });
  };

  // Launch Scenario
  const runScenario = async (id: string) => {
    if (status?.isRunning) return;

    // Direct user to logs tab to view details
    setActiveTab('logs');

    try {
      const res = await fetch(`${API_BASE}/scenarios/run`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          scenario: id,
          targetGnbIp,
          targetGnbPort,
          delay,
          idleSeconds,
          ueCount,
          requests,
          duration,
          ueOnly
        })
      });
      if (res.ok) {
        fetchStatus();
      } else {
        const text = await res.text();
        alert(`Failed to run scenario: ${text}`);
      }
    } catch (err) {
      console.error(err);
    }
  };

  // Run ping test
  const executePing = async () => {
    if (pingRunning) return;
    setPingRunning(true);
    setPingResult('Sending ICMP Echo requests...');
    try {
      const res = await fetch(`${API_BASE}/ping`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ host: pingHost })
      });
      if (res.ok) {
        const data = await res.json();
        setPingResult(data.output);
      } else {
        setPingResult('Ping request failed.');
      }
    } catch (err) {
      setPingResult(`Error: ${err}`);
    } finally {
      setPingRunning(false);
    }
  };

  // Handle configuration update
  const handleConfigChange = (path: string, value: any) => {
    if (!configData) return;
    const newCfg = { ...configData };
    const parts = path.split('.');
    
    let current: any = newCfg;
    for (let i = 0; i < parts.length - 1; i++) {
      current = current[parts[i]];
    }
    current[parts[parts.length - 1]] = value;
    
    setConfigData(newCfg);
  };

  const saveConfig = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!configData) return;

    try {
      const res = await fetch(`${API_BASE}/config`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(configData)
      });
      if (res.ok) {
        alert('Configuration saved successfully! Ready to apply on next scenario execution.');
        fetchStatus();
      } else {
        const text = await res.text();
        alert(`Failed to save configuration: ${text}`);
      }
    } catch (err) {
      console.error('Error saving config:', err);
      alert(`Error saving configuration: ${err}`);
    }
  };

  // Filter logs based on selection and keyword search query
  const filteredLogs = logs.filter((log) => {
    const matchesLevel = logFilter === 'all' || log.level === logFilter;
    const matchesSearch = logSearch === '' || log.text.toLowerCase().includes(logSearch.toLowerCase());
    return matchesLevel && matchesSearch;
  });

  // Check if any active virtual interface exists
  const activeTuns = status?.interfaces?.filter(
    (i) => i.name.startsWith('uetun')
  ) || [];

  return (
    <div className="app-container">
      {/* Sidebar Navigation */}
      <aside className="sidebar">
        <div className="logo-section"> 
          <img 
            src={theme === 'dark' ? '/omniran_dark.png' : '/omniran_light.png'} 
            alt="OmniRAN 5G Logo" 
            className="sidebar-logo" 
            style={{ width: '100%', height: '100%', objectFit: 'contain' }}
          /> 
        </div>

        <nav className="nav-menu">
          <li
            className={`nav-item ${activeTab === 'dashboard' ? 'active' : ''}`}
            onClick={() => setActiveTab('dashboard')}
          >
            <Activity />
            <span>Dashboard</span>
          </li>
          <li
            className={`nav-item ${activeTab === 'scenarios' ? 'active' : ''}`}
            onClick={() => setActiveTab('scenarios')}
          >
            <Play />
            <span>Scenario Runner</span>
          </li>
          <li
            className={`nav-item ${activeTab === 'config' ? 'active' : ''}`}
            onClick={() => setActiveTab('config')}
          >
            <Settings />
            <span>Configuration</span>
          </li>
          <li
            className={`nav-item ${activeTab === 'logs' ? 'active' : ''}`}
            onClick={() => setActiveTab('logs')}
          >
            <Terminal />
            <span>Live Console</span>
          </li>
          <li
            className={`nav-item ${activeTab === 'connectivity' ? 'active' : ''}`}
            onClick={() => setActiveTab('connectivity')}
          >
            <Network />
            <span>Connectivity Tool</span>
          </li>
          <li
            className={`nav-item ${activeTab === 'fleet' ? 'active' : ''}`}
            onClick={() => setActiveTab('fleet')}
          >
            <Radio />
            <span>Fleet Manager</span>
          </li>
          <li
            className={`nav-item ${activeTab === 'diagnostics' ? 'active' : ''}`}
            onClick={() => setActiveTab('diagnostics')}
          >
            <Sliders />
            <span>Diagnostics</span>
          </li>
        </nav>

        <div className="sidebar-footer">
          <span className="footer-label">EMULATOR VERSION</span>
          <span className="footer-value">v1.0.1 SA</span>
        </div>
      </aside>

      {/* Main Panel Content */}
      <main className="main-content">
        <header className="top-header">
          <div className="header-title-section">
            <h1 className="capitalize">
              {activeTab === 'dashboard' && 'Dashboard Panel'}
              {activeTab === 'scenarios' && 'Scenario Runner Panel'}
              {activeTab === 'config' && 'Configuration Panel'}
              {activeTab === 'logs' && 'Live Console Panel'}
              {activeTab === 'connectivity' && 'Connectivity Tool Panel'}
              {activeTab === 'fleet' && 'Fleet Manager'}
              {activeTab === 'diagnostics' && 'Diagnostics & Captures'}
            </h1>
          </div>

          <div className="header-status-bar">
            {/* Stop Scenario Button */}
            {status?.isRunning && (
              <button
                onClick={stopScenario}
                className="status-badge stop-badge"
                title="Stop Scenario"
              >
                <Trash2 size={14} style={{ flexShrink: 0 }} />
                <span className="font-bold">STOP SCENARIO</span>
              </button>
            )}

            {/* Auto-Refresh Toggle Button */}
            <button
              onClick={() => setAutoRefresh(!autoRefresh)}
              className={`status-badge polling-badge ${!autoRefresh ? 'paused' : ''}`}
              title="Toggle Auto Refresh Polling"
            >
              <RefreshCw size={14} className={autoRefresh && status?.isRunning ? 'animate-spin' : ''} style={{ flexShrink: 0 }} />
              <span className="font-semibold">{autoRefresh ? 'LIVE POLLING' : 'PAUSED'}</span>
            </button>

            {/* Show/Hide Warnings/Tips Toggle */}
            <button
              onClick={() => {
                if (showBanners) {
                  setBannerFade(true);
                  setTimeout(() => setShowBanners(false), 500);
                } else {
                  setShowBanners(true);
                  setBannerFade(false);
                }
              }}
              className={`status-badge warnings-badge ${!showBanners ? 'hidden' : ''}`}
              title="Toggle Warning Banners / Info Tips"
            >
              <AlertTriangle size={14} style={{ flexShrink: 0 }} />
              <span className="font-semibold">{showBanners ? 'HIDE WARNINGS/TIPS' : 'SHOW WARNINGS/TIPS'}</span>
            </button>

            {/* 3GPP Release Selector Dropdown */}
            <div className="status-badge" style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '4px 12px' }}>
              <span className="text-muted" style={{ fontSize: '11px', fontWeight: 600 }}>3GPP Spec:</span>
              <select
                value={activeRelease}
                onChange={(e) => updateRelease(e.target.value)}
                className="release-select"
                title="Select active 3GPP release for emulation capabilities"
              >
                {Array.from(new Set([activeRelease, '15', '17', '18', '19'])).filter(Boolean).sort().map((rel) => (
                  <option 
                    key={rel} 
                    value={rel}
                    style={{ background: 'var(--bg-panel)', color: 'var(--text-primary)' }}
                  >
                    {rel === '15' ? 'Release 15/16 (Baseline)' : rel === '17' ? 'Release 17 (RedCap/NTN)' : rel === '18' ? 'Release 18 (5G-Advanced)' : rel === '19' ? 'Release 19 (AI & Sensing)' : `Release ${rel}`}
                  </option>
                ))}
              </select>
            </div>

            {/* Theme Toggle Button */}
            <button
              onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
              className="status-badge theme-badge"
              title="Toggle Theme Mode"
            >
              {theme === 'dark' ? <Sun size={14} style={{ color: '#fbbf24', flexShrink: 0 }} /> : <Moon size={14} style={{ color: '#6366f1', flexShrink: 0 }} />}
              <span className="font-semibold">{theme === 'dark' ? 'LIGHT' : 'DARK'}</span>
            </button>

            {/* AMF Connection status */}
            <div className="status-badge">
              <span className="text-muted">AMF Core:</span>
              <Server 
                size={13} 
                style={{ 
                  color: status?.gnbLinkState && status.gnbLinkState !== 'offline' ? '#10b981' : '#ef4444',
                  flexShrink: 0 
                }} 
              />
              <span className="font-semibold">
                {status?.gnbLinkState && status.gnbLinkState !== 'offline' ? 'CONNECTED' : 'DISCONNECTED'}
              </span>
            </div>

            {/* Local gNB State */}
            <div className="status-badge">
              <span className="text-muted">gNodeB:</span>
              <span className="font-semibold text-sky-400 family-monospace">
                {status?.gnbLinkState === 'listening'
                  ? 'TCP LISTENING'
                  : status?.gnbLinkState === 'socket_active'
                  ? 'UNIX ACTIVE'
                  : 'OFFLINE'}
              </span>
            </div>

            {/* Running Scenario */}
            <div className="status-badge">
              <span className="text-muted">Status:</span>
              <Activity 
                size={13} 
                className={status?.isRunning ? 'animate-pulse' : ''} 
                style={{ 
                  color: status?.isRunning ? '#f59e0b' : '#10b981',
                  flexShrink: 0 
                }} 
              />
              {status?.isRunning ? (
                <span className="text-amber-500 font-bold uppercase">{status.runningName}</span>
              ) : (
                <span className="text-emerald-500 font-bold">IDLE</span>
              )}
            </div>
          </div>
        </header>

        {/* Dynamic Tab Body content */}
        {activeTab === 'dashboard' && (
          <div className="view-body fade-in">
            {showBanners && (
              <div className={`warning-banner ${bannerFade ? 'fade-out' : ''}`}>
                <AlertTriangle size={18} />
                <div>
                  <strong>Root Permissions Reminder:</strong> Running 5G control and user plane scenarios involves setting up Linux network interfaces (e.g. <code>uetun1</code>) and routing tables. Ensure the backend Go process is running with root privileges (<code>sudo ./app web</code>) to avoid network configuration errors.
                </div>
              </div>
            )}

            {/* KPI Cards Grid */}
            <div className="stats-grid">
              {/* Card 1: Active GNodeBs */}
              <div className="card interactive-kpi primary-hover" onClick={() => setActiveTab('fleet')} title="Click to open Fleet Manager">
                <div className="card-header">
                  <span className="card-title">Active GNodeBs</span>
                  <div className="card-icon primary">
                    <Radio size={18} />
                  </div>
                </div>
                <div className="card-value">{status?.runningGnbs?.length || 0}</div>
                <span className="card-desc">
                  {status?.runningGnbs && status.runningGnbs.length > 0
                    ? `Running: ${status.runningGnbs.map((g) => g.profileName).join(', ')}`
                    : 'No active gNB instances'}
                </span>
              </div>

              {/* Card 2: Active UEs Online */}
              <div className="card interactive-kpi success-hover" onClick={() => { const el = document.getElementById('activeUEsTable'); if (el) el.scrollIntoView({ behavior: 'smooth' }); }} title="Click to view active UEs list">
                <div className="card-header">
                  <span className="card-title">Active UEs Online</span>
                  <div className="card-icon success">
                    <Cpu size={18} />
                  </div>
                </div>
                <div className="card-value">
                  {activeUEs.filter((u) => u.stateMmDesc?.includes('REGISTERED')).length}
                </div>
                <span className="card-desc">
                  {activeUEs.filter((u) => u.stateMmDesc?.includes('REGISTERED')).length > 0
                    ? `Registered: ${activeUEs.filter((u) => u.stateMmDesc?.includes('REGISTERED')).map((u) => `UE-${u.id}`).join(', ')} (${activeUEs.length} total)`
                    : activeUEs.length > 0
                    ? `0 of ${activeUEs.length} UEs registered`
                    : 'No active UEs registered'}
                </span>
              </div>

              {/* Card 3: AMF Core Target */}
              <div className="card interactive-kpi info-hover" onClick={() => setActiveTab('connectivity')} title="Click to test core connectivity">
                <div className="card-header">
                  <span className="card-title">AMF Core Target</span>
                  <div className="card-icon info">
                    <Server size={18} />
                  </div>
                </div>
                <div className="card-value">{status?.configSummary?.amfTarget || '127.0.0.1:38412'}</div>
                <span className="card-desc">5G Core control plane SCTP binding</span>
              </div>

              {/* Card 4: Active GTP Tunnels */}
              <div className="card interactive-kpi purple-hover" onClick={() => { const el = document.getElementById('activeUEsTable'); if (el) el.scrollIntoView({ behavior: 'smooth' }); }} title="Click to view active UEs and sessions">
                <div className="card-header">
                  <span className="card-title">Active GTP Tunnels</span>
                  <div className="card-icon purple">
                    <Network size={18} />
                  </div>
                </div>
                <div className="card-value">
                  {activeUEs.reduce((acc, u) => acc + (u.pduSessions?.filter((s: any) => s.stateDesc?.includes('ACTIVE')).length || 0), 0)}
                </div>
                <span className="card-desc">
                  {activeUEs.reduce((acc, u) => acc + (u.pduSessions?.filter((s: any) => s.stateDesc?.includes('ACTIVE')).length || 0), 0) > 0
                    ? `Active PDU sessions tunnel data (${activeTuns.length} interfaces)`
                    : activeTuns.length > 0
                    ? `0 sessions (${activeTuns.length} interfaces)`
                    : 'No virtual interfaces active'}
                </span>
              </div>
            </div>

            {/* Network Topology Visualizer */}
            <div className="topology-panel">
              <h3 className="panel-title">
                <Sliders size={18} /> 3GPP 5G SA Network Topology Visualizer
              </h3>
              
              <div className="topology-layout">
                {/* Left side: Canvas */}
                <div className="topology-canvas" style={{ position: 'relative' }}>
                  <svg className="topology-svg" viewBox="0 0 1000 600" style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none' }}>
                    {renderSvgLinks()}
                    {renderSvgNodes()}
                  </svg>
                </div>

                {/* Right side: Inspector Panel */}
                <div className="node-inspector-card">
                  {selectedNode === 'ue' && (() => {
                    const activeUe = getActiveUeToInspect();
                    if (activeUe) {
                      const tInfo = trafficStats[activeUe.id];
                      const isStreaming = tInfo?.activeAction === 'streaming';
                      const activeCall = tInfo?.vonrCall || vonrActiveCall;
                      const isCallActive = !!activeCall && activeCall.status !== 'disconnected';
                      const otherUes = status?.runningUes?.filter((x: any) => x.id !== activeUe.id) || [];

                      return (
                        <>
                          <h4 className="inspector-title" style={{ color: 'var(--color-success)', marginBottom: '8px' }}>
                            <Cpu size={14} /> UE-{activeUe.id} Inspector
                          </h4>
                          
                          {/* Tab buttons */}
                          <div className="inspector-tabs" style={{ display: 'flex', borderBottom: '1px solid rgba(255,255,255,0.1)', marginBottom: '12px' }}>
                            <button 
                              className={`inspector-tab-btn ${inspectorTab === 'details' ? 'active' : ''}`}
                              onClick={() => setInspectorTab('details')}
                              style={{
                                flex: 1,
                                padding: '6px 8px',
                                background: 'transparent',
                                border: 'none',
                                borderBottom: inspectorTab === 'details' ? '2px solid var(--color-success)' : '2px solid transparent',
                                color: inspectorTab === 'details' ? 'var(--color-success)' : 'var(--text-muted)',
                                cursor: 'pointer',
                                fontWeight: '600',
                                fontSize: '11px',
                                textAlign: 'center',
                                transition: 'all 0.2s'
                              }}
                            >
                              Details
                            </button>
                            <button 
                              className={`inspector-tab-btn ${inspectorTab === 'traffic' ? 'active' : ''}`}
                              onClick={() => setInspectorTab('traffic')}
                              style={{
                                flex: 1,
                                padding: '6px 8px',
                                background: 'transparent',
                                border: 'none',
                                borderBottom: inspectorTab === 'traffic' ? '2px solid var(--color-success)' : '2px solid transparent',
                                color: inspectorTab === 'traffic' ? 'var(--color-success)' : 'var(--text-muted)',
                                cursor: 'pointer',
                                fontWeight: '600',
                                fontSize: '11px',
                                textAlign: 'center',
                                transition: 'all 0.2s'
                              }}
                            >
                              Data Plane Tools
                            </button>
                          </div>

                          {inspectorTab === 'details' ? (
                            <div className="inspector-details" style={{ maxHeight: '380px', overflowY: 'auto' }}>
                              <div className="detail-row">
                                <span className="detail-label">SUPI</span>
                                <span className="detail-val font-mono">{activeUe.supi}</span>
                              </div>
                              <div className="detail-row">
                                <span className="detail-label">SUCI</span>
                                <span className="detail-val font-mono" style={{ fontSize: '11px' }}>
                                  {`suci-0-999-70-0-0-0-${activeUe.supi.slice(-5)}`}
                                </span>
                              </div>
                              <div className="detail-row">
                                <span className="detail-label">5G-GUTI</span>
                                <span className="detail-val font-mono" style={{ fontSize: '11px' }}>
                                  {`guti-999-70-0001-${activeUe.supi.slice(-4)}`}
                                </span>
                              </div>
                              <div className="detail-row">
                                <span className="detail-label">5GMM State</span>
                                <span className="detail-val font-semibold" style={{ color: activeUe.stateMmDesc?.includes('REGISTERED') ? 'var(--color-success)' : 'var(--text-muted)' }}>
                                  {activeUe.stateMmDesc}
                                </span>
                              </div>
                              <div className="detail-row">
                                <span className="detail-label">5GMM Connection</span>
                                <span className="detail-val font-semibold" style={{ color: activeUe.stateMmDesc?.includes('REGISTERED') ? 'var(--color-success)' : 'var(--text-muted)' }}>
                                  {activeUe.stateMmDesc?.includes('REGISTERED') ? 'CONNECTED (N1)' : 'IDLE'}
                                </span>
                              </div>
                              <div className="detail-row">
                                <span className="detail-label">RRC State</span>
                                <span className="detail-val font-semibold" style={{ color: activeUe.stateMmDesc?.includes('REGISTERED') ? 'var(--color-success)' : 'var(--text-muted)' }}>
                                  {activeUe.stateMmDesc?.includes('REGISTERED') ? 'RRC-CONNECTED' : 'RRC-IDLE'}
                                </span>
                              </div>
                              <div className="detail-row">
                                <span className="detail-label">Connected Cell</span>
                                <span className="detail-val font-semibold" style={{ color: 'var(--color-primary)' }}>
                                  {activeUe.gnbProfileName ? `${activeUe.gnbProfileName} (${activeUe.gnbId || '—'})` : '—'}
                                </span>
                              </div>
                              <div className="detail-row" style={{ flexDirection: 'column', alignItems: 'flex-start', borderTop: '1px solid rgba(255,255,255,0.05)', paddingTop: '8px', marginTop: '8px' }}>
                                <span className="detail-label" style={{ marginBottom: '4px' }}>PDU Sessions ({activeUe.pduSessions?.length || 0})</span>
                                {activeUe.pduSessions && activeUe.pduSessions.length > 0 ? (
                                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', width: '100%' }}>
                                    {activeUe.pduSessions.map(s => (
                                      <div key={s.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '11px', width: '100%' }}>
                                        <span style={{ color: 'var(--color-info)' }}>PDU #{s.id} ({s.dnn}):</span>
                                        <span className="font-mono ml-auto" style={{ marginRight: '6px' }}>{s.ueIp || '—'}</span>
                                        <span className={`fleet-state-badge sm ${s.stateDesc?.includes('ACTIVE') ? 'registered' : 'pending'}`}>{s.stateDesc}</span>
                                      </div>
                                    ))}
                                  </div>
                                ) : (
                                  <span className="detail-val" style={{ color: 'var(--text-muted)', fontSize: '11px' }}>None</span>
                                )}
                              </div>
                            </div>
                          ) : (
                            <div className="inspector-details" style={{ maxHeight: '380px', overflowY: 'auto', paddingRight: '4px' }}>
                              
                              {/* Warning overlay if PDU session not active */}
                              {!activeUe.pduSessions?.some((s: any) => s.stateDesc?.includes('ACTIVE')) && (
                                <div style={{
                                  background: 'rgba(245, 158, 11, 0.08)',
                                  border: '1px solid rgba(245, 158, 11, 0.2)',
                                  borderRadius: '6px',
                                  padding: '8px',
                                  marginBottom: '12px',
                                  fontSize: '10px',
                                  color: '#f59e0b',
                                  lineHeight: '1.4'
                                }}>
                                  <div style={{ fontWeight: 'bold', display: 'flex', alignItems: 'center', gap: '4px', marginBottom: '2px' }}>
                                    <AlertTriangle size={12} /> PDU Session Not Active
                                  </div>
                                  <div>
                                    This UE does not have an active user-plane PDU Session. Establish a session in the Details tab or run a scenario first. Real data will fall back to simulation.
                                  </div>
                                </div>
                              )}

                              {/* 1. Ping Tool */}
                              <div className="tool-section" style={{ borderBottom: '1px solid rgba(255,255,255,0.05)', paddingBottom: '12px', marginBottom: '12px' }}>
                                <h5 style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '11px', fontWeight: 'bold', color: 'var(--color-success)', margin: '0 0 6px 0' }}>
                                  <Terminal size={12} /> Ping Utility
                                </h5>
                                <div style={{ display: 'flex', gap: '6px', marginBottom: '6px' }}>
                                  <input 
                                    type="text" 
                                    value={uePingTarget} 
                                    onChange={(e) => setUePingTarget(e.target.value)} 
                                    placeholder="8.8.8.8"
                                    style={{ flex: 1, padding: '4px 6px', borderRadius: '4px', background: '#0f172a', border: '1px solid var(--border-color)', color: '#ffffff', fontSize: '11px' }}
                                  />
                                  <button 
                                    onClick={() => startUePing(activeUe.id)} 
                                    disabled={uePingRunning}
                                    style={{ padding: '4px 8px', borderRadius: '4px', background: 'var(--color-success)', color: '#000000', border: 'none', fontWeight: 'bold', fontSize: '11px', cursor: 'pointer' }}
                                  >
                                    {uePingRunning ? 'Ping...' : 'Run'}
                                  </button>
                                </div>
                                {uePingLog && (
                                  <pre style={{ background: '#0f172a', color: '#10b981', fontFamily: 'monospace', fontSize: '9px', padding: '6px', borderRadius: '4px', maxHeight: '100px', overflowY: 'auto', margin: 0, border: '1px solid rgba(16,185,129,0.1)', position: 'relative' }}>
                                    {uePingLog}
                                    <div ref={pingLogEndRef} style={{ height: 1 }} />
                                  </pre>
                                )}
                              </div>

                              {/* 2. Web Browser */}
                              <div className="tool-section" style={{ borderBottom: '1px solid rgba(255,255,255,0.05)', paddingBottom: '12px', marginBottom: '12px' }}>
                                <h5 style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '11px', fontWeight: 'bold', color: 'var(--color-info)', margin: '0 0 6px 0' }}>
                                  <Globe size={12} /> HTTP Web Client
                                </h5>
                                <div style={{ display: 'flex', gap: '6px', marginBottom: '6px' }}>
                                  <input 
                                    type="text" 
                                    value={browserUrl} 
                                    onChange={(e) => setBrowserUrl(e.target.value)} 
                                    placeholder="example.com"
                                    style={{ flex: 1, padding: '4px 6px', borderRadius: '4px', background: '#0f172a', border: '1px solid var(--border-color)', color: '#ffffff', fontSize: '11px' }}
                                  />
                                  <button 
                                    onClick={() => fetchUeHttp(activeUe.id)} 
                                    disabled={browserRunning}
                                    style={{ padding: '4px 8px', borderRadius: '4px', background: 'var(--color-info)', color: '#ffffff', border: 'none', fontWeight: 'bold', fontSize: '11px', cursor: 'pointer' }}
                                  >
                                    {browserRunning ? 'Fetch...' : 'Fetch'}
                                  </button>
                                </div>
                                {browserResult && (
                                  <div style={{ marginTop: '6px' }}>
                                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '10px', color: 'var(--text-muted)', marginBottom: '4px' }}>
                                      <span>Status: <strong style={{ color: browserResult.statusCode === 200 ? 'var(--color-success)' : '#ef4444' }}>{browserResult.statusCode}</strong></span>
                                      <span>Elapsed: <strong>{browserResult.timeMs}ms</strong> ({browserResult.mode})</span>
                                    </div>
                                    <div style={{ border: '1px solid rgba(255,255,255,0.1)', borderRadius: '6px', overflow: 'hidden' }}>
                                      <div style={{ background: 'rgba(255,255,255,0.05)', padding: '4px 8px', display: 'flex', alignItems: 'center', gap: '4px', borderBottom: '1px solid rgba(255,255,255,0.1)', fontSize: '9px' }}>
                                        <span style={{ color: '#ef4444', marginRight: '2px' }}>●</span>
                                        <span style={{ color: '#f59e0b', marginRight: '2px' }}>●</span>
                                        <span style={{ color: '#10b981' }}>●</span>
                                        <div style={{ background: 'rgba(0,0,0,0.3)', padding: '2px 6px', borderRadius: '4px', flex: 1, textOverflow: 'ellipsis', overflow: 'hidden', whiteSpace: 'nowrap', color: '#94a3b8', marginLeft: '6px', fontFamily: 'monospace' }}>
                                          {browserUrl}
                                        </div>
                                      </div>
                                      <div style={{ background: '#ffffff', color: '#1e293b', padding: '8px', fontSize: '10px', maxHeight: '110px', overflowY: 'auto' }}>
                                        {browserResult.body.includes('<!DOCTYPE html>') || browserResult.body.includes('<html>') ? (
                                          <div dangerouslySetInnerHTML={{ __html: browserResult.body }} />
                                        ) : (
                                          <pre style={{ margin: 0, fontSize: '9px', whiteSpace: 'pre-wrap', fontFamily: 'monospace' }}>{browserResult.body}</pre>
                                        )}
                                      </div>
                                    </div>
                                  </div>
                                )}
                              </div>

                              {/* 3. Video Stream */}
                              <div className="tool-section" style={{ borderBottom: '1px solid rgba(255,255,255,0.05)', paddingBottom: '12px', marginBottom: '12px' }}>
                                <h5 style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '11px', fontWeight: 'bold', color: 'var(--color-primary)', margin: '0 0 6px 0' }}>
                                  <Tv size={12} /> Video Stream Player
                                </h5>
                                <div style={{ display: 'flex', gap: '6px', marginBottom: '6px' }}>
                                  <select 
                                    value={videoQuality} 
                                    onChange={(e) => setVideoQuality(e.target.value)} 
                                    disabled={isStreaming}
                                    style={{ flex: 1, padding: '4px 6px', borderRadius: '4px', background: '#0f172a', border: '1px solid var(--border-color)', color: '#ffffff', fontSize: '11px' }}
                                  >
                                    <option value="720p">720p HD (2.5 Mbps)</option>
                                    <option value="1080p">1080p Full HD (6.5 Mbps)</option>
                                    <option value="4k">4K Ultra HD (20.0 Mbps)</option>
                                  </select>
                                  <button 
                                    onClick={() => toggleUeStream(activeUe.id, isStreaming ? 'stop' : 'start')} 
                                    style={{ padding: '4px 8px', borderRadius: '4px', background: isStreaming ? '#ef4444' : 'var(--color-primary)', color: '#ffffff', border: 'none', fontWeight: 'bold', fontSize: '11px', cursor: 'pointer' }}
                                  >
                                    {isStreaming ? 'Stop' : 'Play'}
                                  </button>
                                </div>
                                {isStreaming ? (
                                  <div style={{ marginTop: '6px' }}>
                                    <div style={{
                                      height: '90px',
                                      background: '#000000',
                                      borderRadius: '6px',
                                      position: 'relative',
                                      display: 'flex',
                                      flexDirection: 'column',
                                      justifyContent: 'center',
                                      alignItems: 'center',
                                      overflow: 'hidden',
                                      border: '1px solid var(--color-primary)'
                                    }}>
                                      {tInfo?.bufferSec < 8 ? (
                                        <div style={{ textAlign: 'center' }}>
                                          <div style={{ 
                                            border: '2px solid rgba(255,255,255,0.1)', 
                                            borderTop: '2px solid var(--color-primary)', 
                                            borderRadius: '50%', 
                                            width: '18px', 
                                            height: '18px', 
                                            animation: 'spin 1.2s linear infinite', 
                                            margin: '0 auto 6px auto' 
                                          }} />
                                          <span style={{ fontSize: '9px', color: '#a1a1aa' }}>Buffering ({tInfo?.bufferSec || 0}s)...</span>
                                        </div>
                                      ) : (
                                        <div style={{ width: '100%', height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'space-between', padding: '6px', boxSizing: 'border-box' }}>
                                          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '8px', color: '#ffffff' }}>
                                            <span style={{ background: 'var(--color-primary)', color: '#ffffff', padding: '1px 3px', borderRadius: '2px', fontWeight: 'bold' }}>LIVE ({videoQuality})</span>
                                            <span style={{ fontFamily: 'monospace' }}>{(tInfo?.speedMbps || 0.0).toFixed(1)} Mbps</span>
                                          </div>
                                          <div style={{ display: 'flex', gap: '2px', justifyContent: 'center', alignItems: 'flex-end', height: '35px' }}>
                                            {/* Audio waves visual simulation */}
                                            {[1,2,3,4,5,6,7,8,9,10,11,12].map(i => {
                                              const rndH = 10 + Math.sin(Date.now() / 200 + i) * 12 + Math.random() * 8;
                                              return (
                                                <div key={i} style={{
                                                  width: '4px',
                                                  height: `${rndH}px`,
                                                  background: 'var(--color-primary)',
                                                  borderRadius: '2px',
                                                  opacity: 0.8
                                                }} />
                                              );
                                            })}
                                          </div>
                                          <div style={{ width: '100%', background: 'rgba(255,255,255,0.2)', height: '3px', borderRadius: '1.5px', overflow: 'hidden' }}>
                                            <div style={{ width: `${(tInfo?.bufferSec || 15) * 4}%`, background: 'var(--color-primary)', height: '100%', borderRadius: '1.5px', transition: 'width 0.5s' }} />
                                          </div>
                                        </div>
                                      )}
                                    </div>
                                    <style>{`
                                      @keyframes spin {
                                        0% { transform: rotate(0deg); }
                                        100% { transform: rotate(360deg); }
                                      }
                                    `}</style>
                                  </div>
                                ) : (
                                  <div style={{ fontSize: '9px', color: 'var(--text-muted)', textAlign: 'center', padding: '8px', background: 'rgba(255,255,255,0.01)', borderRadius: '4px', border: '1px dashed var(--border-color)' }}>
                                    Stream offline. Start streaming above.
                                  </div>
                                )}
                              </div>

                              {/* 4. VoNR Dialer */}
                              <div className="tool-section" style={{ marginBottom: '6px' }}>
                                <h5 style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '11px', fontWeight: 'bold', color: '#10b981', margin: '0 0 6px 0' }}>
                                  <Phone size={12} /> VoNR SIP Phone Dialer
                                </h5>
                                
                                <div style={{ display: 'flex', gap: '6px', marginBottom: '8px' }}>
                                  <select 
                                    value={vonrCallee} 
                                    onChange={(e) => setVonrCallee(e.target.value)} 
                                    disabled={isCallActive}
                                    style={{ flex: 1, padding: '4px 6px', borderRadius: '4px', background: '#0f172a', border: '1px solid var(--border-color)', color: '#ffffff', fontSize: '11px' }}
                                  >
                                    <option value="echo">Voice Echo (Real 5G Core Loopback)</option>
                                    {otherUes.map((u: any) => (
                                      <option key={u.id} value={u.id.toString()}>UE-{u.id} (Direct Core Call)</option>
                                    ))}
                                  </select>
                                  
                                  {!isCallActive ? (
                                    <button 
                                      onClick={() => dialVonr(activeUe.id)} 
                                      style={{ padding: '4px 10px', borderRadius: '4px', background: 'var(--color-success)', color: '#000000', border: 'none', fontWeight: 'bold', fontSize: '11px', cursor: 'pointer' }}
                                    >
                                      Dial
                                    </button>
                                  ) : (
                                    <button 
                                      onClick={() => hangupVonr(activeUe.id)} 
                                      style={{ padding: '4px 10px', borderRadius: '4px', background: '#ef4444', color: '#ffffff', border: 'none', fontWeight: 'bold', fontSize: '11px', cursor: 'pointer' }}
                                    >
                                      Hangup
                                    </button>
                                  )}
                                </div>

                                {isCallActive ? (
                                  <div className="vonr-dialer" style={{ marginTop: '8px' }}>
                                    
                                    {/* Call active stats screen */}
                                    <div className="vonr-status-screen">
                                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                                        <div className={activeCall.status === 'ringing' || activeCall.status === 'dialing' ? 'vonr-ringing' : ''}>
                                          <Phone size={14} color={activeCall.status === 'connected' ? '#10b981' : '#f59e0b'} />
                                        </div>
                                        <span style={{ fontSize: '11px', fontWeight: 'bold', textTransform: 'uppercase' }}>
                                          {activeCall.status}
                                        </span>
                                      </div>
                                      <div style={{ fontSize: '9px', color: 'var(--text-muted)', fontFamily: 'monospace' }}>
                                        Target: {activeCall.calleeId === 'echo' ? 'SIP Voice Echo' : `UE-${activeCall.calleeId}`}
                                      </div>
                                      {activeCall.status === 'connected' && (
                                        <div style={{ fontSize: '13px', fontWeight: 'bold', color: '#ffffff', fontFamily: 'monospace', marginTop: '2px' }}>
                                          {Math.floor(activeCall.callDuration / 60)}:{(activeCall.callDuration % 60).toString().padStart(2, '0')}
                                        </div>
                                      )}
                                    </div>

                                    {/* Metrics Grid */}
                                    {activeCall.status === 'connected' && (
                                      <div className="vonr-metrics-grid" style={{ marginTop: '6px' }}>
                                        <div className="vonr-metric-card">
                                          <span className="vonr-metric-lbl">MOS Score</span>
                                          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                                            <span className="vonr-metric-val">{(activeCall.mosScore || 4.5).toFixed(2)}</span>
                                            <span className={`vonr-mos-badge ${
                                              (activeCall.mosScore || 4.5) >= 4.0 ? 'vonr-mos-excel' : (activeCall.mosScore || 4.5) >= 3.0 ? 'vonr-mos-good' : 'vonr-mos-poor'
                                            }`}>
                                              {(activeCall.mosScore || 4.5) >= 4.0 ? 'Excel' : (activeCall.mosScore || 4.5) >= 3.0 ? 'Good' : 'Poor'}
                                            </span>
                                          </div>
                                        </div>
                                        <div className="vonr-metric-card">
                                          <span className="vonr-metric-lbl">Latency</span>
                                          <span className="vonr-metric-val">{(activeCall.latencyMs || 0).toFixed(1)} ms</span>
                                        </div>
                                        <div className="vonr-metric-card">
                                          <span className="vonr-metric-lbl">Jitter</span>
                                          <span className="vonr-metric-val">{(activeCall.jitterMs || 0).toFixed(1)} ms</span>
                                        </div>
                                        <div className="vonr-metric-card">
                                          <span className="vonr-metric-lbl">Packet Loss</span>
                                          <span className="vonr-metric-val">{(activeCall.packetLossPct || 0.0).toFixed(2)} %</span>
                                        </div>
                                      </div>
                                    )}

                                    {/* SIP Logs */}
                                    {activeCall.sipLogs && activeCall.sipLogs.length > 0 && (
                                      <div style={{ background: '#070b13', padding: '6px', borderRadius: '4px', maxHeight: '80px', overflowY: 'auto', fontSize: '9px', fontFamily: 'monospace', color: '#60a5fa', marginTop: '6px', border: '1px solid rgba(96,165,250,0.15)' }}>
                                        {activeCall.sipLogs.map((l: string, idx: number) => (
                                          <div key={idx} style={{ borderBottom: '1px solid rgba(255,255,255,0.02)', paddingBottom: '2px', marginBottom: '2px', whiteSpace: 'nowrap' }}>{l}</div>
                                        ))}
                                      </div>
                                    )}
                                  </div>
                                ) : (
                                  <div>
                                    <div className="vonr-keypad" style={{ marginTop: '6px' }}>
                                      {['1','2','3','4','5','6','7','8','9','*','0','#'].map(key => (
                                        <button 
                                          key={key} 
                                          className="vonr-key"
                                          onClick={() => {
                                            if (['0','1','2','3','4','5','6','7','8','9'].includes(key)) {
                                              // High fidelity interaction: if key matches another UE, choose it
                                              const candidate = otherUes.find((x: any) => x.id.toString().endsWith(key));
                                              if (candidate) {
                                                setVonrCallee(candidate.id.toString());
                                              }
                                            }
                                          }}
                                        >
                                          {key}
                                        </button>
                                      ))}
                                    </div>
                                  </div>
                                )}
                              </div>

                            </div>
                          )}
                        </>
                      );
                    }
                    return (
                      <>
                        <h4 className="inspector-title" style={{ color: 'var(--color-success)' }}>
                          <Cpu size={14} /> UE Node Inspector
                        </h4>
                        <div className="inspector-details">
                          <div className="detail-row">
                            <span className="detail-label">IMSI</span>
                            <span className="detail-val font-mono">{status?.configSummary?.ueImsi || '999700000000001'}</span>
                          </div>
                          <div className="detail-row">
                            <span className="detail-label">SUCI</span>
                            <span className="detail-val font-mono" style={{ fontSize: '11px' }}>
                              {status?.isRunning ? `suci-0-999-70-0-0-0-${status?.configSummary?.ueImsi?.slice(-5) || '00001'}` : 'None'}
                            </span>
                          </div>
                          <div className="detail-row">
                            <span className="detail-label">5G-GUTI</span>
                            <span className="detail-val font-mono" style={{ fontSize: '11px' }}>
                              {status?.isRunning ? `guti-999-70-0001-${status?.configSummary?.ueImsi?.slice(-4) || '0001'}` : 'None'}
                            </span>
                          </div>
                          <div className="detail-row">
                            <span className="detail-label">5GMM State</span>
                            <span className="detail-val font-semibold" style={{ color: status?.isRunning ? 'var(--color-success)' : 'var(--text-muted)' }}>
                              {status?.isRunning ? '5GMM-REGISTERED' : '5GMM-DEREGISTERED'}
                            </span>
                          </div>
                          <div className="detail-row">
                            <span className="detail-label">5GMM Connection</span>
                            <span className="detail-val font-semibold" style={{ color: status?.isRunning ? 'var(--color-success)' : 'var(--text-muted)' }}>
                              {status?.isRunning ? 'CONNECTED (N1)' : 'IDLE'}
                            </span>
                          </div>
                          <div className="detail-row">
                            <span className="detail-label">RRC State</span>
                            <span className="detail-val font-semibold" style={{ color: status?.isRunning ? 'var(--color-success)' : 'var(--text-muted)' }}>
                              {status?.isRunning ? 'RRC-CONNECTED' : 'RRC-IDLE'}
                            </span>
                          </div>
                          <div className="detail-row">
                            <span className="detail-label">Connected Cell</span>
                            <span className="detail-val font-semibold" style={{ color: 'var(--color-primary)' }}>
                              {status?.isRunning ? 'gNB-Default (000001)' : '—'}
                            </span>
                          </div>
                          <div className="detail-row">
                            <span className="detail-label">Active Tunnels</span>
                            <span className="detail-val font-mono">{activeTuns.length}</span>
                          </div>
                          {activeTuns.length > 0 && (
                            <div className="detail-row-nested">
                              <span className="detail-label">Interface IPs:</span>
                              <span className="detail-val font-mono" style={{ fontSize: '10px' }}>
                                {activeTuns.map(t => `${t.name}: ${t.ips ? t.ips.map(ip => ip.split('/')[0]).join(', ') : 'Pending'}`).join(', ')}
                              </span>
                            </div>
                          )}
                        </div>
                      </>
                    );
                  })()}

                  {selectedNode === 'gnb' && (() => {
                    const activeGnb = getActiveGnbToInspect();
                    if (activeGnb) {
                      return (
                        <>
                          <h4 className="inspector-title" style={{ color: 'var(--color-primary)' }}>
                            <Radio size={14} /> gNodeB Inspector
                          </h4>
                          <div className="inspector-details" style={{ maxHeight: '350px', overflowY: 'auto' }}>
                            <div className="detail-row">
                              <span className="detail-label">Profile Name</span>
                              <span className="detail-val font-semibold">{activeGnb.profileName}</span>
                            </div>
                            <div className="detail-row">
                              <span className="detail-label">gNodeB ID</span>
                              <span className="detail-val font-mono">{activeGnb.gnbId}</span>
                            </div>
                            <div className="detail-row">
                              <span className="detail-label">PLMN ID</span>
                              <span className="detail-val font-mono">{activeGnb.mcc}-{activeGnb.mnc}</span>
                            </div>
                            <div className="detail-row">
                              <span className="detail-label">TAC</span>
                              <span className="detail-val font-mono">{activeGnb.tac}</span>
                            </div>
                            <div className="detail-row">
                              <span className="detail-label">Link Mode</span>
                              <span className="detail-val font-semibold uppercase">{activeGnb.linkType}</span>
                            </div>
                            <div className="detail-row">
                              <span className="detail-label">N2 Connection</span>
                              <span className="detail-val font-semibold" style={{ color: 'var(--color-success)' }}>
                                CONNECTED
                              </span>
                            </div>
                            <div className="detail-row" style={{ flexDirection: 'column', alignItems: 'flex-start', borderTop: '1px solid rgba(255,255,255,0.05)', paddingTop: '8px', marginTop: '8px' }}>
                              <span className="detail-label" style={{ marginBottom: '4px' }}>Connected UEs ({activeGnb.connectedUes?.length || 0})</span>
                              {activeGnb.connectedUes && activeGnb.connectedUes.length > 0 ? (
                                <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap', width: '100%' }}>
                                  {activeGnb.connectedUes.map((ue: string) => (
                                    <span key={ue} className="fleet-tag" style={{ margin: 0, padding: '2px 6px', fontSize: '11px', background: 'rgba(59, 130, 246, 0.15)', color: 'var(--color-primary)' }}>
                                      {ue}
                                    </span>
                                  ))}
                                </div>
                              ) : (
                                <span className="detail-val" style={{ color: 'var(--text-muted)', fontSize: '11px' }}>None</span>
                              )}
                            </div>
                          </div>
                        </>
                      );
                    }
                    return (
                      <>
                        <h4 className="inspector-title" style={{ color: 'var(--color-primary)' }}>
                          <Radio size={14} /> gNodeB Cell Inspector
                        </h4>
                        <div className="inspector-details" style={{ maxHeight: '350px', overflowY: 'auto' }}>
                          <div className="detail-row">
                            <span className="detail-label">gNodeB ID</span>
                            <span className="detail-val font-mono">0001FF (511)</span>
                          </div>
                          <div className="detail-row">
                            <span className="detail-label">PLMN ID</span>
                            <span className="detail-val font-mono">
                              {configData ? `${configData.GNodeB?.PlmnList?.Mcc || '999'}-${configData.GNodeB?.PlmnList?.Mnc || '70'}` : '999-70'}
                            </span>
                          </div>
                          <div className="detail-row">
                            <span className="detail-label">TAC</span>
                            <span className="detail-val font-mono">
                              {configData ? configData.GNodeB?.PlmnList?.Tac || '0001' : '0001'}
                            </span>
                          </div>
                          <div className="detail-row">
                            <span className="detail-label">Link Mode</span>
                            <span className="detail-val font-semibold uppercase">
                              {configData ? configData.GNodeB?.LinkType : 'unix'}
                            </span>
                          </div>
                          <div className="detail-row">
                            <span className="detail-label">N2 Connection</span>
                            <span className="detail-val font-semibold" style={{ color: status?.gnbLinkState !== 'offline' ? 'var(--color-success)' : 'var(--color-danger)' }}>
                              {status?.gnbLinkState === 'listening' ? 'TCP LISTENING' : status?.gnbLinkState === 'socket_active' ? 'SOCKET ACTIVE' : 'OFFLINE'}
                            </span>
                          </div>
                          <div className="detail-row" style={{ flexDirection: 'column', alignItems: 'flex-start', borderTop: '1px solid rgba(255,255,255,0.05)', paddingTop: '8px', marginTop: '8px' }}>
                            <span className="detail-label" style={{ marginBottom: '4px' }}>Connected UEs</span>
                            {status?.isRunning || activeUEs.length > 0 ? (
                              <span className="fleet-tag" style={{ margin: 0, padding: '2px 6px', fontSize: '11px', background: 'rgba(59, 130, 246, 0.15)', color: 'var(--color-primary)' }}>
                                UE-Default
                              </span>
                            ) : (
                              <span className="detail-val" style={{ color: 'var(--text-muted)', fontSize: '11px' }}>None</span>
                            )}
                          </div>
                        </div>
                      </>
                    );
                  })()}

                  {selectedNode === 'amf' && (
                    <>
                      <h4 className="inspector-title" style={{ color: 'var(--color-purple)' }}>
                        <Server size={14} /> AMF Core Inspector
                      </h4>
                      <div className="inspector-details">
                        <div className="detail-row">
                          <span className="detail-label">Function</span>
                          <span className="detail-val font-semibold">Access & Mobility Function</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">N2 Port (SCTP)</span>
                          <span className="detail-val font-mono">38412</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">AMF IP Address</span>
                          <span className="detail-val font-mono">{status?.configSummary?.amfTarget?.split(':')[0] || '127.0.0.1'}</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">State</span>
                          <span className="detail-val font-semibold" style={{ color: status?.gnbLinkState && status.gnbLinkState !== 'offline' ? 'var(--color-success)' : 'var(--color-danger)' }}>
                            {status?.gnbLinkState && status.gnbLinkState !== 'offline' ? 'ACTIVE' : 'OFFLINE'}
                          </span>
                        </div>
                      </div>
                    </>
                  )}

                  {selectedNode === 'upf' && (
                    <>
                      <h4 className="inspector-title" style={{ color: 'var(--color-info)' }}>
                        <Network size={14} /> UPF Gateway Inspector
                      </h4>
                      <div className="inspector-details">
                        <div className="detail-row">
                          <span className="detail-label">Function</span>
                          <span className="detail-val font-semibold">User Plane Function</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">N3 Port (UDP)</span>
                          <span className="detail-val font-mono">2152 (GTP-U)</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">UPF IP Address</span>
                          <span className="detail-val font-mono">{configData ? configData.AMF?.Ip : '127.0.0.1'}</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">State</span>
                          <span className="detail-val font-semibold" style={{ color: activeUEs.some(u => u.pduSessions && u.pduSessions.some((s: any) => s.stateDesc?.includes('ACTIVE'))) ? 'var(--color-success)' : 'var(--text-muted)' }}>
                            {activeUEs.some(u => u.pduSessions && u.pduSessions.some((s: any) => s.stateDesc?.includes('ACTIVE'))) ? 'ROUTING TRAFFIC' : 'STANDBY'}
                          </span>
                        </div>
                      </div>
                    </>
                  )}

                  {selectedNode === 'dn' && (
                    <>
                      <h4 className="inspector-title" style={{ color: '#06b6d4' }}>
                        <Globe size={14} /> Data Network Inspector
                      </h4>
                      <div className="inspector-details">
                        <div className="detail-row">
                          <span className="detail-label">DN Name</span>
                          <span className="detail-val font-semibold">Internet (External)</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">N6 Routing</span>
                          <span className="detail-val font-semibold" style={{ color: 'var(--color-success)' }}>NAT Enabled</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Status</span>
                          <span className="detail-val font-semibold" style={{ color: 'var(--color-success)' }}>ONLINE</span>
                        </div>
                      </div>
                    </>
                  )}

                  {selectedNode === 'uu-link' && (
                    <>
                      <h4 className="inspector-title" style={{ color: '#10b981' }}>
                        <Activity size={14} /> Uu Interface Link
                      </h4>
                      <div className="inspector-details">
                        <div className="detail-row">
                          <span className="detail-label">Type</span>
                          <span className="detail-val">Radio Access Link (5G-NR)</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Protocols</span>
                          <span className="detail-val font-mono">NAS, RRC, PDCP, RLC</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Frequency Band</span>
                          <span className="detail-val">FR1 (Sub-6GHz) Sim</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Status</span>
                          <span className="detail-val font-semibold" style={{ color: 'var(--color-success)' }}>ACTIVE</span>
                        </div>
                      </div>
                    </>
                  )}

                  {selectedNode === 'n2-link' && (
                    <>
                      <h4 className="inspector-title" style={{ color: '#8b5cf6' }}>
                        <Activity size={14} /> N2 Control Interface
                      </h4>
                      <div className="inspector-details">
                        <div className="detail-row">
                          <span className="detail-label">Type</span>
                          <span className="detail-val">Control Plane (gNB &harr; AMF)</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Protocol</span>
                          <span className="detail-val font-mono">NGAP / SCTP</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">SCTP Target Port</span>
                          <span className="detail-val font-mono">38412</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Status</span>
                          <span className="detail-val font-semibold" style={{ color: 'var(--color-success)' }}>ESTABLISHED</span>
                        </div>
                      </div>
                    </>
                  )}

                  {selectedNode === 'n3-link' && (
                    <>
                      <h4 className="inspector-title" style={{ color: '#3b82f6' }}>
                        <Activity size={14} /> N3 User Interface
                      </h4>
                      <div className="inspector-details">
                        <div className="detail-row">
                          <span className="detail-label">Type</span>
                          <span className="detail-val">User Plane (gNB &harr; UPF)</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Protocol</span>
                          <span className="detail-val font-mono">GTP-U / UDP</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">UDP Port</span>
                          <span className="detail-val font-mono">2152</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Status</span>
                          <span className="detail-val font-semibold" style={{ color: activeUEs.some(u => u.pduSessions && u.pduSessions.some((s: any) => s.stateDesc?.includes('ACTIVE'))) ? 'var(--color-success)' : 'var(--text-muted)' }}>
                            {activeUEs.some(u => u.pduSessions && u.pduSessions.some((s: any) => s.stateDesc?.includes('ACTIVE'))) ? 'ACTIVE TUNNEL' : 'INACTIVE'}
                          </span>
                        </div>
                      </div>
                    </>
                  )}

                  {selectedNode === 'n6-link' && (
                    <>
                      <h4 className="inspector-title" style={{ color: '#06b6d4' }}>
                        <Activity size={14} /> N6 Data Interface
                      </h4>
                      <div className="inspector-details">
                        <div className="detail-row">
                          <span className="detail-label">Type</span>
                          <span className="detail-val">User Plane (UPF &harr; DN)</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Protocol</span>
                          <span className="detail-val font-mono">IP Routing (NAT)</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Routing Gateway</span>
                          <span className="detail-val font-mono">127.0.0.2</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Status</span>
                          <span className="detail-val font-semibold" style={{ color: 'var(--color-success)' }}>ACTIVE</span>
                        </div>
                      </div>
                    </>
                  )}
                  
                  {!selectedNode && (
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--text-muted)', fontSize: '13px' }}>
                      Click any node or link badge to inspect 3GPP metrics
                    </div>
                  )}
                </div>
              </div>
            </div>

            {/* Active Connected gNodeBs Table */}
            {status?.runningGnbs && status.runningGnbs.length > 0 && (
              <div className="card" id="activeGnbsTable" style={{ marginBottom: '20px' }}>
                <h3 className="panel-title" style={{ marginBottom: '16px', color: 'var(--color-primary)' }}>
                  <Radio size={18} /> Active Fleet gNodeB Cells ({status.runningGnbs.length})
                </h3>
                <div style={{ overflowX: 'auto' }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '14px' }}>
                    <thead>
                      <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', textAlign: 'left' }}>
                        <th style={{ padding: '10px' }}>Profile Name</th>
                        <th style={{ padding: '10px' }}>gNodeB ID</th>
                        <th style={{ padding: '10px' }}>PLMN</th>
                        <th style={{ padding: '10px' }}>TAC</th>
                        <th style={{ padding: '10px' }}>Link Type</th>
                        <th style={{ padding: '10px' }}>Socket Path / Port</th>
                        <th style={{ padding: '10px' }}>Connected UEs</th>
                        <th style={{ padding: '10px' }}>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {status.runningGnbs.map((gnb) => (
                        <tr key={gnb.profileName} style={{ borderBottom: '1px solid rgba(255, 255, 255, 0.02)' }}>
                          <td style={{ padding: '10px', fontWeight: 'bold', color: 'var(--color-primary)' }}>{gnb.profileName}</td>
                          <td style={{ padding: '10px', fontFamily: 'monospace' }}>{gnb.gnbId}</td>
                          <td style={{ padding: '10px' }}>{gnb.mcc}-{gnb.mnc}</td>
                          <td style={{ padding: '10px', fontFamily: 'monospace' }}>{gnb.tac}</td>
                          <td style={{ padding: '10px', textTransform: 'uppercase' }}>{gnb.linkType}</td>
                          <td style={{ padding: '10px', fontFamily: 'monospace', fontSize: '13px' }}>
                            {gnb.linkType === 'unix' ? gnb.socketPath : gnb.linkPort}
                          </td>
                          <td style={{ padding: '10px' }}>
                            {gnb.connectedUes && gnb.connectedUes.length > 0 ? (
                              <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                                {gnb.connectedUes.map((ue: string) => (
                                  <span key={ue} className="fleet-tag" style={{ margin: 0, padding: '2px 6px', fontSize: '11px', background: 'rgba(59, 130, 246, 0.15)', color: 'var(--color-primary)' }}>
                                    {ue}
                                  </span>
                                ))}
                              </div>
                            ) : (
                              <span style={{ color: 'var(--text-muted)', fontSize: '12px' }}>None</span>
                            )}
                          </td>
                          <td style={{ padding: '10px' }}>
                            <span style={{ 
                              padding: '2px 8px', 
                              borderRadius: '4px', 
                              fontSize: '12px', 
                              fontWeight: 'semibold',
                              background: 'rgba(16, 185, 129, 0.1)', 
                              color: 'var(--color-success)'
                            }}>
                              ACTIVE
                            </span>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Active Connected UEs Table (Richer with MM/SM states) */}
            {activeUEs.length > 0 && (
              <div className="card" id="activeUEsTable" style={{ marginBottom: '20px' }}>
                <h3 className="panel-title" style={{ marginBottom: '16px', color: 'var(--color-success)' }}>
                  <Cpu size={18} /> Active Registered UEs ({activeUEs.length})
                </h3>
                <div style={{ overflowX: 'auto' }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '14px' }}>
                    <thead>
                      <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', textAlign: 'left' }}>
                        <th style={{ padding: '10px' }}>UE ID</th>
                        <th style={{ padding: '10px' }}>SUPI / IMSI</th>
                        <th style={{ padding: '10px' }}>5GMM State</th>
                        <th style={{ padding: '10px' }}>AMF UE NGAP ID</th>
                        <th style={{ padding: '10px' }}>Connected Cell</th>
                        <th style={{ padding: '10px' }}>PDU Sessions (IP Address)</th>
                        <th style={{ padding: '10px', textAlign: 'right' }}>Control</th>
                      </tr>
                    </thead>
                    <tbody>
                      {activeUEs.map((ue) => {
                        const isSelected = controlUeId === ue.id;
                        return (
                          <tr key={ue.id} style={{ borderBottom: '1px solid rgba(255, 255, 255, 0.02)', background: isSelected ? 'rgba(59, 130, 246, 0.05)' : 'none' }}>
                            <td style={{ padding: '10px', fontWeight: 'bold', fontFamily: 'monospace' }}>#{ue.id}</td>
                            <td style={{ padding: '10px', color: 'var(--text-primary)' }}>{ue.supi}</td>
                            <td style={{ padding: '10px' }}>
                              <span style={{ 
                                padding: '2px 8px', 
                                borderRadius: '4px', 
                                fontSize: '11px', 
                                fontWeight: 'bold', 
                                background: ue.stateMm === 3 ? 'rgba(16, 185, 129, 0.15)' : 'rgba(239, 68, 68, 0.15)', 
                                color: ue.stateMm === 3 ? 'var(--color-success)' : 'var(--color-danger)' 
                              }}>
                                {ue.stateMmDesc}
                              </span>
                            </td>
                            <td style={{ padding: '10px', fontFamily: 'monospace' }}>{ue.amfUeNgapId || 'N/A'}</td>
                            <td style={{ padding: '10px' }}>
                              {ue.gnbProfileName ? `${ue.gnbProfileName} (${ue.gnbId || '—'})` : '—'}
                            </td>
                            <td style={{ padding: '10px' }}>
                              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                                {ue.pduSessions && ue.pduSessions.map((pdu: any) => (
                                  <div key={pdu.id} style={{ fontSize: '12px', fontFamily: 'monospace' }}>
                                    <span style={{ color: 'var(--color-info)' }}>PDU #{pdu.id} ({pdu.dnn}):</span> {pdu.ueIp || 'Pending...'}
                                    <span style={{ 
                                      marginLeft: '6px',
                                      padding: '1px 4px', 
                                      borderRadius: '3px', 
                                      fontSize: '9px', 
                                      fontWeight: 'bold', 
                                      background: pdu.state === 8 ? 'rgba(16, 185, 129, 0.12)' : 'rgba(245, 158, 11, 0.12)', 
                                      color: pdu.state === 8 ? 'var(--color-success)' : 'var(--color-warning)' 
                                    }}>
                                      {pdu.stateDesc}
                                    </span>
                                  </div>
                                ))}
                              </div>
                            </td>
                            <td style={{ padding: '10px', textAlign: 'right' }}>
                              <button
                                onClick={() => setControlUeId(ue.id)}
                                className={`btn ${isSelected ? 'btn-primary' : 'btn-secondary'}`}
                                style={{ padding: '4px 10px', fontSize: '12px', width: 'auto' }}
                              >
                                {isSelected ? 'Selected' : 'Select'}
                              </button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Runtime UE Control Panel Card */}
            {controlUeId !== null && activeUEs.some(ue => ue.id === controlUeId) && (
              <div className="card" style={{ marginBottom: '20px' }}>
                <h3 className="panel-title" style={{ marginBottom: '16px', color: 'var(--color-info)', display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <Sliders size={18} /> Runtime UE Controller (Selected UE #{controlUeId})
                </h3>

                <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1fr 1fr', gap: '20px' }}>
                  {/* Column 1: Basic Operations */}
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', borderRight: '1px solid var(--border-color)', paddingRight: '20px' }}>
                    <h4 style={{ fontSize: '13px', fontWeight: 'bold', color: 'var(--text-secondary)' }}>3GPP procedures</h4>
                    <button
                      className="btn btn-secondary"
                      onClick={() => triggerUeAction('service-request')}
                      style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'center' }}
                    >
                      <RefreshCw size={14} /> Send Service Request (Connected)
                    </button>
                    <button
                      className="btn btn-secondary"
                      onClick={() => triggerUeAction('deregister-normal')}
                      style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'center' }}
                    >
                      <Play size={14} style={{ transform: 'rotate(90deg)' }} /> UE Clean Deregister (Normal)
                    </button>
                    <button
                      className="btn btn-secondary"
                      onClick={() => triggerUeAction('deregister-poweroff')}
                      style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'center', background: 'rgba(239, 68, 68, 0.05)', borderColor: 'rgba(239, 68, 68, 0.2)', color: 'var(--color-danger)' }}
                    >
                      <Trash2 size={14} /> UE Power-Off Deregister
                    </button>
                  </div>

                  {/* Column 2: Secondary PDU Session */}
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', borderRight: '1px solid var(--border-color)', paddingRight: '20px' }}>
                    <h4 style={{ fontSize: '13px', fontWeight: 'bold', color: 'var(--text-secondary)' }}>Establish Secondary PDU Session</h4>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px' }}>
                      <div className="form-group" style={{ marginBottom: 0 }}>
                        <label style={{ fontSize: '11px' }}>Session ID</label>
                        <input
                          type="number"
                          min="2"
                          max="15"
                          value={newPduId}
                          onChange={(e) => setNewPduId(parseInt(e.target.value) || 2)}
                          style={{ padding: '6px' }}
                        />
                      </div>
                      <div className="form-group" style={{ marginBottom: 0 }}>
                        <label style={{ fontSize: '11px' }}>DNN</label>
                        <input
                          type="text"
                          value={newPduDnn}
                          onChange={(e) => setNewPduDnn(e.target.value)}
                          style={{ padding: '6px' }}
                        />
                      </div>
                    </div>

                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1.5fr', gap: '10px' }}>
                      <div className="form-group" style={{ marginBottom: 0 }}>
                        <label style={{ fontSize: '11px' }}>SST</label>
                        <input
                          type="number"
                          value={newPduSst}
                          onChange={(e) => setNewPduSst(parseInt(e.target.value) || 1)}
                          style={{ padding: '6px' }}
                        />
                      </div>
                      <div className="form-group" style={{ marginBottom: 0 }}>
                        <label style={{ fontSize: '11px' }}>SD</label>
                        <input
                          type="text"
                          value={newPduSd}
                          onChange={(e) => setNewPduSd(e.target.value)}
                          style={{ padding: '6px' }}
                        />
                      </div>
                    </div>

                    <div className="form-group" style={{ marginBottom: '10px' }}>
                      <label style={{ fontSize: '11px' }}>Session Type</label>
                      <select
                        value={newPduType}
                        onChange={(e) => setNewPduType(e.target.value)}
                        style={{ padding: '6px' }}
                      >
                        <option value="IPv4">IPv4</option>
                        <option value="IPv6">IPv6</option>
                        <option value="IPv4v6">IPv4v6 (Dual Stack)</option>
                      </select>
                    </div>

                    <button
                      className="btn btn-primary"
                      onClick={() => triggerUeAction('pdu-establish', {
                        pduSessionId: newPduId,
                        dnn: newPduDnn,
                        sst: newPduSst,
                        sd: newPduSd,
                        sessionType: newPduType
                      })}
                      style={{ padding: '6px 12px', fontSize: '13px' }}
                    >
                      Establish PDU Session
                    </button>
                  </div>

                  {/* Column 3: Handover */}
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                    <h4 style={{ fontSize: '13px', fontWeight: 'bold', color: 'var(--text-secondary)' }}>N2 Path Switch Handover</h4>
                    
                    {fleetRunning.runningGnbs && fleetRunning.runningGnbs.length > 0 ? (
                      <div className="form-group" style={{ marginBottom: '10px' }}>
                        <label style={{ fontSize: '11px' }}>Target gNB</label>
                        <select
                          value={selectedTargetGnbName}
                          onChange={(e) => {
                            const name = e.target.value;
                            setSelectedTargetGnbName(name);
                            const gnb = fleetRunning.runningGnbs.find(g => g.profileName === name);
                            if (gnb) {
                              setHoTargetIp(gnb.controlIp);
                              setHoTargetPort(gnb.linkPort || 9489);
                              setHoTargetLinkType(gnb.linkType);
                              setHoTargetSocketPath(gnb.socketPath || '');
                            }
                          }}
                          style={{ padding: '6px', width: '100%', borderRadius: '4px', border: '1px solid var(--border-color)', backgroundColor: 'var(--bg-secondary)', color: 'var(--text-primary)' }}
                        >
                          <option value="">-- Select Target gNB --</option>
                          {fleetRunning.runningGnbs.map(g => (
                            <option key={g.profileName} value={g.profileName}>
                              {g.profileName} ({g.gnbId}) - {g.linkType === 'unix' ? 'UNIX' : `${g.controlIp}:${g.linkPort}`}
                            </option>
                          ))}
                        </select>
                      </div>
                    ) : (
                      <>
                        <div className="form-group" style={{ marginBottom: 0 }}>
                          <label style={{ fontSize: '11px' }}>Target gNB IP</label>
                          <input
                            type="text"
                            value={hoTargetIp}
                            onChange={(e) => setHoTargetIp(e.target.value)}
                            style={{ padding: '6px' }}
                          />
                        </div>

                        <div className="form-group" style={{ marginBottom: '15px' }}>
                          <label style={{ fontSize: '11px' }}>Target gNB Port</label>
                          <input
                            type="number"
                            value={hoTargetPort}
                            onChange={(e) => setHoTargetPort(parseInt(e.target.value) || 9489)}
                            style={{ padding: '6px' }}
                          />
                        </div>
                      </>
                    )}

                    {(() => {
                      const targetGnbObj = fleetRunning.runningGnbs?.find(g => g.profileName === selectedTargetGnbName);
                      const targetGnbId = targetGnbObj?.gnbId || "";
                      const targetGnbName = targetGnbObj?.profileName || "";
                      return (
                        <div style={{ display: 'flex', gap: '8px' }}>
                          <button
                            className="btn btn-primary"
                            onClick={() => triggerUeAction('handover', {
                              targetGnbIp: hoTargetIp,
                              targetGnbPort: hoTargetPort,
                              targetGnbLinkType: hoTargetLinkType,
                              targetGnbSocketPath: hoTargetSocketPath,
                              targetGnbId: targetGnbId,
                              targetGnbName: targetGnbName
                            })}
                            style={{ flex: 1, padding: '6px 12px', fontSize: '13px' }}
                            disabled={fleetRunning.runningGnbs?.length > 0 && !selectedTargetGnbName}
                          >
                            N2 HO
                          </button>
                          <button
                            className="btn btn-primary"
                            onClick={() => triggerUeAction('xn-handover', {
                              targetGnbIp: hoTargetIp,
                              targetGnbPort: hoTargetPort,
                              targetGnbLinkType: hoTargetLinkType,
                              targetGnbSocketPath: hoTargetSocketPath,
                              targetGnbId: targetGnbId,
                              targetGnbName: targetGnbName
                            })}
                            style={{ flex: 1, padding: '6px 12px', fontSize: '13px', backgroundColor: 'var(--accent-color)' }}
                            disabled={fleetRunning.runningGnbs?.length > 0 && !selectedTargetGnbName}
                          >
                            Xn HO
                          </button>
                        </div>
                      );
                    })()}
                  </div>
                </div>
              </div>
            )}

            {/* Active Interfaces Lists */}
            <div className="card">
              <h3 className="panel-title" style={{ marginBottom: '16px' }}>
                <Globe size={18} /> Linux Network Interfaces
              </h3>
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '14px' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', textAlign: 'left' }}>
                      <th style={{ padding: '10px' }}>Interface Name</th>
                      <th style={{ padding: '10px' }}>IP Bindings</th>
                      <th style={{ padding: '10px' }}>Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {status?.interfaces?.map((iface) => {
                      const isTun = iface.name.startsWith('ue') || iface.name.includes('tun') || iface.name === 'lo';
                      return (
                        <tr key={iface.name} style={{ borderBottom: '1px solid rgba(255, 255, 255, 0.02)' }}>
                          <td style={{ padding: '10px', fontWeight: 'bold' }}>{iface.name}</td>
                          <td style={{ padding: '10px', fontFamily: 'monospace', color: 'var(--text-secondary)' }}>
                            {iface.ips ? iface.ips.join(', ') : 'None'}
                          </td>
                          <td style={{ padding: '10px' }}>
                            <span
                              style={{
                                padding: '2px 8px',
                                borderRadius: '4px',
                                fontSize: '11px',
                                fontWeight: 'bold',
                                background: isTun ? 'rgba(16, 185, 129, 0.15)' : 'rgba(255, 255, 255, 0.05)',
                                color: isTun ? 'var(--color-success)' : 'var(--text-muted)'
                              }}
                            >
                              {isTun ? 'ACTIVE 5G TUNNEL' : 'SYSTEM INTERFACE'}
                            </span>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}

        {activeTab === 'scenarios' && (
          <div className="view-body fade-in">
            {showBanners && (
              <div className={`warning-banner ${bannerFade ? 'fade-out' : ''}`}>
                <Play size={18} />
                <div>
                  <strong>Scenario Runner Console:</strong> Triggering a scenario will execute sequential control/data actions in the background. Running logs will automatically stream inside the <strong>Live Console</strong> tab.
                </div>
              </div>
            )}

            <div className="scenarios-grid">
              {scenarios.map((scen) => {
                const isRunningThis = status?.isRunning && status.runningName === scen.id;
                return (
                  <div className="card scenario-card" key={scen.id}>
                    <div className="card-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span className="card-title font-bold text-white">{scen.name}</span>
                      {isRunningThis ? (
                        <Activity size={12} className="animate-pulse" style={{ color: '#f59e0b' }} />
                      ) : (
                        <Play size={12} style={{ color: 'var(--text-secondary)', opacity: 0.4 }} />
                      )}
                    </div>
                    <div className="card-body">
                      <p className="scenario-description">{scen.description}</p>

                      {/* Display custom inputs for specific scenarios */}
                      {scen.id === 'handover' && (
                        <div className="scenario-inputs">
                          <div className="input-group">
                            <label>Target gNB IP</label>
                            <input
                              type="text"
                              value={targetGnbIp}
                              onChange={(e) => setTargetGnbIp(e.target.value)}
                            />
                          </div>
                          <div className="input-group">
                            <label>Target gNB Port</label>
                            <input
                              type="number"
                              value={targetGnbPort}
                              onChange={(e) => setTargetGnbPort(parseInt(e.target.value))}
                            />
                          </div>
                          <div className="input-group">
                            <label>Delay (seconds)</label>
                            <input
                              type="number"
                              value={delay}
                              onChange={(e) => setDelay(parseInt(e.target.value))}
                            />
                          </div>
                        </div>
                      )}

                      {scen.id === 'full-lifecycle' && (
                        <div className="scenario-inputs">
                          <div className="input-group">
                            <label>Idle Delay (sec)</label>
                            <input
                              type="number"
                              value={idleSeconds}
                              onChange={(e) => setIdleSeconds(parseInt(e.target.value))}
                            />
                          </div>
                        </div>
                      )}

                      {scen.id === 'load-test' && (
                        <div className="scenario-inputs">
                          <div className="input-group">
                            <label>Number of UEs</label>
                            <input
                              type="number"
                              value={ueCount}
                              onChange={(e) => setUeCount(parseInt(e.target.value))}
                            />
                          </div>
                          <div className="input-group" style={{ justifyContent: 'flex-start', gap: '10px' }}>
                            <input
                              type="checkbox"
                              checked={ueOnly}
                              id="ueOnlyCheckbox"
                              onChange={(e) => setUeOnly(e.target.checked)}
                              style={{ width: 'auto', transform: 'scale(1.1)', cursor: 'pointer' }}
                            />
                            <label htmlFor="ueOnlyCheckbox" style={{ cursor: 'pointer' }}>UE Only (GNodeB already active)</label>
                          </div>
                        </div>
                      )}

                      {scen.id === 'amf-load-loop' && (
                        <div className="scenario-inputs">
                          <div className="input-group">
                            <label>Requests/sec</label>
                            <input
                              type="number"
                              value={requests}
                              onChange={(e) => setRequests(parseInt(e.target.value))}
                            />
                          </div>
                          <div className="input-group">
                            <label>Duration (sec)</label>
                            <input
                              type="number"
                              value={duration}
                              onChange={(e) => setDuration(parseInt(e.target.value))}
                            />
                          </div>
                        </div>
                      )}

                      {scen.id === 'ue-latency-interval' && (
                        <div className="scenario-inputs">
                          <div className="input-group">
                            <label>Total Requests</label>
                            <input
                              type="number"
                              value={requests}
                              onChange={(e) => setRequests(parseInt(e.target.value))}
                            />
                          </div>
                        </div>
                      )}

                      {scen.id === 'amf-availability' && (
                        <div className="scenario-inputs">
                          <div className="input-group">
                            <label>Uptime Duration (sec)</label>
                            <input
                              type="number"
                              value={duration}
                              onChange={(e) => setDuration(parseInt(e.target.value))}
                            />
                          </div>
                        </div>
                      )}

                      <div style={{ marginTop: 'auto' }}>
                        <button
                          className={`btn btn-primary`}
                          disabled={status?.isRunning}
                          onClick={() => runScenario(scen.id)}
                        >
                          {isRunningThis ? (
                            <>
                              <RefreshCw size={14} className="animate-spin" />
                              <span>RUNNING...</span>
                            </>
                          ) : (
                            <>
                              <Play size={14} />
                              <span>LAUNCH SCENARIO</span>
                            </>
                          )}
                        </button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {activeTab === 'config' && (
          <div className="view-body fade-in">
            {configData ? (
              <form onSubmit={saveConfig} className="card config-layout">
                {/* Left Side: AMF Core Connection */}
                <div className="config-section">
                  <h3 className="panel-title" style={{ borderBottom: '1px solid var(--border-color)', paddingBottom: '10px' }}>
                    <Server size={18} /> 5G Core (AMF) Target Connection
                  </h3>
                  
                  <p style={{ fontSize: '13px', color: 'var(--text-muted)', marginBottom: '10px' }}>
                    Configure the target 5G Core Network Access and Mobility Management Function (AMF) address. GNodeBs in the fleet will bind and attempt N2 SCTP association with this target.
                  </p>

                  <div className="form-group">
                    <label>AMF IP Address</label>
                    <input
                      type="text"
                      value={configData.AMF?.Ip || '127.0.0.1'}
                      onChange={(e) => handleConfigChange('AMF.Ip', e.target.value)}
                    />
                  </div>

                  <div className="form-group">
                    <label>AMF Port (NGAP/SCTP)</label>
                    <input
                      type="number"
                      value={configData.AMF?.Port || 38412}
                      onChange={(e) => handleConfigChange('AMF.Port', parseInt(e.target.value) || 38412)}
                    />
                  </div>
                </div>

                {/* Right Side: Logging & Verbosity */}
                <div className="config-section">
                  <h3 className="panel-title" style={{ borderBottom: '1px solid var(--border-color)', paddingBottom: '10px' }}>
                    <Terminal size={18} /> Logging & Verbosity
                  </h3>

                  <p style={{ fontSize: '13px', color: 'var(--text-muted)', marginBottom: '10px' }}>
                    Adjust the emulator logging level. System logs, RAN signaling events, and network packets will be filtered according to this verbosity setting.
                  </p>

                  <div className="form-group">
                    <label>Logging Level</label>
                    <select
                      value={configData.Logs?.Level || 4}
                      onChange={(e) => handleConfigChange('Logs.Level', parseInt(e.target.value) || 4)}
                    >
                      <option value={5}>DEBUG (Verbose)</option>
                      <option value={4}>INFO (Standard)</option>
                      <option value={3}>WARNING (Important warnings)</option>
                      <option value={2}>ERROR (Failures only)</option>
                      <option value={6}>TRACE (Deep packet decoding)</option>
                    </select>
                  </div>

                  <div style={{ marginTop: 'auto', display: 'flex', gap: '10px' }}>
                    <button type="submit" className="btn btn-primary">
                      SAVE CONFIGURATION
                    </button>
                  </div>
                </div>
              </form>
            ) : (
              <div className="card" style={{ textAlign: 'center', padding: '40px' }}>
                <RefreshCw className="animate-spin" style={{ margin: '0 auto 16px' }} />
                <span>Loading Configuration Profiles...</span>
              </div>
            )}
          </div>
        )}

        {activeTab === 'logs' && (
          <div className="view-body fade-in">
            <div className="console-panel">
              <div className="console-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px', color: 'var(--text-secondary)' }}>
                  <Terminal size={14} />
                  <span style={{ fontSize: '11px', fontWeight: 600, letterSpacing: '0.05em' }}>CONSOLE</span>
                </div>

                <div className="console-filter-bar">
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginRight: '10px', borderRight: '1px solid var(--border-color)', paddingRight: '10px' }}>
                    <input
                      type="checkbox"
                      checked={autoscroll}
                      id="autoscrollCheckbox"
                      onChange={(e) => setAutoscroll(e.target.checked)}
                      style={{ cursor: 'pointer', width: '13px', height: '13px' }}
                    />
                    <label htmlFor="autoscrollCheckbox" style={{ fontSize: '11px', color: 'var(--text-secondary)', cursor: 'pointer', userSelect: 'none' }}>
                      AUTOSCROLL
                    </label>
                  </div>
                  <span
                    className={`filter-badge ${logFilter === 'all' ? 'active' : ''}`}
                    onClick={() => setLogFilter('all')}
                  >
                    ALL LOGS
                  </span>
                  <span
                    className={`filter-badge ${logFilter === 'info' ? 'active' : ''}`}
                    onClick={() => setLogFilter('info')}
                  >
                    INFO
                  </span>
                  <span
                    className={`filter-badge ${logFilter === 'warn' ? 'active' : ''}`}
                    onClick={() => setLogFilter('warn')}
                  >
                    WARNINGS
                  </span>
                  <span
                    className={`filter-badge ${logFilter === 'error' ? 'active' : ''}`}
                    onClick={() => setLogFilter('error')}
                  >
                    ERRORS
                  </span>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginLeft: '10px', borderLeft: '1px solid var(--border-color)', paddingLeft: '10px' }}>
                    <input
                      type="text"
                      placeholder="Search logs..."
                      value={logSearch}
                      onChange={(e) => setLogSearch(e.target.value)}
                      style={{ 
                        background: 'var(--bg-input)', 
                        border: '1px solid var(--border-color)', 
                        borderRadius: '4px', 
                        color: 'var(--text-primary)', 
                        padding: '2px 8px', 
                        fontSize: '11px', 
                        width: '120px',
                        outline: 'none',
                        transition: 'width 0.25s ease, border-color 0.25s ease'
                      }}
                      onFocus={(e) => e.target.style.width = '180px'}
                      onBlur={(e) => e.target.style.width = '120px'}
                    />
                  </div>
                  <button
                    style={{ background: 'none', border: 'none', color: 'var(--text-secondary)', cursor: 'pointer', paddingLeft: '10px' }}
                    onClick={() => setLogs([])}
                    title="Clear Terminal Output"
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>

              <div className="console-body">
                {filteredLogs.length === 0 ? (
                  <div style={{ color: 'var(--text-muted)', textAlign: 'center', marginTop: '40px' }}>
                    No log events match current filters. Run a scenario to output logs.
                  </div>
                ) : (
                  filteredLogs.map((log, index) => (
                    <div className={`log-line log-${log.level}`} key={index}>
                      <span style={{ color: 'var(--text-muted)', marginRight: '10px' }}>[{log.timestamp}]</span>
                      {log.text}
                    </div>
                  ))
                )}
                <div ref={consoleEndRef} />
              </div>
            </div>
          </div>
        )}

        {activeTab === 'connectivity' && (
          <div className="view-body fade-in">
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1.5fr', gap: '24px' }}>
              
              {/* Left Column: Diagnostics Check */}
              <div className="card">
                <h3 className="panel-title" style={{ marginBottom: '16px' }}>
                  <Network size={18} /> Network Diagnostics
                </h3>
                <div className="check-list">
                  <div className="check-item">
                    <div className="check-item-info">
                      <Server size={16} />
                      <span className="check-item-title">AMF IP Core Reachability</span>
                    </div>
                    <span className={`check-item-status ${checks.amfReachability}`}>
                      {checks.amfReachability === 'success' ? 'REACHABLE' : checks.amfReachability === 'checking' ? 'CHECKING...' : 'CORE UNREACHABLE'}
                    </span>
                  </div>

                  <div className="check-item">
                    <div className="check-item-info">
                      <Network size={16} />
                      <span className="check-item-title">SCTP Kernel Module</span>
                    </div>
                    <span className={`check-item-status ${checks.sctpModule}`}>
                      {checks.sctpModule === 'success' ? 'LOADED' : 'WARNING (MAY NEED MODPROBE)'}
                    </span>
                  </div>

                  <div className="check-item">
                    <div className="check-item-info">
                      <Globe size={16} />
                      <span className="check-item-title">IPIP User Plane Module</span>
                    </div>
                    <span className={`check-item-status ${checks.ipipModule}`}>
                      {checks.ipipModule === 'success' ? 'LOADED' : 'WARNING (MAY NEED MODPROBE)'}
                    </span>
                  </div>

                  <div className="check-item">
                    <div className="check-item-info">
                      <Trash2 size={16} />
                      <span className="check-item-title">UNIX Socket Clean state</span>
                    </div>
                    <span className={`check-item-status ${checks.socketsClean}`}>
                      {checks.socketsClean === 'success' ? 'READY (SOCKETS CLEAR)' : 'ACTIVE SESSION'}
                    </span>
                  </div>
                </div>
                
                <button 
                  className="btn btn-secondary" 
                  style={{ marginTop: '20px' }}
                  onClick={runConnectivityChecks}
                >
                  <RefreshCw size={14} />
                  <span>REFRESH DIAGNOSTICS</span>
                </button>
              </div>

              {/* Right Column: Dynamic ping CLI */}
              <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                <h3 className="panel-title">
                  <Terminal size={18} /> Interactive Core ICMP Ping Tester
                </h3>

                <div className="form-group">
                  <label>Target Host IP (Core Node IP)</label>
                  <div style={{ display: 'flex', gap: '10px' }}>
                    <input
                      type="text"
                      value={pingHost}
                      onChange={(e) => setPingHost(e.target.value)}
                      style={{ flexGrow: 1 }}
                      placeholder="e.g. 127.0.0.1"
                    />
                    <button
                      id="runPingButton"
                      className="btn btn-primary"
                      style={{ width: 'auto', whiteSpace: 'nowrap' }}
                      disabled={pingRunning}
                      onClick={executePing}
                    >
                      {pingRunning ? 'PINGING...' : 'RUN PING TEST'}
                    </button>
                  </div>
                </div>

                <div style={{ flexGrow: 1, display: 'flex', flexDirection: 'column', minHeight: '200px' }}>
                  <label style={{ fontSize: '13px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '6px' }}>Terminal Output</label>
                  <div className="ping-result-box" style={{ flexGrow: 1 }}>
                    {pingResult || 'Click "Run Ping Test" to evaluate network connectivity to the target 5G Core Node.'}
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* ─── Fleet Manager Tab ───────────────────────────────────────────── */}
        {activeTab === 'fleet' && (
          <div className="view-body fade-in">

            {/* Fleet Toast */}
            {fleetMsg && (
              <div className={`fleet-toast ${fleetMsg.type}`}>
                {fleetMsg.text}
              </div>
            )}

            {/* Section Tabs */}
            <div className="fleet-section-tabs">
              <button
                className={`fleet-stab ${fleetActiveSection === 'ue' ? 'active' : ''}`}
                onClick={() => setFleetActiveSection('ue')}
              >
                <Cpu size={14} /> UE Profiles
                <span className="fleet-badge">{ueProfiles.length}</span>
              </button>
              <button
                className={`fleet-stab ${fleetActiveSection === 'gnb' ? 'active' : ''}`}
                onClick={() => setFleetActiveSection('gnb')}
              >
                <Radio size={14} /> gNB Profiles
                <span className="fleet-badge">{gnbProfiles.length}</span>
              </button>
              <button
                className={`fleet-stab ${fleetActiveSection === 'live' ? 'active' : ''}`}
                onClick={() => { setFleetActiveSection('live'); fetchFleetRunning(); }}
              >
                <Activity size={14} /> Live Fleet
                <span className={`fleet-badge ${(fleetRunning.runningUes?.length || 0) + (fleetRunning.runningGnbs?.length || 0) > 0 ? 'active' : ''}`}>
                  {(fleetRunning.runningUes?.length || 0) + (fleetRunning.runningGnbs?.length || 0)}
                </span>
              </button>
            </div>

            {/* ── UE Profiles Section ── */}
            {fleetActiveSection === 'ue' && (
              <div className="fleet-panel">
                <div className="fleet-panel-header">
                  <h3><Cpu size={16}/> User Equipment Profiles</h3>
                  <button className="btn btn-primary btn-sm" onClick={() => { setEditingUE(defaultUEProfile); setShowUEForm(true); }}>
                    + Add UE Profile
                  </button>
                </div>
                <p className="fleet-hint">Save UE identities here. Each profile is persisted in <code>config/fleet.json</code>. Launch as many as needed simultaneously.</p>

                {/* UE Form Overlay moved to root */}

                {/* Target GNB Selection Overlay moved to root */}

                {/* UE Profiles Table */}
                {ueProfiles.length === 0 ? (
                  <div className="fleet-empty">No UE profiles yet. Create one to get started.</div>
                ) : (
                  <div className="fleet-table-wrapper">
                    <table className="fleet-table">
                      <thead>
                        <tr>
                          <th>Name</th><th>IMSI</th><th>Key</th><th>DNN</th><th>Slice</th><th>Actions</th>
                        </tr>
                      </thead>
                      <tbody>
                        {ueProfiles.map(p => (
                          <tr key={p.name}>
                            <td><strong>{p.name}</strong></td>
                            <td className="mono">{p.hplmn.mcc}{p.hplmn.mnc}{p.msin}</td>
                            <td className="mono masked">{p.key.substring(0, 8)}···</td>
                            <td>{p.dnn}</td>
                            <td>SST:{p.snssai.sst}{p.snssai.sd ? ` SD:${p.snssai.sd}` : ''}</td>
                            <td>
                              <div className="fleet-action-btns">
                                <button className="btn btn-xs btn-success" onClick={() => handleLaunchUEClick(p.name)} title="Launch UE">▶ Launch</button>
                                <button className="btn btn-xs btn-ghost" onClick={() => duplicateUEProfile(p)} title="Duplicate">Clone</button>
                                <button className="btn btn-xs btn-ghost" onClick={() => { setEditingUE(p); setShowUEForm(true); }} title="Edit">Edit</button>
                                <button className="btn btn-xs btn-danger" onClick={() => deleteUEProfile(p.name)} title="Delete">🗑 Delete</button>
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            )}

            {/* ── gNB Profiles Section ── */}
            {fleetActiveSection === 'gnb' && (
              <div className="fleet-panel">
                <div className="fleet-panel-header">
                  <h3><Radio size={16}/> gNodeB Profiles</h3>
                  <button className="btn btn-primary btn-sm" onClick={() => { setEditingGNB(defaultGNBProfile); setShowGNBForm(true); }}>
                    + Add gNB Profile
                  </button>
                </div>
                <p className="fleet-hint">Each gNB profile launches an independent gNodeB with a unique ID that connects to the 5G Core. Multiple gNBs enable handover scenarios.</p>

                {/* GNB Form Overlay moved to root */}

                {/* GNB Profiles Table */}
                {gnbProfiles.length === 0 ? (
                  <div className="fleet-empty">No gNB profiles yet. Create one to launch a virtual gNodeB.</div>
                ) : (
                  <div className="fleet-table-wrapper">
                    <table className="fleet-table">
                      <thead>
                        <tr>
                          <th>Name</th><th>gNB-ID</th><th>PLMN</th><th>AMF Target</th><th>Link</th><th>Actions</th>
                        </tr>
                      </thead>
                      <tbody>
                        {gnbProfiles.map(p => (
                          <tr key={p.name}>
                            <td><strong>{p.name}</strong></td>
                            <td className="mono">{p.gnbId}</td>
                            <td>{p.mcc}-{p.mnc} TAC:{p.tac}</td>
                            <td className="mono">{p.amfIp}:{p.amfPort}</td>
                            <td><span className={`fleet-tag ${p.linkType}`}>{p.linkType.toUpperCase()}</span></td>
                            <td>
                              <div className="fleet-action-btns">
                                <button
                                  className="btn btn-xs btn-success"
                                  onClick={() => launchGNBProfile(p.name)}
                                  disabled={fleetRunning.runningGnbs?.some(g => g.profileName === p.name)}
                                  title={fleetRunning.runningGnbs?.some(g => g.profileName === p.name) ? 'Already running' : 'Launch gNB'}
                                >
                                  {fleetRunning.runningGnbs?.some(g => g.profileName === p.name) ? '● Running' : '▶ Launch'}
                                </button>
                                <button className="btn btn-xs btn-ghost" onClick={() => duplicateGNBProfile(p)} title="Duplicate">Clone</button>
                                <button className="btn btn-xs btn-ghost" onClick={() => { setEditingGNB(p); setShowGNBForm(true); }} title="Edit">Edit</button>
                                <button className="btn btn-xs btn-danger" onClick={() => deleteGNBProfile(p.name)} disabled={fleetRunning.runningGnbs?.some(g => g.profileName === p.name)}>🗑 Delete</button>
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            )}

            {/* ── Live Fleet Section ── */}
            {fleetActiveSection === 'live' && (
              <div className="fleet-panel">
                <div className="fleet-panel-header">
                  <h3><Activity size={16}/> Live Fleet Status</h3>
                  <button className="btn btn-ghost btn-sm" onClick={fetchFleetRunning}>
                    <RefreshCw size={13}/> Refresh
                  </button>
                </div>

                <div className="fleet-live-grid">
                  {/* Active UEs */}
                  <div className="fleet-live-section">
                    <h4 className="fleet-live-title">
                      <Cpu size={14}/> Active User Equipment
                      <span className="fleet-badge ml-2">{fleetRunning.runningUes?.length || 0}</span>
                    </h4>
                    {!fleetRunning.runningUes?.length ? (
                      <div className="fleet-empty-sm">No active UEs. Launch a UE profile to start.</div>
                    ) : (
                      <div className="fleet-ue-cards">
                        {fleetRunning.runningUes.map(u => (
                          <div key={u.id} className="fleet-ue-card">
                            <div className="fleet-ue-card-header">
                              <span className="fleet-ue-id">UE-{u.id}</span>
                              <span className={`fleet-state-badge ${u.stateMmDesc.includes('REGISTERED') && !u.stateMmDesc.includes('INIT') ? 'registered' : 'pending'}`}>
                                {u.stateMmDesc}
                              </span>
                              <button className="btn btn-xs btn-danger ml-auto" onClick={() => stopFleetUE(u.id)}>■ Stop</button>
                            </div>
                            <div className="fleet-ue-card-body">
                              <div className="fleet-ue-detail"><span>SUPI</span><code>{u.supi}</code></div>
                              <div className="fleet-ue-detail"><span>Connected Cell</span><code>{u.gnbProfileName ? `${u.gnbProfileName} (${u.gnbId || '—'})` : u.gnbControlIp}</code></div>
                              <div className="fleet-ue-detail"><span>SM State</span><code>{u.stateSmDesc}</code></div>
                              {u.pduSessions?.length > 0 && (
                                <div className="fleet-pdu-list">
                                  {u.pduSessions.map(s => (
                                    <div key={s.id} className="fleet-pdu-item">
                                      <span>PDU-{s.id}</span>
                                      <code>{s.ueIp || '—'}</code>
                                      <span className="fleet-tag">{s.dnn}</span>
                                      <span className={`fleet-state-badge sm ${s.stateDesc.includes('ACTIVE') ? 'registered' : 'pending'}`}>{s.stateDesc}</span>
                                    </div>
                                  ))}
                                </div>
                              )}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Active gNBs */}
                  <div className="fleet-live-section">
                    <h4 className="fleet-live-title">
                      <Radio size={14}/> Active gNodeBs
                      <span className="fleet-badge ml-2">{fleetRunning.runningGnbs?.length || 0}</span>
                    </h4>
                    {!fleetRunning.runningGnbs?.length ? (
                      <div className="fleet-empty-sm">No active gNBs. Launch a gNB profile to start.</div>
                    ) : (
                      <div className="fleet-gnb-cards">
                        {fleetRunning.runningGnbs.map(g => (
                          <div key={g.profileName} className="fleet-gnb-card">
                            <div className="fleet-gnb-header">
                              <Radio size={14} className="text-blue"/>
                              <strong>{g.profileName}</strong>
                              <span className="fleet-state-badge registered">● RUNNING</span>
                              <button className="btn btn-xs btn-danger ml-auto" onClick={() => stopFleetGNB(g.profileName)}>■ Stop</button>
                            </div>
                            <div className="fleet-gnb-body">
                              <div className="fleet-ue-detail"><span>gNB-ID</span><code>{g.gnbId}</code></div>
                              <div className="fleet-ue-detail"><span>Started</span><code>{new Date(g.startedAt).toLocaleTimeString()}</code></div>
                              <div className="fleet-ue-detail" style={{ flexDirection: 'column', alignItems: 'flex-start', marginTop: '6px' }}>
                                <span style={{ marginBottom: '4px' }}>Connected UEs ({g.connectedUes?.length || 0})</span>
                                {g.connectedUes && g.connectedUes.length > 0 ? (
                                  <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                                    {g.connectedUes.map((ue: string) => (
                                      <span key={ue} className="fleet-tag" style={{ margin: 0 }}>{ue}</span>
                                    ))}
                                  </div>
                                ) : (
                                  <code style={{ fontSize: '11px', color: 'var(--text-muted)' }}>None</code>
                                )}
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                {/* Handover Control Panel */}
                {(fleetRunning.runningUes?.length > 0) && (
                  <div className="fleet-handover-panel">
                    <h4 className="fleet-live-title"><Layers size={14}/> Handover Control</h4>
                    <p className="fleet-hint" style={{ margin: '8px 0 12px 0' }}>
                      Select a registered UE and a target gNodeB cell to trigger N2 or Xn handover procedures.
                    </p>
                    <div className="fleet-ho-form">
                      <div className="form-group" style={{ flex: 1, minWidth: '160px' }}>
                        <label>UE ID</label>
                        <select className="form-input" value={controlUeId ?? ''} onChange={e => setControlUeId(Number(e.target.value))}>
                          <option value="">Select UE</option>
                          {fleetRunning.runningUes.map(u => (
                            <option key={u.id} value={u.id}>UE-{u.id} ({u.supi})</option>
                          ))}
                        </select>
                      </div>

                      <div className="form-group" style={{ flex: 1, minWidth: '200px' }}>
                        <label>Target gNB</label>
                        {fleetRunning.runningGnbs && fleetRunning.runningGnbs.length > 0 ? (
                          <select
                            className="form-input"
                            value={selectedTargetGnbName}
                            onChange={(e) => {
                              const name = e.target.value;
                              setSelectedTargetGnbName(name);
                              const gnb = fleetRunning.runningGnbs.find(g => g.profileName === name);
                              if (gnb) {
                                setHoTargetIp(gnb.controlIp);
                                setHoTargetPort(gnb.linkPort || 9489);
                                setHoTargetLinkType(gnb.linkType);
                                setHoTargetSocketPath(gnb.socketPath || '');
                              }
                            }}
                          >
                            <option value="">Select Target gNB</option>
                            {fleetRunning.runningGnbs.map(g => (
                              <option key={g.profileName} value={g.profileName}>
                                {g.profileName} ({g.gnbId}) - {g.linkType === 'unix' ? 'UNIX' : `${g.controlIp}:${g.linkPort}`}
                              </option>
                            ))}
                          </select>
                        ) : (
                          <input className="form-input" disabled placeholder="No running gNBs" />
                        )}
                      </div>

                      <div className="form-group" style={{ flex: '0 0 auto', alignSelf: 'flex-end', display: 'flex', gap: '8px' }}>
                        <button
                          className="btn btn-primary"
                          disabled={!controlUeId || !selectedTargetGnbName}
                          onClick={async () => {
                            if (!controlUeId) return;
                            const targetGnbObj = fleetRunning.runningGnbs?.find(g => g.profileName === selectedTargetGnbName);
                            const targetGnbId = targetGnbObj?.gnbId || "";
                            const targetGnbName = targetGnbObj?.profileName || "";
                            try {
                              const res = await fetch(`${API_BASE}/ue/action`, {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({
                                  ueId: controlUeId,
                                  action: 'handover',
                                  targetGnbIp: hoTargetIp,
                                  targetGnbPort: hoTargetPort,
                                  targetGnbLinkType: hoTargetLinkType,
                                  targetGnbSocketPath: hoTargetSocketPath,
                                  targetGnbId: targetGnbId,
                                  targetGnbName: targetGnbName
                                })
                              });
                              if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
                              showFleetMsg(`N2 Handover triggered for UE-${controlUeId} → ${selectedTargetGnbName}`, 'success');
                            } catch(e) { showFleetMsg(`Error: ${e}`, 'error'); }
                          }}
                        >
                          Trigger N2 Handover
                        </button>
                        <button
                          className="btn btn-primary"
                          style={{ backgroundColor: 'var(--accent-color)' }}
                          disabled={!controlUeId || !selectedTargetGnbName}
                          onClick={async () => {
                            if (!controlUeId) return;
                            const targetGnbObj = fleetRunning.runningGnbs?.find(g => g.profileName === selectedTargetGnbName);
                            const targetGnbId = targetGnbObj?.gnbId || "";
                            const targetGnbName = targetGnbObj?.profileName || "";
                            try {
                              const res = await fetch(`${API_BASE}/ue/action`, {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({
                                  ueId: controlUeId,
                                  action: 'xn-handover',
                                  targetGnbIp: hoTargetIp,
                                  targetGnbPort: hoTargetPort,
                                  targetGnbLinkType: hoTargetLinkType,
                                  targetGnbSocketPath: hoTargetSocketPath,
                                  targetGnbId: targetGnbId,
                                  targetGnbName: targetGnbName
                                })
                              });
                              if (!res.ok) { showFleetMsg(await res.text(), 'error'); return; }
                              showFleetMsg(`Xn Handover triggered for UE-${controlUeId} → ${selectedTargetGnbName}`, 'success');
                            } catch(e) { showFleetMsg(`Error: ${e}`, 'error'); }
                          }}
                        >
                          Trigger Xn Handover
                        </button>
                      </div>
                    </div>
                    <div className="fleet-info-box">
                      <strong>ℹ Handover Support:</strong> Both <strong>N2 Path Switch Handover</strong> (via AMF path switch) and <strong>Xn Handover</strong> (direct peer-to-peer Xn interface signaling followed by path switch) are supported.
                    </div>
                  </div>
                )}
              </div>
            )}

          </div>
        )}

        {/* ─── Diagnostics Tab ───────────────────────────────────────────── */}
        {activeTab === 'diagnostics' && (
          <div className="view-body fade-in">
            <div className="fleet-toast info" style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '16px', background: 'rgba(59, 130, 246, 0.1)', border: '1px solid rgba(59, 130, 246, 0.2)', padding: '12px', borderRadius: '8px' }}>
              <AlertTriangle size={16} style={{ flexShrink: 0, color: '#eab308' }} />
              <div style={{ fontSize: '13px' }}>
                <strong>Root Access Requirement:</strong> Raw socket binding requires the backend to be run with root permissions (e.g. <code>sudo ./app web</code>). Capturing on virtual tunnel interfaces (<code>uetun*</code>) automatically encapsulates raw IP packets into Ethernet frames for standard Wireshark compatibility.
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1.5fr', gap: '24px', marginBottom: '24px' }}>
              {/* Left Column: Capture Control & Live Status */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
                <div className="card">
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
                    <h3 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Radio size={18} className="text-blue" />
                      Live PCAP Capturer
                    </h3>
                    {activeCaptureStatus?.isCapturing && (
                      <span className="fleet-state-badge registered animate-pulse" style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                        <span style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: '#10b981', display: 'inline-block' }} />
                        CAPTURING
                      </span>
                    )}
                  </div>

                  <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                    <div className="form-group">
                      <label style={{ display: 'block', marginBottom: '6px', fontSize: '12px', opacity: 0.8 }}>Target Interface</label>
                      <select
                        className="form-input"
                        style={{ width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border-color)', background: 'var(--bg-input)', color: 'var(--text-main)' }}
                        value={pcapInterface}
                        onChange={(e) => {
                          setPcapInterface(e.target.value);
                          const ifName = e.target.value === 'any' ? 'any' : e.target.value;
                          setPcapFileName(`capture_${ifName}_${Math.floor(Date.now() / 1000)}.pcap`);
                        }}
                        disabled={activeCaptureStatus?.isCapturing}
                      >
                        <option value="any">Any / Loopback (All Interfaces)</option>
                        {pcapInterfaces.map((iface) => (
                          <option key={iface.name} value={iface.name}>
                            {iface.name} ({iface.ipAddresses.join(', ') || 'No IP'}) {iface.flags.includes('up') ? ' - UP' : ''}
                          </option>
                        ))}
                      </select>
                    </div>

                    <div className="form-group">
                      <label style={{ display: 'block', marginBottom: '6px', fontSize: '12px', opacity: 0.8 }}>Protocol Filter</label>
                      <select
                        className="form-input"
                        style={{ width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border-color)', background: 'var(--bg-input)', color: 'var(--text-main)' }}
                        value={pcapProtocol}
                        onChange={(e) => setPcapProtocol(e.target.value)}
                        disabled={activeCaptureStatus?.isCapturing}
                      >
                        <option value="all">All Protocols (Ethernet/IP/TCP/UDP/SCTP/ICMP)</option>
                        <option value="icmp">ICMP (Ping)</option>
                        <option value="tcp">TCP</option>
                        <option value="udp">UDP</option>
                        <option value="sctp">SCTP (NGAP/Control Plane)</option>
                      </select>
                    </div>

                    <div className="form-group">
                      <label style={{ display: 'block', marginBottom: '6px', fontSize: '12px', opacity: 0.8 }}>Destination Filename</label>
                      <input
                        className="form-input"
                        type="text"
                        style={{ width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border-color)', background: 'var(--bg-input)', color: 'var(--text-main)' }}
                        placeholder="e.g. uetun101_traffic.pcap"
                        value={pcapFileName}
                        onChange={(e) => setPcapFileName(e.target.value)}
                        disabled={activeCaptureStatus?.isCapturing}
                      />
                    </div>

                    <div style={{ marginTop: '8px' }}>
                      {activeCaptureStatus?.isCapturing ? (
                        <button
                          className="btn btn-danger"
                          style={{ width: '100%', padding: '12px', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px', border: 'none', borderRadius: '6px', cursor: 'pointer', background: '#ef4444', color: '#fff', fontWeight: 'bold' }}
                          onClick={stopPcapCapture}
                          disabled={isStoppingPcap}
                        >
                          <Trash2 size={16} />
                          {isStoppingPcap ? 'Stopping Capture...' : 'Stop Packet Capture'}
                        </button>
                      ) : (
                        <button
                          className="btn btn-primary"
                          style={{ width: '100%', padding: '12px', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px', border: 'none', borderRadius: '6px', cursor: 'pointer', background: '#3b82f6', color: '#fff', fontWeight: 'bold' }}
                          onClick={startPcapCapture}
                          disabled={isStartingPcap}
                        >
                          <Play size={16} />
                          {isStartingPcap ? 'Starting Capture...' : 'Start Packet Capture'}
                        </button>
                      )}
                    </div>
                  </div>
                </div>

                {/* Real-time stats dashboard */}
                {activeCaptureStatus?.isCapturing && (
                  <div className="card fade-in" style={{ borderColor: 'rgba(16, 185, 129, 0.4)', background: 'rgba(16, 185, 129, 0.02)' }}>
                    <h4 style={{ margin: '0 0 16px 0', fontSize: '14px', letterSpacing: '0.05em', color: '#10b981', display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <Activity size={14} className="animate-pulse" />
                      LIVE RECORDING TELEMETRY
                    </h4>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                      <div style={{ background: 'rgba(255,255,255,0.02)', padding: '12px', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '4px' }}>Packets Captured</div>
                        <div style={{ fontSize: '20px', fontWeight: 'bold', fontFamily: 'monospace', color: 'var(--text-primary)' }}>
                          {activeCaptureStatus.packetCount?.toLocaleString() || 0}
                        </div>
                      </div>
                      <div style={{ background: 'rgba(255,255,255,0.02)', padding: '12px', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '4px' }}>Data Volume</div>
                        <div style={{ fontSize: '20px', fontWeight: 'bold', fontFamily: 'monospace', color: 'var(--text-primary)' }}>
                          {(() => {
                            const bytes = activeCaptureStatus.bytesCount || 0;
                            if (bytes === 0) return '0 Bytes';
                            const k = 1024;
                            const sizes = ['Bytes', 'KB', 'MB', 'GB'];
                            const i = Math.floor(Math.log(bytes) / Math.log(k));
                            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
                          })()}
                        </div>
                      </div>
                      <div style={{ background: 'rgba(255,255,255,0.02)', padding: '12px', borderRadius: '8px', border: '1px solid var(--border-color)', gridColumn: 'span 2' }}>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '4px' }}>Elapsed Capture Time</div>
                        <div style={{ fontSize: '22px', fontWeight: 'bold', fontFamily: 'monospace', color: '#10b981', display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <RefreshCw size={16} className="animate-spin text-emerald" />
                          {(() => {
                            const seconds = activeCaptureStatus.elapsedSec || 0;
                            const mins = Math.floor(seconds / 60);
                            const secs = seconds % 60;
                            return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
                          })()}
                        </div>
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {/* Right Column: Saved Capture Files */}
              <div className="card" style={{ display: 'flex', flexDirection: 'column' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
                  <h3 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <FileText size={18} className="text-blue" />
                    Diagnostics PCAP Repository
                  </h3>
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={fetchSavedPcapFiles}
                    disabled={isRefreshingPcaps}
                    style={{ padding: '4px 10px', display: 'flex', alignItems: 'center', gap: '4px' }}
                  >
                    <RefreshCw size={13} className={isRefreshingPcaps ? 'animate-spin' : ''} />
                    Refresh
                  </button>
                </div>

                <p className="fleet-hint" style={{ marginTop: 0 }}>
                  Captured packet files are stored under <code>log/captures/</code>. Click download to fetch them locally and open in Wireshark.
                </p>

                <div className="fleet-table-wrapper" style={{ flex: 1, maxHeight: '350px', overflowY: 'auto' }}>
                  {savedPcapFiles.length === 0 ? (
                    <div className="fleet-empty" style={{ padding: '60px 0' }}>
                      No saved PCAP files found in <code>log/captures/</code>. Start a capture session to record network traffic.
                    </div>
                  ) : (
                    <table className="fleet-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
                      <thead>
                        <tr>
                          <th style={{ textAlign: 'left', padding: '8px' }}>Filename</th>
                          <th style={{ textAlign: 'left', padding: '8px' }}>Size</th>
                          <th style={{ textAlign: 'left', padding: '8px' }}>Recorded At</th>
                          <th style={{ textAlign: 'right', padding: '8px' }}>Actions</th>
                        </tr>
                      </thead>
                      <tbody>
                        {savedPcapFiles.map((file) => (
                          <tr key={file.name} style={{ borderBottom: '1px solid var(--border-color)' }}>
                            <td style={{ padding: '8px', verticalAlign: 'middle' }}>
                              <strong className="mono">{file.name}</strong>
                            </td>
                            <td className="mono" style={{ padding: '8px', verticalAlign: 'middle' }}>
                              {(() => {
                                const bytes = file.size;
                                if (bytes === 0) return '0 Bytes';
                                const k = 1024;
                                const sizes = ['Bytes', 'KB', 'MB', 'GB'];
                                const i = Math.floor(Math.log(bytes) / Math.log(k));
                                return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
                              })()}
                            </td>
                            <td style={{ padding: '8px', verticalAlign: 'middle', fontSize: '12px', color: 'var(--text-secondary)' }}>
                              {new Date(file.modTime).toLocaleString()}
                            </td>
                            <td style={{ padding: '8px', textAlign: 'right', verticalAlign: 'middle' }}>
                              <div style={{ display: 'flex', gap: '6px', justifyContent: 'flex-end' }}>
                                <button
                                  className="btn btn-xs btn-success"
                                  onClick={() => window.location.href = `${API_BASE}/diagnostics/pcap/download?file=${encodeURIComponent(file.name)}`}
                                  style={{ padding: '4px 8px', borderRadius: '4px', cursor: 'pointer', border: 'none', background: '#10b981', color: '#fff', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}
                                  title="Download PCAP file"
                                >
                                  <Download size={11} />
                                  Download
                                </button>
                                <button
                                  className="btn btn-xs btn-danger"
                                  onClick={() => deletePcapFile(file.name)}
                                  disabled={activeCaptureStatus?.isCapturing && activeCaptureStatus.fileName === file.name}
                                  style={{ padding: '4px 8px', borderRadius: '4px', cursor: 'pointer', border: 'none', background: '#ef4444', color: '#fff', fontSize: '11px' }}
                                  title="Delete PCAP file"
                                >
                                  Delete
                                </button>
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
              </div>
            </div>

            {/* 3GPP Decoded Frame Inspector Card */}
            <div className="card" style={{ marginBottom: '24px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '12px' }}>
                <h3 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <Layers size={18} className="text-blue" />
                  3GPP Decoded Frame Inspector
                </h3>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Inspector Spec Release:</span>
                  <div style={{ display: 'flex', border: '1px solid var(--border-color)', borderRadius: '6px', overflow: 'hidden' }}>
                    {['15', '17', '18', '19'].map((r) => (
                      <button
                        key={r}
                        onClick={() => setInspectRelease(r)}
                        style={{
                          padding: '4px 8px',
                          fontSize: '11px',
                          border: 'none',
                          cursor: 'pointer',
                          background: inspectRelease === r ? 'rgba(59, 130, 246, 0.2)' : 'transparent',
                          color: inspectRelease === r ? '#60a5fa' : 'var(--text-secondary)',
                          fontWeight: inspectRelease === r ? 'bold' : 'normal',
                          transition: 'all 0.2s'
                        }}
                      >
                        Rel {r === '15' ? '15/16' : r}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              {/* Alert explaining capabilities */}
              <div className="fleet-toast info" style={{ display: 'flex', alignItems: 'center', gap: '8px', background: 'rgba(16, 185, 129, 0.08)', border: '1px solid rgba(16, 185, 129, 0.15)', padding: '8px 12px', borderRadius: '6px', margin: 0 }}>
                <Activity size={14} style={{ color: '#10b981', flexShrink: 0 }} />
                <div style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
                  <strong>APER Decode View:</strong> Real-time APER decoded representation. Highlighted green items represent 3GPP Release capabilities enabled on the simulated control plane.
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '250px 1fr', gap: '20px' }}>
                {/* Left panel: Message select list */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  <div style={{ fontSize: '11px', fontWeight: 600, textTransform: 'uppercase', color: 'var(--text-muted)', marginBottom: '4px', letterSpacing: '0.05em' }}>
                    Control Plane Messages
                  </div>
                  {[
                    { id: 'ng-setup', label: 'NG Setup Request' },
                    { id: 'initial-ue', label: 'Initial UE Message' },
                    { id: 'pdu-setup', label: 'PDU Session Setup Req' },
                    { id: 'path-switch', label: 'Path Switch Request' },
                    { id: 'handover-req', label: 'Handover Required' },
                    { id: 'handover-ack', label: 'Handover Request Ack' },
                    { id: 'ue-release-complete', label: 'UE Context Release Complete' }
                  ].map((msg) => (
                    <button
                      key={msg.id}
                      onClick={() => setSelectedInspectMsg(msg.id)}
                      className={`inspect-msg-btn ${selectedInspectMsg === msg.id ? 'active' : ''}`}
                    >
                      {msg.label}
                    </button>
                  ))}
                </div>

                {/* Right panel: Decode output */}
                {(() => {
                  const decoded = getDecodedPacketData(selectedInspectMsg, inspectRelease);
                  const formattedHex = formatHexDump(decoded.hex);
                  const isR17 = inspectRelease === '17' || inspectRelease === '18' || inspectRelease === '19';
                  const isR18 = inspectRelease === '18' || inspectRelease === '19';
                  const isR19 = inspectRelease === '19';
                  return (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <span style={{ fontSize: '14px', fontWeight: 'bold', color: 'var(--text-primary)' }}>{decoded.name}</span>
                          <span className="badge" style={{ fontSize: '10px', background: 'rgba(59,130,246,0.1)', color: '#60a5fa', border: '1px solid rgba(59,130,246,0.2)', padding: '2px 6px', borderRadius: '4px', fontFamily: 'monospace' }}>
                            {decoded.specRef}
                          </span>
                        </div>
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText(decoded.hex);
                            alert("Copied raw hex stream to clipboard!");
                          }}
                          className="btn btn-ghost btn-sm"
                          style={{ padding: '2px 8px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}
                        >
                          Copy Hex
                        </button>
                      </div>

                      <p style={{ margin: 0, fontSize: '12px', opacity: 0.8, color: 'var(--text-secondary)', lineHeight: '1.4' }}>
                        {decoded.description}
                      </p>

                      <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                        {/* Decoded ASN.1 Tree */}
                        <div>
                          <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px', textTransform: 'uppercase' }}>ASN.1 APER Structured View</div>
                          <div className="asn1-tree">
                            {decoded.ies.map((ie, index) => {
                              const isHighlighted = ie.highlight && ((ie.release === '17' && isR17) || (ie.release === '18' && isR18) || (ie.release === '19' && isR19));
                              return (
                                <div
                                  key={index}
                                  className={`asn1-line ${isHighlighted ? 'highlighted' : ''}`}
                                  style={{ paddingLeft: `${ie.indent * 16}px` }}
                                >
                                  {/* Indent connector lines */}
                                  {Array.from({ length: ie.indent }).map((_, depth) => (
                                    <span key={depth} style={{ color: 'var(--border-color)', marginRight: '8px', userSelect: 'none' }}>│</span>
                                  ))}
                                  <span className="asn1-tag">{ie.name}</span>
                                  {ie.value && <span className="asn1-value">: {ie.value}</span>}
                                  {ie.comment && <span className="asn1-comment"> -- {ie.comment}</span>}
                                  {ie.release && (
                                    <span style={{ fontSize: '9px', fontWeight: 'bold', background: ie.release === '17' ? '#10b981' : ie.release === '18' ? '#7c3aed' : '#0284c7', color: '#fff', padding: '1px 4px', borderRadius: '3px', marginLeft: '6px', fontFamily: 'sans-serif' }}>
                                      Rel {ie.release === '15' ? '15/16' : ie.release}
                                    </span>
                                  )}
                                </div>
                              );
                            })}
                          </div>
                        </div>

                        {/* Raw Hex Pane */}
                        <div>
                          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>Raw APER Byte Stream</span>
                            <span style={{ fontSize: '10px', color: 'var(--text-muted)', fontFamily: 'monospace' }}>Size: {decoded.hex.length / 2} Bytes</span>
                          </div>
                          <div className="hex-pane">
                            {formattedHex}
                          </div>
                        </div>
                      </div>
                    </div>
                  );
                })()}
              </div>
            </div>

            {/* Bottom Row: Permanent System Logs Console */}
            <div className="console-panel" style={{ marginTop: '24px' }}>
              <div className="console-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 20px', flexWrap: 'wrap', gap: '12px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', color: 'var(--text-secondary)' }}>
                    <Terminal size={14} />
                    <span style={{ fontSize: '11px', fontWeight: 600, letterSpacing: '0.05em' }}>SYSTEM LOGS</span>
                  </div>
                  <span style={{ fontSize: '12px', fontWeight: 'bold', color: 'var(--text-secondary)' }}>
                    EMULATOR SYSTEM LOGS HISTORY (PERSISTED ON SERVER)
                  </span>

                  {/* Action buttons side-by-side, compact and small */}
                  <div style={{ display: 'flex', gap: '4px', marginLeft: '8px' }}>
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={fetchDiagnosticsLogs}
                      disabled={isRefreshingLogs}
                      style={{ padding: '4px 6px', display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', background: 'transparent', border: '1px solid var(--border-color)', color: 'var(--text-secondary)', borderRadius: '4px', width: '28px', height: '28px' }}
                      title="Reload system logs from server"
                    >
                      <RefreshCw size={12} className={isRefreshingLogs ? 'animate-spin' : ''} />
                    </button>

                    <button
                      className="btn btn-success btn-sm"
                      onClick={() => window.location.href = `${API_BASE}/diagnostics/logs/download`}
                      style={{ padding: '4px 6px', display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', background: '#10b981', border: 'none', color: '#fff', borderRadius: '4px', width: '28px', height: '28px' }}
                      title="Download full system log file"
                    >
                      <Download size={12} />
                    </button>

                    <button
                      className="btn btn-danger btn-sm"
                      onClick={clearSystemLogs}
                      disabled={isClearingLogs}
                      style={{ padding: '4px 6px', display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', background: '#ef4444', border: 'none', color: '#fff', borderRadius: '4px', width: '28px', height: '28px' }}
                      title="Clear/Truncate server system logs"
                    >
                      <Trash2 size={12} />
                    </button>
                  </div>
                </div>

                <div className="console-filter-bar" style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
                  <span
                    className={`filter-badge ${diagnosticsLogLevel === 'all' ? 'active' : ''}`}
                    onClick={() => setDiagnosticsLogLevel('all')}
                    style={{
                      cursor: 'pointer',
                      padding: '4px 8px',
                      borderRadius: '4px',
                      fontSize: '11px',
                      background: diagnosticsLogLevel === 'all' ? (theme === 'light' ? '#cbd5e1' : 'rgba(255,255,255,0.1)') : 'transparent',
                      color: diagnosticsLogLevel === 'all' ? (theme === 'light' ? '#0f172a' : '#f9fafb') : 'var(--text-secondary)'
                    }}
                  >
                    ALL LOGS
                  </span>
                  <span
                    className={`filter-badge ${diagnosticsLogLevel === 'info' ? 'active' : ''}`}
                    onClick={() => setDiagnosticsLogLevel('info')}
                    style={{
                      cursor: 'pointer',
                      padding: '4px 8px',
                      borderRadius: '4px',
                      fontSize: '11px',
                      background: diagnosticsLogLevel === 'info' ? (theme === 'light' ? '#dbeafe' : 'rgba(59, 130, 246, 0.2)') : 'transparent',
                      color: diagnosticsLogLevel === 'info' ? (theme === 'light' ? '#1d4ed8' : '#60a5fa') : 'var(--text-secondary)'
                    }}
                  >
                    INFO
                  </span>
                  <span
                    className={`filter-badge ${diagnosticsLogLevel === 'warn' ? 'active' : ''}`}
                    onClick={() => setDiagnosticsLogLevel('warn')}
                    style={{
                      cursor: 'pointer',
                      padding: '4px 8px',
                      borderRadius: '4px',
                      fontSize: '11px',
                      background: diagnosticsLogLevel === 'warn' ? (theme === 'light' ? '#fef3c7' : 'rgba(234, 179, 8, 0.2)') : 'transparent',
                      color: diagnosticsLogLevel === 'warn' ? (theme === 'light' ? '#b45309' : '#fde047') : 'var(--text-secondary)'
                    }}
                  >
                    WARNINGS
                  </span>
                  <span
                    className={`filter-badge ${diagnosticsLogLevel === 'error' ? 'active' : ''}`}
                    onClick={() => setDiagnosticsLogLevel('error')}
                    style={{
                      cursor: 'pointer',
                      padding: '4px 8px',
                      borderRadius: '4px',
                      fontSize: '11px',
                      background: diagnosticsLogLevel === 'error' ? (theme === 'light' ? '#fee2e2' : 'rgba(239, 68, 68, 0.2)') : 'transparent',
                      color: diagnosticsLogLevel === 'error' ? (theme === 'light' ? '#b91c1c' : '#fca5a5') : 'var(--text-secondary)'
                    }}
                  >
                    ERRORS
                  </span>

                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginLeft: '6px', borderLeft: '1px solid var(--border-color)', paddingLeft: '8px' }}>
                    <Search size={12} style={{ color: 'var(--text-muted)' }} />
                    <input
                      type="text"
                      placeholder="Search log history..."
                      value={diagnosticsLogSearch}
                      onChange={(e) => setDiagnosticsLogSearch(e.target.value)}
                      style={{
                        background: 'var(--bg-input)',
                        border: '1px solid var(--border-color)',
                        borderRadius: '4px',
                        color: 'var(--text-primary)',
                        padding: '2px 8px',
                        fontSize: '11px',
                        width: '120px',
                        outline: 'none',
                        transition: 'width 0.25s ease'
                      }}
                      onFocus={(e) => e.target.style.width = '180px'}
                      onBlur={(e) => e.target.style.width = '120px'}
                    />
                  </div>
                </div>
              </div>

              <div className="console-body" style={{ maxHeight: '400px' }}>
                {diagnosticsLogs.length === 0 ? (
                  <div style={{ color: 'var(--text-muted)', textAlign: 'center', padding: '40px 0' }}>
                    No log entries found. Reload logs or trigger actions to generate system output.
                  </div>
                ) : (
                  (() => {
                    const filtered = diagnosticsLogs.filter(line => {
                      if (diagnosticsLogSearch && !line.toLowerCase().includes(diagnosticsLogSearch.toLowerCase())) {
                        return false;
                      }
                      if (diagnosticsLogLevel !== 'all') {
                        const lower = line.toLowerCase();
                        if (diagnosticsLogLevel === 'info' && !lower.includes('level=info') && !lower.includes('[info]')) return false;
                        if (diagnosticsLogLevel === 'warn' && !lower.includes('level=warn') && !lower.includes('[warn]') && !lower.includes('warning')) return false;
                        if (diagnosticsLogLevel === 'error' && !lower.includes('level=err') && !lower.includes('[err]') && !lower.includes('error')) return false;
                      }
                      return true;
                    });

                    if (filtered.length === 0) {
                      return (
                        <div style={{ color: 'var(--text-muted)', textAlign: 'center', padding: '40px 0' }}>
                          No log lines match the current search or level filters.
                        </div>
                      );
                    }

                    return filtered.map((line, idx) => {
                      const levelClass = (() => {
                        const lower = line.toLowerCase();
                        if (lower.includes('level=error') || lower.includes('level=fatal') || lower.includes('[error]') || lower.includes('[err]')) return 'log-error';
                        if (lower.includes('level=warning') || lower.includes('level=warn') || lower.includes('[warn]')) return 'log-warn';
                        if (lower.includes('level=debug') || lower.includes('[debug]')) return 'log-debug';
                        return 'log-info';
                      })();

                      return (
                        <div key={idx} className={`log-line ${levelClass}`} style={{ fontSize: '12px', padding: '2px 0', borderBottom: '1px solid var(--border-color)', opacity: 0.9 }}>
                          {line}
                        </div>
                      );
                    });
                  })()
                )}
              </div>
            </div>
          </div>
        )}

      {/* Fleet Form Overlays (Rendered at root layout to prevent transform/clipping side effects) */}
      {showUEForm && (
        <div className="fleet-form-overlay">
          <div className="fleet-form-card">
            <div className="fleet-form-header">
              <h4>{editingUE.name && ueProfiles.find(p => p.name === editingUE.name) ? `Edit: ${editingUE.name}` : 'New UE Profile'}</h4>
              <button className="fleet-form-close" onClick={() => setShowUEForm(false)}>✕</button>
            </div>
            <div className="fleet-form-body">
              <div className="fleet-form-grid">
                <div className="form-group">
                  <label>Profile Name *</label>
                  <input className="form-input" value={editingUE.name} onChange={e => setEditingUE({...editingUE, name: e.target.value})} placeholder="e.g. UE-Alpha" />
                </div>
                <div className="form-group">
                  <label>MSIN (8–10 digits) *</label>
                  <input className="form-input" value={editingUE.msin} onChange={e => setEditingUE({...editingUE, msin: e.target.value})} placeholder="0000000001" />
                </div>
                <div className="form-group">
                  <label>Key (32 hex chars) *</label>
                  <input className="form-input" value={editingUE.key} onChange={e => setEditingUE({...editingUE, key: e.target.value})} placeholder="465b5ce8b199b49faa5f0a2ee238a6bc" />
                </div>
                <div className="form-group">
                  <label>OPC (32 hex chars) *</label>
                  <input className="form-input" value={editingUE.opc} onChange={e => setEditingUE({...editingUE, opc: e.target.value})} placeholder="e8ed289deba952e4283b54e88e6183ca" />
                </div>
                <div className="form-group">
                  <label>AMF (4 hex) *</label>
                  <input className="form-input" value={editingUE.amf} onChange={e => setEditingUE({...editingUE, amf: e.target.value})} placeholder="8000" />
                </div>
                <div className="form-group">
                  <label>SQN (12 hex) *</label>
                  <input className="form-input" value={editingUE.sqn} onChange={e => setEditingUE({...editingUE, sqn: e.target.value})} placeholder="000000000000" />
                </div>
                <div className="form-group">
                  <label>HPLMN MCC</label>
                  <input className="form-input" value={editingUE.hplmn.mcc} onChange={e => setEditingUE({...editingUE, hplmn: {...editingUE.hplmn, mcc: e.target.value}})} placeholder="999" />
                </div>
                <div className="form-group">
                  <label>HPLMN MNC</label>
                  <input className="form-input" value={editingUE.hplmn.mnc} onChange={e => setEditingUE({...editingUE, hplmn: {...editingUE.hplmn, mnc: e.target.value}})} placeholder="70" />
                </div>
                <div className="form-group">
                  <label>DNN</label>
                  <input className="form-input" value={editingUE.dnn} onChange={e => setEditingUE({...editingUE, dnn: e.target.value})} placeholder="internet" />
                </div>
                <div className="form-group">
                  <label>PDU Session Type</label>
                  <select className="form-input" value={editingUE.pduSessionType} onChange={e => setEditingUE({...editingUE, pduSessionType: e.target.value})}>
                    <option>IPv4</option><option>IPv6</option><option>IPv4v6</option>
                  </select>
                </div>
                <div className="form-group">
                  <label>Slice SST</label>
                  <input className="form-input" type="number" value={editingUE.snssai.sst} onChange={e => setEditingUE({...editingUE, snssai: {...editingUE.snssai, sst: Number(e.target.value)}})} />
                </div>
                <div className="form-group">
                  <label>Slice SD (6 hex, optional)</label>
                  <input className="form-input" value={editingUE.snssai.sd} onChange={e => setEditingUE({...editingUE, snssai: {...editingUE.snssai, sst: editingUE.snssai.sst, sd: e.target.value}})} placeholder="010203 or empty" />
                </div>
              </div>
            </div>
            <div className="fleet-form-actions">
              <button className="btn btn-ghost" onClick={() => setShowUEForm(false)}>Cancel</button>
              <button className="btn btn-primary" onClick={saveUEProfile}>Save Profile</button>
            </div>
          </div>
        </div>
      )}

      {ueToLaunch && (
        <div className="fleet-form-overlay">
          <div className="fleet-form-card" style={{ maxWidth: '400px' }}>
            <div className="fleet-form-header">
              <h4>Select Target gNodeB</h4>
              <button className="fleet-form-close" onClick={() => setUeToLaunch(null)}>✕</button>
            </div>
            <div className="fleet-form-body" style={{ padding: '20px' }}>
              <p style={{ marginBottom: '16px', fontSize: '14px', opacity: 0.85 }}>
                Choose which running gNodeB cell the UE <strong>{ueToLaunch}</strong> should register with:
              </p>
              <div className="form-group" style={{ marginBottom: '20px' }}>
                <label style={{ display: 'block', marginBottom: '8px', fontSize: '12px', textTransform: 'uppercase', letterSpacing: '0.05em', opacity: 0.7 }}>
                  Target gNodeB
                </label>
                <select 
                  className="form-input" 
                  style={{ width: '100%', padding: '10px', borderRadius: '8px', background: 'var(--bg-input)', border: '1px solid var(--border-color)', color: 'var(--text-main)' }}
                  value={selectedTargetGnb} 
                  onChange={e => setSelectedTargetGnb(e.target.value)}
                >
                  {fleetRunning.runningGnbs.map(g => (
                    <option key={g.profileName} value={g.profileName}>
                      {g.profileName} (gNB-ID: {g.gnbId})
                    </option>
                  ))}
                  <option value="">Default gNodeB (/tmp/gnb.sock)</option>
                </select>
              </div>
            </div>
            <div className="fleet-form-actions">
              <button className="btn btn-ghost" onClick={() => setUeToLaunch(null)}>Cancel</button>
              <button className="btn btn-primary" onClick={() => {
                launchUEProfile(ueToLaunch, selectedTargetGnb);
                setUeToLaunch(null);
              }}>
                Connect & Launch
              </button>
            </div>
          </div>
        </div>
      )}

      {showGNBForm && (
        <div className="fleet-form-overlay">
          <div className="fleet-form-card">
            <div className="fleet-form-header">
              <h4>{editingGNB.name && gnbProfiles.find(p => p.name === editingGNB.name) ? `Edit: ${editingGNB.name}` : 'New gNB Profile'}</h4>
              <button className="fleet-form-close" onClick={() => setShowGNBForm(false)}>✕</button>
            </div>
            <div className="fleet-form-body">
              <div className="fleet-form-grid">
                <div className="form-group">
                  <label>Profile Name *</label>
                  <input className="form-input" value={editingGNB.name} onChange={e => setEditingGNB({...editingGNB, name: e.target.value})} placeholder="e.g. gNB-West" />
                </div>
                <div className="form-group">
                  <label>gNB-ID (hex) *</label>
                  <input className="form-input" value={editingGNB.gnbId} onChange={e => setEditingGNB({...editingGNB, gnbId: e.target.value})} placeholder="000001" />
                </div>
                <div className="form-group">
                  <label>MCC</label>
                  <input className="form-input" value={editingGNB.mcc} onChange={e => setEditingGNB({...editingGNB, mcc: e.target.value})} placeholder="999" />
                </div>
                <div className="form-group">
                  <label>MNC</label>
                  <input className="form-input" value={editingGNB.mnc} onChange={e => setEditingGNB({...editingGNB, mnc: e.target.value})} placeholder="70" />
                </div>
                <div className="form-group">
                  <label>TAC</label>
                  <input className="form-input" value={editingGNB.tac} onChange={e => setEditingGNB({...editingGNB, tac: e.target.value})} placeholder="000001" />
                </div>
                <div className="form-group">
                  <label>Slice SST</label>
                  <input className="form-input" value={editingGNB.sliceSst} onChange={e => setEditingGNB({...editingGNB, sliceSst: e.target.value})} placeholder="01" />
                </div>
                <div className="form-group">
                  <label>Control IF IP *</label>
                  <input className="form-input" value={editingGNB.controlIp} onChange={e => setEditingGNB({...editingGNB, controlIp: e.target.value})} placeholder="127.0.0.1" />
                </div>
                <div className="form-group">
                  <label>Control IF Port *</label>
                  <input className="form-input" type="number" value={editingGNB.controlPort} onChange={e => setEditingGNB({...editingGNB, controlPort: Number(e.target.value)})} />
                </div>
                <div className="form-group">
                  <label>Data IF IP</label>
                  <input className="form-input" value={editingGNB.dataIp} onChange={e => setEditingGNB({...editingGNB, dataIp: e.target.value})} placeholder="127.0.0.1" />
                </div>
                <div className="form-group">
                  <label>Data IF Port</label>
                  <input className="form-input" type="number" value={editingGNB.dataPort} onChange={e => setEditingGNB({...editingGNB, dataPort: Number(e.target.value)})} />
                </div>
                <div className="form-group">
                  <label>Link Type</label>
                  <select className="form-input" value={editingGNB.linkType} onChange={e => setEditingGNB({...editingGNB, linkType: e.target.value})}>
                    <option value="unix">UNIX Socket</option>
                    <option value="tcp">TCP</option>
                  </select>
                </div>
                <div className="form-group">
                  <label>Link Port *</label>
                  <input className="form-input" type="number" value={editingGNB.linkPort} onChange={e => setEditingGNB({...editingGNB, linkPort: Number(e.target.value)})} />
                </div>
                <div className="form-group">
                  <label>AMF IP *</label>
                  <input className="form-input" value={editingGNB.amfIp} onChange={e => setEditingGNB({...editingGNB, amfIp: e.target.value})} placeholder="127.0.0.1" />
                </div>
                <div className="form-group">
                  <label>AMF Port *</label>
                  <input className="form-input" type="number" value={editingGNB.amfPort} onChange={e => setEditingGNB({...editingGNB, amfPort: Number(e.target.value)})} />
                </div>
              </div>
            </div>
            <div className="fleet-form-actions">
              <button className="btn btn-ghost" onClick={() => setShowGNBForm(false)}>Cancel</button>
              <button className="btn btn-primary" onClick={saveGNBProfile}>Save Profile</button>
            </div>
          </div>
        </div>
      )}
      </main>
    </div>
  );
}
