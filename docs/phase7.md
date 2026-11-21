# Phase 7: evidence, optional AI and final demo

## Verified result — 2026-11-21

The final vet, full Go race suite, root syntax check, frontend tests and frontend
build passed. All ten acceptance criteria passed with a 194-sample browser window
(p95 7,112 ms). A separate 500-collector/20-tenant in-process simulation accepted
49,181 batches in 10 seconds with zero failures, about 4,917 batches/sec, ingest
p95 1.02 ms and peak sampled heap 94 MiB. These are local measurements, not SLAs.

NAB evaluated all 24 pinned CSV traces: 4 true positives, 618 false positives,
29 false negatives, precision 0.00643 (0.64%), recall 0.12121 (12.12%), and 0.05347
false alarms/hour under the exact-timestamp scoring method below. This is weak
detection quality; passing infrastructure checks does not establish a good general
anomaly detector. No tuning against holdout labels was performed. The initial
evaluation rejected upstream duplicate/out-of-order timestamps; normalization was
fixed and the affected evaluator race test plus validation were rerun.

The Bash downloader could not be executed on this Windows host because Bash was
not installed. All 27 manifest entries were independently downloaded and verified
against the pinned SHA-256 hashes using Node's built-in APIs. Cloud-provider
protocols were tested with local mocks; no real cloud credential was used.

No new Go/npm dependencies. Normal execution stays offline. AI defaults to fixed,
deterministic explanations; cloud access is explicitly configured by the operator.

## Validation and datasets

From the repository root with Go in PATH:

```powershell
go run ./loadgen -collectors=500 -tenants=20 -duration=10s
go run ./scripts/validate
```

Loadgen writes `data/loadgen-report.json`, tagged SIMULATED. Without `-url`, it
exercises the actual ingest policy/protocol handler in process; it does not measure
TLS, auth, a real network or 500 physical machines. Peak allocation is sampled Go
process heap, not total RSS. HTTPS mode requires `-url=https://...`, `-ca=...` and
`-credentials=local-identities.json`, an array of pre-minted
`{tenant,collector_id,region,token}`. Every identity must belong to that regional
receiver. Keep this local file private. The current demo seeds two workspace users;
the default twenty-tenant benchmark therefore uses the in-process mode.

The validator executes named acceptance tests for privacy, deduplication, ordering,
30-second outage recovery, tenant/region boundaries and audit tampering. It also
measures quiet-baseline, critical-alert and offline-transition fixtures with a
virtual event clock, and runs 50 simulated collectors / 5 tenants for three seconds.
It writes `data/validation-report.json` even when criteria fail; failure exits 1.
The session-authenticated `/api/validation` serves this shared synthetic engineering
report. It contains no workspace identities or raw logs.

Browser latency requires real evidence: sign in, open Validation, click **Start new
measurement window**, then view Fleet for at least 30 samples (about 75 seconds
with two collectors). Export from Validation and save as
`data/dashboard-latency.json`; run the validator within 15 minutes. This measures a
declared steady-state window. Initial history/backfill and outages can breach the
target in other windows. Missing/stale evidence fails; backend timings never stand
in for browser rendering. The exporter is operator-supplied evidence, not an
attestation or a substitute for a controlled browser benchmark.

Download datasets once while connected using Git Bash or another Bash installation:

```bash
bash scripts/fetch-datasets.sh
# When disconnected: verify available files without attempting downloads.
bash scripts/fetch-datasets.sh --offline
```

`scripts/datasets.sha256` pins immutable NAB/Loghub revisions and SHA-256 hashes.
Existing files are verified, never silently replaced. CSV rows are stably sorted
by event time; duplicate timestamps keep the last source row. NAB's first chronological
half establishes normalization and warms the fixed Medha detector; the second half
is held out. Labels are read only after predictions. The report uses exact labeled
timestamps, not the official NAB anomaly-window scoring function. Values are
mapped using training-only min/max into a CPU proxy range; this is a generic
univariate adaptation, not a claim that every trace represents CPU. Precision,
recall and false alarms/hour are descriptive; no accuracy threshold is claimed.
Missing files/labels produce `datasets_not_downloaded`, without fabricated metrics.

## Medha AI configuration

Set these in the Tier 2 process environment before launch. No `.env` file is
automatically loaded by Go. Never expose provider keys through VITE-prefixed vars.

```powershell
# Default: no AI variables => deterministic_fallback, no external request.
# Local Ollama (install/download a model before going offline):
$env:KALKI_AI_URL = 'http://127.0.0.1:11434/api/chat'
$env:KALKI_AI_MODEL = 'YOUR_INSTALLED_MODEL'
# Cloud OpenAI-compatible: complete chat/completions endpoint and server-side key.
$env:KALKI_AI_URL = 'https://YOUR_PROVIDER/v1/chat/completions'
$env:KALKI_AI_MODEL = 'YOUR_MODEL'
$env:KALKI_AI_KEY = 'YOUR_PRIVATE_KEY'
# Gemini alternative: full endpoint; key is a header, never a query parameter.
$env:KALKI_AI_URL = 'https://generativelanguage.googleapis.com/v1beta/models/YOUR_MODEL:generateContent'
```

Use one configuration at a time. Cloud requires HTTPS and a key; keyless HTTP is
accepted only for literal loopback IPs. Redirects are refused. Requests time out
after six seconds; errors, empty responses and malformed outputs fall back.
Provider protocol references: [OpenAI chat](https://developers.openai.com/api/reference/resources/chat),
[Gemini generateContent](https://ai.google.dev/api/generate-content),
[Ollama chat](https://docs.ollama.com/api/chat).

Ask Medha supports fleet-health and security-posture intents. Raw free-text questions
are never forwarded. Context is reconstructed from same-tenant node pseudonyms,
allow-listed numeric metrics, health/status enums and incident signals; arbitrary
Explanation/playbook text is excluded too. Inspect the exact context in the prompt
drawer. Fixed system instructions additionally constrain responses. Cloud output
is advisory and can be inaccurate. Diagnostic commands are fixed read-only
PowerShell suggestions, never model-executed actions. Only fixed `ai_query` and
`ai_analyze` event names enter audit logs.

## Login and boundaries

The login uses a 50x30 Canvas mesh and CSS glass frame, no external assets or fonts.
Reduced-motion preference disables animation. `VITE_KALKI_HOME_URL` optionally
sets the build-time Back to Kalki destination (default http://localhost:8080).
The label “Cloud” names the console experience; the default deployment remains
self-hosted LAN.

Existing Phase 6 detail: [setup and demo actions](phase6.md). Receiver restart still
clears authentication and in-memory telemetry: log in and remint original collector
IDs to replay the existing WAL. Never delete WAL files just to clear warnings.
