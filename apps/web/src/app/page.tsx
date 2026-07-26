const primaryAreas = [
  "Overview",
  "Providers",
  "Models",
  "Routing",
  "API keys",
  "Users",
  "Roles",
  "Budgets",
  "Usage",
  "Analytics",
  "Audit logs",
  "Organization settings",
] as const;

export default function Home() {
  return (
    <div className="shell">
      <header className="masthead">
        <a className="skip-link" href="#main-content">
          Skip to content
        </a>
        <div className="identity" aria-label="NexusRelay administration">
          <span className="identity-mark" aria-hidden="true">
            NR
          </span>
          <span>
            <strong>NexusRelay</strong>
            <small>Administration</small>
          </span>
        </div>
        <div className="system-status" role="status" aria-label="Implementation status">
          <span className="status-light" aria-hidden="true" />
          <span>
            <strong>Repository scaffold</strong>
            <small>Services not implemented</small>
          </span>
        </div>
      </header>

      <aside className="sidebar">
        <nav aria-label="Administration areas">
          <p className="nav-label">Workspace</p>
          <ul>
            {primaryAreas.map((area) => (
              <li key={area}>
                <span className="nav-item" aria-disabled="true">
                  <span>{area}</span>
                  <small>Unavailable</small>
                </span>
              </li>
            ))}
          </ul>
        </nav>
      </aside>

      <main id="main-content" className="main-content">
        <section className="availability-card" aria-labelledby="availability-title">
          <div className="signal" aria-hidden="true">
            <span />
            <span />
            <span />
          </div>
          <p className="eyebrow">Implementation status</p>
          <h1 id="availability-title">The admin interface is not implemented yet.</h1>
          <p className="summary">
            This repository scaffold establishes the future administrative layout only. It does
            not observe a running control plane, and no configuration or account actions are
            available.
          </p>

          <dl className="readiness-list">
            <div>
              <dt>Web scaffold</dt>
              <dd>
                <span className="ready-dot" aria-hidden="true" /> Present
              </dd>
            </div>
            <div>
              <dt>Control-plane API</dt>
              <dd>
                <span className="waiting-dot" aria-hidden="true" /> Not implemented
              </dd>
            </div>
            <div>
              <dt>Administrative actions</dt>
              <dd>
                <span className="waiting-dot" aria-hidden="true" /> Not implemented
              </dd>
            </div>
          </dl>

          <div className="notice" role="note" aria-label="Operator note">
            <span aria-hidden="true">i</span>
            <p>
              Future phases will implement sign-in and workspace features. This scaffold does not
              provide controls or infer deployment health.
            </p>
          </div>
        </section>
      </main>

      <footer>
        <span>NexusRelay admin shell</span>
        <span>Administrative features are not implemented</span>
      </footer>
    </div>
  );
}
