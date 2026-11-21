import { initMotion, setupDialogReveals } from './motion.js';

const products = [
  {
    "id": "prana",
    "name": "Prana",
    "label": "Local collection",
    "symbol": "◉",
    "description": "Collect real measurements where your systems run.",
    "features": [
      "Live OS metrics and TCP probes",
      "NAB and Loghub replay with explicit source labels",
      "Go collector for Windows, Linux and macOS"
    ],
    "image": "infrastructure",
    "tags": "collection cpu memory network live replay"
  },
  {
    "id": "kavach",
    "name": "Kavach",
    "label": "Privacy gate",
    "symbol": "◇",
    "description": "Keep raw logs and secrets inside the collector.",
    "features": [
      "Secret scrubbing and per-type counts",
      "Tenant-keyed HMAC pseudonyms",
      "Fail-closed outbound allowlist"
    ],
    "image": "engineers",
    "tags": "privacy secrets redaction policy pseudonyms"
  },
  {
    "id": "nadi",
    "name": "Nadi",
    "label": "Local analysis",
    "symbol": "≋",
    "description": "Turn measurements into compact, useful summaries.",
    "features": [
      "Five-second min, average, max and p95 rollups",
      "Rolling 60-second failed-login detection",
      "Separate live and replayed summaries"
    ],
    "image": "cloud",
    "tags": "metrics rollups statistics security brute force"
  },
  {
    "id": "setu",
    "name": "Setu",
    "label": "Reliable delivery",
    "symbol": "⋈",
    "description": "Buffer approved batches and prove what was received.",
    "features": [
      "Append-only disk WAL and HTTPS retries",
      "Idempotent regional ingest with bounded memory rings",
      "Local egress proof and exact-byte hash matching"
    ],
    "image": "infrastructure",
    "tags": "delivery wal outage retry acknowledgements proof"
  },
  {
    "id": "medha",
    "name": "Medha",
    "label": "AIOps console · planned",
    "symbol": "✳",
    "description": "Explain fleet health and guide the next safe action.",
    "features": [
      "Planned: adaptive anomaly detection and correlation",
      "Planned: Excellent, Good, Fair and Poor health states",
      "Planned: live console and safe remediation simulator"
    ],
    "image": "platform",
    "tags": "planned aiops health incidents dashboard explanations"
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
    "product": "prana",
    "label": "PRANA / LOCAL COLLECTION",
    "title": "Collect real measurements where your systems run.",
    "image": "infrastructure"
  },
  {
    "product": "kavach",
    "label": "KAVACH / PRIVACY GATE",
    "title": "Keep raw logs and secrets inside the collector.",
    "image": "engineers"
  },
  {
    "product": "nadi",
    "label": "NADI / LOCAL ANALYSIS",
    "title": "Turn measurements into compact, useful summaries.",
    "image": "cloud"
  },
  {
    "product": "setu",
    "label": "SETU / RELIABLE DELIVERY",
    "title": "Buffer approved batches and prove what was received.",
    "image": "infrastructure"
  },
  {
    "product": "medha",
    "label": "MEDHA / AIOPS CONSOLE · PLANNED",
    "title": "Explain fleet health and guide the next safe action.",
    "image": "platform"
  }
];
const deployments = {
  "self-hosted": {
    "label": "Local evaluation",
    "title": "Run Kalki on your own machines.",
    "description": "Collectors send approved summaries over HTTPS to a regional Go receiver. No database or external SaaS service is required.",
    "items": [
      "Cache dependencies and datasets before disconnecting",
      "Keep raw data and tenant keys on the collectors",
      "Authentication and the live console remain upcoming phases"
    ]
  },
  "cloud": {
    "label": "Regional LAN",
    "title": "Run Kalki on your own machines.",
    "description": "Collectors send approved summaries over HTTPS to a regional Go receiver. No database or external SaaS service is required.",
    "items": [
      "Cache dependencies and datasets before disconnecting",
      "Keep raw data and tenant keys on the collectors",
      "Authentication and the live console remain upcoming phases"
    ]
  },
  "private": {
    "label": "Team LAN",
    "title": "Run Kalki on your own machines.",
    "description": "Collectors send approved summaries over HTTPS to a regional Go receiver. No database or external SaaS service is required.",
    "items": [
      "Cache dependencies and datasets before disconnecting",
      "Keep raw data and tenant keys on the collectors",
      "Authentication and the live console remain upcoming phases"
    ]
  },
  "isolated": {
    "label": "Offline LAN",
    "title": "Run Kalki on your own machines.",
    "description": "Collectors send approved summaries over HTTPS to a regional Go receiver. No database or external SaaS service is required.",
    "items": [
      "Cache dependencies and datasets before disconnecting",
      "Keep raw data and tenant keys on the collectors",
      "Authentication and the live console remain upcoming phases"
    ]
  }
};

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
$('#feature-tabs').innerHTML = stories.map((s,i)=>`<button role="tab" id="story-tab-${i}" aria-selected="${i===0}" aria-controls="story-${i}" tabindex="${i===0?0:-1}" data-story="${i}">${productById(s.product).name}</button>`).join('');
$('#story-track').innerHTML = stories.map((s,i)=>`<button class="story" id="story-${i}" data-product="${s.product}" aria-label="Explore ${productById(s.product).name}"><img src="/assets/${s.image}.webp" alt="" loading="lazy"><div class="story-copy"><span class="story-label">${s.label}</span><h3>${s.title}</h3></div><div class="story-bottom"><span>EXPLORE THE CAPABILITY</span><span aria-hidden="true">↗</span></div></button>`).join('');
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

function showProduct(id){
  const p=productById(id); if(!p)return;
  showContent(`
    <span class="eyebrow">KALKI / ${p.label.toUpperCase()}</span>
    <h2 id="dialog-title">${p.name}</h2>
    <p class="dialog-lead">${p.description}</p>
    <img class="detail-hero" src="/assets/${p.image}.webp" alt="">
    <ul class="capability-list">${p.features.map(f=>`<li>${f}</li>`).join('')}</ul>
    <p class="dialog-note">One of five Kalki modules. Current capabilities and planned work are listed explicitly.</p>
    <div class="dialog-actions">
      <button class="solid-button" data-open="deploy">Plan deployment</button>
      <button class="small-outline" data-open="catalog">All 5 modules</button>
    </div>
  `);
}

function showCatalog(){
  showContent(`
    <span class="eyebrow">THE KALKI PLATFORM</span>
    <h2 id="dialog-title">Every signal.<br>A unified operational mesh.</h2>
    <p class="dialog-lead">Explore the five modules and distinguish the implemented delivery pipeline from upcoming AIOps features.</p>
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
    <p class="dialog-note">The collector and receiver run locally. Authentication and the live console remain planned.</p>
    <button class="solid-button" data-open="deploy">Explore deployment options</button>
  `);
}

function showDeployment(selected='self-hosted'){
  showContent(`
    <span class="eyebrow">KALKI / DEPLOYMENT ARCHITECTURE</span>
    <h2 id="dialog-title">Deploy in your environment.</h2>
    <p class="dialog-lead">Select your operating model. Generate an architectural deployment brief for your infrastructure team.</p>
    <div class="deployment-tabs" role="tablist" aria-label="Deployment model">
      ${Object.entries(deployments).map(([id,d])=>`<button role="tab" aria-selected="${id===selected}" tabindex="${id===selected?0:-1}" data-deploy-tab="${id}" aria-controls="deployment-panel">${d.label}</button>`).join('')}
    </div>
    <div id="deployment-panel" class="deployment-panel" role="tabpanel"></div>
  `);
  renderDeployment(selected);
}

function renderDeployment(id){
  const d=deployments[id]; if(!d)return;
  document.querySelectorAll('[data-deploy-tab]').forEach(b=>{b.setAttribute('aria-selected',String(b.dataset.deployTab===id));b.tabIndex=b.dataset.deployTab===id?0:-1;});
  $('#deployment-panel').innerHTML=`
    <h3>${d.title}</h3>
    <p>${d.description}</p>
    <ul class="capability-list">${d.items.map(x=>`<li>${x}</li>`).join('')}</ul>
    <form class="deployment-form" id="deployment-form">
      <label>Environment name<input name="environment" placeholder="e.g. Production · Core-Fleet-01" maxlength="100" required></label>
      <label>Initial fleet size<select name="fleet"><option>1–10 nodes</option><option>11–100 nodes</option><option>101–500 nodes</option><option>500+ nodes</option></select></label>
      <label>Primary focus<select name="focus"><option>Infrastructure & metrics</option><option>Privacy and security summaries</option><option>Networks & edge</option><option>Offline delivery evaluation</option></select></label>
      <label>Operating system<select name="os"><option>Linux</option><option>Windows</option><option>macOS</option><option>Heterogeneous / Mixed</option></select></label>
      <button class="solid-button" type="submit">Generate deployment brief</button>
    </form>
    <div id="plan-output" aria-live="polite"></div>
    <p class="dialog-note">Data boundary guarantee: configuration briefs are generated entirely in-browser. No customer specifications leave your machine.</p>
  `;
  $('#deployment-form').addEventListener('submit',event=>{
    event.preventDefault();const data=new FormData(event.target);
    const brief='KALKI EVALUATION BRIEF\n\nEnvironment: '+data.get('environment')+'\nDeployment: '+d.label+'\nFleet: '+data.get('fleet')+'\n\n'+d.description+'\n\nCurrent: collection, privacy, rollups, WAL delivery and egress proof.\nUpcoming: authentication, adaptive AIOps and live console.\nFollow the repository README for commands and limits.';
    $('#plan-output').innerHTML=`<div class="plan-result"><strong>Brief ready for ${escapeHTML(data.get('environment'))}.</strong><p>${d.label} · ${escapeHTML(data.get('fleet'))} · ${escapeHTML(data.get('focus'))}</p><button class="text-link" id="download-plan">Download deployment brief (.txt) ↗</button></div>`;
    $('#download-plan').addEventListener('click',()=>download('kalki-deployment-brief.txt',brief));
  });
}

function download(name,text){const blob=new Blob([text],{type:'text/plain;charset=utf-8'});const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}

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

function showArchitecture(){showContent("<span class=\"eyebrow\">KALKI / CURRENT BUILD</span><h2 id=\"dialog-title\">Five modules. One privacy boundary.</h2><div class=\"arch-diagram\"><div class=\"arch-layer\"><h4>Prana — Local collection</h4><p>Collect real measurements where your systems run.</p></div><div class=\"arch-layer\"><h4>Kavach — Privacy gate</h4><p>Keep raw logs and secrets inside the collector.</p></div><div class=\"arch-layer\"><h4>Nadi — Local analysis</h4><p>Turn measurements into compact, useful summaries.</p></div><div class=\"arch-layer\"><h4>Setu — Reliable delivery</h4><p>Buffer approved batches and prove what was received.</p></div><div class=\"arch-layer\"><h4>Medha — AIOps console · planned</h4><p>Explain fleet health and guide the next safe action.</p></div></div><p class=\"dialog-note\">Phases 1–3 are implemented. Medha remains planned; this site is a presentation, not the live console.</p>");}

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
  const tab=event.target.closest('[data-deploy-tab]');if(tab){renderDeployment(tab.dataset.deployTab);return;}
  const anchor=event.target.closest('[data-nav-anchor]');if(anchor)closeDialog($('#navigation-dialog'));
});

$('#open-menu').addEventListener('click',()=>openDialog($('#navigation-dialog')));
$('#dismiss-announcement').addEventListener('click',()=>{$('#announcement').hidden=true;});

const reducedMotion=matchMedia('(prefers-reduced-motion: reduce)');
function setPaused(paused){
  document.body.classList.toggle('paused',paused);
  $('#motion-toggle').setAttribute('aria-pressed',String(paused));
  $('#motion-toggle').setAttribute('aria-label',paused?'Resume background motion':'Pause background motion');
  $('#motion-toggle').textContent=paused?'▷':'Ⅱ';
}
setPaused(reducedMotion.matches);
$('#motion-toggle').addEventListener('click',()=>setPaused(!document.body.classList.contains('paused')));
reducedMotion.addEventListener('change',event=>setPaused(event.matches));

window.addEventListener('scroll',()=>{
  $('#header').classList.toggle('scrolled',window.scrollY>80);
},{passive:true});

// Story carousel
function markStory(index){
  activeStory=index;
  $('#story-position').textContent=`${String(index+1).padStart(2,'0')} / 06`;
  document.querySelectorAll('[data-story]').forEach((b,i)=>{b.setAttribute('aria-selected',String(i===index));b.tabIndex=i===index?0:-1;});
  $('#story-prev').disabled=index===0;
  $('#story-next').disabled=index===stories.length-1;
}

function selectStory(index){
  const i=Math.max(0,Math.min(stories.length-1,index));
  const track=$('#story-track');
  const card=track.children[i];
  track.scrollTo({left:card.offsetLeft-track.children[0].offsetLeft,behavior:reducedMotion.matches?'instant':'smooth'});
  markStory(i);
}

$('#feature-tabs').addEventListener('click',event=>{
  const b=event.target.closest('[data-story]');
  if(b)selectStory(Number(b.dataset.story));
});

$('#feature-tabs').addEventListener('keydown',event=>{
  if(['ArrowRight','ArrowLeft','Home','End'].includes(event.key)){
    event.preventDefault();
    const index=event.key==='Home'?0:event.key==='End'?stories.length-1:Math.max(0,Math.min(stories.length-1,activeStory+(event.key==='ArrowRight'?1:-1)));
    selectStory(index);
    $(`[data-story="${index}"]`).focus();
  }
});

$('#story-prev').addEventListener('click',()=>selectStory(activeStory-1));
$('#story-next').addEventListener('click',()=>selectStory(activeStory+1));

let scrollTimer;
$('#story-track').addEventListener('scroll',()=>{
  clearTimeout(scrollTimer);
  scrollTimer=setTimeout(()=>{
    const t=$('#story-track');
    const width=t.children[1].offsetLeft-t.children[0].offsetLeft;
    markStory(Math.min(stories.length-1,Math.round(t.scrollLeft/width)));
  },180);
},{passive:true});
markStory(0);

// Keyboard nav in deployment tabs
document.addEventListener('keydown',event=>{
  const current=event.target.closest('[data-deploy-tab]');
  if(!current||!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return;
  event.preventDefault();
  const tabs=[...document.querySelectorAll('[data-deploy-tab]')];
  const i=tabs.indexOf(current);
  const next=event.key==='Home'?0:event.key==='End'?tabs.length-1:(i+(event.key==='ArrowRight'?1:-1)+tabs.length)%tabs.length;
  renderDeployment(tabs[next].dataset.deployTab);
  tabs[next].focus();
});

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
