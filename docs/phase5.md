# Phase 5: Medha backend

Phase 5 adds adaptive analysis, explainable incidents, four-state health,
tenant-isolated remediation simulation and header-authenticated SSE. The
Phase 6 frontend is documented in [phase6.md](phase6.md).

## Start and verify

Run from the repository root in PowerShell:

```powershell
$env:Path = "$env:LOCALAPPDATA\KalkiToolsNative\go\bin;$env:Path"
go vet ./...
go test -race -count=1 ./...
go run ./scripts/build-collectors
go run ./tier2
```

Do not start a second server over an existing listener. Restart the old server
to load Phase 5, then log in and mint collector tokens again: auth state is
in memory. Use [Phase 4 onboarding](phase4.md), preserving existing collector
IDs and WALs where applicable. The updated releases include the expanded
allow-list. Upgrade collector binary and policy together.

The server loads `config/thresholds.yaml` and `config/playbooks.yaml` at startup;
`--thresholds` / `--playbooks` can select other local files. Invalid configuration
fails startup. There is no live config-edit API in this phase.

## PowerShell API helper

In your logged-in terminal, `$alpha.token` must contain the workspace session.
The Windows commands use your verified `--ssl-revoke-best-effort` setting while
retaining CA verification. This helper checks curl's exit code before parsing:

```powershell
function Invoke-Kalki {
    param(
        [string]$Path,
        [string]$Method = 'GET',
        [object]$Body = $null
    )
    if (-not $alpha.token) {
        throw 'Run the workspace login command first.'
    }
    $curlOptions = @(
        '--fail', '--silent', '--show-error', '--ssl-revoke-best-effort',
        '--cacert', 'data/certs/ca.pem',
        '-H', "Authorization: Bearer $($alpha.token)",
        '-X', $Method
    )
    if ($null -ne $Body) {
        $Body | ConvertTo-Json -Depth 10 -Compress |
            Set-Content -Encoding ascii data/phase5-request.json
        $curlOptions += @(
            '-H', 'Content-Type: application/json',
            '--data-binary', '@data/phase5-request.json'
        )
    }
    $response = curl.exe @curlOptions "https://localhost:8443$Path"
    if ($LASTEXITCODE -ne 0) {
        throw "API request failed: curl exit code $LASTEXITCODE"
    }
    $response | ConvertFrom-Json
}
```

Inspect the fleet and active/recent incidents:

```powershell
$fleet = Invoke-Kalki '/api/fleet'
$fleet.nodes | Select-Object collector_id,health,connection_state,last_seen_age_s,wal_depth
$fleet.counters

$incidents = Invoke-Kalki '/api/incidents'
$incidents.incidents | ConvertTo-Json -Depth 10

$configuration = Invoke-Kalki '/api/thresholds'
$configuration | ConvertTo-Json -Depth 10
```

Fleet includes latest metric statistics, separate `cpu_pct` and
`collector_cpu_pct`, source badges, last heartbeat/data timestamps and ages,
state transitions, accepted batch/record counts, duplicate counts, sampled WAL
depth and suppression counters. All read scope comes from the session, never
tenant/region query parameters. Both regional listeners support these APIs.

## Health and incident behavior

| State | Meaning |
| --- | --- |
| Excellent | Fresh heartbeat, no active incident, metrics within baseline and below warning thresholds |
| Good | Minor deviation before the persistence requirement |
| Fair | Persistent deviation or warning threshold, normally three consecutive windows |
| Poor | Critical metric, correlated multi-signal/high-confidence incident, or stale/offline collector |

At heartbeat age **>15 seconds**, connection becomes `STALE` and health is
`Poor`. At **>=45 seconds**, connection becomes `OFFLINE`, with a
`collector_down` explanation. A one-second background health tick operates
without dashboard polling. Transitions include exact health labels, old/new
connection states, reason and UTC millisecond timestamp.

To observe staleness, stop a running collector while keeping Tier 2 running.
Read `/api/fleet` after 16 and 46 seconds. Restart with its unchanged identity,
key, token and WAL to observe reconnection. A deliberately closed TCP probe
already represents service impact, so do not expect Excellent while probing
`127.0.0.1:1`; use the running local receiver for a reachable probe instead.

Baselines are per tenant, collector, source and metric. They use a rolling
120-sample history, EWMA, median/MAD robust deviation and rate of change.
Baseline alerts require at least **30 aggregate observations**, typically
150 seconds at five-second windows. Static critical boundaries and offline/full
buffer conditions remain immediately actionable during warm-up. Memory/disk
trend evidence requires sustained growth; peer comparisons never cross tenants.

Persistence, separate clear thresholds, repeated clear windows, cooldown and
deduplication suppress flapping. Active node incidents update instead of creating
a new incident per sample. Evidence is correlated within 30 seconds; precedence
selects a primary playbook while retaining contributing signals. All eight
configured patterns are supported, including same-tenant shared dependencies.
Collector process CPU is normalized by logical CPU count to a 0–100 share of
host capacity. Collector overhead can degrade health but is not mislabeled as
the host-workload playbook.

Every incident includes the complete Explanation object: identity/scope,
pattern/severity/confidence, weighted signals, summary, reasons, correlated
peers, recommended action and justification, playbook steps, detection timestamp,
time to detect, and lifecycle status. Confidence is a bounded heuristic score,
not a calibrated probability or security verdict. Suppression counters count
algorithmic suppression, not independently labelled real-world false positives.

## Safe remediation demo

Select an active incident, for example the offline collector incident above:

```powershell
$incident = (Invoke-Kalki '/api/incidents').incidents |
    Where-Object status -eq 'ACTIVE' |
    Select-Object -First 1

if (-not $incident) { throw 'No active incident to remediate.' }

$proposal = Invoke-Kalki '/api/remediations/propose' 'POST' @{
    incident_id = $incident.id
}
$proposal
```

Expected: `PROPOSED`, with `reversible`, `target_valid`, and
`tenant_authorized` all true. Review its proposed action. To explicitly approve:

```powershell
Invoke-Kalki "/api/remediations/$($proposal.id)/approve" 'POST' @{
    approved = $true
}
```

Expected: `EXECUTED_SIMULATED`. No command runs and no host state changes.
Approval without `approved: true` fails. Resolved incidents cannot be approved.

```powershell
Invoke-Kalki "/api/remediations/$($proposal.id)/rollback" 'POST'
Invoke-Kalki '/api/remediations'
Invoke-Kalki '/api/audit/verify'
```

Expected: `ROLLED_BACK`, the tenant's remediation history, and `valid: true`.
Propose/approve/rollback append tenant audit entries and publish SSE events.
Cross-tenant incident or remediation IDs return 404. Only the built-in reversible
in-memory diagnostic simulation is permitted; playbook text is never executed.

## Live SSE

Use a separate logged-in terminal, or stop the stream with Ctrl+C:

```powershell
curl.exe -N --fail --silent --show-error --ssl-revoke-best-effort `
    --cacert data/certs/ca.pem `
    -H "Authorization: Bearer $($alpha.token)" `
    https://localhost:8443/api/stream
```

Expected: an initial `snapshot`, then `telemetry`, `health`, `incident`, and
`remediation` events as changes occur, plus keepalives. Tokens in URL query
parameters are rejected. The stream rechecks session validity each second;
logout/expiry closes it. Streams do not retain the shared authentication lock.
Writes have a deadline so a stalled client cannot hold a stream indefinitely.

The last 256 events per tenant are retained in memory. Clients may send
`Last-Event-ID`; a gap causes a fresh scoped snapshot. Initial snapshots allow
recovery after a server restart as well. Phase 6 should use a fetch-based SSE
client that supports Authorization headers; native EventSource cannot set them.

## Delivery, replay and retention assumptions

- `wal_depth` is sampled when the batch is sealed, not a live remote disk read.
- Normal collector batches report `wal_buffer_full: 0`. WAL admission reserves
  2 KiB for a policy-approved `wal_buffer_full: 1` status batch. On capacity
  rejection it is appended/fsynced before any send; normal admission still
  fails loudly. Disk exhaustion/corruption can prevent even this status write.
- A disconnected collector cannot notify Tier 2 immediately. The full-buffer
  record remains in the WAL and is replayed after connectivity/authentication
  recovery; offline detection is available independently. Increase capacity
  before resuming normal collection. No raw data bypasses the WAL or policy.
- Duplicate ACKs refresh heartbeat but do not retrain baselines. Out-of-order
  records remain in the receiver's event-time-sorted history without overwriting
  newer analysis. Old replay can warm baselines but does not create fresh metric
  incidents; full-buffer status remains actionable until later normal data.
- Health heartbeat age uses receiver receipt time. `data_age_s` separately
  exposes old measurements during backfill. Detection latency uses UTC event
  timestamps and assumes reasonably synchronized clocks; negative latency is
  clamped to zero.
- State transitions retain the last 128 entries per node. Incidents and SSE
  events retain the last 256 entries per tenant. Remediation history caps at
  256 records per tenant and rejects further proposals at capacity.
- All analysis, remediation, auth and audit state is volatile. Restarting Tier 2
  resets it; the collector WAL remains durable. The existing audit chain is
  append-only for process lifetime and is not silently truncated.

## Focused acceptance tests

```powershell
go test -race -count=1 -v ./tier2/medha
go test -race -count=1 -v ./tier2/workspace
go test -race -count=1 -v ./tier2/privacy -run CollectorCPU
go test -race -count=1 -v ./tier1/collect -run CollectorCPU
go test -race -count=1 -v ./tier1/send -run BufferFull
```

The Medha tests use a simulated clock: the ten-minute quiet-baseline test runs
quickly, with small harmless metric variation and zero alerts. Tests cover all
eight playbooks, tenant peer boundaries, warm-up, persistence, hysteresis,
cooldown, deduplication, schema completeness, CPU separation, stale/offline UTC
transitions, replay safety, remediation audit/isolation and SSE authentication.
Run the complete suite too: it preserves the Phase 1–4 tests, including the
real 30-second transport outage.
