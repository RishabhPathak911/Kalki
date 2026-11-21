// Offline built-asset server and streaming reverse proxy. No Vite dev runtime.
import http from 'node:http';
import https from 'node:https';
import {readFile,stat} from 'node:fs/promises';
import {readFileSync} from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {configuration,guard,forward} from './gateway.mjs';
const config=configuration();const authorize=guard(config);const root=path.resolve(fileURLToPath(new URL('./dist/',import.meta.url)));
const mime={'.html':'text/html','.js':'text/javascript','.css':'text/css','.svg':'image/svg+xml','.json':'application/json'};
const handler=(req,res)=>authorize(req,res,async()=>{
 if(forward(config,req,res))return;
 try{
  const pathname=decodeURIComponent(new URL(req.url,'http://local').pathname);let file=path.resolve(root,'.'+pathname);
  if(file!==root&&!file.startsWith(root+path.sep)){res.writeHead(403);res.end();return;}
  try{if(!(await stat(file)).isFile())file=path.join(root,'index.html');}catch{if(path.extname(pathname)){res.writeHead(404);res.end();return;}file=path.join(root,'index.html');}
  const data=await readFile(file);res.writeHead(200,{'Content-Type':mime[path.extname(file)]||'application/octet-stream','Cache-Control':'no-store','X-Content-Type-Options':'nosniff','Content-Security-Policy':"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; font-src 'self'; frame-ancestors 'none'"});res.end(data);
 }catch{res.writeHead(500);res.end('Console unavailable');}
});
const server=process.env.KALKI_WEB_CERT&&process.env.KALKI_WEB_KEY?https.createServer({cert:readFileSync(process.env.KALKI_WEB_CERT),key:readFileSync(process.env.KALKI_WEB_KEY)},handler):http.createServer(handler);
server.listen(Number(process.env.PORT||5173),process.env.HOST||'127.0.0.1',()=>console.log('Kalki console listening on configured local address'));
