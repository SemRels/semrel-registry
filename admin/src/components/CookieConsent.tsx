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

export default function CookieConsent() {
  useEffect(() => {
    // Force dark mode to match the registry's always-dark theme
    document.documentElement.classList.add('cc--darkmode');

    void run({
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
        }
      },

      onChange: ({ changedCategories }: { changedCategories: string[] }) => {
        if (changedCategories.includes('analytics')) {
          window.gtag?.('consent', 'update', {
            analytics_storage: acceptedCategory('analytics') ? 'granted' : 'denied',
          });
        }
      },

      language: {
        default: 'en',
        translations: {
          en: {
            consentModal: {
              title: 'We use cookies',
              description:
                'We use analytics cookies to understand how visitors use the plugin registry and help us improve it. You can opt out at any time.',
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
                    'Required for the website to function. Cannot be disabled.',
                  linkedCategory: 'necessary',
                },
                {
                  title: 'Analytics',
                  description:
                    'Help us understand how visitors interact with the registry. All data is anonymised and cannot be used to identify you.',
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
        },
      },
    });
  }, []);

  return null;
}
