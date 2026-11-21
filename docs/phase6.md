# Phase 6: live operations console

The React/TypeScript/Recharts console uses authenticated fetch-SSE and local,
CA-verified regional proxies. Runtime operation needs no internet or database.
Phase 7 now adds measured validation and optional AI; see [phase7.md](phase7.md).

## Four-collector evaluation (PowerShell)

Use Node 24 and Go 1.22+. Download dependencies before disconnecting. Run all
commands from `E:\Cicada_2026\Cicada\Cicada` unless a `cd` is shown.

Terminal A:

```powershell
$env:Path = "$env:LOCALAPPDATA\KalkiToolsNative\go\bin;$env:Path"
go mod download
npm ci --prefix dashboard
# Generate only if data/certs does not already exist:
if (-not (Test-Path data/certs/ca.pem)) { go run ./scripts/gen-certs }
go build -o data/phase6-tier2.exe ./tier2
go build -o data/phase6-collector.exe ./tier1
go run ./scripts/build-collectors
node scripts/console-demo.mjs
```

Expected: two HTTPS receivers on 19443/19444 and four actual collectors,
two per tenant. IDs and proof ports 19090-19093 are printed, never tokens.
Ctrl+C stops this harness's processes. Each run keeps its own WAL and local
logs under ignored `data/console-demo-*`; receiver state is volatile.
On POSIX, build the two executables without `.exe`.

Terminal B:

```powershell
$env:KALKI_AP_URL = 'https://127.0.0.1:19443'
$env:KALKI_EU_URL = 'https://127.0.0.1:19444'
$env:KALKI_PROOF_URL = 'http://127.0.0.1:19090'
cd dashboard
npm run dev
```

Open `http://127.0.0.1:5173`. Quick-fill Alpha or Beta and sign in.
Passwords are `alpha-demo-local` and `beta-demo-local` respectively.
Each workspace shows only its two nodes. The default local proof target belongs
to Alpha: Beta must receive an ownership error for Proof/Demo, not Alpha's data.
To inspect Beta's local collector, restart the console with proof port 19092.

For existing Phase 4 services, omit the three overrides: defaults are HTTPS
8443/8444 and loopback proof 9090. Add `--demo-controls` to a collector command
to enable mutation endpoints; they are disabled by default. Collector credentials
can be supplied through `KALKI_TOKEN`. Keep your existing WAL identity unchanged.

## What to demonstrate

1. Fleet: seven timestamp-ordered charts, collector CPU overlay, loaded thresholds,
   provenance and UTC health history. Switch Workspace to demonstrate isolation.
2. Add Collector: generate a token; token and commands start masked. Copy includes
   the credential. POSIX installer requires the trusted CA; Windows uses the
   manual command with the downloaded Windows binary.
3. Egress Proof: inspect an ACKED batch's exact bytes and matching SHA-256 values.
   Attempt Violating Payload returns BLOCKED and `sent: false`.
4. Demo Lab: inject a fake secret (scrubbed before egress), six failed logins,
   a bounded 15-second CPU task, or 20 seconds of simulated network degradation.
   Network values carry SIMULATED provenance; CPU remains measured host data.
5. Kill Link: local collection and WAL continue. Inspect local proof queue depth;
   the fleet becomes STALE after 15 seconds and OFFLINE after 45 seconds. Restore
   Link drains pending data without replacing tokens. Receiver WAL counts in the
   top bar are last reported; they cannot update while transmission is paused.
6. Send Duplicate & Out-of-Order Batch: duplicate count rises, while the approved
   simulated record from 90 seconds earlier appears in event-time order.
7. Expand an active incident, review evidence, propose remediation, explicitly
   approve, then roll back. Expect EXECUTED_SIMULATED then ROLLED_BACK. Verify
   the audit chain. No destructive OS commands execute.

CPU stress does not promise a host-wide threshold crossing on a many-core host.
Detection depends on the actual signals and baseline. Latency measurements include
backfill and outage recovery, so targets can fail visibly; there is no clock-sync
guarantee. Demo completion means the action was queued or its timer elapsed;
delivery evidence is in the proof counters and fleet.

## Production build and local serving

Keep Terminal A running. In Terminal B, retain the same regional environment
variables (or omit them for the standard ports):

```powershell
cd E:\Cicada_2026\Cicada\Cicada\dashboard
npm run build
npm start
```

The built-asset Node server includes the streaming proxy; serving `dist/` alone
does not supply API routes. Default binding is loopback port 5173. For LAN use,
configure HOST, PORT, KALKI_WEB_CERT and KALKI_WEB_KEY for incoming HTTPS, using
a certificate containing the console hostname and trusted by clients. Example
from `dashboard/` for the existing localhost certificate:

```powershell
$env:KALKI_WEB_CERT = '../data/certs/server.pem'
$env:KALKI_WEB_KEY = '../data/certs/server.key'
npm start
# Open https://localhost:5173 after trusting the local CA.
```

KALKI_CA_CERT optionally selects the CA file for upstream TLS. Both dev and built
servers verify upstream certificates; neither disables verification. Proof is
restricted to loopback and checked against the authenticated workspace before
forwarding. The proxy strips session credentials before contacting proof.
With a remote console, this proof endpoint refers to the console server's local
collector, not the browser machine. Configure one explicit target per console.

The marketing server's `/app` redirects to the dashboard (KALKI_DASHBOARD_URL,
default http://localhost:5173). Static marketing hosting may set
`window.KALKI_DASHBOARD_URL` before app.js.

## Checks

```powershell
cd E:\Cicada_2026\Cicada\Cicada
go vet ./...
go test -race ./...
npm run check
npm test --prefix dashboard
npm run build --prefix dashboard
```

Frontend tests cover incremental SSE framing, authenticated reconnect/abort,
event ordering/deduplication, percentiles and a real TLS proof-proxy isolation
test. Generate certificates first so the TLS test runs rather than skips.
Browser checks covered two tenants/two nodes, outage recovery, duplicate count,
onboarding masking, simulated approval/rollback and phone-width layout with no
console errors. A separate built-server launch was blocked by automatic approval
review; its build, syntax and shared proxy tests passed, but production serving
was not manually verified in this session.
