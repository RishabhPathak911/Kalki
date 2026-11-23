import {TraceData} from './EvidenceViews';
import {DatabaseGraphs} from './DatabaseGraphs';
import {useEffect,useState} from 'react';
import {LineChart,Line,XAxis,YAxis,CartesianGrid,Tooltip,ReferenceLine,ResponsiveContainer} from 'recharts';
import type {NodeData,Point,Thresholds} from '../lib/types';
import {number,sourceName,utc} from '../lib/data';

export function Badge({value}:{value:string}){
  return <span className={`badge ${value.toLowerCase().replaceAll(' ','-')}`}>{value}</span>;
}

const metrics=[
  ['cpu_pct','CPU utilization','%'],
  ['mem_pct','Memory','%'],
  ['disk_pct','Disk capacity','%'],
  ['bandwidth_mbps','Network throughput','Mbps'],
  ['latency_ms','Probe latency','ms'],
  ['packet_loss_pct','TCP failure ratio','%'],
  ['service_up','Service reachability','0 / 1']
];

export default function Fleet({
  nodes,
  points,
  thresholds,
  now,
  connection,
  onDeleteNode
}:{
  nodes:NodeData[];
  points:Point[];
  thresholds:Thresholds|null;
  now:number;
  connection:string;
  onDeleteNode?:(collectorId:string)=>void;
}){
  const [search,setSearch]=useState('');
  const [selected,setSelected]=useState('');
  const [source,setSource]=useState('live');
  const [activeSection,setActiveSection]=useState<'all'|'database'|'trace'|'telemetry'>('all');
  const [confirmDelete,setConfirmDelete]=useState<string|null>(null);
  const [toastMessage,setToastMessage]=useState('');

  useEffect(()=>{
    if(!nodes.some(n=>n.collector_id===selected)){
      setSelected(nodes[0]?.collector_id||'');
    }
  },[nodes,selected]);

  const node=nodes.find(n=>n.collector_id===selected);
  const sources=[...new Set(points.filter(p=>p.collector_id===selected).map(p=>p.source))];
  const chartSource=sources.includes(source)?source:sources[0]||'live';
  const data=points.filter(p=>p.collector_id===selected&&p.source===chartSource).map(p=>({
    at:Date.parse(p.collected_at),
    ...Object.fromEntries(Object.entries(p.metrics).map(([k,v])=>[k,v.avg]))
  }));

  const heartbeat=thresholds?.thresholds.heartbeat;
  const stale=typeof heartbeat==='object'?heartbeat.stale_after_s||15:15;
  const offline=typeof heartbeat==='object'?heartbeat.offline_after_s||45:45;

  function handleDelete(collectorId:string){
    if(onDeleteNode){
      onDeleteNode(collectorId);
    }
    setConfirmDelete(null);
    setToastMessage(`Collector ${collectorId} removed from active fleet.`);
    setTimeout(()=>setToastMessage(''),4000);
  }

  return (
    <>
      <section className="ops-section fleet-inventory">
        <div className="section-heading">
          <div>
            <p className="eyebrow">REGIONAL INVENTORY</p>
            <h2>Fleet overview <span className="count">{nodes.length}</span></h2>
          </div>
          <span className="muted">UTC event time · {points.length} retained observations</span>
        </div>

        {toastMessage && (
          <div role="status" className="notice match" style={{marginBottom:16}}>
            ✓ {toastMessage}
          </div>
        )}

        {confirmDelete && (
          <div className="delete-confirm-bar" role="alertdialog" aria-labelledby="delete-prompt">
            <span id="delete-prompt">
              Remove collector <strong>{confirmDelete}</strong> from this workspace? Its active telemetry card will be deleted.
            </span>
            <div className="delete-confirm-actions">
              <button type="button" onClick={()=>setConfirmDelete(null)}>Cancel</button>
              <button type="button" className="delete-confirm-btn" onClick={()=>handleDelete(confirmDelete)}>
                Confirm Remove
              </button>
            </div>
          </div>
        )}

        <div className="fleet-list">
          {!nodes.length ? (
            <div className="empty">
              <h3>No collectors connected</h3>
              <p>Add a collector to begin receiving privacy-approved telemetry.</p>
            </div>
          ) : (
            <>
              <label>
                Find a connected device
                <input
                  value={search}
                  onChange={e=>setSearch(e.target.value)}
                  placeholder="Collector ID, region or health state"
                />
              </label>

              <div className="nodes">
                {nodes
                  .filter(n=>`${n.collector_id} ${n.region} ${n.health} ${n.connection_state}`.toLowerCase().includes(search.toLowerCase().trim()))
                  .map(n=>{
                    const age=Math.max(0,(now-Date.parse(n.last_seen_at))/1000);
                    const conn=age>=offline?'OFFLINE':age>stale?'STALE':n.connection_state;
                    const health=conn==='ONLINE'?n.health:'Poor';
                    const isSelected=selected===n.collector_id;

                    return (
                      <div
                        key={n.collector_id}
                        onClick={()=>setSelected(n.collector_id)}
                        className={`node-card ${isSelected?'selected':''} ${conn!=='ONLINE'?'stale-node':''}`}
                        role="button"
                        tabIndex={0}
                        onKeyDown={e=>{if(e.key==='Enter'||e.key===' ')setSelected(n.collector_id);}}
                      >
                        <div className="row">
                          <span className="mono">{n.collector_id}</span>
                          <div style={{display:'flex',alignItems:'center',gap:6}}>
                            <Badge value={health}/>
                            <button
                              type="button"
                              className="delete-node-btn"
                              title={`Remove ${n.collector_id} from fleet`}
                              aria-label={`Remove ${n.collector_id} from fleet`}
                              onClick={e=>{
                                e.stopPropagation();
                                setConfirmDelete(n.collector_id);
                              }}
                            >
                              ✕ Remove
                            </button>
                          </div>
                        </div>

                        <p className="node-region">{n.region}</p>

                        <div className="node-stats">
                          <span>
                            <small>HOST CPU</small>
                            <strong>{number(n.metrics.cpu_pct?.avg)}<em>%</em></strong>
                          </span>
                          <span>
                            <small>MEMORY</small>
                            <strong>{number(n.metrics.mem_pct?.avg)}<em>%</em></strong>
                          </span>
                        </div>

                        <div className="row">
                          <Badge value={conn}/>
                          <small>last seen {Math.floor(age)}s ago</small>
                        </div>

                        <div className="source-row">
                          {n.source_badges.map(s=>(
                            <span key={s} className="provenance">{sourceName(s)}</span>
                          ))}
                        </div>
                      </div>
                    );
                  })}
              </div>
            </>
          )}
        </div>
      </section>

      {nodes.length > 0 && (
        <>
          <div className="section-heading chart-heading">
            <div>
              <h3>Analytics &amp; Telemetry / <span className="mono">{selected}</span></h3>
              <p className="muted">Host workload, embedded database persistence, and kernel trace streams.</p>
            </div>
            <label className="inline-label">
              Source
              <select value={chartSource} onChange={e=>setSource(e.target.value)}>
                {(sources.length ? sources : ['live']).map(s=>(
                  <option value={s} key={s}>{sourceName(s)}</option>
                ))}
              </select>
            </label>
          </div>

          <div className="fleet-view-selector" role="tablist" aria-label="Fleet detail analytics view">
            <button
              type="button"
              role="tab"
              aria-selected={activeSection==='all'}
              className={`fleet-view-tab ${activeSection==='all'?'active':''}`}
              onClick={()=>setActiveSection('all')}
            >
              ⚡ All Analytics
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={activeSection==='database'}
              className={`fleet-view-tab ${activeSection==='database'?'active':''}`}
              onClick={()=>setActiveSection('database')}
            >
              💾 Database &amp; Storage Graphs
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={activeSection==='trace'}
              className={`fleet-view-tab ${activeSection==='trace'?'active':''}`}
              onClick={()=>setActiveSection('trace')}
            >
              🔍 Trace Data Explorer
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={activeSection==='telemetry'}
              className={`fleet-view-tab ${activeSection==='telemetry'?'active':''}`}
              onClick={()=>setActiveSection('telemetry')}
            >
              📊 Host Telemetry
            </button>
          </div>

          {/* Database & Storage Engine Section */}
          {(activeSection==='all' || activeSection==='database') && (
            <DatabaseGraphs node={node} points={points} selected={selected} />
          )}

          {/* Trace Data Explorer Section */}
          {(activeSection==='all' || activeSection==='trace') && (
            <TraceData points={points} collector={selected} source={chartSource} now={now} connection={connection}/>
          )}

          {/* Host Workload & Hardware Telemetry Charts */}
          {(activeSection==='all' || activeSection==='telemetry') && (
            <div className="charts">
              {metrics.map(([metric,title,unit])=>{
                const bounds=thresholds?.thresholds[metric];
                const threshold=typeof bounds==='object'?bounds:null;
                const hasData=data.some(p=>Number.isFinite(p[metric as keyof typeof p]));

                return (
                  <article className="chart panel" key={metric}>
                    <div className="row">
                      <h3>{title} <small>{unit}</small></h3>
                      <span className="provenance">{sourceName(chartSource)}</span>
                    </div>
                    <div className="chart-area">
                      {hasData ? (
                        <ResponsiveContainer width="100%" height="100%" minWidth={0} initialDimension={{width:400,height:210}}>
                          <LineChart data={data} margin={{top:12,right:18,bottom:0,left:-20}}>
                            <CartesianGrid stroke="#202632" vertical={false}/>
                            <XAxis
                              dataKey="at"
                              type="number"
                              domain={['dataMin','dataMax']}
                              tickFormatter={v=>utc(Number(v)).slice(0,8)}
                              minTickGap={38}
                              tick={{fill:'#8793a5',fontSize:11}}
                              axisLine={false}
                              tickLine={false}
                            />
                            <YAxis tick={{fill:'#8793a5',fontSize:11}} axisLine={false} tickLine={false}/>
                            <Tooltip
                              labelFormatter={v=>utc(Number(v))}
                              contentStyle={{background:'#101319',border:'1px solid #394355',color:'#fff',fontSize:13}}
                            />
                            <Line
                              dataKey={metric}
                              name={title}
                              stroke="#e9edf5"
                              strokeWidth={1.7}
                              dot={data.filter(p=>Number.isFinite(p[metric as keyof typeof p])).length===1?{r:4}:false}
                              isAnimationActive={false}
                              connectNulls={false}
                            />
                            {metric==='cpu_pct' && (
                              <Line
                                dataKey="collector_cpu_pct"
                                name="Collector overhead"
                                stroke="#7b8799"
                                strokeDasharray="4 3"
                                strokeWidth={1.5}
                                dot={false}
                                isAnimationActive={false}
                              />
                            )}
                            {threshold?.warn!==undefined && (
                              <ReferenceLine y={threshold.warn} stroke="#c89b4d" strokeDasharray="3 4" ifOverflow="extendDomain"/>
                            )}
                            {threshold?.critical!==undefined && (
                              <ReferenceLine y={threshold.critical} stroke="#cf6072" strokeDasharray="3 4" ifOverflow="extendDomain"/>
                            )}
                          </LineChart>
                        </ResponsiveContainer>
                      ) : (
                        <p className="muted chart-empty">Awaiting measurements</p>
                      )}
                    </div>
                    {metric==='cpu_pct' && (
                      <div className="chart-legend">
                        <span>━ Host CPU</span>
                        <span>┄ Collector CPU</span>
                      </div>
                    )}
                  </article>
                );
              })}
            </div>
          )}

          <details className="panel history">
            <summary>UTC state transition history <span className="count">{node?.state_transitions.length||0}</span></summary>
            {node?.state_transitions.length ? (
              <div className="table-scroll">
                <table>
                  <thead>
                    <tr>
                      <th>At (UTC)</th>
                      <th>Health</th>
                      <th>Connection</th>
                      <th>Reason</th>
                    </tr>
                  </thead>
                  <tbody>
                    {[...node.state_transitions].reverse().map((tr,i)=>(
                      <tr key={i}>
                        <td className="mono">{tr.at}</td>
                        <td>{tr.from} → {tr.to}</td>
                        <td>{tr.connection_from} → {tr.connection_to}</td>
                        <td>{tr.reason}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <p className="muted">No transitions recorded.</p>
            )}
          </details>

          {points.some(p=>p.collector_id===selected&&p.risk_events?.length>0) && (
            <div className="panel">
              <h3>Local security observations</h3>
              {points.filter(p=>p.collector_id===selected&&p.risk_events?.length>0).slice(-4).map(p=>(
                <p key={p.key}>
                  <span className="provenance">{sourceName(p.source)}</span> {p.risk_events.map(r=>`${r.type}: ${r.count}`).join(' · ')} <small>{utc(p.collected_at)}</small>
                </p>
              ))}
            </div>
          )}
        </>
      )}
    </>
  );
}
