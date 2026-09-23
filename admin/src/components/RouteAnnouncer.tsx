import { useEffect, useRef } from 'react';
import { useLocation } from 'react-router-dom';

/**
 * Page titles per route. A single-page app never reloads, so without this the
 * document title — which is what a screen reader announces on navigation, and
 * what a browser shows in history and tab lists — stays frozen on whatever the
 * first page was.
 */
const TITLES: Array<{ pattern: RegExp; title: string }> = [
  { pattern: /^\/$/,                          title: 'Plugin registry' },
  { pattern: /^\/login$/,                     title: 'Sign in' },
  { pattern: /^\/plugins\/[^/]+$/,            title: 'Plugin details' },
  { pattern: /^\/admin$/,                     title: 'Dashboard' },
  { pattern: /^\/admin\/plugins$/,            title: 'Plugins' },
  { pattern: /^\/admin\/plugins\/new$/,       title: 'New plugin' },
  { pattern: /^\/admin\/plugins\/[^/]+$/,     title: 'Edit plugin' },
  { pattern: /^\/admin\/plugins\/[^/]+\/versions$/, title: 'Versions' },
  { pattern: /^\/admin\/submit$/,             title: 'Submit a plugin' },
  { pattern: /^\/admin\/submissions$/,        title: 'Submissions' },
  { pattern: /^\/admin\/account$/,            title: 'Account' },
];

function titleFor(pathname: string): string {
  const match = TITLES.find(entry => entry.pattern.test(pathname));
  return match ? `${match.title} — semrel Registry` : 'semrel Registry';
}

/**
 * Keeps assistive technology informed about client-side navigation.
 *
 * Updates the document title and announces the new page through a polite live
 * region, skipping the announcement on first render so the initial page load
 * is not narrated twice.
 *
 * It deliberately does not move focus. Doing so on every navigation interrupts
 * whatever the user was doing, and the skip link plus the main landmark
 * already give a one-keystroke route into the content. If it turns out that
 * users are losing their place, focusing `#main-content` here is the change to
 * make.
 */
export default function RouteAnnouncer() {
  const location = useLocation();
  const announcerRef = useRef<HTMLDivElement>(null);
  const isFirstRender = useRef(true);

  useEffect(() => {
    const title = titleFor(location.pathname);
    document.title = title;

    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }

    if (announcerRef.current) {
      announcerRef.current.textContent = `${title.split(' — ')[0]} page loaded`;
    }
  }, [location.pathname]);

  return <div ref={announcerRef} className="sr-only" role="status" aria-live="polite" aria-atomic="true" />;
}
