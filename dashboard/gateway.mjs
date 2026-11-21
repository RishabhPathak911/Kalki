import http from 'node:http';
import https from 'node:https';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
export function configuration(env=process.env) {
 const ca=readFileSync(env.KALKI_CA_CERT || fileURLToPath(new URL('../data/certs/ca.pem',import.meta.url)));
 const origins={ 'ap-south-1':env.KALKI_AP_URL||env.VITE_AP_URL||'https://127.0.0.1:8443', 'eu-central-1':env.KALKI_EU_URL||env.VITE_EU_URL||'https://127.0.0.1:8444' };
 for(const value of Object.values(origins)){const u=new URL(value);if(u.protocol!=='https:'||u.username||u.password||u.search||u.hash||u.pathname!=='/')throw Error('Regional URLs must be HTTPS origins');}
 const proof=new URL(env.KALKI_PROOF_URL||'http://127.0.0.1:9090');
 if(proof.protocol!=='http:'||!['127.0.0.1','[::1]'].includes(proof.hostname)||proof.username||proof.password||proof.search||proof.hash||proof.pathname!=='/')throw Error('Proof target must be a literal loopback HTTP origin');
 return {origins,proof:proof.origin,agent:new https.Agent({ca,rejectUnauthorized:true})};
}
const allowedLocal=new Set(['/api/identity','/api/egress-proof','/api/attempt-violation','/api/demo/inject-secret','/api/demo/bruteforce','/api/demo/cpu-stress','/api/demo/net-degrade','/api/demo/outage','/api/demo/dup-ooo']);
function readJSON(base,path,headers,agent){return new Promise((resolve,reject)=>{
 const u=new URL(path,base);const client=u.protocol==='https:'?https:http;
 const request=client.get(u,{headers,agent:u.protocol==='https:'?agent:undefined,timeout:3000},response=>{let text='';response.on('data',chunk=>{text+=chunk;if(text.length>2e6){response.destroy();reject(Error('response too large'));}});response.on('end',()=>{try{if(response.statusCode!==200)throw Error(String(response.statusCode));resolve(JSON.parse(text));}catch{reject(Error('authorization unavailable'));}});response.on('error',reject);});
 request.on('timeout',()=>request.destroy(Error('timeout')));request.on('error',reject);
});}
export function guard(config){return async (req,res,next)=>{
 const path=new URL(req.url,'http://local');
 if(!path.pathname.startsWith('/region/')&&!path.pathname.startsWith('/local-proof/')){next();return;}
 const fail=(code)=>{res.writeHead(code,{'Content-Type':'application/json','Cache-Control':'no-store'});res.end(JSON.stringify({error:code===403?'Workspace does not own this local collector':'Gateway request unavailable'}));};
 if(path.searchParams.has('token')||path.searchParams.has('access_token')){fail(400);return;}
 if(req.headers.origin){try{const origin=new URL(req.headers.origin);if(origin.host!==req.headers.host){fail(403);return;}}catch{fail(403);return;}}
 if(!path.pathname.startsWith('/local-proof/')){next();return;}
 if(!allowedLocal.has(path.pathname.slice('/local-proof'.length))||path.search){fail(404);return;}
 const region=req.headers['x-kalki-region'];const token=req.headers.authorization;
 if(!config.origins[region]||!/^Bearer [a-f0-9]{64}$/.test(token||'')){fail(401);return;}
 try{
  const account=await readJSON(config.origins[region],'/api/global',{Authorization:token},config.agent);
  const collector=await readJSON(config.proof,'/api/identity',{},config.agent);
  if(account.tenant!==collector.tenant){fail(403);return;}
  // Only after authentication and ownership verification may the proxy rewrite
  // Origin for the loopback listener. Browser origins are checked above.
  delete req.headers.authorization;delete req.headers['x-kalki-region'];req.headers.origin=config.proof;
  next();
 }catch{fail(503);}
};}
export function forward(config,req,res){
 let base,prefix;const match=req.url.match(/^\/region\/(ap-south-1|eu-central-1)(\/.*)$/);
 if(match){base=config.origins[match[1]];prefix='/region/'+match[1];}else if(req.url.startsWith('/local-proof/')){base=config.proof;prefix='/local-proof';}else{return false;}
 const target=new URL(base+req.url.slice(prefix.length));const headers={...req.headers,host:target.host};delete headers.connection;
 const remote=(target.protocol==='https:'?https:http).request(target,{method:req.method,headers,agent:target.protocol==='https:'?config.agent:undefined},up=>{res.writeHead(up.statusCode||502,{...up.headers,'Cache-Control':'no-store'});up.pipe(res);up.on('error',()=>res.destroy());});
 remote.on('error',()=>{if(!res.headersSent)res.writeHead(502,{'Content-Type':'application/json'});res.end('{"error":"Regional service unavailable"}');});
 req.on('aborted',()=>remote.destroy());res.on('close',()=>remote.destroy());req.pipe(remote);return true;
}
