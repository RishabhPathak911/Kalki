// dashboard/src/components/CinematicTour.tsx
import React, { useState, useEffect, useRef, useCallback } from 'react';
import '../cinematic-demo.css';

export interface CinematicTourProps {
  activeTab: string;
  onTabChange: (tab: string) => void;
  onClose: () => void;
  onExitToLogin?: () => void;
}

interface TourStep {
  tab: string;
  targetSelector: string;
  fallbackSelector: string;
  title: string;
  description: string;
  autoOpenDetails?: boolean;
  autoOpenTechnical?: boolean;
  autoOpenAuditDetails?: boolean;
}

const TOUR_STEPS: TourStep[] = [
  // 1. FLEET OVERVIEW: OpsGraphic (Collector Node & Waveform)
  {
    tab: 'Fleet Overview',
    targetSelector: '.ops-graphic',
    fallbackSelector: '.ops-hero',
    title: 'Tier-1 Edge Telemetry Node',
    description: 'Hardware kernel polling in ap-south-1. The oscilloscope renders continuous OS sampling at 1Hz, while the 4 streaming channels pipeline scrubbed metrics with mathematical zero raw payload egress.'
  },
  // 2. FLEET OVERVIEW: Edge Node Cards & Heartbeats
  {
    tab: 'Fleet Overview',
    targetSelector: '.fleet-list .node-card:first-child',
    fallbackSelector: '.fleet-inventory',
    title: 'Edge Fleet Inventory & Health Status',
    description: 'Continuous mTLS heartbeats arrive every 5s. Notice the sub-millisecond probe latency, CPU 19.4%, and memory allocation. If anomalous latency spikes past threshold gates, state transitions trigger autonomously.'
  },
  // 3. FLEET OVERVIEW: Real-Time Multi-Signal Graphs
  {
    tab: 'Fleet Overview',
    targetSelector: '.database-graphs-section, .metrics-grid, .recharts-responsive-container',
    fallbackSelector: '.metrics-grid',
    title: 'Time-Series Waveforms & Baseline Tracking',
    description: 'Dynamic time-series visualizations track p95 and average trends for CPU, Memory, Throughput, and TCP failure ratios. Nadi correlates baseline variances locally before rollups leave the edge.'
  },
  // 4. TRANSITION: Moving pointer to click Incidents tab
  {
    tab: 'Incidents',
    targetSelector: 'nav.ops-views button:nth-child(2)',
    fallbackSelector: '.ops-views',
    title: 'Navigating to Autonomous Detection',
    description: 'Gliding to the Incidents tab. Let us inspect the active security breach detected by Nadi in ap-south-1.'
  },
  // 5. INCIDENTS: Summary Card & Detection Speed
  {
    tab: 'Incidents',
    targetSelector: 'details.incident summary',
    fallbackSelector: '.incident',
    title: 'Sub-Second Breach Detection (1,280ms)',
    description: 'Nadi flagged a synthetic brute-force authentication burst within 1.28 seconds. Detected on host_edge_01 with 96% heuristic confidence score.'
  },
  // 6. INCIDENTS: Opening Details & Multi-Signal Evidence Story
  {
    tab: 'Incidents',
    targetSelector: 'details.incident .incident-story',
    fallbackSelector: 'details.incident',
    autoOpenDetails: true,
    title: 'Multi-Signal Evidence Story',
    description: 'Examining observed attack telemetry: auth_failures_per_s surged to 12.0/s against a 0.1/s baseline (119.0 deviation). Correlated simultaneously with elevated tcp_fail_ratio.'
  },
  // 7. INCIDENTS: Technical Heuristic Grid & Metric Table
  {
    tab: 'Incidents',
    targetSelector: 'details.incident .incident-technical',
    fallbackSelector: 'details.incident',
    autoOpenTechnical: true,
    title: 'Root-Cause Analysis & Signal Attribution',
    description: 'Nadi sliding-window counter exceeded 6 failures locally without external cloud lookups, bounding latency to 1,280ms. The table details baseline deviations, directional shifts, and signal weights.'
  },
  // 8. INCIDENTS: Safe Remediation & Pre-Flight Verification
  {
    tab: 'Incidents',
    targetSelector: 'details.incident .remediation',
    fallbackSelector: 'details.incident',
    title: 'Pre-Flight Verified Safe Remediation',
    description: 'Enforcing strict safety gates before remediation: Reversible ✓, Valid target ✓, Tenant authorized ✓. The proposed socket throttle mitigates the attack with zero dropped customer packets.'
  },
  // 9. AUDIT LOG: Cryptographic SHA-256 Block Chain
  {
    tab: 'Audit Log',
    targetSelector: '.audit-log-view, .audit-log-table',
    fallbackSelector: '.audit-verification-card, main.content',
    autoOpenAuditDetails: true,
    title: 'Cryptographic SHA-256 Audit Trail',
    description: 'Every batch ingestion, anomaly detection, and remediation approval is permanently sealed into an immutable SHA-256 hash-linked chain. Full compliance and auditability guaranteed.'
  }
];

interface LogEntry {
  time: string;
  tag: string;
  text: string;
  highlight?: 'sys' | 'prana' | 'kavach' | 'nadi' | 'setu' | 'ack' | 'cmd';
}

const COLLECTOR_LOGS: LogEntry[] = [
  { time: '12:00:01.050', tag: '$', text: 'kalki-collector --tenant=tenant_alpha --region=ap-south-1 --buffer-dir=/var/log/kalki/wal', highlight: 'cmd' },
  { time: '12:00:01.120', tag: '[SYSTEM]', text: 'KALKI Tier-1 Edge Daemon booting on Linux x86_64...', highlight: 'sys' },
  { time: '12:00:01.280', tag: '[PRANA]', text: 'OS kernel telemetry sampler initialized. Polling metrics (CPU, RAM, Disk, Sockets @ 1Hz)', highlight: 'prana' },
  { time: '12:00:01.460', tag: '[KAVACH]', text: 'In-flight privacy membrane active. 18 redaction scanners loaded (Strict Zero-Egress mode)', highlight: 'kavach' },
  { time: '12:00:01.690', tag: '[NADI]', text: 'Local stream aggregator started. 5-second tumbling baseline window armed.', highlight: 'nadi' },
  { time: '12:00:01.920', tag: '[SETU]', text: 'Establishing mTLS channel to regional receiver https://ap-south-1.kalki.internal:8443...', highlight: 'setu' },
  { time: '12:00:02.140', tag: '[SETU]', text: 'mTLS Handshake verified. Mutual TLS session active (ECDHE-ECDSA-AES256-GCM).', highlight: 'setu' },
  { time: '12:00:02.410', tag: '[PRANA]', text: 'Sample #1 collected: cpu=19.4% mem=49.8% disk=38.2% latency=6.4ms tcp_fail=0.04%', highlight: 'prana' },
  { time: '12:00:02.720', tag: '[KAVACH]', text: '⚠️ SENSITIVE CREDENTIAL INTERCEPTED IN TELEMETRY STREAM:', highlight: 'kavach' },
  { time: '12:00:02.820', tag: '  » Raw:', text: '{"tenant":"tenant_alpha","token":"sk_live_9f81a7bc20421e..."}', highlight: 'kavach' },
  { time: '12:00:02.920', tag: '  » Scrubbed:', text: '{"tenant":"tenant_alpha","token":"[REDACTED_API_KEY]"}', highlight: 'kavach' },
  { time: '12:00:03.020', tag: '  » Proof:', text: '0 raw secret bytes left local device boundary (Fail-Closed verified)', highlight: 'kavach' },
  { time: '12:00:03.250', tag: '[SETU]', text: 'Batch #1041 serialized (60 samples, 0 raw tokens). Payload SHA-256: 7f8a3c9e12bd4f810aa839...', highlight: 'setu' },
  { time: '12:00:03.500', tag: '[WAL]', text: 'fsync to local disk buffer (/var/log/kalki/wal/batch-1041.wal). Depth: 0.', highlight: 'sys' },
  { time: '12:00:03.750', tag: '[SETU EGRESS]', text: 'Batch #1041 dispatched over mTLS to ap-south-1 receiver.', highlight: 'setu' },
  { time: '12:00:04.050', tag: '[RECEIVER ACK]', text: 'Status: 200 OK | Seq: 1041 | Ack-SHA256: 7f8a3c9e12bd4f810aa839... (MATCH)', highlight: 'ack' },
  { time: '12:00:04.250', tag: '[VERIFICATION]', text: '✅ Local SHA-256 matches Receiver SHA-256! Cryptographic delivery verified.', highlight: 'ack' }
];

export default function CinematicTour({ activeTab, onTabChange, onClose, onExitToLogin }: CinematicTourProps) {
  const [phase, setPhase] = useState<'terminal' | 'tour' | 'victory'>('terminal');
  const [terminalMinimized, setTerminalMinimized] = useState(false);
  const [logCount, setLogCount] = useState(0);
  const [currentStepIndex, setCurrentStepIndex] = useState(0);
  const [isPaused, setIsPaused] = useState(false);
  const [cursorPos, setCursorPos] = useState<{ x: number; y: number; visible: boolean; label?: string }>({
    x: window.innerWidth / 2,
    y: window.innerHeight / 2,
    visible: false
  });
  const [targetRect, setTargetRect] = useState<DOMRect | null>(null);
  const terminalEndRef = useRef<HTMLDivElement>(null);
  const stepTimerRef = useRef<NodeJS.Timeout | null>(null);
  const focusedElRef = useRef<HTMLElement | null>(null);

  const visibleLogs = COLLECTOR_LOGS.slice(0, logCount);

  // Clean up focused element highlights
  const clearFocusedElement = useCallback(() => {
    if (focusedElRef.current) {
      focusedElRef.current.classList.remove('cinematic-focused-element');
      focusedElRef.current = null;
    }
    document.querySelectorAll('.cinematic-focused-element').forEach(n => n.classList.remove('cinematic-focused-element'));
  }, []);

  // Update target rect on window scroll or resize so focus stays locked
  useEffect(() => {
    if (phase !== 'tour') return;
    function handleScrollOrResize() {
      if (focusedElRef.current) {
        setTargetRect(focusedElRef.current.getBoundingClientRect());
      }
    }
    window.addEventListener('scroll', handleScrollOrResize, { passive: true });
    window.addEventListener('resize', handleScrollOrResize, { passive: true });
    return () => {
      window.removeEventListener('scroll', handleScrollOrResize);
      window.removeEventListener('resize', handleScrollOrResize);
    };
  }, [phase]);

  // 1. Run Terminal Prototype Logs
  useEffect(() => {
    if (phase !== 'terminal') return;

    const interval = setInterval(() => {
      setLogCount(c => {
        if (c < COLLECTOR_LOGS.length) {
          return c + 1;
        } else {
          clearInterval(interval);
          setTimeout(() => {
            handleMinimizeTerminal();
          }, 1200);
          return c;
        }
      });
    }, 360);

    return () => clearInterval(interval);
  }, [phase]);

  // Auto-scroll terminal logs
  useEffect(() => {
    if (terminalEndRef.current) {
      terminalEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logCount]);

  function handleMinimizeTerminal() {
    setTerminalMinimized(true);
    setPhase('tour');
    setCurrentStepIndex(0);
  }

  function handleFastForwardTerminal() {
    setLogCount(COLLECTOR_LOGS.length);
    setTimeout(() => {
      handleMinimizeTerminal();
    }, 400);
  }

  // 2. Dashboard Tour Step Execution
  const currentStep = TOUR_STEPS[currentStepIndex];

  const updateTargetPosition = useCallback(() => {
    if (phase !== 'tour' || !currentStep) return;

    // Switch tab if step requires it
    if (activeTab !== currentStep.tab) {
      onTabChange(currentStep.tab);
    }

    const applyFocus = () => {
      // Programmatically open incident details if step asks for it
      if (currentStep.autoOpenDetails || currentStep.autoOpenTechnical) {
        const detailsEl = document.querySelector('details.incident') as HTMLDetailsElement;
        if (detailsEl) {
          detailsEl.open = true;
        }
      }
      if (currentStep.autoOpenTechnical) {
        const techEl = document.querySelector('details.incident-technical') as HTMLDetailsElement;
        if (techEl) {
          techEl.open = true;
        }
      }

      // Programmatically open audit log hash details if step is Audit Log
      if (currentStep.tab === 'Audit Log' || currentStep.autoOpenAuditDetails) {
        const auditDetails = document.querySelectorAll('main.content table details, .audit-log-table details, .audit-log-view details');
        auditDetails.forEach(d => {
          (d as HTMLDetailsElement).open = true;
        });
      }

      let el: HTMLElement | null = document.querySelector(currentStep.targetSelector);
      if (!el && currentStep.fallbackSelector) {
        el = document.querySelector(currentStep.fallbackSelector);
      }

      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' });
        
        // Remove focus class from previous element and assign to current element
        clearFocusedElement();
        el.classList.add('cinematic-focused-element');
        focusedElRef.current = el;

        const rect = el.getBoundingClientRect();
        setTargetRect(rect);

        // Move virtual needle pointer smoothly to target
        setCursorPos({
          x: Math.max(20, rect.left + 30),
          y: Math.max(20, rect.top + 20),
          visible: true,
          label: currentStep.title
        });
        return true;
      } else {
        clearFocusedElement();
        setTargetRect(null);
        setCursorPos(prev => ({ ...prev, visible: false }));
        return false;
      }
    };

    // Staggered attempts to guarantee smooth lock across tab switches and data fetching
    const t1 = setTimeout(applyFocus, 100);
    const t2 = setTimeout(applyFocus, 320);
    const t3 = setTimeout(applyFocus, 600);

    return () => {
      clearTimeout(t1);
      clearTimeout(t2);
      clearTimeout(t3);
    };
  }, [phase, currentStep, activeTab, onTabChange, clearFocusedElement]);

  useEffect(() => {
    if (phase !== 'tour') return;
    updateTargetPosition();

    // Pacing: give viewers 9 seconds per step (unless paused)
    if (!isPaused) {
      if (stepTimerRef.current) clearTimeout(stepTimerRef.current);
      stepTimerRef.current = setTimeout(() => {
        handleNextStep();
      }, 9000);
    }

    return () => {
      if (stepTimerRef.current) clearTimeout(stepTimerRef.current);
    };
  }, [phase, currentStepIndex, isPaused, updateTargetPosition]);

  function handleNextStep() {
    if (currentStepIndex < TOUR_STEPS.length - 1) {
      setCurrentStepIndex(i => i + 1);
    } else {
      // Completed all steps -> Victory Phase
      setPhase('victory');
      setCursorPos(prev => ({ ...prev, visible: false }));
    }
  }

  function handlePrevStep() {
    if (currentStepIndex > 0) {
      setCurrentStepIndex(i => i - 1);
    }
  }

  function handleFinishDemo() {
    clearFocusedElement();
    onTabChange('Fleet Overview');
    onClose();
  }

  function handleReadyToUse() {
    clearFocusedElement();
    if (onExitToLogin) {
      onExitToLogin();
    } else {
      handleFinishDemo();
    }
  }

  // Clean up on component unmount
  useEffect(() => {
    return () => {
      clearFocusedElement();
    };
  }, [clearFocusedElement]);

  // Keyboard navigation
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        handleFinishDemo();
      } else if (e.key === 'ArrowRight' && phase === 'tour') {
        handleNextStep();
      } else if (e.key === 'ArrowLeft' && phase === 'tour') {
        handlePrevStep();
      } else if (e.key === ' ') {
        e.preventDefault();
        setIsPaused(p => !p);
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [phase, currentStepIndex]);

  return (
    <>
      {/* Precision Needle Cursor Pointer */}
      {cursorPos.visible && (
        <div
          className="cinematic-virtual-cursor"
          style={{
            transform: `translate3d(${cursorPos.x}px, ${cursorPos.y}px, 0)`
          }}
        >
          <svg className="cursor-arrow-body" viewBox="0 0 24 24" fill="none">
            <path
              d="M3 3l7 18 3-7 7-3L3 3z"
              fill="#38bdf8"
              stroke="#ffffff"
              strokeWidth="1.5"
              strokeLinejoin="round"
            />
          </svg>
          <div className="cursor-click-ring" />
          {cursorPos.label && <div className="cursor-tag">{cursorPos.label}</div>}
        </div>
      )}

      {/* PHASE 1: Terminal Simulation Modal */}
      {phase === 'terminal' && (
        <div className="cinematic-terminal-backdrop">
          <div className="cinematic-terminal-window">
            <div className="cinematic-terminal-header">
              <div className="terminal-dots">
                <span className="terminal-dot dot-red" />
                <span className="terminal-dot dot-yellow" />
                <span className="terminal-dot dot-green" />
              </div>
              <div className="terminal-title">
                <span>kalki-collector-daemon · edge-ap-south-1</span>
                <span className="terminal-badge">LIVE PROTOTYPE</span>
              </div>
              <div className="terminal-controls-top">
                <button
                  type="button"
                  className="terminal-ctrl-btn"
                  onClick={handleFastForwardTerminal}
                >
                  ⚡ Fast Forward
                </button>
                <button
                  type="button"
                  className="terminal-ctrl-btn"
                  onClick={handleFinishDemo}
                >
                  ✕ Exit
                </button>
              </div>
            </div>

            <div className="cinematic-terminal-body">
              {visibleLogs.map((entry, idx) => {
                if (!entry) return null;
                return (
                  <div key={idx} className="terminal-line">
                    <span className="term-time">{entry.time}</span>
                    {entry.highlight === 'cmd' ? (
                      <>
                        <span className="term-prompt">{entry.tag}</span>
                        <span className="term-cmd">{entry.text}</span>
                      </>
                    ) : entry.highlight === 'sys' ? (
                      <>
                        <span className="term-sys">{entry.tag}</span> {entry.text}
                      </>
                    ) : entry.highlight === 'prana' ? (
                      <>
                        <span className="term-prana">{entry.tag}</span> {entry.text}
                      </>
                    ) : entry.highlight === 'kavach' ? (
                      <>
                        <span className="term-kavach">{entry.tag}</span> {entry.text}
                      </>
                    ) : entry.highlight === 'nadi' ? (
                      <>
                        <span className="term-nadi">{entry.tag}</span> {entry.text}
                      </>
                    ) : entry.highlight === 'setu' ? (
                      <>
                        <span className="term-setu">{entry.tag}</span> {entry.text}
                      </>
                    ) : entry.highlight === 'ack' ? (
                      <>
                        <span className="term-ack">{entry.tag}</span> {entry.text}
                      </>
                    ) : (
                      <span>{entry.text}</span>
                    )}
                  </div>
                );
              })}
              <div ref={terminalEndRef} />
            </div>

            <div className="cinematic-terminal-footer">
              <div className="terminal-progress-info">
                <span>Streaming Edge Telemetry Pipeline...</span>
                <div className="terminal-mini-bar">
                  <div
                    className="terminal-mini-fill"
                    style={{
                      width: `${(visibleLogs.length / COLLECTOR_LOGS.length) * 100}%`
                    }}
                  />
                </div>
                <span>{visibleLogs.length} / {COLLECTOR_LOGS.length} events</span>
              </div>
              <div className="terminal-actions-btn-group">
                <button
                  type="button"
                  className="terminal-action-btn terminal-action-primary"
                  onClick={handleMinimizeTerminal}
                >
                  <span>Enter Dashboard Tour</span>
                  <span>→</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* PHASE 2: Minimized Terminal Dock Widget */}
      {terminalMinimized && (
        <div
          className="cinematic-dock-widget"
          onClick={() => {
            setPhase('terminal');
            setTerminalMinimized(false);
          }}
          title="Click to view Collector Terminal logs"
        >
          <span className="dock-icon">📟</span>
          <div className="dock-text">
            <strong>Tier-1 Collector</strong>: Active
          </div>
          <span className="dock-badge">ACK 200 OK</span>
        </div>
      )}

      {/* PHASE 3: Lens Rack-Focus & Precision HUD Card */}
      {phase === 'tour' && currentStep && (
        <>
          {/* Compute Optical Cutout over the Target Element so it is 100% UNBLURRED and sharp */}
          {(() => {
            const pad = 8;
            const x1 = targetRect ? Math.max(0, Math.round(targetRect.left - pad)) : 0;
            const y1 = targetRect ? Math.max(0, Math.round(targetRect.top - pad)) : 0;
            const x2 = targetRect ? Math.min(window.innerWidth, Math.round(targetRect.right + pad)) : 0;
            const y2 = targetRect ? Math.min(window.innerHeight, Math.round(targetRect.bottom + pad)) : 0;
            const hasCutout = targetRect && x2 > x1 && y2 > y1;
            const clipPolygon = hasCutout
              ? `polygon(evenodd, 0px 0px, 100vw 0px, 100vw 100vh, 0px 100vh, 0px 0px, ${x1}px ${y1}px, ${x2}px ${y1}px, ${x2}px ${y2}px, ${x1}px ${y2}px, ${x1}px ${y1}px)`
              : undefined;

            return (
              <>
                {/* Optical Lens Blur Scrim: Soft optical blur over background, but clipped out over target! */}
                <div
                  className="cinematic-lens-blur-scrim"
                  style={{
                    clipPath: clipPolygon,
                    WebkitClipPath: clipPolygon
                  }}
                />

                {/* Precision Bounding Reticle with Viewfinder Crosshairs */}
                {hasCutout && (
                  <div
                    className="cinematic-precision-reticle"
                    style={{
                      top: `${y1}px`,
                      left: `${x1}px`,
                      width: `${x2 - x1}px`,
                      height: `${y2 - y1}px`
                    }}
                  />
                )}
              </>
            );
          })()}

          {/* Editorial HUD Narration Card */}
          <div
            className="cinematic-hud-card"
            style={{
              bottom: '28px',
              left: '28px'
            }}
          >
            <div className="hud-step-pill">
              <span>● STEP {currentStepIndex + 1} OF {TOUR_STEPS.length}</span>
              <span>· {currentStep.tab}</span>
            </div>
            <h4 className="hud-title">{currentStep.title}</h4>
            <p className="hud-desc">{currentStep.description}</p>

            <div className="hud-progress-row">
              {TOUR_STEPS.map((_, i) => (
                <div
                  key={i}
                  className={`hud-progress-pip ${i <= currentStepIndex ? 'done' : ''}`}
                />
              ))}
            </div>

            <div className="hud-actions">
              <div className="hud-controls-left">
                <button
                  type="button"
                  className="hud-btn"
                  onClick={handlePrevStep}
                  disabled={currentStepIndex === 0}
                  style={{ opacity: currentStepIndex === 0 ? 0.35 : 1 }}
                >
                  ← Prev
                </button>
                <button
                  type="button"
                  className="hud-btn"
                  onClick={() => setIsPaused(p => !p)}
                >
                  {isPaused ? '▶ Resume' : '⏸ Pause'}
                </button>
                <button
                  type="button"
                  className="hud-btn hud-btn-primary"
                  onClick={handleNextStep}
                >
                  {currentStepIndex === TOUR_STEPS.length - 1 ? 'Finish Tour →' : 'Next Step →'}
                </button>
              </div>

              <button
                type="button"
                className="hud-btn-ghost"
                onClick={handleFinishDemo}
              >
                Exit Demo
              </button>
            </div>
          </div>
        </>
      )}

      {/* PHASE 4: Victory Modal & Motivating Statement */}
      {phase === 'victory' && (
        <div className="cinematic-victory-backdrop">
          <div className="cinematic-victory-card">
            <div className="victory-icon-wrap">
              <span>✨</span>
            </div>
            <div className="victory-pill">DEMO COMPLETED · ALL SYSTEMS VERIFIED</div>
            <h2 className="victory-title">The Power of Privacy-First AIOps is in Your Hands</h2>
            <p className="victory-statement">
              You have witnessed a complete, enterprise-grade distributed telemetry pipeline in action:
              from <strong>edge OS sampling</strong> and <strong>in-flight secret scrubbing</strong>, to{' '}
              <strong>sub-second autonomous detection</strong> and <strong>cryptographic audit chains</strong>.
              High performance with zero raw data leakage.
            </p>

            <div className="victory-features-row">
              <span className="victory-feature-item">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><polyline points="20 6 9 17 4 12"/></svg>
                Edge OS Polling
              </span>
              <span className="victory-feature-item">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><polyline points="20 6 9 17 4 12"/></svg>
                Zero-Knowledge Scrubbing
              </span>
              <span className="victory-feature-item">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><polyline points="20 6 9 17 4 12"/></svg>
                1,280ms Anomaly Detection
              </span>
              <span className="victory-feature-item">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><polyline points="20 6 9 17 4 12"/></svg>
                SHA-256 Audit Chain
              </span>
            </div>

            <button
              type="button"
              className="ready-to-go-btn"
              onClick={handleReadyToUse}
            >
              <span>⚡ Ready to Use</span>
              <span aria-hidden="true">→</span>
            </button>
          </div>
        </div>
      )}
    </>
  );
}
