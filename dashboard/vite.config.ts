import {defineConfig,loadEnv, type ProxyOptions} from 'vite';
import react from '@vitejs/plugin-react';
// @ts-expect-error Shared Node gateway has runtime-validated configuration.
import {configuration,guard} from './gateway.mjs';
export default defineConfig(({command,mode})=>{
 if(command==='build') return {plugins:[react()],build:{rollupOptions:{output:{manualChunks:{charts:['recharts']}}}}};
 const env={...loadEnv(mode,process.cwd(),''),...process.env};const config=configuration(env);
 const proxy:Record<string,ProxyOptions>={};
 for(const [region,target] of Object.entries(config.origins)){
  const prefix='/region/'+region;proxy[prefix]={target:String(target),secure:true,agent:config.agent,changeOrigin:true,rewrite:path=>path.slice(prefix.length)};
 }
 proxy['/local-proof']={target:config.proof,changeOrigin:true,rewrite:path=>path.slice('/local-proof'.length)};
 const protect={name:'kalki-workspace-proof-guard',configureServer(server:any){server.middlewares.use(guard(config));},configurePreviewServer(server:any){server.middlewares.use(guard(config));}};
 return {plugins:[react(),protect],server:{port:5173,strictPort:true,proxy},preview:{port:4173,strictPort:true,proxy}};
});
