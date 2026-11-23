import React from 'react';

export interface OpsGraphicProps {
  tab: string;
}

export const OpsGraphic = React.memo(function OpsGraphic({ tab }: OpsGraphicProps) {
  switch (tab) {
    case 'Incidents':
      return <IncidentsGraphic />;
    case 'Egress Proof':
      return <EgressProofGraphic />;
    case 'Audit Log':
      return <AuditLogGraphic />;
    case 'Demo Lab':
      return <DemoLabGraphic />;
    case 'Validation':
      return <ValidationGraphic />;
    case 'Limitations':
      return <LimitationsGraphic />;
    case 'Fleet Overview':
    default:
      return <FleetOverviewGraphic />;
  }
});

/**
 * 1. Fleet Overview: Collector Node, Live Telemetry Oscilloscope, Waveform & Streaming Tracks
 */
function FleetOverviewGraphic() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      className="ops-graphic ops-graphic-animated"
      viewBox="0 0 500 300"
      aria-label="Fleet Overview Telemetry Processor"
    >
      <defs>
        <filter id="glow-fleet" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="2" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* Collector Node Box */}
      <rect x="45" y="18" width="170" height="90" rx="4" />
      <text x="56" y="32" className="ops-svg-label">COLLECTOR NODE · AP-SOUTH-1</text>
      
      {/* Microchip Core */}
      <rect x="62" y="42" width="26" height="26" rx="1" />
      <path d="M70 36v6M80 36v6M70 68v6M80 68v6M56 50h6M56 60h6M88 50h6M88 60h6" />
      <circle cx="75" cy="55" r="3.5" className="ops-pulse" fill="white" />
      
      {/* Activity indicator bars inside chip box */}
      <path d="M106 48h64M106 58h52M106 68h64M106 80h32" className="ops-chip-lines" />
      <text x="106" y="94" className="ops-svg-micro">eBPF TELEMETRY · 60s WAL</text>

      {/* Trace to Oscilloscope Beacon */}
      <path
        className="ops-ecg-path"
        d="M215 63h60l16-48 22 48 22 24 24-27 18 27h60"
        style={{ strokeDasharray: '900' }}
      />
      
      {/* Beacon with GPU-composited ping rings */}
      <g transform="translate(474, 66)">
        <circle cx="0" cy="0" r="3.5" fill="white" />
        <circle cx="0" cy="0" r="14" className="ops-ping-ring" />
        <circle cx="0" cy="0" r="14" className="ops-ping-ring-delayed" />
      </g>
      <text x="444" y="86" className="ops-svg-micro" textAnchor="middle">BEACON</text>

      {/* Playhead with GPU-composited pulse */}
      <g transform="translate(16, 149)">
        <circle cx="0" cy="0" r="9" />
        <path d="M-3 -5 l8 5 -8 5z" fill="white" />
        <circle cx="0" cy="0" r="9" className="ops-ping-ring" />
      </g>

      {/* Ghost waveform for depth */}
      <path
        d="M0 188L45 175 90 190 135 158 180 176 225 148 270 165 315 148 360 180 405 162 450 175 500 158"
        stroke="rgba(255,255,255,0.25)"
      />

      {/* Main undulating waveform */}
      <path
        id="ln"
        className="ops-wave-animated"
        d="M0 190L45 178 90 192 135 160 180 178 225 150 270 168 315 150 360 182 405 165 450 178 500 160"
      />

      {/* Timeline Ruler with Ticks */}
      <path d="M0 196H500M0 196v-10M45 196v-6M90 196v-10M135 196v-6M180 196v-10M225 196v-6M270 196v-10M315 196v-6M360 196v-10M405 196v-6M450 196v-10" />
      
      {/* Scanning time cursor */}
      <g className="ops-scanner-sweep">
        <line x1="0" y1="140" x2="0" y2="202" stroke="#ffffff" strokeWidth="1.5" />
        <line x1="0" y1="140" x2="0" y2="202" stroke="#ffffff" strokeWidth="3" opacity="0.3" />
      </g>

      {/* 4 Active Telemetry Streaming Channels */}
      <g>
        <circle cx="6" cy="228" r="3.5" className="ops-pulse" fill="white" />
        <circle cx="6" cy="244" r="3.5" className="ops-pulse" fill="white" />
        <circle cx="6" cy="260" r="3.5" className="ops-pulse" fill="white" />
        <circle cx="6" cy="276" r="3.5" className="ops-pulse" fill="white" />
        
        <path className="ops-stream-line-1" d="M26 228H500" />
        <path className="ops-stream-line-2" d="M26 244H500" />
        <path className="ops-stream-line-3" d="M26 260H500" />
        <path className="ops-stream-line-4" d="M26 276H500" />
      </g>
    </svg>
  );
}

/**
 * 2. Incidents: Threat Radar, Threshold Breach Anomaly Spike & Rapid Containment Barrier
 */
function IncidentsGraphic() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      className="ops-graphic ops-graphic-animated"
      viewBox="0 0 500 300"
      aria-label="Incident Threat Detection and Autonomous Containment"
    >
      <defs>
        <filter id="alert-glow" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="3" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* Threat Radar Scanner Box */}
      <rect x="45" y="18" width="170" height="90" rx="4" stroke="#ff6b6b" />
      <text x="56" y="32" className="ops-svg-label" fill="#ff6b6b">⚠ THREAT DETECTOR · ACTIVE</text>
      
      {/* Radar Circles */}
      <g transform="translate(85, 65)">
        <circle cx="0" cy="0" r="10" stroke="rgba(255,107,107,0.3)" />
        <circle cx="0" cy="0" r="20" stroke="rgba(255,107,107,0.5)" />
        <circle cx="0" cy="0" r="28" stroke="#ff6b6b" />
        <line x1="-28" y1="0" x2="28" y2="0" stroke="rgba(255,107,107,0.4)" />
        <line x1="0" y1="-28" x2="0" y2="28" stroke="rgba(255,107,107,0.4)" />
        {/* Rotating Radar Sweep Line */}
        <line x1="0" y1="0" x2="20" y2="-20" stroke="#ff6b6b" strokeWidth="2" className="ops-radar-sweep" />
        {/* Blinking Anomaly Dot */}
        <circle cx="12" cy="-14" r="3" fill="#ff6b6b" className="ops-pulse" />
      </g>

      <text x="125" y="52" className="ops-svg-micro" fill="#ff6b6b">INCIDENT #4102</text>
      <text x="125" y="66" className="ops-svg-micro">TCP SYN FLOOD</text>
      <text x="125" y="80" className="ops-svg-micro">RATE: 420.5/s</text>
      <text x="125" y="94" className="ops-svg-micro" fill="#e8a838">TTD: 1,240ms</text>

      {/* Dotted Critical Threshold Line */}
      <line x1="225" y1="52" x2="480" y2="52" stroke="#ff6b6b" strokeDasharray="6 4" strokeWidth="1.2" />
      <text x="230" y="44" fill="#ff6b6b" className="ops-svg-micro">
        CRITICAL THRESHOLD (SURGE &gt; 50ms)
      </text>

      {/* Normal baseline into Dramatic Anomaly Spike */}
      <path
        d="M225 82h35 l15 -62 l16 42 l14 -48 l18 68 h157"
        stroke="#ff6b6b"
        strokeWidth="2"
        fill="none"
        filter="url(#alert-glow)"
      />
      
      {/* Breach Epicenter Alert Beacon */}
      <circle cx="275" cy="20" r="4" fill="#ff6b6b" />
      <circle cx="275" cy="20" r="4" stroke="#ff6b6b" className="ops-ping-ring" />
      <circle cx="275" cy="20" r="4" stroke="#ff6b6b" className="ops-ping-ring-delayed" />
      <text x="275" y="10" className="ops-svg-micro" fill="#ff6b6b" textAnchor="middle">PEAK 68.2ms</text>

      {/* Incident Lifecycle Timeline with Containment Barrier */}
      <path d="M0 196H500M0 196v-10M60 196v-8M180 196v-8M300 196v-8M420 196v-8" />
      <text x="60" y="210" className="ops-svg-micro" textAnchor="middle">0ms SPIKE</text>
      <text x="180" y="210" className="ops-svg-micro" textAnchor="middle">+120ms PATTERN</text>
      <text x="300" y="210" className="ops-svg-micro" textAnchor="middle" fill="#ff6b6b">+1,240ms TTD</text>
      <text x="420" y="210" className="ops-svg-micro" textAnchor="middle" fill="#22c55e">+1,800ms ISOLATED</text>

      {/* Autonomous Isolation Gate Barrier at x=340 */}
      <rect x="336" y="140" width="8" height="56" rx="2" fill="#ff6b6b" opacity="0.3" stroke="#ff6b6b" />
      <line x1="332" y1="140" x2="348" y2="140" stroke="#ff6b6b" strokeWidth="2" />
      <line x1="332" y1="196" x2="348" y2="196" stroke="#ff6b6b" strokeWidth="2" />
      <text x="340" y="132" fill="#ff6b6b" className="ops-svg-micro" textAnchor="middle">CONTAINMENT GATE</text>

      {/* Downstream Normalized Waveform (Post-Containment) */}
      <path d="M0 185L60 185 120 182 180 178 240 162 280 148 300 172 336 185" stroke="#ff6b6b" strokeWidth="1.5" />
      <path d="M344 185H500" stroke="#22c55e" strokeWidth="1.8" />

      {/* 4 Mitigation Dispatch Tracks */}
      <g>
        <circle cx="6" cy="228" r="3.5" fill="#ff6b6b" stroke="#ff6b6b" />
        <circle cx="6" cy="244" r="3.5" fill="#e8a838" stroke="#e8a838" />
        <circle cx="6" cy="260" r="3.5" fill="#22c55e" stroke="#22c55e" />
        <circle cx="6" cy="276" r="3.5" fill="white" stroke="white" />
        
        {/* Track 1: eBPF socket drop (amber/red dashes streaming) */}
        <path className="ops-stream-line-1" d="M26 228H500" stroke="#ff6b6b" />
        {/* Track 2: SYN-cookie challenge */}
        <path className="ops-stream-line-2" d="M26 244H500" stroke="#e8a838" />
        {/* Track 3: Healthy sessions unaffected */}
        <path className="ops-stream-line-3" d="M26 260H500" stroke="#22c55e" />
        {/* Track 4: Tamper-evident incident record sealed */}
        <path className="ops-stream-line-4" d="M26 276H500" />
      </g>
    </svg>
  );
}

/**
 * 3. Egress Proof: Kavach Secret Scrubbing, Dual SHA-256 Comparator & Zero-Egress Barrier
 */
function EgressProofGraphic() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      className="ops-graphic ops-graphic-animated"
      viewBox="0 0 500 300"
      aria-label="Kavach Zero-Egress Privacy and Cryptographic Validation"
    >
      <defs>
        <filter id="verified-glow" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="2.5" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* Secret Scrubbing Chamber Box */}
      <rect x="45" y="18" width="165" height="90" rx="4" stroke="#22c55e" />
      <text x="56" y="32" className="ops-svg-label" fill="#22c55e">KAVACH PRIVACY MEMBRANE</text>

      {/* Raw Token Redaction in Chamber */}
      <rect x="58" y="44" width="60" height="15" rx="2" fill="rgba(255,107,107,0.2)" stroke="#ff6b6b" />
      <text x="62" y="55" className="ops-svg-micro" fill="#ff6b6b">API_KEY ***</text>
      <line x1="56" y1="51.5" x2="120" y2="51.5" stroke="#ff6b6b" strokeWidth="1.5" />

      <rect x="58" y="65" width="65" height="15" rx="2" fill="rgba(255,107,107,0.2)" stroke="#ff6b6b" />
      <text x="62" y="76" className="ops-svg-micro" fill="#ff6b6b">BEARER_TOK</text>
      <line x1="56" y1="72.5" x2="125" y2="72.5" stroke="#ff6b6b" strokeWidth="1.5" />

      {/* Vertical Scanning Laser Beam sweeping across secrets */}
      <line x1="88" y1="40" x2="88" y2="85" stroke="#22c55e" strokeWidth="2" className="ops-scrub-laser" />

      <text x="135" y="54" className="ops-svg-micro" fill="#22c55e">42 STRIPPED</text>
      <text x="135" y="68" className="ops-svg-micro" fill="#22c55e">0 LEAKS</text>
      <text x="135" y="82" className="ops-svg-micro">HMAC-SHA256</text>
      <text x="56" y="96" className="ops-svg-micro" fill="#22c55e">✓ FAIL-CLOSED OUTBOUND</text>

      {/* Dual SHA-256 Checksum Pipeline */}
      {/* Top Pipe: Pre-Send Digest */}
      <rect x="235" y="24" width="130" height="24" rx="3" stroke="#22c55e" />
      <text x="242" y="40" className="ops-svg-micro" fill="#22c55e">PRE-SEND: 3f7a1b...5d8e</text>
      
      {/* Bottom Pipe: Tier-2 Echoed Digest */}
      <rect x="235" y="70" width="130" height="24" rx="3" stroke="#22c55e" />
      <text x="242" y="86" className="ops-svg-micro" fill="#22c55e">ECHOED:   3f7a1b...5d8e</text>

      {/* Central Comparator Lock */}
      <path d="M365 36h40v16M365 82h40v-16" stroke="#22c55e" />
      <rect x="395" y="42" width="85" height="34" rx="3" fill="rgba(34,197,94,0.15)" stroke="#22c55e" filter="url(#verified-glow)" />
      <text x="437" y="56" className="ops-svg-micro" fill="#22c55e" textAnchor="middle">MATCH == TRUE</text>
      <text x="437" y="68" className="ops-svg-micro" fill="#22c55e" textAnchor="middle">0 BYTES DIFF ✓</text>
      <circle cx="437" cy="59" r="22" stroke="#22c55e" className="ops-ping-ring" />

      {/* Allowlist Gate & Fail-Closed Boundary Wall at x=260 */}
      <path d="M0 196H500M0 196v-10M90 196v-8M180 196v-8M270 196v-10M360 196v-8M450 196v-8" />
      <text x="90" y="210" className="ops-svg-micro" textAnchor="middle">BATCH #410</text>
      <text x="180" y="210" className="ops-svg-micro" textAnchor="middle">BATCH #411</text>
      <text x="270" y="210" className="ops-svg-micro" textAnchor="middle" fill="#22c55e">BATCH #412</text>
      <text x="360" y="210" className="ops-svg-micro" textAnchor="middle">HEAD VERIFIED</text>

      {/* Fail-Closed Wall */}
      <line x1="260" y1="135" x2="260" y2="202" stroke="#22c55e" strokeWidth="2.5" />
      {/* Diode One-way Gate */}
      <polygon points="255,160 268,168 255,176" fill="#22c55e" />
      <text x="260" y="128" fill="#22c55e" className="ops-svg-micro" textAnchor="middle">EGRESS ALLOWLIST GATE</text>

      {/* Blocked Raw Leak Attempt (repelled) */}
      <path d="M160 168h75" stroke="#ff6b6b" strokeWidth="1.5" strokeDasharray="4 2" />
      <text x="195" y="160" fill="#ff6b6b" className="ops-svg-micro" textAnchor="middle">BLOCKED ⛔</text>

      {/* 4 Verified Egress Streams */}
      <g>
        <circle cx="6" cy="228" r="3.5" fill="#22c55e" stroke="#22c55e" />
        <circle cx="6" cy="244" r="3.5" fill="#22c55e" stroke="#22c55e" />
        <circle cx="6" cy="260" r="3.5" fill="#38bdf8" stroke="#38bdf8" />
        <circle cx="6" cy="276" r="3.5" fill="white" stroke="white" />
        
        <path className="ops-stream-line-1" d="M26 228H500" stroke="#22c55e" />
        <path className="ops-stream-line-2" d="M26 244H500" stroke="#22c55e" />
        <path className="ops-stream-line-3" d="M26 260H500" stroke="#38bdf8" />
        <path className="ops-stream-line-4" d="M26 276H500" />
      </g>
    </svg>
  );
}

/**
 * 4. Audit Log: Immutable Hash Chain, Cryptographic Merkle Links & Integrity Scanner
 */
function AuditLogGraphic() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      className="ops-graphic ops-graphic-animated"
      viewBox="0 0 500 300"
      aria-label="Tamper-Evident Hash-Linked Audit Chain"
    >
      <defs>
        <filter id="hash-glow" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="2.5" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* Block 46 */}
      <rect x="40" y="20" width="112" height="84" rx="3" />
      <text x="48" y="34" className="ops-svg-label">BLOCK #46</text>
      <text x="48" y="48" className="ops-svg-micro">PREV: 8b2c4d...</text>
      <text x="48" y="62" className="ops-svg-micro">HASH: 3f7a1b...</text>
      <text x="48" y="78" className="ops-svg-micro">MERKLE: 0x9a4f</text>
      <text x="48" y="94" className="ops-svg-micro" fill="#22c55e">✓ SEALED</text>

      {/* Hash Link Arrow 1 (46 -> 47) */}
      <path d="M152 62h34m-6 -6l6 6-6 6" stroke="#22c55e" strokeWidth="1.5" />
      <circle cx="168" cy="62" r="3" fill="#22c55e" className="ops-hash-travel" />

      {/* Block 47 */}
      <rect x="190" y="20" width="112" height="84" rx="3" stroke="#22c55e" />
      <text x="198" y="34" className="ops-svg-label" fill="#22c55e">BLOCK #47</text>
      <text x="198" y="48" className="ops-svg-micro">PREV: 3f7a1b...</text>
      <text x="198" y="62" className="ops-svg-micro">HASH: 9a7b6c...</text>
      <text x="198" y="78" className="ops-svg-micro">MERKLE: 0x4e2d</text>
      <text x="198" y="94" className="ops-svg-micro" fill="#22c55e">✓ SEALED</text>

      {/* Hash Link Arrow 2 (47 -> 48) */}
      <path d="M302 62h34m-6 -6l6 6-6 6" stroke="#38bdf8" strokeWidth="1.5" />
      <circle cx="318" cy="62" r="3" fill="#38bdf8" className="ops-hash-travel" />

      {/* Block 48 / Head */}
      <rect x="340" y="20" width="112" height="84" rx="3" stroke="#38bdf8" />
      <text x="348" y="34" className="ops-svg-label" fill="#38bdf8">HEAD #48</text>
      <text x="348" y="48" className="ops-svg-micro">PREV: 9a7b6c...</text>
      <text x="348" y="62" className="ops-svg-micro">HASH: e4f5a6...</text>
      <text x="348" y="78" className="ops-svg-micro">MERKLE: 0x1b8c</text>
      <text x="348" y="94" className="ops-svg-micro" fill="#38bdf8">● SIGNING...</text>

      {/* Continuous Verification Scanner Timeline */}
      <path d="M0 196H500M0 196v-10M50 196v-8M150 196v-8M250 196v-10M350 196v-8M450 196v-8" />
      <text x="50" y="210" className="ops-svg-micro" textAnchor="middle">12:00:00 UTC</text>
      <text x="150" y="210" className="ops-svg-micro" textAnchor="middle">12:00:15 UTC</text>
      <text x="250" y="210" className="ops-svg-micro" textAnchor="middle" fill="#22c55e">CHAIN VERIFIED ✓</text>
      <text x="350" y="210" className="ops-svg-micro" textAnchor="middle">HEAD COMMITTED</text>

      {/* Sweeping Chain Verification Laser */}
      <line y1="135" y2="202" className="ops-scanner-sweep" stroke="#22c55e" />

      {/* Padlock Security Badge */}
      <g transform="translate(460, 160)">
        <rect x="0" y="8" width="18" height="14" rx="2" fill="none" stroke="#22c55e" />
        <path d="M4 8V4a5 5 0 0 1 10 0v4" fill="none" stroke="#22c55e" />
      </g>
      <text x="440" y="152" fill="#22c55e" className="ops-svg-micro" textAnchor="middle">IMMUTABLE</text>

      {/* 4 Append-Only Event Record Tracks */}
      <g>
        <circle cx="6" cy="228" r="3.5" fill="#22c55e" stroke="#22c55e" />
        <circle cx="6" cy="244" r="3.5" fill="#22c55e" stroke="#22c55e" />
        <circle cx="6" cy="260" r="3.5" fill="#38bdf8" stroke="#38bdf8" />
        <circle cx="6" cy="276" r="3.5" fill="white" stroke="white" />
        
        <path className="ops-stream-line-1" d="M26 228H500" stroke="#22c55e" />
        <path className="ops-stream-line-2" d="M26 244H500" stroke="#22c55e" />
        <path className="ops-stream-line-3" d="M26 260H500" stroke="#38bdf8" />
        <path className="ops-stream-line-4" d="M26 276H500" />
      </g>
    </svg>
  );
}

/**
 * 5. Demo Lab: Controlled Scenario Waveform & Fault Recovery Simulator
 */
function DemoLabGraphic() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" className="ops-graphic ops-graphic-animated" viewBox="0 0 500 300">
      <rect x="45" y="18" width="170" height="90" rx="4" stroke="#e8a838" />
      <text x="56" y="32" className="ops-svg-label" fill="#e8a838">DEMO LAB · CHAOS SIMULATOR</text>
      <text x="56" y="52" className="ops-svg-micro">INJECTION: CPU_SPIKE</text>
      <text x="56" y="66" className="ops-svg-micro">SURGE: 420.5 MB/s</text>
      <text x="56" y="80" className="ops-svg-micro" fill="#22c55e">CONVERGENCE: &lt;1,240ms</text>
      <text x="56" y="94" className="ops-svg-micro">AUTONOMOUS RESTORE</text>

      {/* Square wave chaos surge recovering to sine wave */}
      <path d="M225 60h30v-35h30v70h30v-35h30l20 -15 20 15 20 -15 20 15h45" stroke="#e8a838" strokeWidth="1.8" fill="none" />
      <circle cx="285" cy="25" r="4" fill="#e8a838" />

      <path d="M0 196H500M0 196v-10M90 196v-8M180 196v-8M270 196v-10M360 196v-8M450 196v-8" />
      <line y1="140" y2="202" className="ops-scanner-sweep" stroke="#e8a838" />

      <g>
        <circle cx="6" cy="228" r="3.5" fill="#e8a838" stroke="#e8a838" />
        <circle cx="6" cy="244" r="3.5" fill="#22c55e" stroke="#22c55e" />
        <circle cx="6" cy="260" r="3.5" fill="#38bdf8" stroke="#38bdf8" />
        <circle cx="6" cy="276" r="3.5" fill="white" stroke="white" />
        <path className="ops-stream-line-1" d="M26 228H500" stroke="#e8a838" />
        <path className="ops-stream-line-2" d="M26 244H500" stroke="#22c55e" />
        <path className="ops-stream-line-3" d="M26 260H500" stroke="#38bdf8" />
        <path className="ops-stream-line-4" d="M26 276H500" />
      </g>
    </svg>
  );
}

/**
 * 6. Validation: Latency Distribution Bell Curve & Performance Envelope
 */
function ValidationGraphic() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" className="ops-graphic ops-graphic-animated" viewBox="0 0 500 300">
      <rect x="45" y="18" width="170" height="90" rx="4" stroke="#38bdf8" />
      <text x="56" y="32" className="ops-svg-label" fill="#38bdf8">VALIDATION BENCHMARK</text>
      <text x="56" y="52" className="ops-svg-micro">E2E P50: 346ms (≤10s)</text>
      <text x="56" y="66" className="ops-svg-micro">E2E P95: 1,033ms (≤15s)</text>
      <text x="56" y="80" className="ops-svg-micro" fill="#22c55e">HEARTBEAT AGE: 0s</text>
      <text x="56" y="94" className="ops-svg-micro" fill="#22c55e">PASS RATE: 100%</text>

      {/* Bell Curve Distribution */}
      <path d="M225 90 C 270 90, 285 24, 320 24 C 355 24, 370 90, 480 90" stroke="#38bdf8" strokeWidth="2" fill="none" />
      <line x1="320" y1="20" x2="320" y2="90" stroke="#38bdf8" strokeDasharray="4 2" />
      <text x="320" y="14" className="ops-svg-micro" fill="#38bdf8" textAnchor="middle">MEDIAN P50</text>

      <path d="M0 196H500M0 196v-10M100 196v-8M200 196v-8M300 196v-10M400 196v-8" />
      <line y1="140" y2="202" className="ops-scanner-sweep" stroke="#38bdf8" />

      <g>
        <circle cx="6" cy="228" r="3.5" fill="#38bdf8" stroke="#38bdf8" />
        <circle cx="6" cy="244" r="3.5" fill="#38bdf8" stroke="#38bdf8" />
        <circle cx="6" cy="260" r="3.5" fill="#22c55e" stroke="#22c55e" />
        <circle cx="6" cy="276" r="3.5" fill="white" stroke="white" />
        <path className="ops-stream-line-1" d="M26 228H500" stroke="#38bdf8" />
        <path className="ops-stream-line-2" d="M26 244H500" stroke="#38bdf8" />
        <path className="ops-stream-line-3" d="M26 260H500" stroke="#22c55e" />
        <path className="ops-stream-line-4" d="M26 276H500" />
      </g>
    </svg>
  );
}

/**
 * 7. Limitations: Boundaries and Hardware Cage Guardrails
 */
function LimitationsGraphic() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" className="ops-graphic ops-graphic-animated" viewBox="0 0 500 300">
      <rect x="45" y="18" width="170" height="90" rx="4" />
      <text x="56" y="32" className="ops-svg-label">OPERATING BOUNDARIES</text>
      <text x="56" y="52" className="ops-svg-micro">MEMORY CEILING: 256MB</text>
      <text x="56" y="66" className="ops-svg-micro">WAL RETENTION: 60 SECONDS</text>
      <text x="56" y="80" className="ops-svg-micro">FAIL-CLOSED ENFORCEMENT</text>
      <text x="56" y="94" className="ops-svg-micro" fill="#ff6b6b">IMMUTABLE ENCLAVE</text>

      {/* Boundary Cage Box */}
      <rect x="235" y="24" width="235" height="74" rx="4" strokeDasharray="8 4" stroke="#ff6b6b" />
      <text x="352" y="64" className="ops-svg-label" fill="#ff6b6b" textAnchor="middle">STRICT BOUNDARY PERIMETER</text>

      <path d="M0 196H500M0 196v-10M100 196v-8M200 196v-8M300 196v-10M400 196v-8" />
      <line y1="140" y2="202" className="ops-scanner-sweep" />

      <g>
        <circle cx="6" cy="228" r="3.5" fill="white" stroke="white" />
        <circle cx="6" cy="244" r="3.5" fill="white" stroke="white" />
        <circle cx="6" cy="260" r="3.5" fill="#ff6b6b" stroke="#ff6b6b" />
        <circle cx="6" cy="276" r="3.5" fill="white" stroke="white" />
        <path className="ops-stream-line-1" d="M26 228H500" />
        <path className="ops-stream-line-2" d="M26 244H500" />
        <path className="ops-stream-line-3" d="M26 260H500" stroke="#ff6b6b" />
        <path className="ops-stream-line-4" d="M26 276H500" />
      </g>
    </svg>
  );
}
