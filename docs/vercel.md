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

The Go receiver now supports optional PostgreSQL persistence via DATABASE_URL.
Follow [Render + Aiven deployment](hosted-deployment.md) for the build command,
secret environment settings, one-line installers and durability limits. Vercel
functions proxy the receiver; they do not run device collectors. LAN-only proof
and demo controls remain local.

A browser cannot enumerate arbitrary private devices or read their OS CPU, memory
and logs. Each monitored device needs a collector or another authorized telemetry
source. Configure the receiver and collectors as startup services once; visitors
then just open this website, sign in and search the fleet by collector ID, region
or health. Raw hostnames remain pseudonymized by the privacy boundary.
