export type SSEMessage={event:string;id:string;data:string};
// Incremental CR/LF parser; incomplete chunks and multi-line data are retained.
export class SSEParser {
 private buffer='';private data:string[]=[];private event='message';private id='';
 feed(chunk:string):SSEMessage[]{this.buffer+=chunk;if(this.buffer.length>4_194_304)throw new Error('Event limit exceeded');const events:SSEMessage[]=[];
  while(true){const match=/\r\n|\r|\n/.exec(this.buffer);if(!match)break;if(match[0]==='\r'&&match.index===this.buffer.length-1)break;const line=this.buffer.slice(0,match.index);this.buffer=this.buffer.slice(match.index+match[0].length);
   if(line===''){if(this.data.length)events.push({event:this.event,id:this.id,data:this.data.join('\n')});this.data=[];this.event='message';continue;}
   if(line.startsWith(':'))continue;const colon=line.indexOf(':');const field=colon<0?line:line.slice(0,colon);let value=colon<0?'':line.slice(colon+1);if(value.startsWith(' '))value=value.slice(1);
   if(field==='data')this.data.push(value);else if(field==='event')this.event=value;else if(field==='id'&&!value.includes('\0'))this.id=value;
  }return events;
 }
}
function delay(ms:number,signal:AbortSignal){return new Promise<void>(resolve=>{if(signal.aborted){resolve();return;}const done=()=>{clearTimeout(timer);signal.removeEventListener('abort',done);resolve();};const timer=setTimeout(done,ms);signal.addEventListener('abort',done,{once:true});});}
export async function stream(url:string,token:string,signal:AbortSignal,onMessage:(event:SSEMessage)=>void,onState:(state:string)=>void,onUnauthorized:()=>void){
 let lastID='';let backoff=500;
 while(!signal.aborted){let reader:ReadableStreamDefaultReader<Uint8Array>|undefined;
  try{onState('CONNECTING');const response=await fetch(url,{headers:{Authorization:`Bearer ${token}`,...(lastID?{'Last-Event-ID':lastID}:{})},signal,cache:'no-store'});
   if(response.status===401||response.status===403){onUnauthorized();return;}if(!response.ok||!response.body||!response.headers.get('Content-Type')?.includes('text/event-stream'))throw new Error('Stream unavailable');onState('CONNECTED');
   reader=response.body.getReader();const parser=new SSEParser();const decoder=new TextDecoder();
   while(!signal.aborted){const {value,done}=await reader.read();if(done)break;for(const message of parser.feed(decoder.decode(value,{stream:true}))){lastID=message.id;onMessage(message);backoff=500;}}
  }catch{if(signal.aborted)return;}finally{await reader?.cancel().catch(()=>{});reader?.releaseLock();}
  if(signal.aborted)return;onState('RECONNECTING');await delay(backoff,signal);backoff=Math.min(8000,backoff*2);
 }
}
