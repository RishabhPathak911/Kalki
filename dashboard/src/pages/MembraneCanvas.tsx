import {useEffect,useRef} from 'react';
const COLS=50,ROWS=30,SPRING=.025,DAMPING=.93,SPREAD=.18;
export default function MembraneCanvas(){
  const ref=useRef<HTMLCanvasElement>(null);
  useEffect(()=>{
    const canvas=ref.current;
    if(!canvas)return;
    const ctx=canvas.getContext('2d');
    if(!ctx)return;
    const surface=canvas.parentElement!;
    const motion=matchMedia('(prefers-reduced-motion: reduce)');
    let frame=0,w=0,h=0,last=0,t=0;
    let cursor={x:-9999,y:-9999,down:false};
    type Point={x:number;y:number;dx:number;dy:number;vx:number;vy:number;z:number;vz:number};
    let points:Point[]=[];

    function resize(){
      w=innerWidth;
      h=innerHeight;
      const dpr=devicePixelRatio||1;
      canvas!.width=Math.round(w*dpr);
      canvas!.height=Math.round(h*dpr);
      ctx!.setTransform(dpr,0,0,dpr,0,0);
      points=Array.from({length:COLS*ROWS},(_,i)=>({
        x:(i%COLS)*w/(COLS-1),
        y:Math.floor(i/COLS)*h/(ROWS-1),
        dx:0,dy:0,vx:0,vy:0,z:0,vz:0
      }));
      draw();
    }

    function step(){
      t+=1/60;
      const previous=points.map(p=>p.z);
      points.forEach((p,i)=>{
        const col=i%COLS,row=Math.floor(i/COLS);
        // Living, breathing organic mesh wave
        const breathe=Math.sin(t*0.85+col*0.22+row*0.18)*3.5+Math.cos(t*0.55-col*0.14+row*0.16)*2;
        p.vz+=(breathe-p.z)*0.012;
        p.vz-=p.z*SPRING;
        for(const j of [col>0?i-1:-1,col<COLS-1?i+1:-1,row>0?i-COLS:-1,row<ROWS-1?i+COLS:-1]){
          if(j>=0)p.vz+=(previous[j]-previous[i])*SPREAD;
        }
        const dx=p.x+p.dx-cursor.x,dy=p.y+p.dy-cursor.y;
        const dist=Math.hypot(dx,dy);
        const radius=cursor.down?220:150;
        if(dist<radius){
          const force=(1-dist/radius)*(cursor.down?30:15);
          p.vz+=force*0.4;
          if(dist>0.01){
            p.vx+=dx/dist*force*0.12;
            p.vy+=dy/dist*force*0.12;
          }
        }
        p.vx=(p.vx-p.dx*SPRING)*DAMPING;
        p.vy=(p.vy-p.dy*SPRING)*DAMPING;
        p.vz*=DAMPING;
        p.dx+=p.vx;
        p.dy+=p.vy;
        p.z+=p.vz;
      });
    }

    function draw(){
      ctx!.fillStyle='#08080a';
      ctx!.fillRect(0,0,w,h);
      ctx!.strokeStyle='rgba(255,255,255,0.024)';
      ctx!.lineWidth=0.5;
      ctx!.beginPath();
      points.forEach((p,i)=>{
        for(const j of [i%COLS<COLS-1?i+1:-1,i<COLS*(ROWS-1)?i+COLS:-1]){
          if(j>=0){
            const q=points[j];
            ctx!.moveTo(p.x+p.dx,p.y+p.dy-p.z*0.35);
            ctx!.lineTo(q.x+q.dx,q.y+q.dy-q.z*0.35);
          }
        }
      });
      ctx!.stroke();

      const maxDist=Math.hypot(w/2,h/2);
      points.forEach(p=>{
        const x=p.x+p.dx,y=p.y+p.dy-p.z*0.35;
        const centerFactor=Math.pow(1-Math.min(1,Math.hypot(p.x-w/2,p.y-h/2)/maxDist*1.3),1.4);
        const displacement=Math.abs(p.z);
        const active=displacement>1?Math.min(1,(displacement-1)/12):0;
        const color=[130+115*centerFactor,130+105*centerFactor,140+70*centerFactor].map((v,i)=>Math.round(v+([255,252,245][i]-v)*active));
        let alpha=0.25+0.45*centerFactor+active*0.3;
        const ellipseDist=Math.hypot((x-w/2)/(w*0.35),(y-h/2)/(h*0.35));
        if(ellipseDist<1)alpha*=1-(1-ellipseDist)*0.65;
        const size=0.8+centerFactor*0.6+active*0.7;
        if(displacement>1||centerFactor>0.4){
          const glow=ctx!.createRadialGradient(x,y,0,x,y,size+6);
          glow.addColorStop(0,`rgba(${color.join(',')},${alpha*0.35})`);
          glow.addColorStop(1,`rgba(${color.join(',')},0)`);
          ctx!.fillStyle=glow;
          ctx!.beginPath();
          ctx!.arc(x,y,size+6,0,Math.PI*2);
          ctx!.fill();
        }
        ctx!.fillStyle=`rgba(${color.join(',')},${alpha})`;
        ctx!.beginPath();
        ctx!.arc(x,y,size,0,Math.PI*2);
        ctx!.fill();
      });
    }

    function animate(now:number){
      if(now-last>=1000/60){
        step();
        draw();
        last=now;
      }
      frame=requestAnimationFrame(animate);
    }

    function move(e:MouseEvent){
      cursor.x=e.clientX;
      cursor.y=e.clientY;
      const box=surface.querySelector<HTMLElement>('.glass-shell');
      if(box){
        const r=box.getBoundingClientRect();
        box.style.setProperty('--mx',`${e.clientX-r.left}px`);
        box.style.setProperty('--my',`${e.clientY-r.top}px`);
      }
    }

    const leave=()=>{cursor={x:-9999,y:-9999,down:false}};
    const down=()=>{cursor.down=true};
    const up=()=>{cursor.down=false};
    function mode(){
      cancelAnimationFrame(frame);
      leave();
      resize();
      if(!motion.matches)frame=requestAnimationFrame(animate);
    }

    surface.addEventListener('mousemove',move);
    surface.addEventListener('mouseleave',leave);
    surface.addEventListener('mousedown',down);
    window.addEventListener('mouseup',up);
    window.addEventListener('blur',leave);
    window.addEventListener('resize',resize);
    motion.addEventListener('change',mode);
    mode();

    return()=>{
      cancelAnimationFrame(frame);
      surface.removeEventListener('mousemove',move);
      surface.removeEventListener('mouseleave',leave);
      surface.removeEventListener('mousedown',down);
      window.removeEventListener('mouseup',up);
      window.removeEventListener('blur',leave);
      window.removeEventListener('resize',resize);
      motion.removeEventListener('change',mode);
    };
  },[]);

  return <canvas ref={ref} className="membrane-canvas" aria-hidden="true"/>;
}
