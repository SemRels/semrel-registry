// @ts-check
import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://registry.semrel.io',
  output: 'static',
  server: {
    host: true,
    port: 3000,
  },
  preview: {
    host: true,
    port: 3000,
  },
});
