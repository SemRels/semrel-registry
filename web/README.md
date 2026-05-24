# go-semrel-registry web

Astro-based landing page and static documentation for `registry.semrel.io`.

## Commands

All commands run from `web/`:

| Command | Action |
| :-- | :-- |
| `npm install` | Install dependencies and mirror `../plugins.json` into `public/` |
| `npm run dev` | Start the Astro dev server on `http://localhost:3000` |
| `npm run build` | Create the static production build in `dist/` |
| `npm run preview` | Preview the generated site locally on port `3000` |

## Notes

- The raw registry payload lives at the repository root in `../plugins.json`.
- `scripts/sync-public-assets.mjs` creates a symlink to that file when possible and falls back to copying it on platforms without symlink support.
- The site is designed for static hosting on GitHub Pages today and a dedicated `registry.semrel.io` deployment later.
