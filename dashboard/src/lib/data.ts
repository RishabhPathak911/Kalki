import type {Point} from './types';
export function orderedPoints(existing:Point[],incoming:Point[],limit=2400):Point[]{const map=new Map(existing.map(p=>[p.key,p]));for(const p of incoming)map.set(p.key,p);return [...map.values()].sort((a,b)=>Date.parse(a.collected_at)-Date.parse(b.collected_at)||a.key.localeCompare(b.key)).slice(-limit);}
export function percentile(values:number[],p:number){if(!values.length)return null;const sorted=[...values].sort((a,b)=>a-b);return sorted[Math.max(0,Math.ceil(p*sorted.length)-1)];}
export const utc=(value:string|number)=>new Date(value).toISOString().slice(11,19)+' UTC';
export const number=(value:number|undefined,digits=1)=>value===undefined?'—':value.toLocaleString('en-US',{maximumFractionDigits:digits});
export const sourceName=(source:string)=>({live:'LIVE',replayed_real_trace:'REPLAYED REAL TRACE',simulated:'SIMULATED'}[source]||source.toUpperCase());
