import type {Incident,Point,Remediation} from './types';

const measurements:Record<string,[string,string]>={cpu_pct:['Processor use','%'],collector_cpu_pct:['Monitoring app processor use','%'],mem_pct:['Memory use','%'],disk_pct:['Storage used','%'],bandwidth_mbps:['Network traffic','Mbps'],latency_ms:['Connection response time','ms'],packet_loss_pct:['Failed connection checks','%'],service_up:['Service reachable','0 = no, 1 = yes'],wal_depth:['Measurements waiting to send','batches'],wal_buffer_full:['Local storage buffer full','0 = no, 1 = yes'],bruteforce_suspected:['Repeated failed sign-ins','attempts'],heartbeat_age_s:['Time since last contact','seconds']};
export function measurement(key:string):[string,string]{
 const name=key.replace(/^risk:/,'');
 if(name.endsWith('_rate')){const [label,unit]=measurement(name.slice(0,-5));return [`${label} change speed`,unit==='%'?'percentage points per second':`${unit} per second`.trim()];}
 if(name.endsWith('_peer')){const [label,unit]=measurement(name.slice(0,-5));return [`${label} compared with other devices`,unit];}
 return measurements[name]||[name.replaceAll('_',' '),''];
}
export function signalValue(value:number){return value.toLocaleString('en-US',{maximumSignificantDigits:4});}
export function availableMeasurements(points:Point[]){return [...new Set(points.flatMap(p=>[...Object.keys(p.metrics),...p.risk_events.map(r=>'risk:'+r.type)]))].sort();}
export function evidenceSeries(points:Point[],collector:string,source:string,metric:string){return points.filter(p=>p.collector_id===collector&&p.source===source).map(p=>({at:Date.parse(p.collected_at),value:metric.startsWith('risk:')?p.risk_events.find(r=>r.type===metric.slice(5))?.count:p.metrics[metric]?.avg,key:p.key})).filter(p=>Number.isFinite(p.at)).sort((a,b)=>a.at-b.at||a.key.localeCompare(b.key));}
const patterns:Record<string,{title:string;meaning:string;next:string}>={
 network_path_issue:{title:'Connection problems',meaning:'Connections became slower or less reliable. This may affect services that need the network.',next:'Ask your support team to check the internet connection and whether the service can be reached.'},
 local_workload:{title:'Device working harder than usual',meaning:'Processor use increased without the same network symptoms. An application or scheduled task may be busy.',next:'Check which applications are busy and whether a scheduled task or recent change explains the increase.'},
 possible_attack_exfiltration:{title:'Unusual sign-in and network activity',meaning:'Failed sign-ins appeared alongside resource or network changes. This needs investigation; it does not confirm an attack.',next:'Ask your security team to review sign-ins and network activity on this device before taking action.'},
 memory_leak:{title:'Memory use keeps growing',meaning:'The device is using more memory over time. An application may be holding memory longer than expected.',next:'Ask your support team to check which application is growing and review recent changes.'},
 capacity_risk:{title:'Storage capacity needs attention',meaning:'Storage use is approaching a limit or growing unusually quickly.',next:'Review what is using storage and your retention policy. Plan more capacity if needed; do not delete files without review.'},
 shared_dependency:{title:'Several devices changed together',meaning:'Multiple devices changed around the same time. A shared service or connection may be involved.',next:'Ask your support team to compare the affected devices and check the services they share.'},
 collector_down:{title:'Device has stopped reporting',meaning:'Kalki has not heard from the monitoring app within its reporting deadline. This alone does not prove the whole device is down.',next:'Check that the device is powered on, connected to the network, and that the Kalki collector is still running.'},
 buffer_full:{title:'Measurements cannot be queued safely',meaning:'The device’s local delivery buffer is full. New measurements may be blocked until delivery recovers.',next:'Ask your support team to restore the receiver connection and check available local storage.'},
};
export function incidentGuide(i:Incident){return patterns[i.pattern]||{title:i.pattern.replaceAll('_',' '),meaning:i.summary,next:i.recommended_action};}
export function responseStage(r?:Remediation){
 if(!r)return {title:'Review needed',text:'No response has been proposed. Review the recommendation before requesting a safe simulation.'};
 if(r.status==='PROPOSED')return {title:'Waiting for your approval',text:'A safe simulation has been proposed. Nothing will run until you approve it.'};
 if(r.status==='EXECUTED_SIMULATED')return {title:'Simulation completed',text:'A dry run was completed. No device settings or operating-system commands were changed.'};
 if(r.status==='ROLLED_BACK')return {title:'Simulation rolled back',text:'The simulated response was undone. No real device changes were made.'};
 return {title:r.status.replaceAll('_',' '),text:r.result};
}
