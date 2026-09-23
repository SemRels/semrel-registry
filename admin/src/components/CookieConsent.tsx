import { useEffect } from 'react';
import { run, acceptedCategory } from 'vanilla-cookieconsent';
import 'vanilla-cookieconsent/dist/cookieconsent.css';
import './CookieConsent.css';

declare global {
  interface Window {
    gtag: (...args: unknown[]) => void;
    dataLayer: unknown[];
  }
}

const GA_MEASUREMENT_ID = 'G-0RD0CMG2RV';

/**
 * Loads the Google Analytics tag.
 *
 * Called only after the visitor consents. Loading it up front — as index.html
 * used to — sends their IP address and current URL to Google before they have
 * been asked, which Consent Mode does not prevent: it governs cookies, not the
 * request.
 */
function loadAnalytics() {
  if (document.getElementById('ga-tag')) return;

  const script = document.createElement('script');
  script.id = 'ga-tag';
  script.async = true;
  script.src = `https://www.googletagmanager.com/gtag/js?id=${GA_MEASUREMENT_ID}`;
  document.head.appendChild(script);

  window.gtag?.('js', new Date());
  window.gtag?.('config', GA_MEASUREMENT_ID, { anonymize_ip: true });
}

/** Returns the banner language, following the browser's preference. */
function preferredLanguage(): 'de' | 'en' {
  const languages = navigator.languages ?? [navigator.language];
  return languages.some(lang => lang?.toLowerCase().startsWith('de')) ? 'de' : 'en';
}

/**
 * Keeps the consent banner's own dark-mode class in step with the page theme.
 *
 * It used to be forced on unconditionally, which was right while the registry
 * was dark-only and wrong the moment it gained a light mode.
 */
function useConsentTheme() {
  useEffect(() => {
    const root = document.documentElement;
    const media = globalThis.matchMedia?.('(prefers-color-scheme: dark)');

    const sync = () => {
      const explicit = root.getAttribute('data-theme');
      const dark = explicit ? explicit === 'dark' : (media?.matches ?? true);
      root.classList.toggle('cc--darkmode', dark);
    };

    sync();
    media?.addEventListener('change', sync);
    const observer = new MutationObserver(sync);
    observer.observe(root, { attributes: true, attributeFilter: ['data-theme'] });

    return () => {
      media?.removeEventListener('change', sync);
      observer.disconnect();
    };
  }, []);
}

export default function CookieConsent() {
  useConsentTheme();

  useEffect(() => {
    void run({
      // The banner follows the visitor's theme rather than forcing dark, now
      // that the registry itself has a light mode.
      mode: 'opt-in',

      categories: {
        necessary: { enabled: true, readOnly: true },
        analytics: {
          autoClear: {
            cookies: [{ name: /^_ga/ }, { name: '_gid' }],
          },
        },
      },

      onConsent: () => {
        if (acceptedCategory('analytics')) {
          window.gtag?.('consent', 'update', { analytics_storage: 'granted' });
          loadAnalytics();
        }
      },

      onChange: ({ changedCategories }: { changedCategories: string[] }) => {
        if (!changedCategories.includes('analytics')) return;

        if (acceptedCategory('analytics')) {
          window.gtag?.('consent', 'update', { analytics_storage: 'granted' });
          loadAnalytics();
        } else {
          window.gtag?.('consent', 'update', { analytics_storage: 'denied' });
        }
      },

      language: {
        default: preferredLanguage(),
        translations: {
          en: {
            consentModal: {
              title: 'We use cookies',
              description:
                'We use analytics cookies to understand how visitors use the plugin registry and help us improve it. Analytics is only loaded if you accept. You can change your choice at any time.',
              acceptAllBtn: 'Accept all',
              acceptNecessaryBtn: 'Reject all',
              showPreferencesBtn: 'Manage preferences',
            },
            preferencesModal: {
              title: 'Cookie preferences',
              acceptAllBtn: 'Accept all',
              acceptNecessaryBtn: 'Reject all',
              savePreferencesBtn: 'Save preferences',
              closeIconLabel: 'Close',
              sections: [
                {
                  title: 'Cookie usage',
                  description:
                    'We use cookies to ensure basic functionality and to understand how you use the registry. Choose which categories to enable below.',
                },
                {
                  title: 'Strictly necessary',
                  description:
                    'Required for the website to function, including your sign-in session and your theme choice. Cannot be disabled.',
                  linkedCategory: 'necessary',
                },
                {
                  title: 'Analytics',
                  description:
                    'Help us understand how visitors interact with the registry. The Google Analytics script is only loaded after you accept this category.',
                  linkedCategory: 'analytics',
                  cookieTable: {
                    headers: { name: 'Name', domain: 'Domain', desc: 'Description' },
                    body: [
                      { name: '_ga',   domain: 'registry.semrel.io', desc: 'Distinguishes users (2 years)' },
                      { name: '_ga_*', domain: 'registry.semrel.io', desc: 'Session state (1 year)' },
                      { name: '_gid',  domain: 'registry.semrel.io', desc: 'Session (24 hours)' },
                    ],
                  },
                },
              ],
            },
          },
          de: {
            consentModal: {
              title: 'Wir verwenden Cookies',
              description:
                'Wir verwenden Analyse-Cookies, um zu verstehen, wie die Plugin-Registry genutzt wird, und sie zu verbessern. Die Analyse wird nur geladen, wenn Sie zustimmen. Sie können Ihre Auswahl jederzeit ändern.',
              acceptAllBtn: 'Alle akzeptieren',
              acceptNecessaryBtn: 'Alle ablehnen',
              showPreferencesBtn: 'Einstellungen verwalten',
            },
            preferencesModal: {
              title: 'Cookie-Einstellungen',
              acceptAllBtn: 'Alle akzeptieren',
              acceptNecessaryBtn: 'Alle ablehnen',
              savePreferencesBtn: 'Auswahl speichern',
              closeIconLabel: 'Schließen',
              sections: [
                {
                  title: 'Verwendung von Cookies',
                  description:
                    'Wir verwenden Cookies für die Grundfunktionen der Website und um zu verstehen, wie Sie die Registry nutzen. Wählen Sie unten, welche Kategorien Sie zulassen möchten.',
                },
                {
                  title: 'Unbedingt erforderlich',
                  description:
                    'Für den Betrieb der Website erforderlich, einschließlich Ihrer Anmeldesitzung und Ihrer Theme-Auswahl. Kann nicht deaktiviert werden.',
                  linkedCategory: 'necessary',
                },
                {
                  title: 'Analyse',
                  description:
                    'Hilft uns zu verstehen, wie Besucher die Registry nutzen. Das Google-Analytics-Skript wird erst geladen, nachdem Sie diese Kategorie akzeptiert haben.',
                  linkedCategory: 'analytics',
                  cookieTable: {
                    headers: { name: 'Name', domain: 'Domain', desc: 'Beschreibung' },
                    body: [
                      { name: '_ga',   domain: 'registry.semrel.io', desc: 'Unterscheidet Nutzer (2 Jahre)' },
                      { name: '_ga_*', domain: 'registry.semrel.io', desc: 'Sitzungsstatus (1 Jahr)' },
                      { name: '_gid',  domain: 'registry.semrel.io', desc: 'Sitzung (24 Stunden)' },
                    ],
                  },
                },
              ],
            },
          },
        },
      },
    });
  }, []);

  return null;
}
