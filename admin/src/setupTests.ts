import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

// Testing Library only auto-cleans when Vitest globals are enabled, which this
// project does not use. Without this, each test renders into the same document
// as the previous one and queries start matching elements from earlier tests.
afterEach(cleanup);
