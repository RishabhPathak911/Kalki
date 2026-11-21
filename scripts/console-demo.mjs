// Phase 6 local evaluation harness: real Go receivers + four real collectors.
// Run from repository root after building data/phase6-tier2(.exe) and collector.
import {spawn} from 'node:child_process';
import https from 'node:https';
import {readFileSync,mkdirSync,openSync} from 'node:fs';
import {randomBytes} from 'node:crypto';
const ext=process.platform==='win32'?'.exe':'';
const children=[];const ca=readFileSync('data/certs/ca.pem');const agent=new https.Agent({ca,rejectUnauthorized:true});
const run=`data/console-demo-${Date.now()}`;mkdirSync(run,{recursive:true});
function start(binary,args,env,name){const log=openSync(`${run}/${name}.log`,'a');const child=spawn(binary,args,{env:{...process.env,...env},windowsHide:true,stdio:['ignore',log,log]});children.push(child);child.on('error',()=>{console.error('A demo executable could not start. Build the binaries first.');stop();});return child;}
function call(port,path,body,token){return new Promise((resolve,reject)=>{const data=JSON.stringify(body);const req=https.request(`https://127.0.0.1:${port}${path}`,{method:'POST',agent,headers:{'Content-Type':'application/json',...(token?{Authorization:`Bearer ${token}`}:{})}},r=>{let text='';r.on('data',b=>text+=b);r.on('end',()=>{try{if(r.statusCode!==200)throw Error();resolve(JSON.parse(text));}catch{reject(Error('Demo onboarding failed'));}});});req.on('error',()=>reject(Error('Receiver unavailable')));req.end(data);});}
let stopping=false;function stop(){if(stopping)return;stopping=true;for(const child of children)child.kill();agent.destroy();setTimeout(()=>process.exit(),300).unref();}
process.on('SIGINT',stop);process.on('SIGTERM',stop);
start(`./data/phase6-tier2${ext}`,['--addr=127.0.0.1:19443','--eu-addr=127.0.0.1:19444','--public-url=https://127.0.0.1:19443','--eu-public-url=https://127.0.0.1:19444'],{},'receiver');
try{
 let alpha;for(let i=0;i<30;i++){try{alpha=await call(19443,'/api/auth/login',{username:'tenant_alpha',password:process.env.KALKI_ALPHA_PASSWORD||'alpha-demo-local'});break;}catch{await new Promise(r=>setTimeout(r,300));}}
 if(!alpha)throw Error('Receiver did not start');
 const beta=await call(19444,'/api/auth/login',{username:'tenant_beta',password:process.env.KALKI_BETA_PASSWORD||'beta-demo-local'});
 for(const [index,session] of [alpha,alpha,beta,beta].entries()){
  const port=index<2?19443:19444;const id=`host_${randomBytes(4).toString('hex')}`;const minted=await call(port,'/api/collectors/tokens',{collector_id:id},session.token);
  start(`./data/phase6-collector${ext}`,[`--tenant=${session.tenant}`,`--region=${session.region}`,`--collector-id=${id}`,`--tier2-url=https://127.0.0.1:${port}`,`--probe-target=127.0.0.1:${port}`,`--wal-path=${run}/${id}.wal`,`--key-file=${run}/${session.tenant}.key`,`--log-file=${run}/${id}-auth.log`,`--proof-addr=127.0.0.1:${19090+index}`,'--demo-controls'],{KALKI_TOKEN:minted.token},id);
  console.log(`${session.tenant}: ${id}, proof port ${19090+index}`);
 }
 console.log('Four real collectors running. Configure dashboard AP=19443 EU=19444 proof=19090. Ctrl+C stops only this demo.');
}catch(e){console.error(e.message);stop();}
