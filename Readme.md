Final Deployment

# Kalki — privacy-first AIOps

For the deployed website, see [Vercel routing and backend connection](docs/vercel.md).
The combined build serves the landing page at `/` and the console at `/app`.

Phases 1–7 implement an offline LAN platform with Go services, collector disk WALs,
in-memory regional receivers and a React console. No database or external SaaS is
required. Optional AI defaults to deterministic offline briefs.

```text
Prana collection → Kavach privacy → Nadi rollups/security
                                      ↓
                               Setu WAL → HTTPS/ACK
                                      ↓
                         Medha health, incidents, AI, console
```

| Module | Responsibility                                             |
| ------ | ---------------------------------------------------------- |
| Prana  | OS/TCP observations and NAB/Loghub replay                  |
| Kavach | Secret scrubbing, tenant pseudonyms, fail-closed policy    |
| Nadi   | Five-second rollups and brute-force detection              |
| Setu   | Durable retry delivery, SHA-256 proof, regional ingest     |
| Medha  | Adaptive health, evidence, safe simulation, AI and console |

[Architecture](docs/kalki-architecture.md) · [Authentication](docs/phase4.md) ·
[Engine](docs/phase5.md) · [Demo controls](docs/phase6.md) ·
[Validation/AI methodology](docs/phase7.md)

## One-laptop demo — PowerShell

Prerequisites: Go 1.22+, Node 24, C compiler for race tests. Download dependencies
before disconnecting. Terminal A, repository root:

```powershell
cd E:\Cicada_2026\Cicada\Cicada
$env:Path = "$env:LOCALAPPDATA\KalkiToolsNative\go\bin;$env:Path"
go mod download
npm ci --prefix dashboard
if (-not (Test-Path data/certs/ca.pem)) { go run ./scripts/gen-certs }
go build -o data/phase6-tier2.exe ./tier2
go build -o data/phase6-collector.exe ./tier1
go run ./scripts/build-collectors
node scripts/console-demo.mjs
```

Expected: four actual collectors, two per tenant; receivers on 19443/19444.
Each run retains separate WALs. Ctrl+C stops its owned processes.
On Linux/macOS, build the executable paths without `.exe`.

Terminal B:

```powershell
cd E:\Cicada_2026\Cicada\Cicada\dashboard
$env:KALKI_AP_URL = 'https://127.0.0.1:19443'
$env:KALKI_EU_URL = 'https://127.0.0.1:19444'
$env:KALKI_PROOF_URL = 'http://127.0.0.1:19090'
npm run dev
```

Open http://127.0.0.1:5173. Quick-fill Alpha (`alpha-demo-local`) or Beta
(`beta-demo-local`), then Enter Kalki Cloud. Each sees only its two nodes.
Proof port 19090 belongs to Alpha; Beta access must fail. To inspect Beta's local
collector, restart the console with proof port 19092.

For built assets, stop dev and run `npm run build` then `npm start` in dashboard/,
retaining the same environment. The Node server supplies static assets and
streaming proxies. Static dist/ alone has no API routes. Upstream TLS is verified.

## Four-laptop offline LAN — Windows

Laptop 1 hosts Tier 2, Dashboard and Node 1. Example LAN address: **192.168.1.10**;
replace it throughout with its actual address. Allow inbound TCP 8443, 8444 and
5173 on Laptop 1's private-network firewall. Synchronize clocks.

Laptop 1, repository root:

```powershell
go run ./scripts/gen-certs --out=data/lan-certs --hosts=localhost,127.0.0.1,192.168.1.10
go run ./scripts/build-collectors
go build -o data/tier2-server.exe ./tier2
.\data\tier2-server.exe --addr=0.0.0.0:8443 --eu-addr=0.0.0.0:8444 --cert=data/lan-certs/server.pem --key=data/lan-certs/server.key --public-url=https://192.168.1.10:8443 --eu-public-url=https://192.168.1.10:8444
```

Reuse existing lan-certs instead of rerunning the generator against it. Copy
**only** its ca.pem, plus data/releases/collector-windows-amd64.exe and
data/releases/allowlist.yaml, to a Kalki folder on every laptop (including
Laptop 1). Keep private keys on Laptop 1. The policy file is required.

Laptop 1, separate dashboard terminal, initially at repository root:

```powershell
$env:KALKI_AP_URL = 'https://192.168.1.10:8443'
$env:KALKI_EU_URL = 'https://192.168.1.10:8444'
$env:KALKI_CA_CERT = (Resolve-Path data/lan-certs/ca.pem).Path
$env:KALKI_WEB_CERT = (Resolve-Path data/lan-certs/server.pem).Path
$env:KALKI_WEB_KEY = (Resolve-Path data/lan-certs/server.key).Path
$env:KALKI_PROOF_URL = 'http://127.0.0.1:9090'
$env:HOST = '0.0.0.0'
$env:PORT = '5173'
cd dashboard
npm run build
npm start
```

Trust this CA in each browser/OS trust store, then open https://192.168.1.10:5173.
No TLS verification bypass is needed.

Run the collector block below in each laptop's Kalki folder using this map:

| Laptop/node | $tenant      | $password        | $region      | $port |
| ----------- | ------------ | ---------------- | ------------ | ----- |
| 1 / Node 1  | tenant_alpha | alpha-demo-local | ap-south-1   | 8443  |
| 2 / Node 2  | tenant_alpha | alpha-demo-local | ap-south-1   | 8443  |
| 3 / Node 3  | tenant_beta  | beta-demo-local  | eu-central-1 | 8444  |
| 4 / Node 4  | tenant_beta  | beta-demo-local  | eu-central-1 | 8444  |

```powershell
# Laptop 1/2 values; change all four using the table on Laptops 3/4.
$tenant = 'tenant_alpha'
$password = 'alpha-demo-local'
$region = 'ap-south-1'
$port = 8443
$receiver = "https://192.168.1.10:$port"
@{username=$tenant;password=$password} | ConvertTo-Json -Compress | Set-Content -Encoding ascii login.json
$loginResponse = curl.exe --fail --silent --show-error --ssl-revoke-best-effort --cacert ca.pem -H 'Content-Type: application/json' --data-binary '@login.json' "$receiver/api/auth/login"
if ($LASTEXITCODE -ne 0) { throw 'Login failed' }
$session = $loginResponse | ConvertFrom-Json
if (-not $session.token) { throw 'Authentication token missing' }
'{}' | Set-Content -Encoding ascii mint.json
$mintResponse = curl.exe --fail --silent --show-error --ssl-revoke-best-effort --cacert ca.pem -H "Authorization: Bearer $($session.token)" -H 'Content-Type: application/json' --data-binary '@mint.json' "$receiver/api/collectors/tokens"
if ($LASTEXITCODE -ne 0) { throw 'Collector mint failed' }
$onboard = $mintResponse | ConvertFrom-Json
$env:KALKI_TOKEN = $onboard.token
$id = $onboard.collector_id
.\collector-windows-amd64.exe --mode=live --tenant=$tenant --region=$region --collector-id=$id --tier2-url=$receiver --ca-cert=ca.pem --allowlist=allowlist.yaml --wal-path="$id.wal" --key-file="$tenant.key" --probe-target="192.168.1.10:$port" --proof-addr=127.0.0.1:9090 --demo-controls
```

Each mint produces a distinct ID. Retain ID/token/key/WAL for restarts; never
mint a different identity against an existing WAL. After receiver restart,
remint original IDs as described in Phase 4. Dashboard reads are tenant-scoped.
Its proof proxy controls Laptop 1 only; remote proof remains loopback-only.
Demo controls are opt-in.

## Evaluator sequence

Show Alpha's two nodes, then Beta's two nodes. Inspect provenance, health history
and incident evidence. Show matching egress hashes and a blocked violation.
Kill/restore delivery: local WAL grows and drains. Propose, approve and roll back
simulated remediation; verify the audit. Ask Medha and inspect the sanitized prompt.
Finish with the Validation scorecard.

## Validation and AI

```powershell
go run ./loadgen -collectors=500 -tenants=20 -duration=10s
go run ./scripts/validate
```

For browser latency, start a new measurement window in Validation, view Fleet for

> =30 samples, export to data/dashboard-latency.json and validate within 15 minutes.
> Missing evidence fails honestly. Download datasets once with
> `bash scripts/fetch-datasets.sh`; append `--offline` for local verification.
> NAB uses a 50% chronological holdout, not official NAB scoring.
> See [methodology and provider examples](docs/phase7.md).

AI environment variables belong only to Tier 2:

| Mode                   | Configuration                                                                                |
| ---------------------- | -------------------------------------------------------------------------------------------- |
| Deterministic fallback | Leave KALKI_AI_URL unset; no network                                                         |
| Local Ollama           | KALKI_AI_URL=http://127.0.0.1:11434/api/chat; KALKI_AI_MODEL=installed model                 |
| OpenAI-compatible      | KALKI_AI_URL=full HTTPS /chat/completions endpoint; KALKI_AI_KEY; KALKI_AI_MODEL             |
| Gemini                 | KALKI_AI_URL=full HTTPS /models/MODEL:generateContent endpoint; KALKI_AI_KEY; KALKI_AI_MODEL |

Six-second timeout, no redirects, fallback on errors. Never put keys in VITE vars.
Free-text questions select local health/security intents; only allow-listed
pseudonymized context is sent. AI never executes commands.

Final checks, from repository root:

```powershell
go vet ./...
go test -race ./...
go run ./scripts/validate
npm run check
npm test --prefix dashboard
npm run build --prefix dashboard
```

## Limitations

Tier 2/auth/audits are in memory and reset on restart. WAL history is capped with
no automatic compaction. Demo certificates use a private/self-signed CA. Regions
are logical LAN listeners, not geographic isolation. Thresholds are defaults;
baselines need warm-up. NAB/Loghub are replayed traces with explicit provenance.
TCP loss is a small-window connection-failure estimate. Playbooks/confidence are
heuristic, not calibrated security verdicts. Clock sync is not guaranteed;
backfill/outages may exceed latency targets. Loadgen is simulated, not a physical
fleet capacity SLA. Remediation is non-destructive simulation. Optional cloud AI
is advisory and can be wrong; deterministic mode is narrower but fully offline.
