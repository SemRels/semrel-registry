import { Link, useLocation } from 'react-router-dom';

export default function NotFoundPage() {
  const location = useLocation();

  return (
    <main className="centered-status centered-status--page" id="main-content">
      <h1 className="not-found__title">Page not found</h1>
      <p className="muted max-w-38">
        Nothing is published at <code>{location.pathname}</code>. The plugin may have been
        renamed, or the link may be incomplete.
      </p>
      <div className="flex gap-md flex-wrap justify-center mt-1">
        <Link to="/" className="btn btn--primary">Browse the registry</Link>
        <Link to="/admin" className="btn btn--secondary">Go to the admin panel</Link>
      </div>
    </main>
  );
}
