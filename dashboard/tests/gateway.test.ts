import {test} from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import https from 'node:https';
import {readFileSync,existsSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import {configuration,guard,forward} from '../gateway.mjs';
const caPath=fileURLToPath(new URL('../../data/certs/ca.pem',import.meta.url));
const listen=(server:http.Server|https.Server)=>new Promise<number>(resolve=>server.listen(0,'127.0.0.1',()=>resolve((server.address() as {port:number}).port)));
test('Proof proxy verifies tenant ownership, strips credentials and rejects foreign origins',{skip:!existsSync(caPath)},async()=>{
 const cert=readFileSync(fileURLToPath(new URL('../../data/certs/server.pem',import.meta.url)));const key=readFileSync(fileURLToPath(new URL('../../data/certs/server.key',import.meta.url)));
 const receiver=https.createServer({cert,key},(req,res)=>{res.setHeader('Content-Type','application/json');res.end(JSON.stringify({tenant:req.headers.authorization==='Bearer '+ 'a'.repeat(64)?'tenant_alpha':'tenant_beta'}));});
 let tokenForwarded=false;const proof=http.createServer((req,res)=>{tokenForwarded ||= !!req.headers.authorization;res.setHeader('Content-Type','application/json');res.end(JSON.stringify({tenant:'tenant_alpha',collector_id:'host_11111111',batches:[]}));});
 const regionPort=await listen(receiver),proofPort=await listen(proof);const config=configuration({KALKI_CA_CERT:caPath,KALKI_AP_URL:`https://127.0.0.1:${regionPort}`,KALKI_EU_URL:`https://127.0.0.1:${regionPort}`,KALKI_PROOF_URL:`http://127.0.0.1:${proofPort}`});const protect=guard(config);
 const proxy=http.createServer((req,res)=>protect(req,res,()=>forward(config,req,res)));const port=await listen(proxy);const url=`http://127.0.0.1:${port}/local-proof/api/egress-proof`;
 try{
  const headers={'Authorization':'Bearer '+'a'.repeat(64),'X-Kalki-Region':'ap-south-1'};
  assert.equal((await fetch(url,{headers})).status,200);assert.equal(tokenForwarded,false);
  assert.equal((await fetch(url,{headers:{...headers,Authorization:'Bearer '+'b'.repeat(64)}})).status,403);
  assert.equal((await fetch(url,{headers:{...headers,Origin:'http://attacker.invalid'}})).status,403);
  assert.equal((await fetch(url)).status,401);
  assert.equal((await fetch(url+'?token=not-allowed',{headers})).status,400);
  assert.throws(()=>configuration({KALKI_CA_CERT:caPath,KALKI_PROOF_URL:'http://192.0.2.5:9090'}));
 }finally{config.agent.destroy();for(const server of [proxy,receiver,proof]){server.closeAllConnections();await new Promise<void>(resolve=>server.close(()=>resolve()));}}
});
