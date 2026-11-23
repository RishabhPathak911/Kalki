// dashboard/src/lib/demo-data.ts
import type { NodeData, Point, Incident, Thresholds, Verification, Session } from './types';

export function getDemoNodes(session: Session): NodeData[] {
  const now = new Date();
  return [
    {
      collector_id: 'host_edge_01',
      tenant: session.tenant || 'tenant_alpha',
      region: session.region || 'ap-south-1',
      health: 'Optimal',
      connection_state: 'ONLINE',
      last_seen_at: now.toISOString(),
      last_seen_age_s: 1,
      metrics: {
        cpu_pct: { min: 14.2, max: 28.5, avg: 19.4, p95: 26.1, count: 60 },
        mem_pct: { min: 46.1, max: 54.0, avg: 49.8, p95: 53.2, count: 60 },
        disk_pct: { min: 38.0, max: 38.4, avg: 38.2, p95: 38.3, count: 60 },
        bandwidth_mbps: { min: 120, max: 380, avg: 245.0, p95: 350.0, count: 60 },
        latency_ms: { min: 4.1, max: 12.8, avg: 6.4, p95: 9.8, count: 60 },
        tcp_fail_ratio: { min: 0, max: 0.002, avg: 0.0004, p95: 0.001, count: 60 },
        service_up: { min: 1, max: 1, avg: 1, p95: 1, count: 60 }
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
      tenant: session.tenant || 'tenant_alpha',
      region: session.region || 'ap-south-1',
      health: 'Optimal',
      connection_state: 'ONLINE',
      last_seen_at: now.toISOString(),
      last_seen_age_s: 1,
      metrics: {
        cpu_pct: { min: 22.0, max: 38.0, avg: 28.5, p95: 34.0, count: 60 },
        mem_pct: { min: 54.0, max: 62.0, avg: 57.2, p95: 60.5, count: 60 },
        disk_pct: { min: 42.0, max: 44.0, avg: 43.1, p95: 43.8, count: 60 },
        bandwidth_mbps: { min: 140, max: 280, avg: 195.0, p95: 250.0, count: 60 },
        latency_ms: { min: 5.0, max: 14.0, avg: 7.5, p95: 11.0, count: 60 },
        tcp_fail_ratio: { min: 0, max: 0.003, avg: 0.0006, p95: 0.002, count: 60 },
        service_up: { min: 1, max: 1, avg: 1, p95: 1, count: 60 }
      },
      source_badges: ['live', 'prana-probe', 'setu-wal'],
      state_transitions: [],
      duplicates_rejected: 0,
      wal_depth: 0,
      records_accepted: 2890,
      data_age_s: 1
    },
    {
      collector_id: 'host_edge_03',
      tenant: session.tenant || 'tenant_alpha',
      region: session.region || 'ap-south-1',
      health: 'Nominal',
      connection_state: 'ONLINE',
      last_seen_at: now.toISOString(),
      last_seen_age_s: 1,
      metrics: {
        cpu_pct: { min: 12.0, max: 24.0, avg: 16.2, p95: 21.0, count: 60 },
        mem_pct: { min: 38.0, max: 45.0, avg: 41.5, p95: 44.0, count: 60 },
        disk_pct: { min: 28.0, max: 29.5, avg: 28.8, p95: 29.2, count: 60 },
        bandwidth_mbps: { min: 80, max: 190, avg: 130.0, p95: 175.0, count: 60 },
        latency_ms: { min: 3.5, max: 9.0, avg: 5.2, p95: 8.0, count: 60 },
        tcp_fail_ratio: { min: 0, max: 0.001, avg: 0.0002, p95: 0.0008, count: 60 },
        service_up: { min: 1, max: 1, avg: 1, p95: 1, count: 60 }
      },
      source_badges: ['live', 'prana-os'],
      state_transitions: [],
      duplicates_rejected: 0,
      wal_depth: 0,
      records_accepted: 1980,
      data_age_s: 1
    }
  ];
}

export function getDemoPoints(session: Session): Point[] {
  const points: Point[] = [];
  const now = Date.now();
  const nodeIds = ['host_edge_01', 'host_edge_02', 'host_edge_03'];

  for (let i = 24; i >= 0; i--) {
    const time = new Date(now - i * 5000).toISOString();
    for (const colId of nodeIds) {
      const baseCpu = colId === 'host_edge_01' ? 19.4 : colId === 'host_edge_02' ? 28.5 : 16.2;
      const baseMem = colId === 'host_edge_01' ? 49.8 : colId === 'host_edge_02' ? 57.2 : 41.5;
      const wave = Math.sin((now - i * 5000) / 22000) * 5.5;
      points.push({
        key: `${colId}:${i}:live:${time}`,
        tenant: session.tenant || 'tenant_alpha',
        collector_id: colId,
        region: session.region || 'ap-south-1',
        collected_at: time,
        source: 'live',
        metrics: {
          cpu_pct: { min: baseCpu - 4 + wave, max: baseCpu + 8 + wave, avg: Math.round((baseCpu + wave) * 10) / 10, p95: baseCpu + 6 + wave, count: 60 },
          mem_pct: { min: baseMem - 2, max: baseMem + 3, avg: Math.round((baseMem + wave * 0.3) * 10) / 10, p95: baseMem + 2, count: 60 },
          disk_pct: { min: 38, max: 39, avg: 38.4, p95: 38.6, count: 60 },
          bandwidth_mbps: { min: 120, max: 340, avg: Math.round(220 + wave * 15), p95: 310, count: 60 },
          latency_ms: { min: 4, max: 12, avg: Math.round((6.4 + Math.abs(wave) * 0.4) * 10) / 10, p95: 10, count: 60 },
          tcp_fail_ratio: { min: 0, max: 0.002, avg: 0.0004, p95: 0.001, count: 60 },
          service_up: { min: 1, max: 1, avg: 1, p95: 1, count: 60 }
        },
        risk_events: [],
        redaction_counts: { api_key: 1, token: 1 }
      });
    }
  }
  return points;
}

export function getDemoIncidents(session: Session): Incident[] {
  return [
    {
      id: 'inc_edge_01',
      tenant: session.tenant || 'tenant_alpha',
      collector_id: 'host_edge_01',
      region: session.region || 'ap-south-1',
      pattern: 'BRUTE_FORCE_BURST',
      severity: 'HIGH',
      confidence: 0.96,
      summary: '6 synthetic auth failures detected in 5s sliding window',
      time_to_detect_ms: 1280,
      detected_at: new Date(Date.now() - 45000).toISOString(),
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
  ];
}

export function getDemoThresholds(): Thresholds {
  return {
    thresholds: {
      cpu_pct: { warn: 75, critical: 90 },
      mem_pct: { warn: 80, critical: 95 },
      heartbeat: { stale_after_s: 30, offline_after_s: 90 }
    },
    playbooks: [
      {
        pattern: 'BRUTE_FORCE_BURST',
        recommended_action: 'Engage temporary socket throttle on test subnet',
        why: 'Prevents CPU exhaustion while maintaining SLA for authenticated sessions.',
        steps: ['Filter inbound port', 'Verify customer connectivity']
      }
    ]
  };
}

export function getDemoVerification(): Verification {
  return {
    valid: true,
    entries: 142,
    head_sha256: '7f8a3c9e12bd4f810aa839cf9b1782e4'
  };
}
