import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Where the Go API is reachable during development. Configurable so a second
// instance can be run alongside one already holding the default port.
const apiTarget = process.env.VITE_API_TARGET ?? 'http://localhost:8080';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: './src/setupTests.ts',
  },
  server: {
    port: 5173,
    proxy: {
      '^/api/': {
        target: apiTarget,
        changeOrigin: true,
      },
      '^/auth/': {
        target: apiTarget,
        changeOrigin: true,
      },
      // The catalogue, feed and schemas are served by the API too.
      '^/(plugins\\.json|feed\\.atom|sitemap\\.xml|openapi\\.json)$': {
        target: apiTarget,
        changeOrigin: true,
      },
      '^/schemas/': {
        target: apiTarget,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
});
