// tools/mock-api.mjs
// Offline Mock API provider for local Vite / Node gateway preview.
// Uses real Kalki module names (Prana, Kavach, Nadi, Setu, Medha) with no external services.
// Note: Real Kalki has NO eBPF, NO SQLite, NO hardware enclaves.
// Real Tier 1 uses Go OS polling, regex/entropy filtering, 5s rollups, and disk WAL.

let mockNodes = [
  {
    collector_id: 'host_edge_01',
    tenant: 'tenant_alpha',
    region: 'ap-south-1',
    health: 'Optimal',
    connection_state: 'ONLINE',
    last_seen_at: new Date().toISOString(),
    last_seen_age_s: 1,
    metrics: {
      cpu_pct: { min: 14.2, max: 28.5, avg: 19.4, p95: 26.1, count: 60 },
      mem_pct: { min: 46.1, max: 54.0, avg: 49.8, p95: 53.2, count: 60 },
      disk_pct: { min: 38.0, max: 38.4, avg: 38.2, p95: 38.3, count: 60 },
      bandwidth_mbps: { min: 120, max: 380, avg: 245.0, p95: 350.0, count: 60 },
      latency_ms: { min: 4.1, max: 12.8, avg: 6.4, p95: 9.8, count: 60 },
      tcp_fail_ratio: { min: 0, max: 0.002, avg: 0.0004, p95: 0.001, count: 60 }
    },
    source_badges: ['live', 'prana-os', 'setu-wal'],
    state_transitions: [
      {
        at: new Date(Date.now() - 3600000).toISOString().replace('T', ' ').slice(0, 19) + ' UTC',
        from: 'Nominal',
        to: 'Optimal',
        connection_from: 'ONLINE',
        connection_to: 'ONLINE',
        reason: 'Adaptive baseline converged within normal variance'
      }
    ],
    duplicates_rejected: 0,
    wal_depth: 0,
    records_accepted: 3420,
    data_age_s: 1
  },
  {
    collector_id: 'host_edge_02',
    tenant: 'tenant_alpha',
    region: 'ap-south-1',
    health: 'Nominal',
    connection_state: 'ONLINE',
    last_seen_at: new Date().toISOString(),
    last_seen_age_s: 2,
    metrics: {
      cpu_pct: { min: 22.0, max: 48.0, avg: 32.5, p95: 42.0, count: 60 },
      mem_pct: { min: 58.0, max: 66.0, avg: 61.2, p95: 64.5, count: 60 },
      disk_pct: { min: 42.0, max: 44.0, avg: 43.1, p95: 43.8, count: 60 },
      bandwidth_mbps: { min: 90, max: 280, avg: 180.0, p95: 250.0, count: 60 },
      latency_ms: { min: 5.0, max: 18.0, avg: 8.5, p95: 14.0, count: 60 },
      tcp_fail_ratio: { min: 0, max: 0.005, avg: 0.001, p95: 0.003, count: 60 }
    },
    source_badges: ['live', 'prana-probe'],
    state_transitions: [],
    duplicates_rejected: 0,
    wal_depth: 0,
    records_accepted: 2890,
    data_age_s: 2
  }
];

export function handleMockApi(req, res, pathname) {
  const method = req.method;

  // Handle remove collector / delete fleet
  if (pathname.includes('/api/fleet/remove') || (method === 'DELETE' && pathname.includes('/api/fleet'))) {
    let body = '';
    req.on('data', chunk => { body += chunk; });
    req.on('end', () => {
      let targetId = '';
      try {
        const parsed = JSON.parse(body || '{}');
        targetId = parsed.collector_id || '';
      } catch {}
      if (!targetId) {
        const parts = pathname.split('/');
        targetId = parts[parts.length - 1];
      }
      if (targetId) {
        mockNodes = mockNodes.filter(n => n.collector_id !== targetId);
      }
      res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
      res.end(JSON.stringify({ success: true, removed: targetId, remaining: mockNodes.length }));
    });
    return true;
  }

  // Auth Login
  if (pathname.endsWith('/api/auth/login')) {
    res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    res.end(JSON.stringify({
      token: 'a'.repeat(64),
      tenant: 'tenant_alpha',
      region: 'ap-south-1',
      expires_at: new Date(Date.now() + 86400000).toISOString()
    }));
    return true;
  }

  // Fleet
  if (pathname.endsWith('/api/fleet')) {
    const now = new Date();
    const updated = mockNodes.map((n, i) => ({
      ...n,
      last_seen_at: now.toISOString(),
      last_seen_age_s: 1
    }));
    res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    res.end(JSON.stringify({
      nodes: updated,
      counters: { suppressed_alerts: 4 }
    }));
    return true;
  }

  // Incidents
  if (pathname.endsWith('/api/incidents')) {
    res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    res.end(JSON.stringify({
      incidents: [
        {
          id: 'inc_mock_01',
          tenant: 'tenant_alpha',
          collector_id: 'host_edge_01',
          region: 'ap-south-1',
          pattern: 'BRUTE_FORCE_BURST',
          severity: 'HIGH',
          confidence: 0.96,
          summary: '6 synthetic auth failures detected in 5s sliding window',
          time_to_detect_ms: 1280,
          detected_at: new Date(Date.now() - 60000).toISOString(),
          signals: [
            { metric: 'auth_failures_per_s', observed: 12.0, baseline: 0.1, deviation: 119.0, weight: 0.9, direction: 'UP' }
          ],
          why_flagged: 'Nadi sliding-window counter exceeded 6 failures locally without external lookups.',
          why_not_just_threshold: 'Multi-signal correlation verified pattern in 1,280ms.',
          correlated_with: ['tcp_fail_ratio'],
          recommended_action: 'Engage temporary socket throttle on test subnet',
          why_this_action: 'Bounds traffic without impacting verified customer sessions.',
          playbook_steps: ['Engage socket filter', 'Verify zero dropped customer packets'],
          status: 'ACTIVE'
        }
      ]
    }));
    return true;
  }

  // Remediations
  if (pathname.endsWith('/api/remediations')) {
    res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    res.end(JSON.stringify({
      remediations: [
        {
          id: 'rem_mock_01',
          incident_id: 'inc_mock_01',
          status: 'APPLIED',
          action: 'Socket throttle on test subnet',
          result: 'Throttled test burst. 0 legitimate connections dropped.',
          reversible: true,
          target_valid: true,
          tenant_authorized: true
        }
      ]
    }));
    return true;
  }

  // Thresholds
  if (pathname.endsWith('/api/thresholds')) {
    res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    res.end(JSON.stringify({
      thresholds: {
        cpu_pct: { warn: 75, critical: 90 },
        mem_pct: { warn: 80, critical: 95 },
        latency_ms: { warn: 50, critical: 100 },
        heartbeat: { stale_after_s: 15, offline_after_s: 45 }
      },
      playbooks: [
        {
          pattern: 'BRUTE_FORCE_BURST',
          recommended_action: 'Engage temporary socket throttle on test subnet',
          why: 'Bounds traffic without impacting verified customer sessions.',
          steps: ['Engage socket filter', 'Verify zero dropped customer packets']
        }
      ]
    }));
    return true;
  }

  // Audit
  if (pathname.endsWith('/api/audit')) {
    res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    res.end(JSON.stringify({
      entries: [
        {
          index: 48,
          at: new Date(Date.now() - 5000).toISOString(),
          event: 'REMEDIATION_EXECUTED_SIMULATED',
          status: 200,
          previous_sha256: '9e8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d',
          sha256: 'a1b2c3d4e5f60718293a4b5c6d7e8f901a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d'
        },
        {
          index: 47,
          at: new Date(Date.now() - 30000).toISOString(),
          event: 'BATCH_INGEST_ACK',
          status: 200,
          previous_sha256: '8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c',
          sha256: '9e8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d'
        },
        {
          index: 46,
          at: new Date(Date.now() - 60000).toISOString(),
          event: 'REMEDIATION_DRYRUN_PROPOSED',
          status: 200,
          previous_sha256: '7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b',
          sha256: '8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c'
        },
        {
          index: 45,
          at: new Date(Date.now() - 120000).toISOString(),
          event: 'INCIDENT_DETECTED_BRUTEFORCE',
          status: 200,
          previous_sha256: '6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a',
          sha256: '7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b'
        }
      ],
      head_sha256: 'a1b2c3d4e5f60718293a4b5c6d7e8f901a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d'
    }));
    return true;
  }

  // Audit Verify
  if (pathname.endsWith('/api/audit/verify')) {
    res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    res.end(JSON.stringify({
      valid: true,
      entries: 48,
      head_sha256: 'a1b2c3d4e5f60718293a4b5c6d7e8f901a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d'
    }));
    return true;
  }

  // Records
  if (pathname.includes('/api/records')) {
    const url = new URL(pathname, 'http://localhost');
    const colId = url.searchParams.get('collector_id') || 'host_edge_01';
    const now = Date.now();
    const records = [];
    for (let i = 24; i >= 0; i--) {
      const time = new Date(now - i * 5000).toISOString();
      const wave = Math.sin((now - i * 5000) / 20000) * 5;
      records.push({
        seq: 1000 + (24 - i),
        record: {
          tenant: 'tenant_alpha',
          collector_id: colId,
          region: 'ap-south-1',
          collected_at: time,
          source: 'live',
          metrics: {
            cpu_pct: { min: 14 + wave, max: 28 + wave, avg: Math.round((20 + wave) * 10) / 10, p95: 26 + wave, count: 60 },
            mem_pct: { min: 46, max: 54, avg: Math.round((50 + wave * 0.2) * 10) / 10, p95: 53, count: 60 },
            disk_pct: { min: 38, max: 39, avg: 38.3, p95: 38.4, count: 60 },
            bandwidth_mbps: { min: 120, max: 340, avg: 230, p95: 310, count: 60 },
            latency_ms: { min: 4, max: 12, avg: 6.4, p95: 9.8, count: 60 },
            tcp_fail_ratio: { min: 0, max: 0.002, avg: 0.0004, p95: 0.001, count: 60 },
            service_up: { min: 1, max: 1, avg: 1, p95: 1, count: 60 }
          },
          risk_events: [],
          redaction_counts: { api_key: 1 }
        }
      });
    }
    res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    res.end(JSON.stringify({ records }));
    return true;
  }

  // Proof
  if (pathname.includes('/api/egress-proof')) {
    res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    res.end(JSON.stringify({
      batches: [
        {
          seq: 412,
          batch_id: 'batch_ap_south_412',
          pre_send_bytes: '{"collector_id":"host_edge_01","tenant":"tenant_alpha","metrics":{"cpu_pct":19.4}}',
          pre_send_sha256: '7f8b9a2c3d4e5f6a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a',
          tier2_echoed_sha256: '7f8b9a2c3d4e5f6a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a',
          status: 'ACKED',
          match_bool: true,
          redaction_counts: { credentials: 1, bearer_tokens: 1 },
          stripped_count: 2
        }
      ],
      counters: { batches_sent: 412, total_redactions: 2 }
    }));
    return true;
  }

  // SSE Stream
  if (pathname.endsWith('/api/stream')) {
    res.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection': 'keep-alive'
    });
    res.write('event: snapshot\ndata: {}\n\n');
    const timer = setInterval(() => {
      res.write('event: ping\ndata: {}\n\n');
    }, 15000);
    req.on('close', () => clearInterval(timer));
    return true;
  }

  return false;
}
