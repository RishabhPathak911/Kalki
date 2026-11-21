import https from 'node:https';

const origins = { 'ap-south-1': 'KALKI_AP_URL', 'eu-central-1': 'KALKI_EU_URL' };

function origin(region) {
  const value = process.env[origins[region]];
  if (!value) return null;
  try {
    const u = new URL(value);
    if (u.protocol !== 'https:' || u.username || u.password || u.search || u.hash || u.pathname !== '/') return null;
    return u;
  } catch {
    return null;
  }
}

const allowed = /^api\/(auth\/(login|logout)|collectors\/tokens|fleet|incidents|thresholds|records|global|audit(\/verify)?|stream|validation|ai\/(ask|analyze-incident)|remediations(\/propose|\/[a-zA-Z0-9_-]+\/(approve|rollback))?)$/;

export default async function handler(req, res) {
  res.setHeader('Cache-Control', 'no-store');
  res.setHeader('X-Content-Type-Options', 'nosniff');
  const fail = (code, error) => {
    if (!res.headersSent) {
      res.statusCode = code;
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify({ error }));
    } else res.end();
  };

  const query = req.query || Object.fromEntries(new URL(req.url, 'https://local').searchParams);
  const hasRealBackend = Object.values(origins).some(k => Boolean(process.env[k]));

  if (query.status === 'true') {
    if (req.method !== 'GET') return fail(405, 'Method not allowed');
    res.setHeader('Content-Type', 'application/json');
    // If real backend origins configured, report actual connectivity; otherwise in standalone mode both regions are enabled
    const regions = hasRealBackend
      ? Object.fromEntries(Object.keys(origins).map(r => [r, Boolean(origin(r))]))
      : { 'ap-south-1': false, 'eu-central-1': false };
    return res.end(JSON.stringify({ regions, local_proof: false }));
  }

  if (query.local === 'true') return fail(503, 'Local collector inspection is available only from its LAN console.');

  const region = query.region;
  const path = query.path;
  if (typeof region !== 'string' || !Object.hasOwn(origins, region) || typeof path !== 'string' || !allowed.test(path)) {
    return fail(404, 'Route not found');
  }
  if (!['GET', 'POST'].includes(req.method)) return fail(405, 'Method not allowed');

  const destination = origin(region);
  const headers = { Accept: req.headers.accept || 'application/json' };
  for (const name of ['authorization', 'last-event-id']) {
    if (req.headers[name]) headers[name] = req.headers[name];
  }

  let body;
  if (req.method === 'POST') {
    try {
      if (req.body !== undefined) {
        body = typeof req.body === 'string' ? req.body : JSON.stringify(req.body);
      } else {
        const chunks = [];
        let size = 0;
        for await (const chunk of req) {
          size += chunk.length;
          if (size > 8192) return fail(413, 'Request too large');
          chunks.push(chunk);
        }
        body = Buffer.concat(chunks).toString();
      }
      if (Buffer.byteLength(body) > 8192) return fail(413, 'Request too large');
      JSON.parse(body);
      headers['Content-Type'] = 'application/json';
    } catch {
      return fail(400, 'Invalid JSON');
    }
  }

  // If a real backend origin is configured for any region, enforce proxy rules
  if (hasRealBackend || destination) {
    if (!destination) return fail(503, 'This workspace service is not connected yet. Contact the site owner.');
    const params = new URLSearchParams();
    for (const [k, v] of Object.entries(query)) {
      if (['region', 'path'].includes(k)) continue;
      if (k !== 'collector_id' || path !== 'api/records' || typeof v !== 'string') return fail(400, 'Unsupported query parameter');
      params.set(k, v);
    }
    if (path !== 'api/auth/login' && !/^Bearer [a-f0-9]{64}$/.test(headers.authorization || '')) {
      return fail(401, 'Sign in to continue');
    }
    const ca = process.env.KALKI_CA_PEM?.replaceAll('\\n', '\n');
    return new Promise(resolve => {
      let finished = false;
      const done = () => {
        if (finished) return;
        finished = true;
        clearTimeout(timer);
        resolve();
      };
      const upstream = https.request(
        new URL('/' + path + (params.size ? '?' + params : ''), destination),
        { method: req.method, headers, ca: ca || undefined, rejectUnauthorized: true },
        response => {
          res.statusCode = response.statusCode || 502;
          res.setHeader('Content-Type', response.headers['content-type'] || 'application/json');
          if (path === 'api/stream') res.setHeader('X-Accel-Buffering', 'no');
          res.flushHeaders?.();
          response.pipe(res);
          response.on('error', () => {
            fail(502, 'Workspace connection interrupted');
            done();
          });
          response.on('end', done);
        }
      );
      const timer = setTimeout(() => {
        upstream.destroy();
        fail(504, 'Workspace connection timed out');
        done();
      }, path === 'api/stream' ? 50000 : 10000);
      upstream.on('error', () => {
        fail(502, 'Workspace service is unavailable');
        done();
      });
      res.on('close', () => {
        upstream.destroy();
        done();
      });
      upstream.end(body);
    });
  }

  return fail(503, 'This workspace service is not connected yet. Contact the site owner.');
}
