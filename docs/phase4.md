# Phase 4: login, regional access, audit and onboarding

Phase 4 provides backend authentication. The Phase 5 analysis, simulation and
SSE APIs are now implemented; see [Phase 5](phase5.md). The sign-in screen and
console remain Phase 6 work. No database, external auth
provider or runtime internet connection is required.

## Run the demo

Run from the repository root. Each terminal needs Go on PATH:

```powershell
$env:Path = "$env:LOCALAPPDATA\KalkiToolsNative\go\bin;$env:Path"
```

1. Generate certificates once and build collector releases:

   ```powershell
   go run ./scripts/gen-certs
   go run ./scripts/build-collectors
   ```

   Existing certificates are never overwritten. If `data/certs/` already
   contains your certificates, skip generation. Releases appear under
   `data/releases/`: Linux amd64, macOS arm64, Windows amd64, and the required
   `allowlist.yaml`. Cache build dependencies before disconnecting.

2. Start both regional receivers in terminal A:

   ```powershell
   go run ./tier2
   ```

   Expected: authenticated HTTPS listeners on `127.0.0.1:8443` (ap-south-1)
   and `127.0.0.1:8444` (eu-central-1). They have separate receiver epochs and
   telemetry buffers. One supervisor process shares an in-memory authority
   and global read layer between these two regional instances. These are
   logical LAN regions, not separate geographic or distributed deployments.

3. In terminal B, log in and mint a collector token. Use `curl.exe`, not the
   PowerShell alias. JSON files avoid native-command quoting differences:

   ```powershell
   '{"username":"tenant_alpha","password":"alpha-demo-local"}' | Set-Content -Encoding ascii data/login.json
   $login = curl.exe --fail --silent --show-error --ssl-revoke-best-effort --cacert data/certs/ca.pem -H 'Content-Type: application/json' --data-binary '@data/login.json' https://localhost:8443/api/auth/login | ConvertFrom-Json
   $session = $login.token
   '{"collector_id":"host_12345678"}' | Set-Content -Encoding ascii data/mint.json
   $onboard = curl.exe --fail --silent --show-error --ssl-revoke-best-effort --cacert data/certs/ca.pem -H "Authorization: Bearer $session" -H 'Content-Type: application/json' --data-binary '@data/mint.json' https://localhost:8443/api/collectors/tokens | ConvertFrom-Json
   $onboard.collector_id
   ```

   Expected: login identifies `tenant_alpha`, pinned to `ap-south-1`; mint
   returns `token`, `collector_id`, `collector_command`, and `install_command`.
   An empty object allocates a random opaque collector ID. Requests cannot
   choose a different tenant or region. Minting an existing ID requires rotation.

4. Start the collector in terminal B:

   ```powershell
   $env:KALKI_TOKEN = $onboard.token
   go run ./tier1 --tenant=tenant_alpha --region=ap-south-1 --collector-id=host_12345678 --mode=live --probe-target=127.0.0.1:8443 --tier2-url=https://localhost:8443 --wal-path=data/phase4-alpha.wal --key-file=data/tenant_alpha.key --proof-addr=127.0.0.1:9090
   ```

   A new onboarding ID needs a new WAL. Keep existing Phase 3 WALs intact.
   To deliver historical data, mint a token for that WAL's original collector
   ID and use its original node/key/WAL configuration instead. The optional
   onboarding ID overrides only the envelope identity; raw identities still
   pass through the original privacy pipeline.

5. While collection runs, open
   **http://127.0.0.1:9090/api/egress-proof**. Expected: `ACKED`,
   `match_bool: true`, matching SHA-256 hashes, and WAL depth returning to zero.
   The safe local policy test remains:

   ```powershell
   Invoke-RestMethod http://127.0.0.1:9090/api/attempt-violation
   ```

   Expected: `blocked: true`, `sent: false`. This never sends the invalid
   payload. The proof service remains loopback-only and origin-restricted.

6. Stop the collector with Ctrl+C to reuse terminal B's session variable, then
   inspect data and audit:

   ```powershell
   curl.exe --ssl-revoke-best-effort --cacert data/certs/ca.pem -H "Authorization: Bearer $session" 'https://localhost:8443/api/records?collector_id=host_12345678'
   curl.exe --ssl-revoke-best-effort --cacert data/certs/ca.pem -H "Authorization: Bearer $session" https://localhost:8443/api/global
   curl.exe --ssl-revoke-best-effort --cacert data/certs/ca.pem -H "Authorization: Bearer $session" https://localhost:8443/api/audit
   curl.exe --ssl-revoke-best-effort --cacert data/certs/ca.pem -H "Authorization: Bearer $session" https://localhost:8443/api/audit/verify
   ```

   Expected: your numeric summaries, your collector metadata, fixed audit
   events, and `valid: true`. Adding `tenant=tenant_beta` to `/api/records`
   returns **403**. Reading beta's collector ID with alpha's session returns
   an empty list. Other endpoints reject query selectors with **400**.

7. Rotate the token:

   ```powershell
   '{"collector_id":"host_12345678","rotate":true}' | Set-Content -Encoding ascii data/rotate.json
   $rotated = curl.exe --fail --silent --show-error --ssl-revoke-best-effort --cacert data/certs/ca.pem -H "Authorization: Bearer $session" -H 'Content-Type: application/json' --data-binary '@data/rotate.json' https://localhost:8443/api/collectors/tokens | ConvertFrom-Json
   curl.exe -i --ssl-revoke-best-effort --cacert data/certs/ca.pem -H "Authorization: Bearer $env:KALKI_TOKEN" https://localhost:8443/api/session
   $env:KALKI_TOKEN = $rotated.token
   ```

   Expected: the old token returns **401** immediately. Restart the collector
   with the updated environment token and unchanged WAL. Sending an alpha
   collector token to port 8444 returns **421** and records `wrong_region` in
   alpha's audit chain. A token cannot claim a different tenant or collector.

8. Log out:

   ```powershell
   curl.exe --ssl-revoke-best-effort --cacert data/certs/ca.pem -X POST -H "Authorization: Bearer $session" https://localhost:8443/api/auth/logout
   ```

   This session now returns **401**. Collector tokens are independent of
   workspace logout. Sessions expire after eight hours.

For beta, log in with `tenant_beta` / `beta-demo-local` at port 8444 and mint a
different collector ID. Use `--tenant=tenant_beta --region=eu-central-1`, its
own key/WAL, port 8444, and a separate proof port such as 9091. Session reads
can enter either listener; the session's region determines the data source.
Collector traffic is pinned strictly to its assigned listener.

## Installation and LAN addresses

Securely copy only the trusted `ca.pem` to the target machine, then run the
returned `install_command` in a POSIX shell. It uses `curl --cacert`, passes
flags through `sh -s --`, detects Linux amd64/macOS arm64, downloads the binary
and policy, sets executable permissions, and runs in the foreground. Temporary
binary/policy files are removed when it exits; keys and WAL remain in the
working directory's `data/`. Windows users download the `.exe` and policy or
use the repository command above. `collector_command` expects the policy at
`config/allowlist.yaml` and the trusted CA at `ca.pem`.

The static installer and download URLs contain no tokens. Generated commands
include the requested `--token` flag; it can be visible in local shell history
or process tools. Use `KALKI_TOKEN` for manual runs. Keep generated commands out
of screenshots, shared logs, and source control.

For a LAN host, generate a certificate with its IP/DNS SAN and set advertised
origins explicitly (substitute the actual IP):

```powershell
go run ./scripts/gen-certs --out=data/lan-certs --hosts=localhost,127.0.0.1,SERVER_LAN_IP
go run ./tier2 --addr=0.0.0.0:8443 --eu-addr=0.0.0.0:8444 --public-url=https://SERVER_LAN_IP:8443 --eu-public-url=https://SERVER_LAN_IP:8444 --cert=data/lan-certs/server.pem --key=data/lan-certs/server.key
```

Both seeded users are demo workspace administrators. Override their passwords
with `KALKI_ALPHA_PASSWORD` / `KALKI_BETA_PASSWORD` before starting Tier 2.
This phase does not add user registration or granular roles. There is a
process-wide login limit of 60 attempts/minute and a cap of 10,000 active
sessions and collector registrations each.

## Restart behavior and audit limits

Network interruption preserves credentials in a running receiver. A **Tier 2
process restart clears sessions, collector tokens, telemetry and audit chains**.
Log in again, mint each original collector ID (`rotate: false` for the new
registry), and restart collectors with their new token and unchanged WAL.
The receiver-session handshake replays retained history. Never delete a WAL
to clear an authorization failure.

Collector/session credentials are opaque random tokens stored as SHA-256
hashes in RAM. Rotation and authorization are serialized so an old credential
cannot start another accepted request after rotation returns. Only the
authenticated workspace gateway is wired to production listeners; the internal
ingest handler is a protocol test component.

Audit is append-only in memory with a separate chain per tenant. Entries have
a UTC millisecond timestamp, index, fixed event label, status, previous hash,
and current hash. Queries are themselves audited. Unattributable auth failures
and startup configuration events go to an internal system chain, never a
customer's feed. Logs never contain request bodies, failed-login usernames,
arbitrary paths, IPs, passwords, or tokens.

Verification checks all links and the retained head/count, detecting modified
entries and truncation. It is not durable forensic storage: restart clears it,
and a fully compromised process could alter both a chain and its anchor.
Audit memory grows during process lifetime; it is not silently truncated.
Global reads project only authenticated-tenant collector ID, region, last-seen
time and accepted-record count, never raw logs or metric values.

## Acceptance checks

```powershell
go vet ./...
go test -race ./...
go test -race ./tier2/workspace -v
```

Tests cover both listeners, A/B data isolation across current routes, rejected
tenant selectors, token/session role separation, wrong claims and 421 audit
events, rotation/revocation, logout, hashed credential storage, audit mutation
and truncation, installer/download contents, HTTPS enforcement, and authenticated
WAL delivery. Existing Phase 1–3 tests include the real 30-second process outage.
The three release targets build locally; Linux/macOS native execution remains
a CI/environment check, not a claim of local runtime verification.
