import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import http from 'node:http';
import https from 'node:https';
import handler from '../api/gateway.js';
test('Missing backend fails closed instead of generating demo sessions',async()=>{
 const saved=[process.env.KALKI_AP_URL,process.env.KALKI_EU_URL];delete process.env.KALKI_AP_URL;delete process.env.KALKI_EU_URL;
 const app=http.createServer((req,res)=>{req.query={region:'ap-south-1',path:'api/auth/login'};handler(req,res)});
 try {await new Promise(r=>app.listen(0,'127.0.0.1',r));const response=await fetch(`http://127.0.0.1:${app.address().port}`,{method:'POST',body:JSON.stringify({username:'anything',password:'anything'})});assert.equal(response.status,503);assert.equal((await response.json()).token,undefined);}
 finally {await new Promise(r=>app.close(r));for(const [i,key] of ['KALKI_AP_URL','KALKI_EU_URL'].entries()){if(saved[i]===undefined)delete process.env[key];else process.env[key]=saved[i]}}
});

test('Proxy preserves Windows onboarding JSON and flushes SSE before upstream closes',async()=>{
 const saved={...process.env};const token='b'.repeat(64);let upstreamAuth='',upstreamBody='';
 const receiver=https.createServer({key:readFileSync('data/certs/server.key'),cert:readFileSync('data/certs/server.pem')},async(req,res)=>{
  upstreamAuth=req.headers.authorization;
  if(req.url==='/api/stream'){res.writeHead(200,{'Content-Type':'text/event-stream'});res.write('event: telemetry\ndata: {"seq":1}\n\n');const timer=setTimeout(()=>res.end(),10000);res.on('close',()=>clearTimeout(timer));return;}
  for await(const chunk of req)upstreamBody+=chunk;
  res.setHeader('Content-Type','application/json');res.end(JSON.stringify({windows_install_command:'test PowerShell command',install_command:'test POSIX command'}));
 });
 const app=http.createServer((req,res)=>{req.query=Object.fromEntries(new URL(req.url,'http://test').searchParams);handler(req,res)});
 try {
  await new Promise(r=>receiver.listen(0,'127.0.0.1',r));await new Promise(r=>app.listen(0,'127.0.0.1',r));
  process.env.KALKI_AP_URL=`https://127.0.0.1:${receiver.address().port}`;process.env.KALKI_CA_PEM=readFileSync('data/certs/ca.pem','utf8');
  const base=`http://127.0.0.1:${app.address().port}/?region=ap-south-1&path=`;
  const minted=await fetch(base+'api/collectors/tokens',{method:'POST',headers:{Authorization:'Bearer '+token},body:'{}'});
  assert.equal(minted.status,200);assert.equal((await minted.json()).windows_install_command,'test PowerShell command');assert.equal(upstreamAuth,'Bearer '+token);assert.equal(upstreamBody,'{}');
  const controller=new AbortController();const timeout=setTimeout(()=>controller.abort(),2000);
  try {const response=await fetch(base+'api/stream',{headers:{Authorization:'Bearer '+token},signal:controller.signal});assert.equal(response.headers.get('x-accel-buffering'),'no');const reader=response.body.getReader();assert.match(new TextDecoder().decode((await reader.read()).value),/event: telemetry/);await reader.cancel();}
  finally {clearTimeout(timeout);controller.abort();}
 }finally {for(const key of ['KALKI_AP_URL','KALKI_CA_PEM']){if(saved[key]===undefined)delete process.env[key];else process.env[key]=saved[key]}app.closeAllConnections();receiver.closeAllConnections();await new Promise(r=>app.close(r));await new Promise(r=>receiver.close(r));}
});
test('Hosted routing packages both pages and all dashboard assets',()=>{const config=JSON.parse(readFileSync('vercel.json'));assert.equal(config.outputDirectory,'dist');const home=readFileSync('dist/index.html','utf8'),app=readFileSync('dist/app/index.html','utf8');assert.match(home,/id="dashboard-start" href="\/app"/);assert.doesNotMatch(app,/localhost/);for(const [,asset] of app.matchAll(/(?:src|href)="(\/app\/[^"?]+)"/g))assert.ok(readFileSync('dist'+asset).length);assert.ok(readFileSync('dist/app/kalki-eye.svg').length)});
test('Hosted gateway verifies TLS, forwards scoped auth, rejects unknown targets and missing backend',async()=>{const saved={...process.env};const receiver=https.createServer({key:readFileSync('data/certs/server.key'),cert:readFileSync('data/certs/server.pem')},(req,res)=>{res.setHeader('Content-Type','application/json');res.end(JSON.stringify({path:req.url,auth:req.headers.authorization||null}))});const app=http.createServer((req,res)=>{req.query=Object.fromEntries(new URL(req.url,'http://test').searchParams);handler(req,res)});try{await new Promise(r=>receiver.listen(0,'127.0.0.1',r));await new Promise(r=>app.listen(0,'127.0.0.1',r));const base=`http://127.0.0.1:${app.address().port}/?`;process.env.KALKI_AP_URL=`https://127.0.0.1:${receiver.address().port}`;delete process.env.KALKI_EU_URL;process.env.KALKI_CA_PEM=readFileSync('data/certs/ca.pem','utf8');const token='a'.repeat(64);const call=(q,headers={})=>fetch(base+q,{headers});let res=await call('region=ap-south-1&path=api/fleet',{Authorization:'Bearer '+token});assert.equal(res.status,200);assert.deepEqual(await res.json(),{path:'/api/fleet',auth:'Bearer '+token});assert.equal((await call('region=ap-south-1&path=api/fleet')).status,401);assert.equal((await call('region=eu-central-1&path=api/fleet')).status,503);assert.equal((await call('region=https://evil.example&path=api/fleet')).status,404);assert.equal((await call('region=ap-south-1&path=api/fleet&token=secret')).status,400);assert.equal((await call('local=true')).status,503);delete process.env.KALKI_CA_PEM;assert.equal((await call('region=ap-south-1&path=api/fleet',{Authorization:'Bearer '+token})).status,502)}finally{for(const key of ['KALKI_AP_URL','KALKI_EU_URL','KALKI_CA_PEM']){if(saved[key]===undefined)delete process.env[key];else process.env[key]=saved[key]}await new Promise(r=>app.close(r));await new Promise(r=>receiver.close(r))}});
