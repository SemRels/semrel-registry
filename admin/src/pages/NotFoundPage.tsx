import { Link, useLocation } from 'react-router-dom';

export default function NotFoundPage() {
  const location = useLocation();

  return (
    <main className="centered-status centered-status--page" id="main-content">
      <h1 style={{ fontSize: '2rem', marginBottom: '.25rem' }}>Page not found</h1>
      <p className="muted" style={{ maxWidth: '38rem' }}>
        Nothing is published at <code>{location.pathname}</code>. The plugin may have been
        renamed, or the link may be incomplete.
      </p>
      <div style={{ display: 'flex', gap: '.75rem', flexWrap: 'wrap', justifyContent: 'center', marginTop: '.5rem' }}>
        <Link to="/" className="btn btn--primary">Browse the registry</Link>
        <Link to="/admin" className="btn btn--secondary">Go to the admin panel</Link>
      </div>
    </main>
  );
}
