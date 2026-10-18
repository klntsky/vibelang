# VibeLang documentation site

The site at [vibelang.sh](https://vibelang.sh) is built with [Vocs](https://vocs.dev). The home page is `src/pages/index.mdx`. Every other page comes from the VibeLang user docs, `docs/user` in the language repository (formerly Effeck), and `sidebar.ts` comes from its `inventory.md`.

## Update the pages

```bash
node scripts/sync-user-docs.mjs <language repo>/docs/user
```

The script replaces every page in `src/pages` except `index.mdx` and `_root.css`, rewrites `sidebar.ts`, and converts the Markdown that MDX rejects. Each page keeps its source path: `docs/user/src/error-management/014-expected-errors.md` becomes `/error-management/014-expected-errors`. Edit the source docs, not the generated pages, and run the script again. `vibe.tmLanguage.json` is the grammar from the language repository's `editors/vscode/syntaxes/VibeLang.tmLanguage.json`; it highlights `vibe` code blocks.

## Development

```bash
npm ci
npm run dev
```

The local site is available at `http://localhost:5173`.

## Production build

```bash
npm run build
npm run preview
```

Vocs checks internal links during the production build, and a dead link fails it. Generated output is written to `dist/` and is ignored by Git.

## Deploy

`alchemy.run.ts` deploys `dist/public` as a static-assets Worker on `vibelang.sh`, with state in Cloudflare.

```bash
npm run deploy:plan
npm run deploy
```

Both need `CLOUDFLARE_ACCOUNT_ID`, plus `CLOUDFLARE_API_TOKEN` with Workers Scripts edit and Account Secrets Store read access. Alchemy's state store keeps its own credentials in the account's Secrets Store, so a token without Secrets Store access fails with `Unauthorized: Authentication error` before planning.
