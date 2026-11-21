# Hosted Kalki: Render + Aiven + Vercel

The device collector samples locally, scrubs and allow-lists metadata, writes its
local WAL, and sends HTTPS batches to Render. Vercel proxies authenticated console
requests and SSE to Render. Aiven stores hashed credentials, audit entries,
remediations, approved telemetry, delivery progress and incident snapshots.
Raw logs and raw hostnames stay on the monitored device.

## Render

Use one Go web service at the repository root, with Go 1.25 or newer.

Build command:

```sh
go run ./scripts/build-collectors && CGO_ENABLED=0 go build -tags netgo -ldflags '-s -w' -o tier2-server ./tier2
```

Start command: `./tier2-server`. Health check: `/healthz`.

Configure these in Render's environment settings:

| Variable | Setting |
| --- | --- |
| DATABASE_URL | Aiven PostgreSQL connection string, including `sslmode=require`; secret |
| KALKI_REGION | `ap-south-1` |
| RENDER_EXTERNAL_URL | Render supplies `https://kalki-8v0v.onrender.com` |
| KALKI_PUBLIC_TLS | `true` |
| KALKI_ALPHA_PASSWORD | Your workspace password; secret |
| KALKI_BETA_PASSWORD | Separate workspace password; secret |

Render supplies `PORT`. Only explicit Render mode listens on plain HTTP behind
its TLS ingress. Other deployments retain the two private-CA HTTPS listeners.
Do not expose Render's internal HTTP port directly to the internet.

The DB pool uses at most five connections, one minimum, five-minute idle expiry,
30-minute connection lifetime and five-second query deadlines. Run **one receiver
process per database**: in-memory auth/audit coordination is not a multi-replica
protocol. Do not enable overlapping replicas or rolling instances writing the
same database. The offline two-region process remains supported; the current
single Render deployment hosts only `ap-south-1`.

Startup migrates tables idempotently and verifies/hydrates persisted state before
opening listeners. Missing DATABASE_URL selects volatile offline memory mode;
an invalid configured database fails startup instead of silently losing durability.

## Vercel

Keep repository root as Root Directory, `npm ci --prefix dashboard` as install,
`npm run build` as build, and `dist` as output (also in `vercel.json`). Set:

```text
KALKI_AP_URL=https://kalki-8v0v.onrender.com
```

Leave `KALKI_CA_PEM` unset for Render's publicly trusted certificate. Leave
`KALKI_EU_URL` unset until a corresponding EU deployment exists. Never put database
credentials or collector tokens in VITE variables. Redeploy after changing server
environment variables. An unconfigured receiver returns an explicit unavailable
state; the proxy does not manufacture demo metrics or login sessions.

## Connect a device

1. Open `https://kalki-systems.vercel.app/`, click **Get Started**, and sign in to
   `tenant_alpha` using the configured password.
2. Choose **+ Add Collector**, generate a token, and copy the POSIX command for
   Linux/macOS or the PowerShell command for Windows x64.
3. Paste into that device's terminal. No Go installation or CA copying is needed
   for public HTTPS. Keep the terminal running; Ctrl+C stops collection.
4. Open Fleet Overview. The collector appears under its opaque `host_...` ID.
   The first approved sample flushes immediately and subsequent windows are two
   seconds. CPU needs a second sample; network and service cold starts add latency.

Supported binaries: Linux amd64/arm64, macOS amd64/arm64, Windows amd64.
The release build also copies `allowlist.yaml`. Windows ARM is not a native target.
POSIX state lives in `~/.kalki`; Windows state in `%LOCALAPPDATA%\Kalki\data` and
binaries in `%LOCALAPPDATA%\Kalki\bin\<collector-id>`. Reuse the same command and
collector ID when restarting to preserve WAL identity. These commands run in the
foreground; they do not install an operating-system background service.

The probe measures reachability/latency to the receiver's public HTTPS port,
not every application running on the device. Explicit private `--ca-cert` remains
available for LAN collectors and installers. Credentials are masked in the UI;
copying includes the actual token, so keep terminal history private.

## Durability and capacity

Sessions, token rotation, remediation changes and audit writes complete
synchronously before a successful response. Telemetry uses a bounded queue of
256 batches; flush is every 250 ms or 25 batches. Failed writes retry; a full queue
rejects new ingest with 503 so the collector retains its WAL. Graceful shutdown
drains for up to ten seconds.

**Telemetry ACK precedes DB commit**, as required for the asynchronous pipeline.
A sudden process kill can lose acknowledged, unflushed samples (normally the
250 ms window, potentially longer during a DB outage). This is not zero-loss
durability across abrupt receiver failure. The collector deletes acknowledged
WAL entries. Strict zero-loss ACKs would require committing before ACK.

Telemetry retention prunes to 4096 rows per collector every minute. Hydration
loads at most `--ring-capacity` rows per collector and reconstructs analysis from
that bounded history; derived baseline/counter/history reconstruction is not a
byte-for-byte process snapshot. Audit entries remain append-only. Monitor DB disk
and process memory: per-collector retention alone cannot guarantee an 8 GB limit
for unlimited collectors or indefinitely growing audit history.

## Verification

Run from the repository root:

```powershell
go mod tidy
go vet ./...
go test -race ./...
go run ./scripts/build-collectors
go build -tags netgo -ldflags '-s -w' -o tier2-server ./tier2
npm test --prefix dashboard
npm run build
npm run test:hosting
```

To run actual database restart tests, set `KALKI_TEST_DATABASE_URL` privately in
the test shell, then run `go test -race -p 1 ./tier2/store ./tier2/workspace`.
Each DB test uses a random isolated schema and drops only that schema afterwards;
without this variable the integration tests explicitly skip. `-p 1` limits
concurrent test pools against Aiven's 15-connection cap.

After deploying, verify `/healthz` returns `{"status":"ok"}`, all five download
URLs and both installers return 200, and `/app` loads after direct navigation and
refresh. Connect one collector, restart the Render service, then confirm the same
workspace session, collector token, records and valid audit chain still work.
The next sample should continue under the same ID.

Render Free cold starts can delay the first request; two-second sampling is not
a guaranteed two-second first render. An optional UptimeRobot HTTP monitor can
check `https://kalki-8v0v.onrender.com/healthz`; it is an availability probe, not a
guarantee against platform restarts or free-tier limitations.
