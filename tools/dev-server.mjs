import http from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const initialPort = Number(process.env.PORT || 8080);
const types = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.webp': 'image/webp',
  '.jpg': 'image/jpeg',
  '.png': 'image/png',
  '.mp4': 'video/mp4',
  '.woff2': 'font/woff2',
  '.md': 'text/plain; charset=utf-8'
};

const server = http.createServer(async (req, res) => {
  try {
    const requested = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);
    if (requested === '/app') {
      if (process.env.KALKI_DASHBOARD_URL) {
        res.writeHead(302, { Location: process.env.KALKI_DASHBOARD_URL });
        res.end();
        return;
      }
      res.writeHead(302, { Location: '/app/' });
      res.end();
      return;
    }

    if (requested.startsWith('/app/')) {
      const distAppDir = path.join(root, 'dist');
      let relPath = requested.slice(1);
      if (relPath === 'app/') relPath = 'app/index.html';
      let filePath = path.resolve(distAppDir, relPath);
      let fileStat;
      try {
        fileStat = await stat(filePath);
        if (!fileStat.isFile()) throw new Error();
      } catch {
        filePath = path.join(distAppDir, 'app/index.html');
        try {
          fileStat = await stat(filePath);
        } catch {
          res.writeHead(302, { Location: 'http://localhost:5173' });
          res.end();
          return;
        }
      }
      const body = await readFile(filePath);
      res.writeHead(200, {
        'Content-Type': types[path.extname(filePath)] || 'application/octet-stream',
        'Content-Length': body.length,
        'Cache-Control': 'no-cache',
        'X-Content-Type-Options': 'nosniff'
      });
      res.end(req.method === 'HEAD' ? undefined : body);
      return;
    }

    if (requested === '/api/deployment-status') {
      res.writeHead(200, {
        'Content-Type': 'application/json',
        'Cache-Control': 'no-cache'
      });
      res.end(JSON.stringify({ regions: { 'ap-south-1': true, 'eu-central-1': true }, local_proof: false }));
      return;
    }

    const isDoc = requested.startsWith('/docs/');
    const base = path.join(root, isDoc ? 'docs' : 'public');
    const relative = isDoc ? requested.slice('/docs/'.length) : requested.slice(1) || 'index.html';
    const file = path.resolve(base, relative);
    if (!file.startsWith(base + path.sep) && file !== path.join(base, 'index.html')) {
      throw new Error('access denied');
    }
    const fileStat = await stat(file);
    if (!fileStat.isFile()) throw new Error('not found');
    const body = await readFile(file);
    res.writeHead(200, {
      'Content-Type': types[path.extname(file)] || 'application/octet-stream',
      'Content-Length': body.length,
      'Cache-Control': 'no-cache',
      'X-Content-Type-Options': 'nosniff'
    });
    res.end(req.method === 'HEAD' ? undefined : body);
  } catch {
    res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
    res.end('Page not found');
  }
});

function startServer(port) {
  server.listen(port, '127.0.0.1', () => {
    console.log(`Kalki is running at http://localhost:${port}`);
  });

  server.on('error', (err) => {
    if (err.code === 'EADDRINUSE') {
      console.warn(`Port ${port} in use, trying ${port + 1}...`);
      server.close();
      startServer(port + 1);
    } else {
      console.error('Server error:', err);
    }
  });
}

startServer(initialPort);
