import React from 'react';
import type {Point,Session} from './lib/types';
import {sourceName,utc} from './lib/data';
import {OpsGraphic} from './OpsGraphic';

const titles:Record<string,string>={
 'Fleet Overview':'The Local Operations Layer for Private Fleet Telemetry',
 'Incidents':'Every Incident, Traced to the Collector That Saw It',
 'Egress Proof':'Proof That Raw Data Never Left Your Boundary',
 'Audit Log':'A Hash-Linked Record of Every Action',
 'Demo Lab':'Controlled Scenarios. Observable Outcomes.',
 'Validation':'Measure the Integrity of Your Operations',
 'Limitations':'Know the Boundaries of Your Operations Layer',
};
const mainViews=['Fleet Overview','Incidents','Egress Proof','Audit Log'];
const otherViews=['Demo Lab','Validation','Limitations'];

export const ConsoleHero = React.memo(function ConsoleHero({tab,onTab,active}:{tab:string;onTab:(tab:string)=>void;active:number}){
 const all=[...mainViews,...otherViews];
 function step(direction:number){onTab(all[(all.indexOf(tab)+direction+all.length)%all.length]);}
 return <section className="ops-hero" aria-label="Operations navigation">
  <h1>{titles[tab]} <span aria-hidden="true">↗</span></h1>
  <div className="ops-side"><nav className="ops-views" aria-label="Console views">{mainViews.map(name=><button key={name} className={tab===name?'on':''} aria-current={tab===name?'page':undefined} onClick={()=>onTab(name)}>{name==='Fleet Overview'?'Fleet overview':name==='Egress Proof'?'Egress proof':name==='Audit Log'?'Audit log':name}{name==='Incidents'&&active>0&&<span className="ops-incident-count">{active}</span>}</button>)}</nav><OpsGraphic tab={tab} /></div>
  <div className="ops-navigation"><button onClick={()=>step(-1)} aria-label="Previous view">←</button><nav className="ops-tools" aria-label="Additional console views">{otherViews.map(name=><button key={name} aria-current={tab===name?'page':undefined} onClick={()=>onTab(name)}>{name}</button>)}</nav><button onClick={()=>step(1)} aria-label="Next view">→</button></div>
 </section>;
});

export function TelemetryFeed({points,connected}:{points:Point[];connected:boolean}){
 return <section className="ops-section ops-events"><div><p className="eyebrow">TELEMETRY</p><h2>Event stream</h2><p className="eyebrow ops-status"><i className={connected?'is-live':''}/>{connected?'Live stream':'Stream reconnecting'} · approved metadata</p></div><div className="ops-log" role="log" aria-label="Recent telemetry observations" aria-live="off">{points.length?[...points].slice(-12).reverse().map(p=><div className="ops-log-row" key={p.key}><time dateTime={p.collected_at}>{utc(p.collected_at)}</time><span>DATA</span><span>{p.collector_id}</span><span>{Object.keys(p.metrics).length} metrics · {p.risk_events.length} risk events</span><span>{sourceName(p.source)}</span></div>):<p className="muted">Waiting for approved collector observations.</p>}</div></section>;
}

export function ConsoleFooter({session,connected}:{session:Session;connected:boolean}){
 return <footer className="ops-footer"><div className="ops-footnotes"><p>Zero raw data egress. Only allow-listed metadata crosses your collector boundary.</p><p className="muted">Tenant: {session.tenant} → {session.region}<br/>{import.meta.env.VITE_HOSTED==='true'?'Encrypted HTTPS → hosted receiver':'Offline LAN → self-hosted'}<br/>All timestamps UTC</p><p><span className="ops-status"><i className={connected?'is-live':''}/>{connected?'Live stream':'Stream reconnecting'}</span> → Kalki captures fleet, network and auth activity at the edge. Raw samples never touch the network.</p></div><div className="ops-wordmark" aria-hidden="true"><span>Kalki</span><svg viewBox="0 0 100 160"><g><path d="M50 0V160M5 25L95 135M95 25L5 135"/></g><circle cx="50" cy="80" r="7"/></svg><span>Console</span></div></footer>;
}
