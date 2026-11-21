import type {Session} from './types';
export class ApiError extends Error {status:number;constructor(status:number){super(status===401?'Session expired. Sign in again.':status===403?'This workspace does not own the selected local collector.':status===503||status===502?'Service unavailable. Check the receiver or local collector.':`Request could not complete (${status}).`);this.status=status;}}
export async function api<T>(session:Session,path:string,options:RequestInit={}):Promise<T>{
 const local=path.startsWith('/local-proof/');const response=await fetch(local?path:`/region/${session.region}${path}`,{...options,cache:'no-store',headers:{'Content-Type':'application/json',Authorization:`Bearer ${session.token}`,'X-Kalki-Region':session.region,...options.headers}});
 if(!response.ok)throw new ApiError(response.status);return response.json() as Promise<T>;
}
export const post=<T>(s:Session,path:string,body:unknown={})=>api<T>(s,path,{method:'POST',body:JSON.stringify(body)});
export function loadSession():Session|null {try{const s=JSON.parse(sessionStorage.getItem('kalki.session')||'null');if(s&&/^[a-f0-9]{64}$/.test(s.token)&&['ap-south-1','eu-central-1'].includes(s.region)&&Date.parse(s.expires_at)>Date.now())return s;}catch{}sessionStorage.removeItem('kalki.session');return null;}
export function saveSession(s:Session|null){if(s)sessionStorage.setItem('kalki.session',JSON.stringify(s));else sessionStorage.removeItem('kalki.session');}
