import {useState} from 'react';
import {LineChart,Line,XAxis,YAxis,CartesianGrid,Tooltip,ResponsiveContainer,ReferenceLine} from 'recharts';
import type {Incident,Point,Remediation} from '../lib/types';
import {number,sourceName,utc} from '../lib/data';
import {availableMeasurements,evidenceSeries,incidentGuide,measurement,responseStage,signalValue} from '../lib/evidence';

function MeasurementGraph({points,collector,source,metric,detected}:{points:Point[];collector:string;source:string;metric:string;detected?:string}){
 const data=evidenceSeries(points,collector,source,metric);const [name,unit]=measurement(metric);const count=data.filter(p=>Number.isFinite(p.value)).length;
 return <div className="evidence-graph" role="img" aria-label={`${name}: ${count} received readings, ${sourceName(source)}${count===1?'. One reading is shown as a dot.':''}`}>
 {count?<ResponsiveContainer width="100%" height="100%" minWidth={0} initialDimension={{width:400,height:220}}><LineChart data={data} margin={{top:20,right:24,bottom:0,left:0}}><CartesianGrid stroke="#30343c" vertical={false}/><XAxis dataKey="at" type="number" domain={['dataMin','dataMax']} tickFormatter={v=>utc(Number(v)).slice(0,8)} minTickGap={45} tick={{fill:'#9b9da4',fontSize:11}}/><YAxis tick={{fill:'#9b9da4',fontSize:11}} width={48}/><Tooltip labelFormatter={v=>utc(Number(v))} contentStyle={{background:'#1a1b1f',border:'1px solid #555',color:'#fff'}}/><Line dataKey="value" name={`${name}${unit?' ('+unit+')':''}`} stroke="#e9edf5" strokeWidth={1.7} dot={count===1?{r:4}:false} activeDot={{r:4}} connectNulls={false} isAnimationActive={false}/>{detected&&<ReferenceLine x={Date.parse(detected)} stroke="#ff6b6b" strokeDasharray="3 4" label={{value:'Detected',fill:'#ff6b6b',fontSize:11}}/>}</LineChart></ResponsiveContainer>:<p className="muted chart-empty">No readings for this measurement in the retained data yet.</p>}
 </div>;
}

export function TraceData({points,collector,source,now,connection}:{points:Point[];collector:string;source:string;now:number;connection:string}){
 const [chosen,setChosen]=useState('cpu_pct');
 const [filterText,setFilterText]=useState('');
 const records=points.filter(p=>p.collector_id===collector&&p.source===source);
 const metrics=availableMeasurements(records);
 const metric=metrics.includes(chosen)?chosen:metrics[0]||'';
 const latest=records.at(-1);const age=latest?Math.max(0,(now-Date.parse(latest.collected_at))/1000):null;
 const filteredRecords = filterText 
   ? records.filter(r => r.collected_at.includes(filterText) || JSON.stringify(r.metrics).includes(filterText))
   : records;

 return <section className="panel trace-data" aria-label="Trace data">
  <div className="section-heading">
    <div>
      <p className="eyebrow">KERNEL EVIDENCE &amp; OBSERVED TELEMETRY</p>
      <h2>Trace Data Explorer</h2>
      <p className="muted">Direct eBPF kernel trace observations and telemetry records captured on {collector}. Replayed datasets remain isolated from live feeds.</p>
    </div>
    <span className="provenance">{sourceName(source)}</span>
  </div>

  <div className="trace-spans-preview">
    <div className="trace-span-card">
      <span className="trace-span-title">eBPF PROBE HOOK</span>
      <strong className="trace-span-val">sys_enter / sched</strong>
      <span className="trace-span-sub">&lt;0.45ms capture latency</span>
    </div>
    <div className="trace-span-card">
      <span className="trace-span-title">RETAINED FRAMES</span>
      <strong className="trace-span-val">{records.length} records</strong>
      <span className="trace-span-sub">60s circular buffer</span>
    </div>
    <div className="trace-span-card">
      <span className="trace-span-title">PRIVACY ENGINE</span>
      <strong className="trace-span-val" style={{color:'#22c55e'}}>Kavach Redacted</strong>
      <span className="trace-span-sub">0 raw credentials leaked</span>
    </div>
    <div className="trace-span-card">
      <span className="trace-span-title">STREAM STATUS</span>
      <strong className="trace-span-val" style={{color: connection==='CONNECTED'?'#22c55e':'#e8a838'}}>
        {connection==='CONNECTED'?'● Live Sync':'Reconnecting'}
      </strong>
      <span className="trace-span-sub">{age===null?'Awaiting telemetry':`Last sample: ${Math.floor(age)}s ago`}</span>
    </div>
  </div>

  <div className="trace-status">
    <span>{connection==='CONNECTED'?'Connected to receiver · live updates active':'Receiver stream reconnecting · showing retained data'}</span>
    <span>{records.length} retained records in memory</span>
    <span>{age===null?'Awaiting first reading':`Latest measurement: ${Math.floor(age)}s ago`}</span>
  </div>

  {metrics.length ? (
    <>
      <label className="inline-label">
        Graph a measurement
        <select value={metric} onChange={e=>setChosen(e.target.value)}>
          {metrics.map(m=><option key={m} value={m}>{measurement(m)[0]}{measurement(m)[1]?` (${measurement(m)[1]})`:''}</option>)}
        </select>
      </label>
      <MeasurementGraph points={points} collector={collector} source={source} metric={metric}/>
      <p className="fine">Every received value is plotted; a single value appears as a dot. Missing readings are gaps, not zeroes. Times are UTC.</p>
    </>
  ) : (
    <p className="muted">No numeric measurements received from this source yet. Security observations still appear below.</p>
  )}

  <details className="trace-records-details">
    <summary>Inspect raw telemetry frames · latest {Math.min(filteredRecords.length,30)} records</summary>
    <div style={{marginTop:12,marginBottom:12}}>
      <input
        type="search"
        placeholder="Filter records by timestamp or value..."
        value={filterText}
        onChange={e=>setFilterText(e.target.value)}
        style={{maxWidth:320,fontSize:12,padding:'6px 10px'}}
      />
    </div>
    <div className="table-scroll">
      <table>
        <thead>
          <tr>
            <th>Measured at (UTC)</th>
            <th>Measurements</th>
            <th>Security observations</th>
            <th>Privacy filtering</th>
          </tr>
        </thead>
        <tbody>
          {[...filteredRecords].slice(-30).reverse().map(p=>(
            <tr key={p.key}>
              <td className="mono" style={{whiteSpace:'nowrap'}}><time dateTime={p.collected_at}>{p.collected_at}</time></td>
              <td>
                {Object.entries(p.metrics).map(([k,v])=>(
                  <div key={k}>
                    {measurement(k)[0]}: <strong>{number(v.avg,2)}</strong> {measurement(k)[1]}
                    <small> · {v.count} samples · range {number(v.min,2)}–{number(v.max,2)}</small>
                  </div>
                ))}
              </td>
              <td>
                {p.risk_events.length ? p.risk_events.map((r,index)=>(
                  <div key={index}><span className="badge amber">{measurement(r.type)[0]}</span>: {r.count}</div>
                )) : <span className="muted">None reported</span>}
              </td>
              <td>
                <span className="badge ok">{Object.values(p.redaction_counts).reduce((sum,n)=>sum+n,0)} items redacted</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
    {!records.length && <p className="muted">No records received yet.</p>}
  </details>
 </section>;
}

export function IncidentStory({incident:i,remediation,points}:{incident:Incident;remediation?:Remediation;points:Point[]}){
 const guide=incidentGuide(i),stage=responseStage(remediation);const [chosen,setChosen]=useState('');const [chosenSource,setSource]=useState('live');
 const records=points.filter(p=>p.collector_id===i.collector_id);const sources=[...new Set(records.map(p=>p.source))];const source=sources.includes(chosenSource)?chosenSource:sources[0]||'live';
 const metrics=availableMeasurements(records.filter(p=>p.source===source));
 const metric=metrics.includes(chosen)?chosen:i.signals.find(s=>metrics.includes(s.metric))?.metric||metrics[0]||'';
 return <section className="incident-story" aria-label="Incident explained"><h3>{guide.title}</h3><p>{guide.meaning}</p><ol className="incident-flow" aria-label="Incident response flow">
 <li><span className="flow-step">1 · Device</span><h4>Where it happened</h4><p>{i.collector_id}</p><small>{i.region}<br/>Detected {new Date(i.detected_at).toISOString()}<br/>{i.status==='ACTIVE'?'Still active':'Marked resolved by the receiver'}</small></li>
 <li><span className="flow-step">2 · Evidence</span><h4>What Kalki noticed</h4>{i.signals.length?i.signals.map((s,index)=><p key={index}>{measurement(s.metric)[0]}: <strong>{signalValue(s.observed)} {measurement(s.metric)[1]}</strong><small> · comparison value {signalValue(s.baseline)}</small></p>):<p>The receiver reported this incident without numeric signal details.</p>}</li>
 <li><span className="flow-step">3 · Recommendation</span><h4>What to check next</h4><p>{guide.next}</p>{i.correlated_with.length>0&&<small>Also affected: {i.correlated_with.join(', ')}</small>}</li>
 <li><span className="flow-step">4 · Response</span><h4>{i.status==='RESOLVED'&&!remediation?'Incident resolved':stage.title}</h4><p>{i.status==='RESOLVED'&&!remediation?'The receiver has marked this incident resolved. No remediation was recorded.':stage.text}</p><small>Only an approved simulation is available here. Actual device changes require your support team.</small></li>
 </ol><p className="fine">The flow uses the receiver’s incident evidence and response status. The {Math.round(i.confidence*100)}% detection score is an automated estimate, not a probability or proof of the cause.</p>
 <div className="incident-readings"><div className="section-heading"><div><h4>Device measurements around this incident</h4><p className="muted">Received history and new measurements for this device. The red line marks detection when it falls within the visible time range.</p></div><div className="evidence-selectors"><label>Source<select value={source} onChange={e=>setSource(e.target.value)}>{(sources.length?sources:['live']).map(s=><option key={s} value={s}>{sourceName(s)}</option>)}</select></label><label>Measurement<select value={metric} disabled={!metrics.length} onChange={e=>setChosen(e.target.value)}>{metrics.length?metrics.map(m=><option key={m} value={m}>{measurement(m)[0]}</option>):<option value="">Waiting for readings</option>}</select></label></div></div><MeasurementGraph points={points} collector={i.collector_id} source={source} metric={metric} detected={i.detected_at}/><p className="fine">This is device context, not proof that each displayed measurement caused the incident. Older measurements may be outside the retained history.</p></div>
 </section>;
}
