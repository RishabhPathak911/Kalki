/**
 * KALKI Cinematic WebGL Telemetry Engine
 * 3D procedural operational mesh & particle network loop (28s loop)
 */

export function initCinematic() {
  const cv = document.getElementById('hero-canvas');
  if (!cv) return null;

  const gl = cv.getContext('webgl', {
    antialias: false,
    alpha: false,
    powerPreference: 'high-performance'
  });
  if (!gl) return null;

  const reduceMotionQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
  const N = 30000;
  const LOOP = 28;

  const sh = (t, s) => {
    const o = gl.createShader(t);
    gl.shaderSource(o, s);
    gl.compileShader(o);
    return o;
  };

  const pr = (v, f) => {
    const p = gl.createProgram();
    gl.attachShader(p, sh(35633, v));
    gl.attachShader(p, sh(35632, 'precision highp float;' + f));
    gl.linkProgram(p);
    p.u = {};
    return p;
  };

  const U = (p, n) => p.u[n] || (p.u[n] = gl.getUniformLocation(p, n));

  const buf = a => {
    const b = gl.createBuffer();
    gl.bindBuffer(34962, b);
    gl.bufferData(34962, a, 35044);
    return b;
  };

  const at = (p, n, b, s, st, o) => {
    const l = gl.getAttribLocation(p, n);
    gl.bindBuffer(34962, b);
    gl.enableVertexAttribArray(l);
    gl.vertexAttribPointer(l, s, 5126, false, st * 4, o * 4);
  };

  const cl = () => {
    for (let i = 0; i < 8; i++) gl.disableVertexAttribArray(i);
  };

  const mt = (w, h) => {
    const t = gl.createTexture();
    gl.bindTexture(3553, t);
    gl.texImage2D(3553, 0, 6408, w, h, 0, 6408, 5121, null);
    [10240, 10241].forEach(k => gl.texParameteri(3553, k, 9729));
    [10242, 10243].forEach(k => gl.texParameteri(3553, k, 33071));
    const f = gl.createFramebuffer();
    gl.bindFramebuffer(36160, f);
    gl.framebufferTexture2D(36160, 36064, 3553, t, 0);
    return { t, f, w, h };
  };

  const WV = 'float wv(vec3 p,float t){return sin(p.x*.6+t*.8)*cos(p.z*.5+t*.6)*.7;}';

  const PP = pr(
    WV + `attribute vec3 a0,a1,a2,a3;attribute vec4 r;uniform mat4 V,P;uniform vec4 w;uniform float t,k,fo;varying vec3 vc;
void main(){vec3 p=a0*w.x+a1*w.y+a2*w.z+a3*w.w;float tr=1.-max(max(w.x,w.y),max(w.z,w.w));
p+=tr*tr*4.*vec3(sin(t*.7+r.y*40.+p.y),cos(t*.6+r.y*30.+p.z),sin(t*.5+r.y*20.+p.x))*3.;
p+=w.x*normalize(a0)*(.14*sin(t*1.5+a0.y*2.+r.y*6.)+.07*sin(t*2.3+a0.x*3.));
p.y+=w.w*wv(p,t);
p+=.03*vec3(sin(t+r.y*50.),cos(t*1.1+r.y*37.),sin(t*.9+r.y*23.));
vec4 mv=V*vec4(p,1.);float d=max(-mv.z,.05);gl_Position=P*mv;float coc=abs(d-fo)/d;
gl_PointSize=clamp(r.w*(1.+coc*1.5)*k/d,1.,90.);
vec3 tint=w.x*vec3(.7,.86,1.)+w.y*vec3(.5,.82,1.)+w.z*vec3(.25,.55,1.)+w.w*vec3(.18,.5,1.);
vec3 col=tint*(.45+.9*r.x);float bl=step(.5,sin(t*(2.+r.y*5.)+r.y*100.));
vec3 led=mix(vec3(.2,.6,1.),mix(vec3(.2,1.,.7),vec3(1.,.5,.3),step(.93,r.x)),step(.55,r.x));
col=mix(col,led*(.35+1.3*bl),r.z*w.z);
col*=exp(-d*.04)/(1.+coc*coc*2.)*(.65+.35*sin(t*2.+r.y*60.));vc=col*.9;}`,
    `varying vec3 vc;void main(){float d=length(gl_PointCoord-.5)*2.;float a=exp(-d*d*3.5)*(1.-smoothstep(.8,1.,d));gl_FragColor=vec4(vc*a,1.);}`
  );

  const PL = pr(
    WV + `attribute vec3 q0;attribute vec3 m;uniform mat4 V,P;uniform vec4 w;uniform float t;varying vec3 vc;
void main(){vec3 q=q0;float k=m.y;if(k>.5&&k<1.5)q.y+=wv(q,t);
vec4 mv=V*vec4(q,1.);gl_Position=P*mv;float d=max(-mv.z,.05);
float inn=k<.5?w.y:k<1.5?w.w:k<2.5?w.z:w.x;
vec3 c=k<.5?vec3(.4,.7,1.):k<1.5?vec3(.2,.55,1.):k<2.5?vec3(.3,.9,1.):vec3(.6,.8,1.);
float ph=fract(m.x-t*.22+m.z);vc=c*inn*(.14+1.6*exp(-ph*9.))*exp(-d*.04);}`,
    `varying vec3 vc;void main(){gl_FragColor=vec4(vc,1.);}`
  );

  const QV = 'attribute vec2 q;varying vec2 uv;void main(){uv=q*.5+.5;gl_Position=vec4(q,0,1);}';

  const PB = pr(
    QV,
    `varying vec2 uv;uniform sampler2D S;uniform vec2 d;void main(){vec3 c=(texture2D(S,uv+d).rgb+texture2D(S,uv-d).rgb+texture2D(S,uv+vec2(d.x,-d.y)).rgb+texture2D(S,uv+vec2(-d.x,d.y)).rgb)*.25;gl_FragColor=vec4(max(c-.28,0.)*1.5,1.);}`
  );

  const PG = pr(
    QV,
    `varying vec2 uv;uniform sampler2D S;uniform vec2 d;void main(){vec3 c=texture2D(S,uv).rgb*.227;
c+=(texture2D(S,uv+d).rgb+texture2D(S,uv-d).rgb)*.1945;c+=(texture2D(S,uv+d*2.).rgb+texture2D(S,uv-d*2.).rgb)*.1216;
c+=(texture2D(S,uv+d*3.).rgb+texture2D(S,uv-d*3.).rgb)*.054;c+=(texture2D(S,uv+d*4.).rgb+texture2D(S,uv-d*4.).rgb)*.016;gl_FragColor=vec4(c,1.);}`
  );

  const PF = pr(
    QV,
    `varying vec2 uv;uniform sampler2D S,B;uniform float t;
float h(vec2 p){return fract(sin(dot(p,vec2(12.9898,78.233)))*43758.5453);}
void main(){vec2 o=(uv-.5)*.006;vec3 c=vec3(texture2D(S,uv+o).r,texture2D(S,uv).g,texture2D(S,uv-o).b);
vec3 st=vec3(0.);for(int i=-6;i<=6;i++)st+=texture2D(B,uv+vec2(float(i)*.014,0.)).rgb*(1.-abs(float(i))/7.);
c+=texture2D(B,uv).rgb*1.35+st*.08;c=1.-exp(-c*1.3);c*=vec3(.88,.96,1.12);c+=vec3(.003,.008,.02);
float v=smoothstep(1.1,.3,length((uv-.5)*vec2(1.,.8)*1.5));c*=mix(.3,1.,v);
c+=(h(uv*vec2(1920.,1080.)+fract(t)*97.)-.5)*.035;gl_FragColor=vec4(c,1.);}`
  );

  const qb = buf(new Float32Array([-1, -1, 3, -1, -1, 3]));

  // Geometry construction
  const R = Math.random;
  const g = () => R() + R() + R() - 1.5;
  const S = [0, 1, 2, 3].map(() => new Float32Array(N * 3));
  const Rr = new Float32Array(N * 4);
  const nodes = [];
  for (let i = 0; i < 90; i++) nodes.push([(R() - 0.5) * 16, (R() - 0.5) * 9, R() * 15 - 12]);

  const edges = [];
  nodes.forEach((a) => {
    nodes
      .map((b, j) => [j, (a[0] - b[0]) ** 2 + (a[1] - b[1]) ** 2 + (a[2] - b[2]) ** 2])
      .sort((x, y) => x[1] - y[1])
      .slice(1, 4)
      .forEach(([j]) => {
        const b = nodes[j];
        edges.push([
          a,
          [
            (a[0] + b[0]) / 2 + g() * 2.4,
            (a[1] + b[1]) / 2 + g() * 2.4,
            (a[2] + b[2]) / 2 + g() * 2.4
          ],
          b
        ]);
      });
  });

  const bz = (e, u) =>
    [0, 1, 2].map(c => (1 - u) ** 2 * e[0][c] + 2 * (1 - u) * u * e[1][c] + u * u * e[2][c]);

  for (let i = 0; i < N; i++) {
    const f = i / N;
    let s = 0.03 + R() * 0.03;
    let p;
    const k = i / (N * 0.72);
    const y = 1 - 2 * k;
    const rr = Math.sqrt(Math.max(0, 1 - y * y));
    const th = i * 2.39996;

    S[0].set(
      f < 0.72
        ? [rr * Math.cos(th) * 3, y * 3, rr * Math.sin(th) * 3]
        : (() => {
            const a = R() * 6.283;
            const b = Math.acos(2 * R() - 1);
            const r = 3.3 + R() * 3;
            return [r * Math.sin(b) * Math.cos(a), r * Math.cos(b), r * Math.sin(b) * Math.sin(a)];
          })(),
      i * 3
    );

    if (f < 0.22) {
      const n = nodes[i % 90];
      p = [n[0] + g() * 0.45, n[1] + g() * 0.45, n[2] + g() * 0.45];
      s = 0.07;
    } else if (f < 0.8) {
      p = bz(edges[(R() * edges.length) | 0], R()).map(v => v + g() * 0.06);
    } else {
      p = [(R() - 0.5) * 18, (R() - 0.5) * 10, R() * 16 - 12];
    }
    S[1].set(p, i * 3);

    const rk = (R() * 90) | 0;
    const sd = rk & 1 ? 1 : -1;
    const z0 = -(rk >> 1) * 1.4 - 0.3;
    let led = 0;
    if (f < 0.62) {
      p = [sd * 2.2, 0.2 + ((R() * 14) | 0) * 0.16, z0 + (((R() * 6) | 0) - 2.5) * 0.14];
      led = 1;
    } else if (f < 0.84) {
      const t = R();
      p =
        t < 0.4
          ? [sd * 2.2, R() * 2.4, z0 + (R() < 0.5 ? 0.5 : -0.5)]
          : t < 0.7
          ? [sd * 2.2, 2.4, z0 + (R() - 0.5)]
          : [sd * 2.2 + sd * R() * 0.8, ((R() * 6) | 0) * 0.4, z0 + (R() - 0.5)];
    } else {
      p = [(R() - 0.5) * 3.6, R() < 0.5 ? 0.02 : 2.7, -R() * 64];
    }
    S[2].set(p, i * 3);

    const mx = (R() - 0.5) * 24;
    const mz = (R() - 0.5) * 24;
    S[3].set(
      [
        mx,
        i % 10
          ? Math.sin(mx * 0.5) * Math.cos(mz * 0.45) * 1.2 - 1 + (R() - 0.5) * 0.15
          : (R() - 0.5) * 10,
        mz
      ],
      i * 3
    );

    Rr.set([R(), R(), led, s], i * 4);
  }

  const LV = [];
  const poly = (pts, k, id) => {
    for (let j = 0; j < pts.length - 1; j++) {
      for (const q of [j, j + 1]) {
        LV.push(...pts[q], q / (pts.length - 1), k, id);
      }
    }
  };

  edges.forEach(e => poly(Array.from({ length: 15 }, (_, j) => bz(e, j / 14)), 0, R()));
  const sp = (la, lo) => [Math.cos(la) * Math.cos(lo) * 3, Math.sin(la) * 3, Math.cos(la) * Math.sin(lo) * 3];
  for (let i = 0; i < 18; i++) {
    poly(Array.from({ length: 49 }, (_, j) => sp(-1.5708 + (j * 3.1416) / 48, i * 0.3491)), 3, R());
  }
  for (let i = 1; i < 14; i++) {
    poly(Array.from({ length: 65 }, (_, j) => sp(-1.5708 + (i * 3.1416) / 14, j * 0.09817)), 3, R());
  }
  for (const x of [-1.9, 1.9]) {
    for (const y of [0.02, 2.6]) {
      poly(Array.from({ length: 101 }, (_, j) => [x, y, 2 - (67 * j) / 100]), 2, R());
    }
  }
  const my = (x, z) => Math.sin(x * 0.5) * Math.cos(z * 0.45) * 1.2 - 1;
  for (let i = 0; i <= 24; i++) {
    const c = -12 + i;
    poly(Array.from({ length: 61 }, (_, j) => {
      const x = -12 + j * 0.4;
      return [x, my(x, c), c];
    }), 1, R());
    poly(Array.from({ length: 61 }, (_, j) => {
      const z = -12 + j * 0.4;
      return [c, my(c, z), z];
    }), 1, R());
  }

  const bS = S.map(buf);
  const bR = buf(Rr);
  const bL = buf(new Float32Array(LV));
  const LN = LV.length / 6;

  // Camera spline keyframes
  const KT = [0, 5, 9, 13, 18, 23, 26];
  const KP = [
    [0, 0.4, 12, 0, 0, 0],
    [2, 0.3, 5, 0, 0, 0],
    [1, 0.4, 1, -2, 0, -6],
    [-2, 0, -6, 2, 0.5, -13],
    [0, 1.2, 1.5, 0, 1.15, -20],
    [0, 1.25, -34, 0, 1.2, -55],
    [0, 5, 10, 0, -1, 0]
  ];

  const cam = t => {
    const n = KT.length;
    let i = n - 1;
    KT.forEach((k, j) => {
      if (t >= k) i = j;
    });
    const t2 = i === n - 1 ? LOOP : KT[i + 1];
    const u = (t - KT[i]) / (t2 - KT[i]);
    const A = KP[(i + n - 1) % n];
    const B = KP[i];
    const C = KP[(i + 1) % n];
    const D = KP[(i + 2) % n];
    return B.map(
      (_, c) =>
        0.5 *
        (2 * B[c] +
          (-A[c] + C[c]) * u +
          (2 * A[c] - 5 * B[c] + 4 * C[c] - D[c]) * u * u +
          (-A[c] + 3 * B[c] - 3 * C[c] + D[c]) * u * u * u)
    );
  };

  const VK = [[0, 0], [5.5, 0], [8.5, 1], [13, 1], [16, 2], [22, 2], [25, 3], [26.5, 3], [28, 4]];
  const wts = t => {
    let v = 0;
    for (let i = 0; i < 8; i++) {
      const [a, x] = VK[i];
      const [b, y] = VK[i + 1];
      if (t >= a && t <= b) {
        v = x + ((y - x) * (t - a)) / (b - a);
        break;
      }
    }
    const i = Math.floor(v);
    const f = v - i;
    const s = f * f * (3 - 2 * f);
    const w = [0, 0, 0, 0];
    w[i % 4] += 1 - s;
    w[(i + 1) % 4] += s;
    return w;
  };

  const nz = a => {
    const l = Math.hypot(...a);
    return a.map(v => v / l);
  };
  const cr = (a, b) => [
    a[1] * b[2] - a[2] * b[1],
    a[2] * b[0] - a[0] * b[2],
    a[0] * b[1] - a[1] * b[0]
  ];
  const dt = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];

  const look = (e, c) => {
    const z = nz([e[0] - c[0], e[1] - c[1], e[2] - c[2]]);
    const x = nz(cr([0, 1, 0], z));
    const y = cr(z, x);
    return new Float32Array([
      x[0], y[0], z[0], 0,
      x[1], y[1], z[1], 0,
      x[2], y[2], z[2], 0,
      -dt(x, e), -dt(y, e), -dt(z, e), 1
    ]);
  };

  const persp = (fv, a, n, f) => {
    const q = 1 / Math.tan(fv / 2);
    return new Float32Array([
      q / a, 0, 0, 0,
      0, q, 0, 0,
      0, 0, (f + n) / (n - f), -1,
      0, 0, (2 * f * n) / (n - f), 0
    ]);
  };

  // Render setup
  let sc = 1;
  let F, P1, P2;
  const size = () => {
    cv.width = Math.round(1920 * sc);
    cv.height = Math.round(1080 * sc);
    F = mt(cv.width, cv.height);
    P1 = mt(cv.width >> 2, cv.height >> 2);
    P2 = mt(cv.width >> 2, cv.height >> 2);
  };
  size();

  const fp = (p, o, ins, set) => {
    gl.disable(3042);
    gl.bindFramebuffer(36160, o ? o.f : null);
    gl.viewport(0, 0, o ? o.w : cv.width, o ? o.h : cv.height);
    gl.useProgram(p);
    cl();
    at(p, 'q', qb, 2, 2, 0);
    ins.forEach((x, i) => {
      gl.activeTexture(33984 + i);
      gl.bindTexture(3553, x.t);
      gl.uniform1i(U(p, i ? 'B' : 'S'), i);
    });
    if (set) set();
    gl.drawArrays(4, 0, 3);
  };

  const draw = tt => {
    const c = cam(tt);
    const e = [c[0] + 0.02 * Math.sin(tt * 1.3), c[1] + 0.015 * Math.sin(tt * 1.7), c[2]];
    const tg = c.slice(3);
    const w = wts(tt);
    const V = look(e, tg);
    const Pm = persp(0.95, cv.width / cv.height, 0.1, 200);
    const fo = Math.min(Math.max(0.6 * Math.hypot(e[0] - tg[0], e[1] - tg[1], e[2] - tg[2]), 3), 16);

    gl.bindFramebuffer(36160, F.f);
    gl.viewport(0, 0, cv.width, cv.height);
    gl.clearColor(0.01, 0.02, 0.05, 1);
    gl.clear(16384);
    gl.enable(3042);
    gl.blendFunc(1, 1);

    gl.useProgram(PL);
    cl();
    at(PL, 'q0', bL, 3, 6, 0);
    at(PL, 'm', bL, 3, 6, 3);
    gl.uniformMatrix4fv(U(PL, 'V'), false, V);
    gl.uniformMatrix4fv(U(PL, 'P'), false, Pm);
    gl.uniform4fv(U(PL, 'w'), w);
    gl.uniform1f(U(PL, 't'), tt);
    gl.drawArrays(1, 0, LN);

    gl.useProgram(PP);
    cl();
    bS.forEach((b, i) => at(PP, 'a' + i, b, 3, 3, 0));
    at(PP, 'r', bR, 4, 4, 0);
    gl.uniformMatrix4fv(U(PP, 'V'), false, V);
    gl.uniformMatrix4fv(U(PP, 'P'), false, Pm);
    gl.uniform4fv(U(PP, 'w'), w);
    gl.uniform1f(U(PP, 't'), tt);
    gl.uniform1f(U(PP, 'k'), cv.height * 0.9);
    gl.uniform1f(U(PP, 'fo'), fo);
    gl.drawArrays(0, 0, N);

    const tx = 1 / P1.w;
    const ty = 1 / P1.h;
    fp(PB, P1, [F], () => gl.uniform2f(U(PB, 'd'), 1 / F.w, 1 / F.h));
    [[1, 0], [0, 1], [3, 0], [0, 3], [6, 0], [0, 6]].forEach(([a, b], i) => {
      const s = i % 2 ? P1 : P2;
      const o = i % 2 ? P2 : P1;
      fp(PG, o, [s], () => gl.uniform2f(U(PG, 'd'), a * tx, b * ty));
    });
    fp(PF, null, [F, P1], () => gl.uniform1f(U(PF, 't'), tt));
  };

  // Animation loop management
  let tt = 0;
  let isPlaying = !reduceMotionQuery.matches && !document.body.classList.contains('paused');
  let isVisible = true;
  let rafId = null;
  let last = performance.now();
  let acc = 0;
  let n = 0;

  const frame = now => {
    if (!isPlaying || !isVisible) {
      rafId = null;
      return;
    }
    const d = Math.min((now - last) / 1000, 0.1);
    last = now;
    tt = (tt + d) % LOOP;
    draw(tt);

    acc += d;
    if (++n === 90) {
      if (acc / 90 > 0.026 && sc > 0.5) {
        sc *= 0.8;
        size();
      }
      acc = 0;
      n = 0;
    }

    rafId = requestAnimationFrame(frame);
  };

  function start() {
    if (!isPlaying) {
      isPlaying = true;
      last = performance.now();
      if (!rafId && isVisible) {
        rafId = requestAnimationFrame(frame);
      }
    }
  }

  function pause() {
    isPlaying = false;
    if (rafId) {
      cancelAnimationFrame(rafId);
      rafId = null;
    }
  }

  function setPausedState(paused) {
    if (paused || reduceMotionQuery.matches) {
      pause();
      draw(3); // Render static frame at t=3s
    } else {
      start();
    }
  }

  // Viewport intersection observer to avoid rendering offscreen
  const heroEl = document.querySelector('.hero');
  if (heroEl && 'IntersectionObserver' in window) {
    const observer = new IntersectionObserver(
      entries => {
        entries.forEach(entry => {
          isVisible = entry.isIntersecting;
          if (isVisible && isPlaying && !rafId) {
            last = performance.now();
            rafId = requestAnimationFrame(frame);
          } else if (!isVisible && rafId) {
            cancelAnimationFrame(rafId);
            rafId = null;
          }
        });
      },
      { threshold: 0.02 }
    );
    observer.observe(heroEl);
  }

  // Handle motion state initially
  if (reduceMotionQuery.matches || document.body.classList.contains('paused')) {
    draw(3);
  } else {
    rafId = requestAnimationFrame(frame);
  }

  return {
    setPausedState,
    drawStatic: () => draw(3)
  };
}
