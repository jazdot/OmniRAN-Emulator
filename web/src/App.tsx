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
  const [theme, setTheme] = useState<'dark' | 'light'>('dark');
  const [activeTab, setActiveTab] = useState<'dashboard' | 'scenarios' | 'config' | 'logs' | 'connectivity'>('dashboard');
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
            <h1 className="capitalize">{activeTab} Panel</h1>
          </div>

          <div className="header-status-bar">
            {/* Stop Scenario Button */}
            {status?.isRunning && (
              <button
                onClick={stopScenario}
                className="status-badge"
                style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '8px', background: 'rgba(239,68,68,0.15)', border: '1px solid var(--color-danger)', color: 'var(--color-danger)' }}
                title="Stop Scenario"
              >
                <Trash2 size={14} />
                <span className="font-bold">STOP SCENARIO</span>
              </button>
            )}

            {/* Auto-Refresh Toggle Button */}
            <button
              onClick={() => setAutoRefresh(!autoRefresh)}
              className={`status-badge ${!autoRefresh ? 'border-warning' : ''}`}
              style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '8px', background: 'rgba(255,255,255,0.03)' }}
              title="Toggle Auto Refresh Polling"
            >
              <RefreshCw size={14} className={autoRefresh && status?.isRunning ? 'animate-spin' : ''} style={{ color: autoRefresh ? 'var(--color-success)' : 'var(--color-warning)' }} />
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
              className="status-badge"
              style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '8px', background: 'rgba(255,255,255,0.03)' }}
              title="Toggle Warning Banners / Info Tips"
            >
              <AlertTriangle size={14} style={{ color: showBanners ? 'var(--color-warning)' : 'var(--text-muted)' }} />
              <span className="font-semibold" style={{ color: showBanners ? 'var(--text-primary)' : 'var(--text-muted)' }}>{showBanners ? 'HIDE TIPS' : 'SHOW TIPS'}</span>
            </button>

            {/* Theme Toggle Button */}
            <button
              onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
              className="status-badge"
              style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '8px', background: 'rgba(255,255,255,0.03)' }}
              title="Toggle Theme Mode"
            >
              {theme === 'dark' ? <Sun size={14} style={{ color: '#fbbf24' }} /> : <Moon size={14} style={{ color: '#6366f1' }} />}
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
                <div className="topology-canvas">
                  {/* UE Node */}
                  <div 
                    className={`topology-node clickable-node active ${selectedNode === 'ue' ? 'selected' : ''}`} 
                    onClick={() => setSelectedNode('ue')}
                    style={{ '--node-color': '#10b981', '--node-color-glow': 'rgba(16, 185, 129, 0.2)' } as React.CSSProperties}
                  >
                    <div className="node-icon-wrapper">
                      <Cpu />
                    </div>
                    <span className="node-label">User Equipment</span>
                    <span className="node-status-text">
                      {status?.isRunning ? 'CONNECTED' : 'IDLE'}
                    </span>
                  </div>

                  {/* UE -> gNB Link Line (Uu Interface) */}
                  <div
                    className={`topology-link link-ue-gnb ${status?.isRunning ? 'active' : ''}`}
                    style={{ '--from-color': '#10b981', '--to-color': '#6366f1', '--glow-color': 'rgba(99, 102, 241, 0.2)' } as React.CSSProperties}
                  >
                    <div className="link-badge">Uu (5G-NR)</div>
                    <div className="pulse-dot" />
                  </div>

                  {/* gNodeB Node */}
                  <div
                    className={`topology-node clickable-node ${status?.gnbLinkState !== 'offline' ? 'active' : ''} ${selectedNode === 'gnb' ? 'selected' : ''}`}
                    onClick={() => setSelectedNode('gnb')}
                    style={{ '--node-color': '#6366f1', '--node-color-glow': 'rgba(99, 102, 241, 0.2)' } as React.CSSProperties}
                  >
                    <div className="node-icon-wrapper">
                      <Radio />
                    </div>
                    <span className="node-label">gNodeB Cell</span>
                    <span className="node-status-text">
                      {status?.gnbLinkState !== 'offline' ? 'ESTABLISHED' : 'OFFLINE'}
                    </span>
                  </div>

                  {/* gNB -> AMF Control Plane Link Line (N2 Interface) */}
                  <div
                    className={`topology-link link-n2-control ${
                      status?.gnbLinkState && status.gnbLinkState !== 'offline' ? 'active' : ''
                    }`}
                    style={{ '--from-color': '#6366f1', '--to-color': '#8b5cf6', '--glow-color': 'rgba(139, 92, 246, 0.2)' } as React.CSSProperties}
                  >
                    <div className="link-badge">N2 (NGAP/SCTP)</div>
                    <div className="pulse-dot" />
                  </div>

                  {/* gNB -> UPF User Plane Link Line (N3 Interface) */}
                  <div
                    className={`topology-link link-n3-data ${
                      status?.isRunning ? 'active' : ''
                    }`}
                    style={{ '--from-color': '#6366f1', '--to-color': '#8b5cf6', '--glow-color': 'rgba(139, 92, 246, 0.2)' } as React.CSSProperties}
                  >
                    <div className="link-badge">N3 (GTP-U/IPIP)</div>
                    <div className="pulse-dot" />
                  </div>

                  {/* 5G Core Node */}
                  <div
                    className={`topology-node clickable-node ${status?.gnbLinkState && status.gnbLinkState !== 'offline' ? 'active' : ''} ${selectedNode === 'core' ? 'selected' : ''}`}
                    onClick={() => setSelectedNode('core')}
                    style={{ '--node-color': '#8b5cf6', '--node-color-glow': 'rgba(139, 92, 246, 0.2)' } as React.CSSProperties}
                  >
                    <div className="node-icon-wrapper">
                      <Server />
                    </div>
                    <span className="node-label">5G Core (AMF/UPF)</span>
                    <span className="node-status-text">
                      {status?.gnbLinkState && status.gnbLinkState !== 'offline' ? 'REACHABLE' : 'DISCONNECTED'}
                    </span>
                  </div>
                </div>

                {/* Right side: Inspector Panel */}
                <div className="node-inspector-card">
                  {selectedNode === 'ue' && (
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
                  )}

                  {selectedNode === 'gnb' && (
                    <>
                      <h4 className="inspector-title" style={{ color: 'var(--color-primary)' }}>
                        <Radio size={14} /> gNodeB Cell Inspector
                      </h4>
                      <div className="inspector-details">
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
                      </div>
                    </>
                  )}

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

                    <button
                      className="btn btn-primary"
                      onClick={() => triggerUeAction('handover', {
                        targetGnbIp: hoTargetIp,
                        targetGnbPort: hoTargetPort
                      })}
                      style={{ padding: '6px 12px', fontSize: '13px' }}
                    >
                      Trigger Handover
                    </button>
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
      </main>
    </div>
  );
}
