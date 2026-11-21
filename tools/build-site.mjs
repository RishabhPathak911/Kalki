// One deployable site: marketing at /, authenticated console at /app/.
import {cp,mkdir,rm} from 'node:fs/promises';
import {spawnSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
const root=fileURLToPath(new URL('../',import.meta.url));
const result=spawnSync(process.execPath,[path.join(root,'dashboard/node_modules/typescript/bin/tsc'),'-b'],{cwd:path.join(root,'dashboard'),stdio:'inherit'});
if(result.status!==0)process.exit(result.status||1);
const build=spawnSync(process.execPath,[path.join(root,'dashboard/node_modules/vite/bin/vite.js'),'build','--base=/app/'],{cwd:path.join(root,'dashboard'),stdio:'inherit',env:{...process.env,VITE_HOSTED:'true',VITE_KALKI_HOME_URL:'/'}});
if(build.status!==0)process.exit(build.status||1);
// The only cleaned directory is the fixed generated workspace dist directory.
const output=path.join(root,'dist');await rm(output,{recursive:true,force:true});await mkdir(output,{recursive:true});
await cp(path.join(root,'public'),output,{recursive:true});
await cp(path.join(root,'dashboard/dist'),path.join(output,'app'),{recursive:true});
await cp(path.join(root,'docs'),path.join(output,'docs'),{recursive:true});
console.log('Built unified site: / and /app/');
