# One website on Vercel

The repository now builds marketing at `/` and the console at `/app` into one
`dist/` output. Get Started, Back to Kalki, console assets, direct console links
and hash navigation stay on the same origin. The older LAN commands remain valid.

## Deploy the routing fix

In the existing **kalki-systems** Vercel project, deploy this repository revision
with **Root Directory = repository root**, not `public/` or `dashboard/`.
`vercel.json` supplies these settings:

- Framework: Other
- Install: `npm ci --prefix dashboard`
- Build: `npm run build`
- Output directory: `dist`

Redeploy after uploading/committing these changes. Expected:
`https://kalki-systems.vercel.app/` → Get Started → `/app` sign-in, with no 404.
Back to Kalki returns to `/`. Dashboard tabs use URL hashes and survive refresh
and browser back/forward. `/app/incidents` also opens the selected view after login.

## Connect real data

Set **server-side** Vercel environment variables to your reachable receiver origins:

| Variable | Value |
| --- | --- |
| KALKI_AP_URL | Public HTTPS origin of the running ap-south-1 receiver |
| KALKI_EU_URL | Public HTTPS origin of the running eu-central-1 receiver |
| KALKI_CA_PEM | Optional private CA certificate PEM; omit for public trusted TLS |

Do not put private keys or bearer tokens in URLs or VITE-prefixed variables.
The proxy preserves session Authorization headers; Tier 2 remains responsible
for tenant scoping. TLS verification stays enabled. SSE connections are renewed
before the function timeout, using the existing Last-Event-ID reconnect logic.
Missing backend configuration produces an explicit service-not-connected state.
No mock nodes or fabricated device measurements are supplied.

The current in-memory Go receiver requires a running service with stable state.
Simply deploying these static assets and proxy functions does not deploy that
receiver or convert it into a durable serverless backend. A public Vercel function
cannot reach `127.0.0.1` on your laptop. LAN-only proof/demo controls remain on the
LAN console; the hosted proxy does not expose them.

A browser cannot enumerate arbitrary private devices or read their OS CPU, memory
and logs. Each monitored device needs a collector or another authorized telemetry
source. Configure the receiver and collectors as startup services once; visitors
then just open this website, sign in and search the fleet by collector ID, region
or health. Raw hostnames remain pseudonymized by the privacy boundary.

For a fully Vercel-managed backend, persistent tenant/auth/telemetry storage and a
receiver deployment design are still required; the prior offline memory-only
architecture cannot provide that by changing routing alone.

## Checks

`npm run build` packages both applications. `npm run test:hosting` checks output
assets and proxy TLS/auth/route restrictions using local test certificates.
`npm test --prefix dashboard` checks the existing stream and local gateway logic.

The existing site's deployment was not updated in this session: Vercel CLI
reported an invalid token. Authenticate the CLI or redeploy through your existing
Vercel dashboard after committing these changes. Backend URLs were not supplied.

References: [Vercel project configuration](https://vercel.com/docs/project-configuration),
[Node function runtime](https://vercel.com/docs/functions/runtimes/node-js).
