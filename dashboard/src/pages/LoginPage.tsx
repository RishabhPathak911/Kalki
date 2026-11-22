import HostedStatus from './HostedStatus';
import '../membrane.css';
import { useState, type FormEvent } from 'react';
import type { Region, Session } from '../lib/types';
import MembraneCanvas from './MembraneCanvas';

export default function LoginPage({ onLogin }: { onLogin: (s: Session) => void }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [region, setRegion] = useState<Region>('ap-south-1');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const fill = (tenant: 'alpha' | 'beta') => {
    setUsername(`tenant_${tenant}`);
    setPassword(`${tenant}-demo-local`);
    setRegion(tenant === 'alpha' ? 'ap-south-1' : 'eu-central-1');
    setError('');
  };

  async function login(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      const r = await fetch(`/region/${region}/api/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password })
      });
      if (!r.ok) {
        throw new Error(
          r.status === 401
            ? 'Username or password is incorrect.'
            : r.status === 503
            ? 'This workspace service is not connected yet. Contact the site owner.'
            : 'Workspace service is unavailable. Please try again shortly.'
        );
      }
      onLogin((await r.json()) as Session);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unable to sign in.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="membrane-login">
      <MembraneCanvas />
      <div className="login-center">
        <a
          className="back-kalki"
          href={import.meta.env.VITE_KALKI_HOME_URL || (import.meta.env.DEV ? 'http://localhost:8080' : '/')}
        >
          ← Back to Kalki
        </a>
        <div className="glass-shell">
          <section className="solid-login">
            <div data-reveal className="membrane-wordmark">
              <img src={`${import.meta.env.BASE_URL}kalki-eye.svg`} alt="" />
              <h1>KALKI</h1>
            </div>
            <p data-reveal className="access-pill">
              CLOUD ACCESS · ZERO RAW EGRESS
            </p>
            <h2 data-reveal>Enter your workspace</h2>
            <p data-reveal className="muted">
              Private infrastructure. Clear intelligence.
            </p>
            <HostedStatus region={region} />
            <form onSubmit={login}>
              <label data-reveal>
                Workspace username
                <input
                  autoComplete="username"
                  required
                  value={username}
                  onChange={e => setUsername(e.target.value)}
                  placeholder="tenant_alpha"
                />
              </label>
              <label data-reveal>
                Password
                <input
                  type="password"
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                />
              </label>
              <label data-reveal>
                Regional gateway
                <select value={region} onChange={e => setRegion(e.target.value as Region)}>
                  <option>ap-south-1</option>
                  <option>eu-central-1</option>
                </select>
              </label>
              {error && (
                <p role="alert" className="error">
                  {error}
                </p>
              )}
              <button data-reveal className="cloud-submit" disabled={busy}>
                {busy ? 'Connecting…' : 'Enter Kalki Cloud'}
              </button>
            </form>
            <div data-reveal className="membrane-quickfill">
              <button type="button" onClick={() => fill('alpha')}>
                Tenant Alpha · ap-south-1
              </button>
              <button type="button" onClick={() => fill('beta')}>
                Tenant Beta · eu-central-1
              </button>
            </div>
            <p data-reveal className="fine security-footer">
              Session-scoped access · Verified regional TLS
              <br />
              Self-hosted on your network. Raw logs stay local.
            </p>
          </section>
        </div>
      </div>
    </main>
  );
}
