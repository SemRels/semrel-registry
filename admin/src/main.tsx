import React from 'react';
import { createRoot } from 'react-dom/client';
import './index.css';
import App from './App';
import { initTheme } from './hooks/useTheme';

async function clearLocalServiceWorkers(): Promise<void> {
	if (!('serviceWorker' in navigator)) return;
	const host = globalThis.location.hostname;
	if (host !== 'localhost' && host !== '127.0.0.1') return;

	const registrations = await navigator.serviceWorker.getRegistrations();
	await Promise.all(registrations.map((registration) => registration.unregister()));
}

void clearLocalServiceWorkers();

// Applied before the first paint so a light-theme visitor never sees a flash
// of the dark palette.
initTheme();

const root = document.getElementById('root');
if (!root) throw new Error('No #root element');
createRoot(root).render(<React.StrictMode><App /></React.StrictMode>);
