import {test} from 'node:test';
import assert from 'node:assert/strict';
import {SSEParser,stream} from '../src/lib/sse.ts';
import {orderedPoints,percentile} from '../src/lib/data.ts';
test('SSE handles split CRLF, multiline data, comments and persistent IDs',()=>{
 const p=new SSEParser();assert.deepEqual(p.feed(': comment\r\nid: 7\r'),[]);assert.deepEqual(p.feed('\nevent: telemetry\r\ndata: {"value":\r\ndata: 1}\r\n\r\n'),[{event:'telemetry',id:'7',data:'{"value":\n1}'}]);
 assert.deepEqual(p.feed('data: next\n\n'),[{event:'message',id:'7',data:'next'}]);
 assert.deepEqual(p.feed('id: bad\0id\ndata: final\n\n'),[{event:'message',id:'7',data:'final'}]);
});
test('SSE reconnects with Bearer and Last-Event-ID, abort releases readers',async()=>{
 const original=globalThis.fetch;const controller=new AbortController();let calls=0;const received:string[]=[];
 globalThis.fetch=async(input,options)=>{calls++;assert.equal(String(input),'/api/stream');const headers=options?.headers as Record<string,string>;assert.equal(headers.Authorization,'Bearer test-credential');if(calls>1)assert.equal(headers['Last-Event-ID'],'41');const data=`id: ${calls===1?41:42}\nevent: telemetry\ndata: {}\n\n`;return new Response(new ReadableStream({start(c){c.enqueue(new TextEncoder().encode(data));c.close();}}),{headers:{'Content-Type':'text/event-stream'}});};
 try{await stream('/api/stream','test-credential',controller.signal,e=>{received.push(e.id);if(e.id==='42')controller.abort();},()=>{},()=>assert.fail('unexpected auth failure'));assert.deepEqual(received,['41','42']);assert.equal(calls,2);}finally{globalThis.fetch=original;}
});
test('Out-of-order chart points sort by event time and deduplicate',()=>{
 const make=(key:string,at:string)=>({key,collected_at:at,tenant:'tenant_alpha',collector_id:'host_12345678',region:'ap-south-1' as const,source:'live',metrics:{},risk_events:[],redaction_counts:{}});
 const old=make('1','2026-01-01T00:02:00.000Z');const late=make('2','2026-01-01T00:00:30.000Z');assert.deepEqual(orderedPoints([old],[late,old]).map(p=>p.key),['2','1']);assert.equal(percentile([100,300,200],.5),200);assert.equal(percentile([], .95),null);
});
