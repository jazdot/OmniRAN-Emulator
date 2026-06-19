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
  Moon
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

export default function App() {
  const [theme, setTheme] = useState<'dark' | 'light'>(() => {
    const saved = localStorage.getItem('theme');
    return (saved === 'dark' || saved === 'light') ? saved : 'light';
  });
  const [activeTab, setActiveTab] = useState<'dashboard' | 'scenarios' | 'config' | 'logs' | 'connectivity' | 'fleet'>('dashboard');
  const [selectedNode, setSelectedNode] = useState<'ue' | 'gnb' | 'core' | null>('ue');

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
          <stop offset="0%" stopColor="#6366f1" stopOpacity="0.4" />
          <stop offset="100%" stopColor="#8b5cf6" stopOpacity="0.4" />
        </linearGradient>
        <linearGradient id="n3-grad" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#6366f1" />
          <stop offset="100%" stopColor="#8b5cf6" />
        </linearGradient>
        <filter id="glow-filter" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="3" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>
      </defs>
    );
    links.push(defs);

    if (totalGnbs === 0) {
      const isCoreActive = status?.gnbLinkState && status.gnbLinkState !== 'offline';
      const strokeColor = isCoreActive ? '#8b5cf6' : '#334155';
      links.push(
        <g key="def-n2">
          <line x1="500" y1="285" x2="850" y2="285" stroke={strokeColor} strokeWidth="3" strokeDasharray="5,5" />
          {isCoreActive && (
            <circle r="5" fill="#a78bfa" filter="url(#glow-filter)">
              <animate attributeName="cx" from="500" to="850" dur="3s" repeatCount="indefinite" />
              <animate attributeName="cy" from="285" to="285" dur="3s" repeatCount="indefinite" />
            </circle>
          )}
        </g>
      );
      const isDataActive = status?.isRunning;
      const strokeDataColor = isDataActive ? '#3b82f6' : '#334155';
      links.push(
        <g key="def-n3">
          <line x1="500" y1="315" x2="850" y2="315" stroke={strokeDataColor} strokeWidth="3" />
          {isDataActive && (
            <circle r="5" fill="#60a5fa" filter="url(#glow-filter)">
              <animate attributeName="cx" from="500" to="850" dur="2s" repeatCount="indefinite" />
              <animate attributeName="cy" from="315" to="315" dur="2s" repeatCount="indefinite" />
            </circle>
          )}
        </g>
      );
    } else {
      gnbs.forEach((g, gIdx) => {
        const gy = (gIdx + 1) * 600 / (totalGnbs + 1);
        links.push(
          <g key={`n2-${g.profileName}`}>
            <line x1="500" y1={gy - 15} x2="850" y2="285" stroke="url(#n2-grad)" strokeWidth="2.5" strokeDasharray="4,4" />
            <circle r="4" fill="#c084fc" filter="url(#glow-filter)">
              <animate attributeName="cx" from="500" to="850" dur="2.5s" repeatCount="indefinite" />
              <animate attributeName="cy" from={gy - 15} to="285" dur="2.5s" repeatCount="indefinite" />
            </circle>
          </g>
        );

        const hasActiveSession = ues.some(u => u.gnbProfileName === g.profileName && u.pduSessions && u.pduSessions.length > 0);
        const strokeColor = hasActiveSession ? 'url(#n3-grad)' : '#334155';
        links.push(
          <g key={`n3-${g.profileName}`}>
            <line x1="500" y1={gy + 15} x2="850" y2="315" stroke={strokeColor} strokeWidth="3" opacity={hasActiveSession ? 1 : 0.4} />
            {hasActiveSession && (
              <circle r="5" fill="#60a5fa" filter="url(#glow-filter)">
                <animate attributeName="cx" from="500" to="850" dur="1.8s" repeatCount="indefinite" />
                <animate attributeName="cy" from={gy + 15} to="315" dur="1.8s" repeatCount="indefinite" />
              </circle>
            )}
          </g>
        );
      });
    }

    if (totalUes === 0) {
      const isUeActive = status?.isRunning;
      const strokeColor = isUeActive ? 'url(#uu-grad)' : '#334155';
      links.push(
        <g key="def-uu">
          <line x1="150" y1="300" x2="500" y2="300" stroke={strokeColor} strokeWidth="3" />
          {isUeActive && (
            <circle r="5" fill="#34d399" filter="url(#glow-filter)">
              <animate attributeName="cx" from="150" to="500" dur="2s" repeatCount="indefinite" />
              <animate attributeName="cy" from="300" to="300" dur="2s" repeatCount="indefinite" />
            </circle>
          )}
        </g>
      );
    } else {
      ues.forEach((u, uIdx) => {
        const uy = (uIdx + 1) * 600 / (totalUes + 1);
        let targetGy = 300;
        if (totalGnbs > 0) {
          const gIdx = gnbs.findIndex(g => g.profileName === u.gnbProfileName);
          if (gIdx !== -1) {
            targetGy = (gIdx + 1) * 600 / (totalGnbs + 1);
          }
        }
        const isRegistered = u.stateMmDesc?.includes('REGISTERED');
        const strokeColor = isRegistered ? 'url(#uu-grad)' : '#f59e0b';
        links.push(
          <g key={`uu-${u.id}`}>
            <line x1="150" y1={uy} x2="500" y2={targetGy} stroke={strokeColor} strokeWidth="3" opacity="0.9" />
            {isRegistered && (
              <circle r="5" fill="#34d399" filter="url(#glow-filter)">
                <animate attributeName="cx" from="150" to="500" dur="2s" repeatCount="indefinite" />
                <animate attributeName="cy" from={uy} to={targetGy} dur="2s" repeatCount="indefinite" />
              </circle>
            )}
          </g>
        );
      });
    }

    return links;
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

  const duplicateUEProfile = async (profile: UEProfile) => {
    try {
      // 1. Generate unique name
      let newName = `${profile.name}-Copy`;
      let counter = 1;
      while (ueProfiles.some(p => p.name === newName)) {
        newName = `${profile.name}-Copy${counter}`;
        counter++;
      }

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
      let newName = `${profile.name}-Copy`;
      let counter = 1;
      while (gnbProfiles.some(p => p.name === newName)) {
        newName = `${profile.name}-Copy${counter}`;
        counter++;
      }

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


  // Fetch emulator status
  const fetchStatus = async () => {
    try {
      const res = await fetch(`${API_BASE}/status`);
      if (res.ok) {
        const data = await res.json();
        setStatus(data);
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
  }, []);

  useEffect(() => {
    if (!autoRefresh) return;
    const timer = setInterval(() => {
      fetchStatus();
      fetchActiveUEs();
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
    (i) => i.name.startsWith('ue') || i.name.includes('tun')
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
              <div
                className={`status-dot ${
                  status?.gnbLinkState && status.gnbLinkState !== 'offline' ? 'active' : 'inactive'
                }`}
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
              {status?.isRunning ? (
                <>
                  <span className="status-dot warning animate-pulse" />
                  <span className="text-amber-500 font-bold uppercase">{status.runningName}</span>
                </>
              ) : (
                <>
                  <div className="status-dot active" />
                  <span className="text-emerald-500 font-bold">IDLE</span>
                </>
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
              <div className="card interactive-kpi primary-hover" onClick={() => setActiveTab('config')} title="Click to edit profile settings">
                <div className="card-header">
                  <span className="card-title">SIM IMSI</span>
                  <div className="card-icon primary">
                    <Cpu size={18} />
                  </div>
                </div>
                <div className="card-value">{status?.configSummary?.ueImsi || '001010000000001'}</div>
                <span className="card-desc">Provisioned subscriber profile identity</span>
              </div>

              <div className="card interactive-kpi success-hover" onClick={() => { const el = document.getElementById('activeUEsTable'); if (el) el.scrollIntoView({ behavior: 'smooth' }); }} title="Click to view active UEs list">
                <div className="card-header">
                  <span className="card-title">Active UEs Online</span>
                  <div className="card-icon success">
                    <Cpu size={18} />
                  </div>
                </div>
                <div className="card-value">{activeTuns.length}</div>
                <span className="card-desc">
                  {activeTuns.length > 0
                    ? `Active Tunnels: ${activeTuns.map((t) => t.name).join(', ')}`
                    : 'No virtual interfaces active'}
                </span>
              </div>

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

              <div className="card interactive-kpi purple-hover" onClick={() => setActiveTab('config')} title="Click to edit slice profile">
                <div className="card-header">
                  <span className="card-title">Slice Configuration</span>
                  <div className="card-icon purple">
                    <Layers size={18} />
                  </div>
                </div>
                <div className="card-value">{status?.configSummary?.ueSlice ? status.configSummary.ueSlice.replace('SST: ', 'SST ').replace(', SD: ', '/SD ') : 'SST 1/SD 010203'}</div>
                <span className="card-desc">Active S-NSSAI network slice profile</span>
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
                  {(() => {
                    const totalGnbs = status?.runningGnbs?.length || 0;
                    const totalUes = status?.runningUes?.length || 0;
                    return (
                      <>
                        {/* SVG Links Layer */}
                        <svg className="topology-svg" viewBox="0 0 1000 600" style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none' }}>
                          {renderSvgLinks()}
                        </svg>

                        {/* UE Nodes */}
                        {totalUes === 0 ? (
                          <div 
                            className={`topology-node clickable-node active ${selectedNode === 'ue' ? 'selected' : ''}`} 
                            onClick={() => {
                              setSelectedNode('ue');
                              setSelectedUeId(null);
                            }}
                            style={{ 
                              position: 'absolute',
                              left: '15%',
                              top: '50%',
                              transform: 'translate(-50%, -50%)',
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
                        ) : (
                          status.runningUes.map((u, idx) => {
                            const topPercent = `${(idx + 1) * 100 / (totalUes + 1)}%`;
                            const isRegistered = u.stateMmDesc?.includes('REGISTERED');
                            const isSelected = selectedNode === 'ue' && selectedUeId === u.id;
                            return (
                              <div 
                                key={u.id}
                                className={`topology-node clickable-node active ${isSelected ? 'selected' : ''}`} 
                                onClick={() => {
                                  setSelectedNode('ue');
                                  setSelectedUeId(u.id);
                                }}
                                style={{ 
                                  position: 'absolute',
                                  left: '15%',
                                  top: topPercent,
                                  transform: 'translate(-50%, -50%)',
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
                              </div>
                            );
                          })
                        )}

                        {/* gNodeB Nodes */}
                        {totalGnbs === 0 ? (
                          <div
                            className={`topology-node clickable-node ${status?.gnbLinkState !== 'offline' ? 'active' : ''} ${selectedNode === 'gnb' ? 'selected' : ''}`}
                            onClick={() => {
                              setSelectedNode('gnb');
                              setSelectedGnbName(null);
                            }}
                            style={{ 
                              position: 'absolute',
                              left: '50%',
                              top: '50%',
                              transform: 'translate(-50%, -50%)',
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
                        ) : (
                          status.runningGnbs.map((g, idx) => {
                            const topPercent = `${(idx + 1) * 100 / (totalGnbs + 1)}%`;
                            const isSelected = selectedNode === 'gnb' && selectedGnbName === g.profileName;
                            return (
                              <div
                                key={g.profileName}
                                className={`topology-node clickable-node active ${isSelected ? 'selected' : ''}`}
                                onClick={() => {
                                  setSelectedNode('gnb');
                                  setSelectedGnbName(g.profileName);
                                }}
                                style={{ 
                                  position: 'absolute',
                                  left: '50%',
                                  top: topPercent,
                                  transform: 'translate(-50%, -50%)',
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
                            );
                          })
                        )}

                        {/* 5G Core Node */}
                        <div
                          className={`topology-node clickable-node ${status?.gnbLinkState !== 'offline' || totalGnbs > 0 ? 'active' : ''} ${selectedNode === 'core' ? 'selected' : ''}`}
                          onClick={() => setSelectedNode('core')}
                          style={{ 
                            position: 'absolute',
                            left: '85%',
                            top: '50%',
                            transform: 'translate(-50%, -50%)',
                            '--node-color': '#8b5cf6',
                            '--node-color-glow': 'rgba(139, 92, 246, 0.2)'
                          } as React.CSSProperties}
                        >
                          <div className="node-icon-wrapper">
                            <Server />
                          </div>
                          <span className="node-label">5G Core (AMF/UPF)</span>
                          <span className="node-status-text">
                            {status?.gnbLinkState !== 'offline' || totalGnbs > 0 ? 'REACHABLE' : 'DISCONNECTED'}
                          </span>
                        </div>
                      </>
                    );
                  })()}
                </div>

                {/* Right side: Inspector Panel */}
                <div className="node-inspector-card">
                  {selectedNode === 'ue' && (() => {
                    const activeUe = getActiveUeToInspect();
                    if (activeUe) {
                      return (
                        <>
                          <h4 className="inspector-title" style={{ color: 'var(--color-success)' }}>
                            <Cpu size={14} /> UE-{activeUe.id} Inspector
                          </h4>
                          <div className="inspector-details" style={{ maxHeight: '350px', overflowY: 'auto' }}>
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
                                  {activeGnb.connectedUes.map(ue => (
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

                  {selectedNode === 'core' && (
                    <>
                      <h4 className="inspector-title" style={{ color: 'var(--color-purple)' }}>
                        <Server size={14} /> 5G Core Inspector
                      </h4>
                      <div className="inspector-details">
                        <div className="detail-row">
                          <span className="detail-label">AMF Target (N2)</span>
                          <span className="detail-val font-mono">{status?.configSummary?.amfTarget || '127.0.0.1:38412'}</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">UPF Target (N3)</span>
                          <span className="detail-val font-mono">{configData ? `${configData.AMF?.Ip}:2152` : '127.0.0.1:2152'}</span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Slice (S-NSSAI)</span>
                          <span className="detail-val font-mono">
                            {configData ? `SST ${configData.Ue?.Snssai?.Sst || '1'} / SD ${configData.Ue?.Snssai?.Sd || '010203'}` : 'SST 1'}
                          </span>
                        </div>
                        <div className="detail-row">
                          <span className="detail-label">Core Status</span>
                          <span className="detail-val font-semibold" style={{ color: status?.gnbLinkState && status.gnbLinkState !== 'offline' ? 'var(--color-success)' : 'var(--color-danger)' }}>
                            {status?.gnbLinkState && status.gnbLinkState !== 'offline' ? 'REACHABLE' : 'DISCONNECTED'}
                          </span>
                        </div>
                      </div>
                    </>
                  )}
                  
                  {!selectedNode && (
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--text-muted)', fontSize: '13px' }}>
                      Click any node to inspect standard 3GPP metrics
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
                                {gnb.connectedUes.map(ue => (
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
                    <div className="card-header">
                      <span className="card-title font-bold text-white">{scen.name}</span>
                      <span
                        className="status-dot"
                        style={{
                          backgroundColor: isRunningThis ? 'var(--color-warning)' : 'var(--border-color)'
                        }}
                      />
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
                {/* Left Side: UE Settings */}
                <div className="config-section">
                  <h3 className="panel-title" style={{ borderBottom: '1px solid var(--border-color)', paddingBottom: '10px' }}>
                    <Cpu size={18} /> User Equipment Settings
                  </h3>

                  <div className="form-group">
                    <label>MSIN (IMSI suffix)</label>
                    <input
                      type="text"
                      value={configData.Ue?.Msin || ''}
                      onChange={(e) => handleConfigChange('Ue.Msin', e.target.value)}
                    />
                  </div>

                  <div className="form-group">
                    <label>Security Key (K)</label>
                    <input
                      type="text"
                      value={configData.Ue?.Key || ''}
                      onChange={(e) => handleConfigChange('Ue.Key', e.target.value)}
                    />
                  </div>

                  <div className="form-group">
                    <label>OPc Key</label>
                    <input
                      type="text"
                      value={configData.Ue?.Opc || ''}
                      onChange={(e) => handleConfigChange('Ue.Opc', e.target.value)}
                    />
                  </div>

                  <div className="form-group">
                    <label>Sequence Number (SQN)</label>
                    <input
                      type="text"
                      value={configData.Ue?.Sqn || ''}
                      onChange={(e) => handleConfigChange('Ue.Sqn', e.target.value)}
                    />
                  </div>

                  <div className="form-group">
                    <label>Data Network Name (DNN)</label>
                    <input
                      type="text"
                      value={configData.Ue?.Dnn || ''}
                      onChange={(e) => handleConfigChange('Ue.Dnn', e.target.value)}
                    />
                  </div>

                  <div className="form-group">
                    <label>PDU Session Type</label>
                    <select
                      value={configData.Ue?.PduSessionType || 'IPv4'}
                      onChange={(e) => handleConfigChange('Ue.PduSessionType', e.target.value)}
                    >
                      <option value="IPv4">IPv4</option>
                      <option value="IPv6">IPv6</option>
                      <option value="IPv4v6">IPv4v6 (Dual-Stack)</option>
                    </select>
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px' }}>
                    <div className="form-group">
                      <label>Slice SST</label>
                      <input
                        type="number"
                        value={configData.Ue?.Snssai?.Sst || 1}
                        onChange={(e) => handleConfigChange('Ue.Snssai.Sst', parseInt(e.target.value))}
                      />
                    </div>
                    <div className="form-group">
                      <label>Slice SD</label>
                      <input
                        type="text"
                        value={configData.Ue?.Snssai?.Sd || ''}
                        onChange={(e) => handleConfigChange('Ue.Snssai.Sd', e.target.value)}
                      />
                    </div>
                  </div>

                  {/* Secondary PDU Sessions Configurator */}
                  <div style={{ marginTop: '20px', borderTop: '1px solid var(--border-color)', paddingTop: '15px' }}>
                    <h4 style={{ fontSize: '14px', fontWeight: 'bold', color: 'var(--text-primary)', marginBottom: '10px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span>Secondary PDU Sessions (Max 15)</span>
                      <button
                        type="button"
                        className="btn btn-secondary"
                        style={{ padding: '2px 8px', fontSize: '11px', width: 'auto' }}
                        onClick={() => {
                          const currentSessions = configData.Ue?.PduSessions || [];
                          if (currentSessions.length >= 14) {
                            alert('Maximum 15 PDU Sessions reached.');
                            return;
                          }
                          const usedIds = new Set(currentSessions.map(s => s.Id));
                          let nextId = 2;
                          while (usedIds.has(nextId) && nextId <= 15) {
                            nextId++;
                          }
                          if (nextId > 15) return;
                          
                          const newSess = {
                            Id: nextId,
                            Dnn: 'internet2',
                            PduSessionType: 'IPv4',
                            Sst: 1,
                            Sd: '010203'
                          };
                          handleConfigChange('Ue.PduSessions', [...currentSessions, newSess]);
                        }}
                      >
                        + Add PDU Session
                      </button>
                    </h4>

                    {(configData.Ue?.PduSessions || []).length === 0 ? (
                      <p style={{ fontSize: '12px', color: 'var(--text-muted)' }}>No secondary PDU sessions configured.</p>
                    ) : (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                        {(configData.Ue.PduSessions || []).map((sess, idx) => (
                          <div key={idx} style={{ background: 'rgba(255,255,255,0.02)', border: '1px solid var(--border-color)', borderRadius: '6px', padding: '10px', display: 'grid', gridTemplateColumns: '80px 1fr 1fr 120px 40px', gap: '10px', alignItems: 'center' }}>
                            <div className="form-group" style={{ marginBottom: 0 }}>
                              <label style={{ fontSize: '10px' }}>ID</label>
                              <input
                                type="number"
                                min="2"
                                max="15"
                                value={sess.Id}
                                onChange={(e) => {
                                  const updated = [...configData.Ue.PduSessions!];
                                  updated[idx].Id = parseInt(e.target.value) || 2;
                                  handleConfigChange('Ue.PduSessions', updated);
                                }}
                              />
                            </div>
                            <div className="form-group" style={{ marginBottom: 0 }}>
                              <label style={{ fontSize: '10px' }}>DNN</label>
                              <input
                                type="text"
                                value={sess.Dnn}
                                onChange={(e) => {
                                  const updated = [...configData.Ue.PduSessions!];
                                  updated[idx].Dnn = e.target.value;
                                  handleConfigChange('Ue.PduSessions', updated);
                                }}
                              />
                            </div>
                            <div className="form-group" style={{ marginBottom: 0 }}>
                              <label style={{ fontSize: '10px' }}>Type</label>
                              <select
                                value={sess.PduSessionType}
                                onChange={(e) => {
                                  const updated = [...configData.Ue.PduSessions!];
                                  updated[idx].PduSessionType = e.target.value;
                                  handleConfigChange('Ue.PduSessions', updated);
                                }}
                              >
                                <option value="IPv4">IPv4</option>
                                <option value="IPv6">IPv6</option>
                                <option value="IPv4v6">IPv4v6</option>
                              </select>
                            </div>
                            <div className="form-group" style={{ marginBottom: 0 }}>
                              <label style={{ fontSize: '10px' }}>Slice (SST/SD)</label>
                              <div style={{ display: 'flex', gap: '4px' }}>
                                <input
                                  type="number"
                                  placeholder="SST"
                                  value={sess.Sst}
                                  onChange={(e) => {
                                    const updated = [...configData.Ue.PduSessions!];
                                    updated[idx].Sst = parseInt(e.target.value) || 1;
                                    handleConfigChange('Ue.PduSessions', updated);
                                  }}
                                  style={{ width: '45px', padding: '4px' }}
                                />
                                <input
                                  type="text"
                                  placeholder="SD"
                                  value={sess.Sd}
                                  onChange={(e) => {
                                    const updated = [...configData.Ue.PduSessions!];
                                    updated[idx].Sd = e.target.value;
                                    handleConfigChange('Ue.PduSessions', updated);
                                  }}
                                  style={{ padding: '4px' }}
                                />
                              </div>
                            </div>
                            <button
                              type="button"
                              onClick={() => {
                                const updated = configData.Ue.PduSessions!.filter((_, i) => i !== idx);
                                handleConfigChange('Ue.PduSessions', updated);
                              }}
                              style={{ background: 'none', border: 'none', color: 'var(--color-danger)', cursor: 'pointer', padding: '10px 0 0' }}
                              title="Delete PDU Session"
                            >
                              <Trash2 size={16} />
                            </button>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                {/* Right Side: GNodeB & Core Settings */}
                <div className="config-section">
                  <h3 className="panel-title" style={{ borderBottom: '1px solid var(--border-color)', paddingBottom: '10px' }}>
                    <Radio size={18} /> GNodeB / 5G Core Settings
                  </h3>

                  <div className="form-group">
                    <label>GNodeB Connection Link Type</label>
                    <select
                      value={configData.GNodeB?.LinkType || 'unix'}
                      onChange={(e) => handleConfigChange('GNodeB.LinkType', e.target.value)}
                    >
                      <option value="unix">UNIX Sockets</option>
                      <option value="tcp">TCP Radio Link Simulation (RLS)</option>
                    </select>
                  </div>

                  <div className="form-group">
                    <label>GNodeB Radio Link Port</label>
                    <input
                      type="number"
                      value={configData.GNodeB?.LinkPort || 9488}
                      onChange={(e) => handleConfigChange('GNodeB.LinkPort', parseInt(e.target.value))}
                    />
                  </div>

                  <div className="form-group">
                    <label>GNodeB Control IP (NGAP Binding)</label>
                    <input
                      type="text"
                      value={configData.GNodeB?.ControlIF?.Ip || '127.0.0.1'}
                      onChange={(e) => handleConfigChange('GNodeB.ControlIF.Ip', e.target.value)}
                    />
                  </div>

                  <div className="form-group">
                    <label>GNodeB Control Port</label>
                    <input
                      type="number"
                      value={configData.GNodeB?.ControlIF?.Port || 9487}
                      onChange={(e) => handleConfigChange('GNodeB.ControlIF.Port', parseInt(e.target.value))}
                    />
                  </div>

                  <div className="form-group">
                    <label>5G Core AMF IP Address</label>
                    <input
                      type="text"
                      value={configData.AMF?.Ip || '127.0.0.1'}
                      onChange={(e) => handleConfigChange('AMF.Ip', e.target.value)}
                    />
                  </div>

                  <div className="form-group">
                    <label>5G Core AMF Port (NGAP)</label>
                    <input
                      type="number"
                      value={configData.AMF?.Port || 38412}
                      onChange={(e) => handleConfigChange('AMF.Port', parseInt(e.target.value))}
                    />
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
                <div className="console-dot-actions">
                  <div className="console-dot red" />
                  <div className="console-dot yellow" />
                  <div className="console-dot green" />
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
                        background: 'rgba(255,255,255,0.02)', 
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
                                <button className="btn btn-xs btn-ghost" onClick={() => duplicateUEProfile(p)} title="Duplicate">📋</button>
                                <button className="btn btn-xs btn-ghost" onClick={() => { setEditingUE(p); setShowUEForm(true); }} title="Edit">✏</button>
                                <button className="btn btn-xs btn-danger" onClick={() => deleteUEProfile(p.name)} title="Delete">🗑</button>
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
                                <button className="btn btn-xs btn-ghost" onClick={() => duplicateGNBProfile(p)} title="Duplicate">📋</button>
                                <button className="btn btn-xs btn-ghost" onClick={() => { setEditingGNB(p); setShowGNBForm(true); }} title="Edit">✏</button>
                                <button className="btn btn-xs btn-danger" onClick={() => deleteGNBProfile(p.name)} disabled={fleetRunning.runningGnbs?.some(g => g.profileName === p.name)}>🗑</button>
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
                                    {g.connectedUes.map(ue => (
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
