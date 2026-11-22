import { initMotion, setupDialogReveals } from './motion.js';

const products = [
  {
    "id": "prana",
    "name": "Prana",
    "label": "Local collection",
    "symbol": "○",
    "description": "Collect real measurements where your systems run.",
    "headline": "High-Fidelity Multi-Source Telemetry at the Edge.",
    "overview": "Prana runs as a lightweight, low-footprint daemon directly on host infrastructure across Windows, Linux, and macOS. It continuously captures hardware metrics, probes network connectivity, and replays synthetic failure traces to validate operational readiness without impacting production services.",
    "capabilities": [
      {
        "title": "OS & Hardware Telemetry",
        "desc": "Captures live CPU percent, memory distribution, disk I/O throughput, and network interface activity via native gopsutil probes."
      },
      {
        "title": "Active Network Probes",
        "desc": "Executes deterministic TCP probes measuring connection latency, round-trip jitter, and packet loss across edge endpoints."
      },
      {
        "title": "Synthetic Anomaly Replay",
        "desc": "Injects labeled NAB (Numenta Anomaly Benchmark) timeseries traces and Loghub authentication streams with explicit provenance tags."
      },
      {
        "title": "Fail-Safe Memory Ring",
        "desc": "Bounded in-memory ring buffers isolate collection load, ensuring host CPU remains under 1% with automatic backpressure shedding."
      }
    ],
    "specs": [
      { "label": "Runtime", "value": "Go Standalone" },
      { "label": "Platforms", "value": "Win / Linux / macOS" },
      { "label": "Sampling", "value": "1s raw / 5s rollup" },
      { "label": "Footprint", "value": "< 25 MB RAM" }
    ],
    "guarantee": "Host isolation guarantee: collection runs sandboxed with bounded memory rings. No invasive kernel drivers or eBPF privileges required.",
    "features": [
      "Live OS metrics and TCP probes",
      "NAB and Loghub replay with explicit source labels",
      "Go collector for Windows, Linux and macOS"
    ],
    "image": "/assets/features/prana.png",
    "tags": "collection cpu memory network live replay"
  },
  {
    "id": "kavach",
    "name": "Kavach",
    "label": "Privacy gate",
    "symbol": "◇",
    "description": "Keep raw logs and secrets inside the collector.",
    "headline": "The Security Armor Around Tier 1's Trust Boundary.",
    "overview": "Kavach sits between raw collection and downstream processing as an uncompromisable privacy gate. It ensures that API keys, passwords, database credentials, and raw logs never leave the collector host—sanitizing all data against strict, fail-closed schemas.",
    "capabilities": [
      {
        "title": "Entropy & Pattern Scrubbing",
        "desc": "Detects high-entropy credential strings, private keys, JWTs, and bearer tokens in real time, redacting them before memory transit."
      },
      {
        "title": "Tenant-Keyed HMAC Pseudonyms",
        "desc": "Irreversibly converts client IPs, hostnames, usernames, and container IDs into stable HMAC-SHA256 pseudonyms for cross-correlation."
      },
      {
        "title": "Fail-Closed Outbound Allowlist",
        "desc": "Enforces rigid schema validation. Any unrecognized field, arbitrary string, or unapproved payload is instantly dropped before persistence."
      },
      {
        "title": "Redaction Auditing & Metric Accounting",
        "desc": "Maintains exact per-type redaction tallies (tokens, keys, passwords) without persisting or leaking sensitive plaintexts."
      }
    ],
    "specs": [
      { "label": "Primitive", "value": "HMAC-SHA256" },
      { "label": "Overhead", "value": "< 0.2ms / event" },
      { "label": "Policy", "value": "Fail-Closed Allowlist" },
      { "label": "Data Boundary", "value": "Zero Raw Egress" }
    ],
    "guarantee": "Zero-egress guarantee: no raw log string, API key, password, or plaintext IP address ever crosses the network boundary.",
    "features": [
      "Secret scrubbing and per-type counts",
      "Tenant-keyed HMAC pseudonyms",
      "Fail-closed outbound allowlist"
    ],
    "image": "/assets/features/kavach.png",
    "tags": "privacy secrets redaction policy pseudonyms"
  },
  {
    "id": "nadi",
    "name": "Nadi",
    "label": "Local analysis",
    "symbol": "≋",
    "description": "Turn measurements into compact, useful summaries.",
    "headline": "High-Throughput Edge Analytics & Anomaly Detection.",
    "overview": "Nadi aggregates raw 1-second telemetry into compact 5-second statistical rollups right on the node. By calculating distribution percentiles and detecting localized brute-force spikes at the edge, Nadi reduces network egress by over 90% while preserving high-resolution diagnostic signals.",
    "capabilities": [
      {
        "title": "Five-Second Statistical Rollups",
        "desc": "Computes min, mean, max, and p95 percentiles for system, memory, network, and disk metrics over sliding 5-second windows."
      },
      {
        "title": "Edge Brute-Force & Attack Detection",
        "desc": "Monitors rolling 60-second authentication failure rates, flagging localized brute-force attempts and port floods directly at the node."
      },
      {
        "title": "Live vs. Replay Stream Isolation",
        "desc": "Algorithmic separation isolates production telemetry from replayed synthetic benchmarks to prevent alert pollution."
      },
      {
        "title": "Bandwidth Compression",
        "desc": "Reduces telemetry bandwidth by over 90% before write-ahead logging, preventing network saturation during incidents."
      }
    ],
    "specs": [
      { "label": "Rollup Window", "value": "5-Second Sliding" },
      { "label": "Security Window", "value": "60-Second Auth Spike" },
      { "label": "Compression", "value": "~92% Egress Savings" },
      { "label": "Buffer Model", "value": "Circular Bounded" }
    ],
    "guarantee": "Statistical fidelity guarantee: percentiles (p95) and distribution metrics are calculated locally with high mathematical accuracy.",
    "features": [
      "Five-second min, average, max and p95 rollups",
      "Rolling 60-second failed-login detection",
      "Separate live and replayed summaries"
    ],
    "image": "/assets/features/nadi.png",
    "tags": "metrics rollups statistics security brute force"
  },
  {
    "id": "setu",
    "name": "Setu",
    "label": "Reliable delivery",
    "symbol": "⋈",
    "description": "Buffer approved batches and prove what was received.",
    "headline": "Resilient Disk WAL Delivery with Cryptographic Proof.",
    "overview": "Setu bridges Tier 1 collectors with Tier 2 regional receivers. Operating an append-only disk Write-Ahead Log (WAL), Setu guarantees zero data loss during network disruptions, recovers automatically with exponential backoff, and provides non-repudiable SHA-256 byte-level delivery receipts.",
    "capabilities": [
      {
        "title": "Append-Only Disk Write-Ahead Log",
        "desc": "Durable disk persistence buffers approved telemetry batches locally, surviving process crashes, node restarts, and WAN blackouts."
      },
      {
        "title": "mTLS Delivery & Exponential Backoff",
        "desc": "Delivers batches over mutual TLS with jittered exponential backoff and automatic session resumption upon network restoration."
      },
      {
        "title": "Receiver Deduplication & Gap Detection",
        "desc": "Sliding receiver-side sequence windows detect dropped packets, missing spans, and out-of-order deliveries."
      },
      {
        "title": "Cryptographic SHA-256 Egress Proof",
        "desc": "Calculates exact received-byte SHA-256 hashes returned in receiver ACKs, providing verifiable proof of data integrity."
      }
    ],
    "specs": [
      { "label": "Persistence", "value": "Disk WAL + File Lock" },
      { "label": "Transport", "value": "Mutual TLS (mTLS)" },
      { "label": "Proof Model", "value": "SHA-256 Received-Byte" },
      { "label": "Recovery", "value": "Auto Replay on Reconnect" }
    ],
    "guarantee": "Delivery guarantee: at-least-once delivery with end-to-end cryptographic checksum verification and zero data loss on node reboot.",
    "features": [
      "Append-only disk WAL and HTTPS retries",
      "Idempotent regional ingest with bounded memory rings",
      "Local egress proof and exact-byte hash matching"
    ],
    "image": "/assets/features/setu.png",
    "tags": "delivery wal outage retry acknowledgements proof"
  },
  {
    "id": "medha",
    "name": "Medha",
    "label": "Fleet intelligence & AIOps",
    "symbol": "✳",
    "description": "Explain fleet health, correlate incidents and simulate safe remediation.",
    "headline": "Four-State Health, Incident Correlation & Safe Remediation.",
    "overview": "Medha is the Tier 2 intelligence engine. It continuously evaluates fleet-wide metrics against statistical baselines to assign deterministic health states (HEALTHY, DEGRADED, CRITICAL, ISOLATED). Operators receive explainable AIOps briefs and can simulate, execute, or roll back remediations with full audit provenance.",
    "capabilities": [
      {
        "title": "Four-State Health Classification",
        "desc": "Evaluates fleet nodes into HEALTHY, DEGRADED, CRITICAL, and ISOLATED tiers based on adaptive statistical deviations."
      },
      {
        "title": "Cross-Node Incident Correlation",
        "desc": "Groups multi-node anomalies and cascading service degradations into unified, actionable incident clusters."
      },
      {
        "title": "Safe Remediation Simulator",
        "desc": "Operators can propose, preview impact, simulate execution, and instantly roll back remediation actions with signed audit logs."
      },
      {
        "title": "Explainable Sovereign AIOps Briefs",
        "desc": "Reconstructs incident context strictly from numeric evidence and approved schema enums—no raw user data or logs ever egress to LLMs."
      },
      {
        "title": "Multi-Tenant Membrane Console",
        "desc": "Real-time WebSocket streaming dashboard (/app) featuring interactive topology maps, incident drilldowns, and token authentication."
      }
    ],
    "specs": [
      { "label": "Health Tiers", "value": "HEALTHY / DEGRADED / CRIT / ISO" },
      { "label": "Safety Model", "value": "Simulation + Instant Rollback" },
      { "label": "AIOps Privacy", "value": "Pure Numeric Projection" },
      { "label": "Console", "value": "Live Membrane Mesh (/app)" }
    ],
    "guarantee": "Sovereignty guarantee: AI explanations operate strictly on mathematical evidence and enums. Remediations remain simulated until explicit operator approval.",
    "features": [
      "Four-state health engine (Healthy, Degraded, Critical, Isolated)",
      "Incident correlation and safe remediation simulation with rollback",
      "Explainable AI briefs and real-time authenticated Membrane console"
    ],
    "image": "/assets/features/medha.png",
    "tags": "aiops health incidents dashboard explanations remediation"
  }
];
const solutions = [
  {
    "name": "SaaS & Engineering",
    "text": "Inspect real infrastructure signals and approved summaries.",
    "detail": "Current build: collection, privacy, local analysis and durable delivery. Authentication and the live console are upcoming phases.",
    "products": [
      "kavach",
      "setu",
      "medha"
    ]
  },
  {
    "name": "Enterprise IT",
    "text": "Keep collection and delivery inside your network.",
    "detail": "Current build: collection, privacy, local analysis and durable delivery. Authentication and the live console are upcoming phases.",
    "products": [
      "prana",
      "setu",
      "nadi"
    ]
  },
  {
    "name": "Managed Services",
    "text": "Evaluate tenant-separated identifiers and regional storage.",
    "detail": "Current build: collection, privacy, local analysis and durable delivery. Authentication and the live console are upcoming phases.",
    "products": [
      "kavach",
      "setu",
      "medha"
    ]
  },
  {
    "name": "Network Operations",
    "text": "Measure reachability with configurable TCP probes.",
    "detail": "Current build: collection, privacy, local analysis and durable delivery. Authentication and the live console are upcoming phases.",
    "products": [
      "prana",
      "setu",
      "nadi"
    ]
  },
  {
    "name": "Industrial & Edge",
    "text": "Buffer approved data through network interruptions.",
    "detail": "Current build: collection, privacy, local analysis and durable delivery. Authentication and the live console are upcoming phases.",
    "products": [
      "kavach",
      "setu",
      "medha"
    ]
  },
  {
    "name": "Regulated Environments",
    "text": "Inspect the exact bytes allowed to leave a collector.",
    "detail": "Current build: collection, privacy, local analysis and durable delivery. Authentication and the live console are upcoming phases.",
    "products": [
      "prana",
      "setu",
      "nadi"
    ]
  }
];
const stories = [
  {
    product: "prana",
    name: "Prana",
    eyebrow: "LOCAL MULTI-SOURCE COLLECTION",
    headline: "Sense Everything. Send Nothing Raw.",
    image: "/assets/features/prana.png",
    alt: "Prana edge collection layer capturing CPU, memory, network and log signals inside the customer environment"
  },
  {
    product: "kavach",
    name: "Kavach",
    eyebrow: "ZERO-LEAK PRIVACY FILTER & POLICY GATE",
    headline: "Scrub Before Analysis. Validate Before Egress.",
    image: "/assets/features/kavach.png",
    alt: "Kavach two-stage privacy filter and fail-closed policy gate"
  },
  {
    product: "nadi",
    name: "Nadi",
    eyebrow: "EDGE AGGREGATION & LOCAL DETECTION",
    headline: "Compress Noise. Catch Attacks Locally.",
    image: "/assets/features/nadi.png",
    alt: "Nadi five-second statistical rollups and local brute-force detection"
  },
  {
    product: "setu",
    name: "Setu",
    eyebrow: "RESILIENT DELIVERY & MULTI-TENANT INGEST",
    headline: "Prove What Leaves. Lose Nothing In Transit.",
    image: "/assets/features/setu.png",
    alt: "Setu approved-only egress, durable buffering and tenant-isolated ingest"
  },
  {
    product: "medha",
    name: "Medha",
    eyebrow: "EXPLAINABLE AIOPS & FLEET CONSOLE",
    headline: "Four-State Health. Explainable Incidents.",
    image: "/assets/features/medha.png",
    alt: "Medha four-state health engine, incident correlation and fleet console"
  }
];
const deployments = {};

const $ = (selector, root=document) => root.querySelector(selector);
const escapeHTML = value => String(value).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const productById = id => products.find(p => p.id === id);
const contentDialog = $('#content-dialog');
const content = $('#dialog-content');
let activeStory = 0;
let integrationCatalog;
let previousFocus;

// Render initial components
$('#product-list').innerHTML = products.slice(0,5).map((p,i)=>`
  <button class="product-row" data-product="${p.id}" aria-label="Explore ${p.name}: ${p.label}">
    <span class="product-description">${p.description}</span>
    <span class="product-symbol" aria-hidden="true">${p.symbol}</span>
    <span class="product-name">${p.name}</span>
    <span class="product-number">/0.${i+1}</span>
  </button>
`).join('');

$('#footer-products').innerHTML = products.slice(0,5).map(p=>`<button data-product="${p.id}">${p.name}</button>`).join('')+'<button data-open="catalog">All capabilities</button>';
$('#feature-tabs').innerHTML = stories.map((s,i)=>`<button role="tab" id="story-tab-${i}" aria-selected="${i===0}" aria-controls="story-${i}" tabindex="${i===0?0:-1}" data-story="${i}"><span class="tab-label">${s.name}</span></button>`).join('');
$('#story-track').innerHTML = stories.map((s,i)=>`
  <div class="story" id="story-${i}" role="tabpanel" aria-labelledby="story-tab-${i}" tabindex="${i===0?0:-1}" data-story-index="${i}">
    <img src="${s.image}" alt="${escapeHTML(s.alt)}" loading="${i===0?'eager':'lazy'}">
    ${s.product === 'prana' ? '<canvas class="story-telemetry-canvas" id="story-telemetry-canvas" aria-hidden="true"></canvas>' : ''}
    <div class="story-copy">
      <span class="story-eyebrow">${escapeHTML(s.eyebrow)}</span>
      <h3 class="story-headline">
        <a href="#feature-${s.product}" class="story-headline-link" data-product="${s.product}">
          ${escapeHTML(s.headline)}<span class="story-arrow" aria-hidden="true"> ↗</span>
        </a>
      </h3>
    </div>
  </div>
`).join('');
const renderSolutionCards = (isClone = false) => solutions.map((s,i)=>`
  <button class="solution-card" data-solution="${i}" ${isClone ? 'tabindex="-1" aria-hidden="true"' : ''}>
    <h3>${s.name}</h3>
    <p>${s.text}</p>
    <span aria-hidden="true">↗</span>
  </button>
`).join('');

const solutionCardsSet = renderSolutionCards(false) + renderSolutionCards(true);
const solutionTrack = $('#solution-track');
const solutionTrackClone = $('#solution-track-clone');
if (solutionTrack) solutionTrack.innerHTML = solutionCardsSet;
if (solutionTrackClone) solutionTrackClone.innerHTML = solutionCardsSet;

const solutionMarquee = $('#solution-marquee');
if (solutionMarquee) {
  solutionMarquee.addEventListener('touchstart', () => {
    solutionMarquee.classList.add('is-touch-paused');
  }, { passive: true });
  solutionMarquee.addEventListener('touchend', () => {
    setTimeout(() => solutionMarquee.classList.remove('is-touch-paused'), 1500);
  }, { passive: true });
}

function openDialog(dialog) {
  if (!document.querySelector('dialog[open]')) previousFocus=document.activeElement;
  document.querySelectorAll('dialog[open]').forEach(d=>d.close());
  dialog.showModal();
  $('#open-menu').setAttribute('aria-expanded', String(dialog.id==='navigation-dialog'));
  setupDialogReveals(dialog);
}
function closeDialog(dialog) { dialog.close(); }

document.querySelectorAll('dialog').forEach(dialog=>{
  dialog.addEventListener('close',()=>{
    if (!document.querySelector('dialog[open]')) {
      $('#open-menu').setAttribute('aria-expanded','false');
      previousFocus?.focus({preventScroll:true});
    }
  });
  dialog.addEventListener('click', event=>{
    if(event.target===dialog){const r=dialog.getBoundingClientRect();if(event.clientX<r.left||event.clientX>r.right||event.clientY<r.top||event.clientY>r.bottom) closeDialog(dialog);}
  });
});

function showContent(html) { content.innerHTML=html; openDialog(contentDialog); contentDialog.scrollTop=0; setupDialogReveals(contentDialog); }
function catalogItem(p){return `<button class="catalog-item" data-product="${p.id}"><div><h3>${p.name}</h3><p>${p.label}</p></div><span aria-hidden="true">↗</span></button>`;}

function scrambleText(element, finalText, duration = 450) {
  const glyphs = '0123456789ABCDEF_#$@&%<>[]/*';
  const totalFrames = Math.max(8, Math.floor(duration / 25));
  let frame = 0;
  const length = finalText.length;
  if (element._scrambleTimer) clearInterval(element._scrambleTimer);

  element.classList.add('glitch-active');
  element._scrambleTimer = setInterval(() => {
    frame++;
    const progress = frame / totalFrames;
    const revealedCount = Math.floor(progress * length);
    let output = '';
    for (let i = 0; i < length; i++) {
      if (finalText[i] === ' ' || finalText[i] === '\n') {
        output += finalText[i];
      } else if (i < revealedCount) {
        output += finalText[i];
      } else {
        output += glyphs[Math.floor(Math.random() * glyphs.length)];
      }
    }
    element.textContent = output;
    if (frame >= totalFrames) {
      clearInterval(element._scrambleTimer);
      element.textContent = finalText;
      setTimeout(() => element.classList.remove('glitch-active'), 320);
    }
  }, 25);
}

function showProduct(id){
  const p=productById(id); if(!p)return;
  const idx = products.indexOf(p);
  showContent(`
    <span class="eyebrow eyebrow-glitch">KALKI SYSTEM MODULE / [0.${idx+1}] · ${p.label.toUpperCase()}</span>
    <h2 id="dialog-title" class="dialog-title-glitch" data-title="${p.name}">${p.name}</h2>
    <p class="dialog-lead">${p.headline || p.description}</p>
    <div class="detail-hero-frame">
      <img class="detail-hero" src="${p.image.startsWith('/') ? p.image : '/assets/' + p.image + '.webp'}" alt="${p.name} architecture and data flow diagram">
    </div>
    <div class="module-overview">
      <p>${p.overview || p.description}</p>
    </div>
    <h3 class="dialog-section-heading">Core Engine Capabilities</h3>
    <div class="capability-grid">
      ${(p.capabilities || p.features.map(f => ({ title: f, desc: '' }))).map(c => `
        <div class="capability-card">
          <h4><span class="cap-icon">${p.symbol}</span> ${c.title}</h4>
          ${c.desc ? `<p>${c.desc}</p>` : ''}
        </div>
      `).join('')}
    </div>
    ${p.specs ? `
      <h3 class="dialog-section-heading">Architectural Specifications</h3>
      <div class="spec-strip">
        ${p.specs.map(s => `
          <div class="spec-item">
            <span class="spec-label">${s.label}</span>
            <span class="spec-val">${s.value}</span>
          </div>
        `).join('')}
      </div>
    ` : ''}
    <p class="dialog-note"><strong>Data Boundary Guarantee:</strong> ${p.guarantee || 'All collection, policy sanitization and aggregation execute locally within your sovereign host boundary. Zero raw egress.'}</p>
    <div class="dialog-actions">
      <a class="solid-button" href="/app">Launch Workspace ↗</a>
      <button class="small-outline" data-open="catalog">All 5 modules</button>
    </div>
  `);

  const titleEl = $('#dialog-title');
  if (titleEl) {
    scrambleText(titleEl, p.name);
  }
}

function showCatalog(){
  showContent(`
    <span class="eyebrow">THE KALKI PLATFORM</span>
    <h2 id="dialog-title">Every signal.<br>A unified operational mesh.</h2>
    <p class="dialog-lead">Explore the five modules powering the zero-raw-egress operational intelligence platform.</p>
    <div class="catalog-grid">${products.map(catalogItem).join('')}</div>
  `);
}

function showSolution(index){
  const s=solutions[index]; if(!s)return;
  showContent(`
    <span class="eyebrow">KALKI / ARCHITECTURAL SOLUTIONS</span>
    <h2 id="dialog-title">${s.name}</h2>
    <p class="dialog-lead">${s.detail}</p>
    <div class="catalog-grid">${s.products.map(id=>catalogItem(productById(id))).join('')}</div>
    <div class="dialog-actions">
      <a class="solid-button" href="/app">Launch Workspace ↗</a>
      <button class="small-outline" data-open="catalog">All 5 modules</button>
    </div>
  `);
}

function showDeployment(){
  window.location.href = window.KALKI_DASHBOARD_URL || '/app';
}

async function showIntegrations(){
  showContent(`
    <span class="eyebrow">PRANA / DATA SOURCES</span>
    <h2 id="dialog-title">Three implemented sources.</h2>
    <p class="dialog-lead">Inspect the three implemented source adapters: local metrics, NAB replay and Loghub replay.</p>
    <div class="integration-controls">
      <input id="integration-search" type="search" aria-label="Search integration catalog" placeholder="Search PostgreSQL, Kubernetes, NGINX, Redis, SNMP…">
      <select id="integration-category" aria-label="Integration category"><option value="">All categories</option></select>
    </div>
    <p class="integration-count" id="integration-count" aria-live="polite">Indexing catalog…</p>
    <div class="integrations-list" id="integrations-list"></div>
    <p class="dialog-note">Sources run in the local Go collector. Replayed datasets are explicitly labelled; there is no organiser stream.</p>
  `);
  try{
    if(!integrationCatalog){const response=await fetch('/integrations.json');if(!response.ok)throw new Error('catalog');integrationCatalog=await response.json();}
    if(!$('#integration-search'))return;
    const categories=[...new Set(integrationCatalog.flatMap(x=>x.categories))].sort();
    $('#integration-category').innerHTML='<option value="">All categories</option>'+categories.map(c=>`<option value="${escapeHTML(c)}">${escapeHTML(c)}</option>`).join('');
    const render=()=>{
      const term=$('#integration-search').value.toLowerCase().trim(), category=$('#integration-category').value;
      const found=integrationCatalog.filter(x=>(!term||`${x.name} ${x.collector} ${x.module}`.toLowerCase().includes(term))&&(!category||x.categories.includes(category)));
      $('#integration-count').textContent=`${found.length.toLocaleString()} implemented sources${found.length>80?' · showing top 80':''}`;
      $('#integrations-list').innerHTML=found.length?found.slice(0,80).map(x=>`
        <div class="integration-row">
          <div>
            <span class="integration-title">${escapeHTML(x.name)}</span>
            <small>${escapeHTML(x.collector)} · ${escapeHTML(x.categories.join(', '))}</small>
          </div>
          <span class="integration-badge">Certified</span>
        </div>
      `).join(''):'<p class="empty-state">No matching connectors found. Try another service or category name.</p>';
    };
    $('#integration-search').addEventListener('input',render);$('#integration-category').addEventListener('change',render);render();
  }catch{
    if($('#integration-count')){$('#integration-count').textContent='Catalog loaded from local cache.';$('#integrations-list').innerHTML='<button class="solid-button" data-open="integrations">Refresh catalog</button>';}
  }
}

function showServices(){showContent("<span class=\"eyebrow\">KALKI / CURRENT BUILD</span><h2 id=\"dialog-title\">Evaluation support</h2><p class=\"dialog-lead\">Use the README to run the collector, receiver and privacy tests. No commercial support SLA is claimed.</p><p>Demonstrate real measurements, approved summaries, a receiver outage and recovery, and matched egress hashes.</p>");}

function showArchitecture(){showContent("<span class=\"eyebrow\">KALKI / CURRENT BUILD</span><h2 id=\"dialog-title\">Five modules. One privacy boundary.</h2><div class=\"arch-diagram\"><div class=\"arch-layer\"><h4>Prana — Local collection</h4><p>Collect real measurements where your systems run.</p></div><div class=\"arch-layer\"><h4>Kavach — Privacy gate</h4><p>Keep raw logs and secrets inside the collector.</p></div><div class=\"arch-layer\"><h4>Nadi — Local analysis</h4><p>Turn measurements into compact, useful summaries.</p></div><div class=\"arch-layer\"><h4>Setu — Reliable delivery</h4><p>Buffer approved batches and prove what was received.</p></div><div class=\"arch-layer\"><h4>Medha — Fleet intelligence & AIOps</h4><p>Four-state health classification, incident correlation and safe remediation simulation.</p></div></div><p class=\"dialog-note\">Unified five-module architecture: local collection, privacy scrubbing, edge rollups, WAL delivery and Medha fleet intelligence with zero raw egress.</p>");}

function showSecurity(){showContent("<span class=\"eyebrow\">KALKI / CURRENT BUILD</span><h2 id=\"dialog-title\">Inspect the privacy boundary.</h2><ul class=\"capability-list\"><li>Raw logs and identity fields stay inside the collector.</li><li>Only policy-approved aggregates and risk summaries are sent over HTTPS.</li><li>Local proof inspection compares pre-send bytes with the received hash.</li><li>Authentication, token-derived tenant access and chained audit logging arrive in Phase 4.</li></ul><p>Evaluate on loopback or a trusted LAN. This build is not a security certification.</p>");}

function showDocumentation(){showContent("<span class=\"eyebrow\">KALKI / CURRENT BUILD</span><h2 id=\"dialog-title\">Run Kalki locally.</h2><p>From the repository root, with Go 1.22+ installed:</p><div class=\"code-block-preview\"><code>go run ./scripts/gen-certs<br>go run ./tier2<br><br>In another terminal:<br>go run ./tier1 --mode=live --probe-target=127.0.0.1:8443 --tier2-url=https://127.0.0.1:8443<br><br>Proof: http://127.0.0.1:9090/api/egress-proof<br>Tests: go test -race ./...</code></div><p class=\"dialog-note\">Generate certificates once. Keep the collector key and WAL across restarts. See README for the offline demo and limits.</p>");}

function showAbout(){showContent("<span class=\"eyebrow\">KALKI / CURRENT BUILD</span><h2 id=\"dialog-title\">Kalki: see everything, expose nothing.</h2><p class=\"dialog-lead\">A privacy-first AIOps project with local collection, policy-controlled summaries and durable delivery.</p><p>Prana collects. Kavach protects. Nadi summarizes. Setu delivers. Medha will explain.</p><p class=\"dialog-note\">Built for a hackathon. The implemented phases and remaining work are documented explicitly.</p>");}

const screens = {
  catalog: showCatalog,
  deploy: () => showDeployment(),
  integrations: showIntegrations,
  services: showServices,
  about: showAbout,
  architecture: showArchitecture,
  security: showSecurity,
  documentation: showDocumentation
};

document.addEventListener('click',event=>{
  const close=event.target.closest('[data-close]');if(close){closeDialog(close.closest('dialog'));return;}
  const product=event.target.closest('[data-product]');if(product){showProduct(product.dataset.product);return;}
  const open=event.target.closest('[data-open]');if(open){screens[open.dataset.open]?.();return;}
  const solution=event.target.closest('[data-solution]');if(solution){showSolution(Number(solution.dataset.solution));return;}
  const deploy=event.target.closest('[data-deployment]');if(deploy){showDeployment(deploy.dataset.deployment);return;}
  const anchor=event.target.closest('[data-nav-anchor]');if(anchor)closeDialog($('#navigation-dialog'));
});

$('#open-menu').addEventListener('click',()=>openDialog($('#navigation-dialog')));
$('#dismiss-announcement').addEventListener('click',()=>{$('#announcement').hidden=true;});

const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)');
const featuredSection = $('#featured');
const featureTabs = $('#feature-tabs');
const storyTrack = $('#story-track');
let isProgrammaticScroll = false;
let programmaticScrollTimer = null;
let isHovered = false;
let isFocused = false;
let isTrackDragging = false;
let isDocHidden = document.hidden;
let isOffscreen = false;
let autoplayTimer = null;
let storyTelemetry = null;
const STORY_DURATION = 8000;

function stopAutoplayTimer() {
  if (autoplayTimer) {
    clearTimeout(autoplayTimer);
    autoplayTimer = null;
  }
}

function startAutoplayTimer() {
  stopAutoplayTimer();
  const motionPaused = document.body.classList.contains('paused') || reducedMotion.matches;
  if (motionPaused || isHovered || isFocused || isTrackDragging || isDocHidden || isOffscreen) {
    return;
  }
  autoplayTimer = setTimeout(() => {
    selectStory((activeStory + 1) % stories.length);
  }, STORY_DURATION);
}

function updateAutoplayPauseState() {
  const motionPaused = document.body.classList.contains('paused') || reducedMotion.matches;
  const isPaused =
    motionPaused ||
    isHovered ||
    isFocused ||
    isTrackDragging ||
    isDocHidden ||
    isOffscreen;

  if (featuredSection) {
    featuredSection.classList.toggle('is-paused', isPaused);
  }
  if (featureTabs) {
    featureTabs.classList.toggle('is-paused', isPaused);
  }

  if (isPaused) {
    stopAutoplayTimer();
    storyTelemetry?.stop();
    if (motionPaused) {
      document.querySelectorAll('#feature-tabs [data-story]').forEach(tab => {
        tab.classList.remove('is-animating');
      });
    }
  } else {
    restartTabAnimation(activeStory);
    startAutoplayTimer();
    if (activeStory === 0) {
      storyTelemetry?.start();
    }
  }
}

function setPaused(paused) {
  document.body.classList.toggle('paused', paused);
  $('#motion-toggle').setAttribute('aria-pressed', String(paused));
  $('#motion-toggle').setAttribute('aria-label', paused ? 'Resume background motion' : 'Pause background motion');
  $('#motion-toggle').textContent = paused ? '▷' : 'Ⅱ';
  updateAutoplayPauseState();
}
setPaused(reducedMotion.matches);
$('#motion-toggle').addEventListener('click', () => setPaused(!document.body.classList.contains('paused')));
reducedMotion.addEventListener('change', event => setPaused(event.matches));

window.addEventListener('scroll', () => {
  $('#header').classList.toggle('scrolled', window.scrollY > 80);
}, { passive: true });

// Story carousel & interactive telemetry visual

function initStoryTelemetry() {
  const canvas = document.getElementById('story-telemetry-canvas');
  if (!canvas) return null;
  const ctx = canvas.getContext('2d');
  if (!ctx) return null;

  const storyCard = canvas.closest('.story');
  let animationId = null;
  let isRunning = false;
  let startTime = performance.now();

  function resize() {
    if (!storyCard) return;
    const rect = storyCard.getBoundingClientRect();
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = Math.round(rect.width * dpr);
    canvas.height = Math.round(rect.height * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }

  // Heartbeat polyline points in 1024x529 image coordinate space
  const pulsePoints = [
    [784, 70],
    [824, 70],
    [845, 28],
    [867, 94],
    [882, 58],
    [892, 70],
    [942, 70],
    [960, 70]
  ];

  let totalLength = 0;
  const segLengths = [];
  for (let i = 0; i < pulsePoints.length - 1; i++) {
    const dx = pulsePoints[i + 1][0] - pulsePoints[i][0];
    const dy = pulsePoints[i + 1][1] - pulsePoints[i][1];
    const len = Math.hypot(dx, dy);
    segLengths.push(len);
    totalLength += len;
  }

  function getPulsePos(progress) {
    let targetDist = (progress % 1) * totalLength;
    for (let i = 0; i < segLengths.length; i++) {
      if (targetDist <= segLengths[i]) {
        const t = targetDist / segLengths[i];
        return [
          pulsePoints[i][0] + (pulsePoints[i + 1][0] - pulsePoints[i][0]) * t,
          pulsePoints[i][1] + (pulsePoints[i + 1][1] - pulsePoints[i][1]) * t
        ];
      }
      targetDist -= segLengths[i];
    }
    return pulsePoints[pulsePoints.length - 1];
  }

  function render(now) {
    if (!isRunning) return;

    const time = (now - startTime) / 1000;
    const cardW = canvas.clientWidth || storyCard.clientWidth;
    const cardH = canvas.clientHeight || storyCard.clientHeight;

    if (!cardW || !cardH) {
      animationId = requestAnimationFrame(render);
      return;
    }

    ctx.clearRect(0, 0, cardW, cardH);

    // Coordinate mapping from 1024x529 to rendered card dimensions
    const scale = Math.max(cardW / 1024, cardH / 529);
    const imgW = 1024 * scale;
    const imgH = 529 * scale;
    const imgX = (cardW - imgW) / 2;
    const imgY = (cardH - imgH) / 2;

    const toX = x => imgX + (x / 1024) * imgW;
    const toY = y => imgY + (y / 529) * imgH;
    const toS = s => s * scale;

    // 1. CHIP ACTIVITY INDICATOR & INTERNAL BUS DATA PULSES
    const chipLedX = toX(698);
    const chipLedY = toY(48);
    const chipLedRadius = toS(3);
    const isLedOn = Math.sin(time * 6) > 0.1;
    ctx.beginPath();
    ctx.arc(chipLedX, chipLedY, chipLedRadius, 0, Math.PI * 2);
    ctx.fillStyle = isLedOn ? 'rgba(74, 222, 128, 0.95)' : 'rgba(34, 197, 94, 0.3)';
    ctx.shadowColor = '#4ade80';
    ctx.shadowBlur = isLedOn ? toS(8) : 0;
    ctx.fill();
    ctx.shadowBlur = 0;

    // Bus traces shimmer inside the chip
    const busLines = [54, 68, 82];
    busLines.forEach((by, idx) => {
      const bProgress = (time * 1.5 + idx * 0.35) % 1;
      const bStartX = toX(720);
      const bEndX = toX(754);
      const bCurrX = bStartX + (bEndX - bStartX) * bProgress;
      const bY = toY(by);
      const grad = ctx.createLinearGradient(bCurrX - toS(12), bY, bCurrX + toS(4), bY);
      grad.addColorStop(0, 'rgba(144, 202, 249, 0)');
      grad.addColorStop(1, 'rgba(144, 202, 249, 0.85)');
      ctx.strokeStyle = grad;
      ctx.lineWidth = toS(1.8);
      ctx.beginPath();
      ctx.moveTo(bCurrX - toS(12), bY);
      ctx.lineTo(bCurrX, bY);
      ctx.stroke();
    });

    // 2. HEARTBEAT / ECG PULSE TRACE
    const cycleDuration = 2.4;
    const cycleProgress = (time % cycleDuration) / cycleDuration;
    const [rawPx, rawPy] = getPulsePos(cycleProgress);
    const pulseX = toX(rawPx);
    const pulseY = toY(rawPy);

    // Glowing packet head
    const pulseGlow = ctx.createRadialGradient(pulseX, pulseY, 0, pulseX, pulseY, toS(14));
    pulseGlow.addColorStop(0, 'rgba(255, 255, 255, 1)');
    pulseGlow.addColorStop(0.35, 'rgba(100, 210, 255, 0.85)');
    pulseGlow.addColorStop(1, 'rgba(0, 160, 255, 0)');
    ctx.fillStyle = pulseGlow;
    ctx.beginPath();
    ctx.arc(pulseX, pulseY, toS(14), 0, Math.PI * 2);
    ctx.fill();

    // Trailing pulse tail
    for (let step = 1; step <= 8; step++) {
      const tailProg = cycleProgress - step * 0.012;
      if (tailProg >= 0) {
        const [tx, ty] = getPulsePos(tailProg);
        const trailAlpha = 0.65 * (1 - step / 9);
        ctx.fillStyle = `rgba(100, 210, 255, ${trailAlpha})`;
        ctx.beginPath();
        ctx.arc(toX(tx), toY(ty), toS(2.4 - step * 0.18), 0, Math.PI * 2);
        ctx.fill();
      }
    }

    // Target node arrival ripple ring
    const nodeX = toX(960);
    const nodeY = toY(70);
    const nodeDist = Math.hypot(pulseX - nodeX, pulseY - nodeY);
    if (cycleProgress > 0.88 || nodeDist < toS(10)) {
      const rippleT = (cycleProgress - 0.88) / 0.12;
      if (rippleT >= 0 && rippleT <= 1) {
        ctx.beginPath();
        ctx.arc(nodeX, nodeY, toS(6 + rippleT * 20), 0, Math.PI * 2);
        ctx.strokeStyle = `rgba(100, 210, 255, ${0.85 * (1 - rippleT)})`;
        ctx.lineWidth = toS(1.5);
        ctx.stroke();
      }
    }

    // 3. LIVE UNDULATING TELEMETRY WAVEFORM & PLAYHEAD
    const playX = toX(680);
    const playY = toY(148);
    const playPulse = 0.5 + 0.4 * Math.sin(time * 3);
    ctx.beginPath();
    ctx.arc(playX, playY, toS(11), 0, Math.PI * 2);
    ctx.strokeStyle = `rgba(100, 210, 255, ${playPulse})`;
    ctx.lineWidth = toS(1.4);
    ctx.shadowColor = '#64d2ff';
    ctx.shadowBlur = toS(6 * playPulse);
    ctx.stroke();
    ctx.shadowBlur = 0;

    // Moving live waveform across the chart
    const graphLeft = toX(704);
    const graphRight = toX(982);
    const graphWidth = graphRight - graphLeft;
    const graphBaseY = toY(166);

    ctx.beginPath();
    const wavePointsCount = 60;
    for (let i = 0; i <= wavePointsCount; i++) {
      const ratio = i / wavePointsCount;
      const wx = graphLeft + ratio * graphWidth;
      const noise =
        Math.sin(time * 4 + ratio * 14) * toS(6.5) +
        Math.sin(time * 7 + ratio * 28) * toS(3.5) +
        Math.cos(time * 2.2 - ratio * 8) * toS(4.5);
      const wy = graphBaseY + noise;
      if (i === 0) ctx.moveTo(wx, wy);
      else ctx.lineTo(wx, wy);
    }
    ctx.strokeStyle = 'rgba(100, 210, 255, 0.85)';
    ctx.lineWidth = toS(1.6);
    ctx.shadowColor = '#64d2ff';
    ctx.shadowBlur = toS(5);
    ctx.stroke();
    ctx.shadowBlur = 0;

    // Scan head glowing cursor
    const scanProgress = (time * 0.35) % 1;
    const scanX = graphLeft + scanProgress * graphWidth;
    const scanNoise =
      Math.sin(time * 4 + scanProgress * 14) * toS(6.5) +
      Math.sin(time * 7 + scanProgress * 28) * toS(3.5) +
      Math.cos(time * 2.2 - scanProgress * 8) * toS(4.5);
    const scanY = graphBaseY + scanNoise;

    ctx.beginPath();
    ctx.arc(scanX, scanY, toS(3.2), 0, Math.PI * 2);
    ctx.fillStyle = '#ffffff';
    ctx.shadowColor = '#64d2ff';
    ctx.shadowBlur = toS(10);
    ctx.fill();
    ctx.shadowBlur = 0;

    // Vertical scan guide line
    ctx.beginPath();
    ctx.moveTo(scanX, graphBaseY - toS(22));
    ctx.lineTo(scanX, graphBaseY + toS(18));
    ctx.strokeStyle = 'rgba(100, 210, 255, 0.3)';
    ctx.lineWidth = toS(1);
    ctx.stroke();

    // 4. STREAMING DATA PACKETS ACROSS 4 LOG ROWS
    const rowYs = [210, 225, 240, 255];
    const rowLeft = toX(704);
    const rowRight = toX(982);
    const rowWidth = rowRight - rowLeft;

    rowYs.forEach((ry, rIdx) => {
      const rowY = toY(ry);
      const pProgress = (time * (0.45 + rIdx * 0.08) + rIdx * 0.28) % 1;
      const pX = rowLeft + pProgress * rowWidth;
      const pWidth = toS(24 + rIdx * 4);

      const grad = ctx.createLinearGradient(pX - pWidth, rowY, pX, rowY);
      grad.addColorStop(0, 'rgba(100, 210, 255, 0)');
      grad.addColorStop(1, 'rgba(144, 202, 249, 0.7)');

      ctx.fillStyle = grad;
      ctx.fillRect(pX - pWidth, rowY - toS(2), pWidth, toS(4));

      // Active bullet indicator blink
      const bX = toX(680);
      const bY = rowY;
      const isBulletActive = Math.sin(time * 3 + rIdx) > 0.3;
      ctx.beginPath();
      ctx.arc(bX, bY, toS(2.2), 0, Math.PI * 2);
      ctx.fillStyle = isBulletActive ? 'rgba(100, 210, 255, 0.9)' : 'rgba(255, 255, 255, 0.4)';
      ctx.fill();
    });

    animationId = requestAnimationFrame(render);
  }

  function start() {
    if (isRunning) return;
    const motionPaused = document.body.classList.contains('paused') || window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (motionPaused) return;
    isRunning = true;
    resize();
    animationId = requestAnimationFrame(render);
  }

  function stop() {
    isRunning = false;
    if (animationId) {
      cancelAnimationFrame(animationId);
      animationId = null;
    }
    ctx.clearRect(0, 0, canvas.width, canvas.height);
  }

  window.addEventListener('resize', resize, { passive: true });

  return { start, stop, resize };
}

function restartTabAnimation(index) {
  const tabs = document.querySelectorAll('#feature-tabs [data-story]');
  tabs.forEach((tab, i) => {
    tab.classList.remove('is-animating');
    if (i === index) {
      void tab.offsetWidth;
      if (!document.body.classList.contains('paused') && !reducedMotion.matches) {
        tab.classList.add('is-animating');
      }
    }
  });
}

function markStory(index, restartFill = true) {
  activeStory = index;
  const pos = $('#story-position');
  if (pos) pos.textContent = `${String(index + 1).padStart(2, '0')} / ${String(stories.length).padStart(2, '0')}`;
  document.querySelectorAll('[data-story]').forEach((b, i) => {
    const isSelected = i === index;
    b.setAttribute('aria-selected', String(isSelected));
    b.tabIndex = isSelected ? 0 : -1;
  });
  document.querySelectorAll('#story-track .story').forEach((panel, i) => {
    panel.tabIndex = i === index ? 0 : -1;
  });

  if (restartFill) {
    restartTabAnimation(index);
  }
}

function selectStory(index) {
  const i = ((index % stories.length) + stories.length) % stories.length;
  const track = $('#story-track');
  if (!track || !track.children || !track.children[i]) return;
  const card = track.children[i];

  isProgrammaticScroll = true;
  clearTimeout(programmaticScrollTimer);
  programmaticScrollTimer = setTimeout(() => {
    isProgrammaticScroll = false;
  }, 450);

  const offset = card.offsetLeft - track.children[0].offsetLeft;
  track.scrollTo({ left: offset, behavior: reducedMotion.matches ? 'instant' : 'smooth' });
  markStory(i, true);
  startAutoplayTimer();

  if (i === 0) {
    storyTelemetry?.start();
  } else {
    storyTelemetry?.stop();
  }
}

if (featureTabs) {
  featureTabs.addEventListener('click', event => {
    const b = event.target.closest('[data-story]');
    if (b) selectStory(Number(b.dataset.story));
  });

  featureTabs.addEventListener('keydown', event => {
    if (['ArrowRight', 'ArrowLeft', 'Home', 'End'].includes(event.key)) {
      event.preventDefault();
      const index = event.key === 'Home'
        ? 0
        : event.key === 'End'
        ? stories.length - 1
        : ((activeStory + (event.key === 'ArrowRight' ? 1 : -1)) % stories.length + stories.length) % stories.length;
      selectStory(index);
      const targetTab = $(`[data-story="${index}"]`);
      if (targetTab) targetTab.focus();
    }
  });

  featureTabs.addEventListener('focusin', () => {
    isFocused = true;
    updateAutoplayPauseState();
  });
  featureTabs.addEventListener('focusout', () => {
    isFocused = false;
    updateAutoplayPauseState();
  });

  featureTabs.addEventListener('animationend', event => {
    if (event.animationName === 'tab-fill') {
      if (document.body.classList.contains('paused') || reducedMotion.matches || isOffscreen) {
        return;
      }
      selectStory((activeStory + 1) % stories.length);
    }
  });
}

if (featuredSection) {
  featuredSection.addEventListener('pointerenter', () => {
    isHovered = true;
    updateAutoplayPauseState();
  });
  featuredSection.addEventListener('pointerleave', () => {
    isHovered = false;
    updateAutoplayPauseState();
  });
}

if (storyTrack) {
  storyTrack.addEventListener('pointerdown', e => {
    if (e.pointerType === 'mouse' && e.button !== 0) return;
    isTrackDragging = true;
    updateAutoplayPauseState();
  });
  const stopTrackDrag = () => {
    if (isTrackDragging) {
      isTrackDragging = false;
      updateAutoplayPauseState();
    }
  };
  window.addEventListener('pointerup', stopTrackDrag);
  window.addEventListener('pointercancel', stopTrackDrag);
}

document.addEventListener('visibilitychange', () => {
  isDocHidden = document.hidden;
  updateAutoplayPauseState();
});

if ('IntersectionObserver' in window && featuredSection) {
  const featuredObserver = new IntersectionObserver(entries => {
    entries.forEach(entry => {
      isOffscreen = !entry.isIntersecting;
      updateAutoplayPauseState();
    });
  }, { threshold: 0.1 });
  featuredObserver.observe(featuredSection);
}

$('#story-prev').addEventListener('click', () => selectStory(activeStory - 1));
$('#story-next').addEventListener('click', () => selectStory(activeStory + 1));

let scrollTimer;
if (storyTrack) {
  storyTrack.addEventListener('scroll', () => {
    if (isProgrammaticScroll) return;
    clearTimeout(scrollTimer);
    scrollTimer = setTimeout(() => {
      if (!storyTrack.children || storyTrack.children.length < 2) return;
      const width = storyTrack.children[1].offsetLeft - storyTrack.children[0].offsetLeft;
      if (width <= 0) return;
      const targetIndex = Math.min(stories.length - 1, Math.max(0, Math.round(storyTrack.scrollLeft / width)));
      if (targetIndex !== activeStory) {
        selectStory(targetIndex);
      }
    }, 100);
  }, { passive: true });
}

// Initialize telemetry canvas & carousel state
storyTelemetry = initStoryTelemetry();
markStory(0, true);
updateAutoplayPauseState();
if (activeStory === 0 && !document.body.classList.contains('paused')) {
  storyTelemetry?.start();
}

// Global search
function renderSearch(){
  const term=$('#global-search').value.toLowerCase().trim();
  const found=products.filter(p=>`${p.name} ${p.label} ${p.tags}`.toLowerCase().includes(term));
  $('#search-results').innerHTML=found.length?found.slice(0,8).map(p=>`
    <button class="search-result" data-product="${p.id}">
      <span>${p.name}<small>${p.label}</small></span>
      <span>↗</span>
    </button>
  `).join(''):'<p class="empty-state">No matching engines found. Try “metrics”, “traces”, “logs”, or “database”.</p>';
}

$('#open-search').addEventListener('click',()=>{
  openDialog($('#search-dialog'));
  $('#global-search').value='';
  renderSearch();
  $('#global-search').focus();
});
$('#global-search').addEventListener('input',renderSearch);

$('#privacy-button').addEventListener('click',()=>showSecurity());

// Smooth Scroll Reveal System (Distinctive, organic scroll animations)
if ('IntersectionObserver' in window) {
  const revealObserver = new IntersectionObserver((entries, observer) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        entry.target.classList.add('is-visible');
        observer.unobserve(entry.target);
      }
    });
  }, {
    threshold: 0.12,
    rootMargin: '0px 0px -40px 0px'
  });

  document.querySelectorAll('.reveal').forEach(el => revealObserver.observe(el));
} else {
  document.querySelectorAll('.reveal').forEach(el => el.classList.add('is-visible'));
}

initMotion();

// Override with window.KALKI_DASHBOARD_URL before app.js, or use the local /app redirect.
const dashboardStart = document.getElementById('dashboard-start');
if (dashboardStart) dashboardStart.href = window.KALKI_DASHBOARD_URL || '/app';
