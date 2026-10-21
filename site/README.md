# VibeLang documentation site

The site at [vibelang.sh](https://vibelang.sh) is built with [Vocs](https://vocs.dev). The home page is `src/pages/index.mdx`. Every other page comes from the VibeLang user docs in this repository, `../docs/user`, and `sidebar.ts` comes from its `inventory.md`. Run the commands below from `site/`; `npm run deploy` deploys the site from here.

## Update the pages

```bash
node scripts/sync-user-docs.mjs ../docs/user
```

The script replaces every page in `src/pages` except `index.mdx`, `_root.css`, `proposals.mdx`, and `spec/`, rewrites `sidebar.ts`, and converts the Markdown that MDX rejects. Each page keeps its source path: `docs/user/src/error-management/014-expected-errors.md` becomes `/error-management/014-expected-errors`. Edit the source docs, not the generated pages, and run the script again. `vibe.tmLanguage.json` is the grammar from the language repository's `editors/vscode/syntaxes/VibeLang.tmLanguage.json`; it highlights `vibe` code blocks.

## Update the specification

```bash
node scripts/sync-spec-docs.mjs ../docs --facts <facts.json>
node scripts/sync-user-docs.mjs ../docs/user
```

The specification importer validates the Specification inventory and writes 33 chapters plus the introduction at `src/pages/spec/index.mdx`, and `spec-sidebar.ts`. Its shared Markdown converter escapes MDX syntax while preserving code. Proposed blocks become info callouts; Not designed yet blocks become warning callouts. Bare fragments resolve to their owning chapter, with missing or ambiguous targets rejected. A link to another file in the language repository keeps only its text, so the importer needs a full checkout to confirm that the file exists.

The facts JSON must supply `testCount`, `testFiles`, and `kernelLines` as nonnegative integers (or integer strings), taken from a real language repository test run. Example facts are for build verification only and must be replaced before publishing. Unknown or missing placeholders fail before pages are written. Syncing user docs after the spec exists changes specification links to local routes and preserves the spec pages and its independent sidebar. Without the spec, user sync retains its existing external links.

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

### Temporary Vocs search accessibility patch

[`#1`](https://github.com/smithersai/vibelang/issues/1) tracks the search combobox's missing active-result relationship. npm has no native `patchedDependencies` mechanism here. The checked-in patch targets Vocs 2.8.5's `Search.tsx`; npm's `postinstall`, `prebuild` and `predev` hooks apply it with exact version/source checks and zero patch fuzz. The script recompiles only that module because Vocs loads its shipped `dist` JavaScript. It preserves the upstream [MIT license](https://github.com/wevm/vocs/blob/main/LICENSE), package and normal Vocs commands. Node, the existing TypeScript dependency, and the standard `patch` utility are required.

The hook is idempotent and refuses borrowed/symlinked Vocs files or unreviewed source/compiled changes. A workspace borrowing another checkout's dependencies needs its own Vocs directory before running these hooks; shared dependencies must stay unchanged. `node --test scripts/patch-vocs-search.test.mjs` checks these guards. With an existing Playwright installation, run `BASE_URL=http://localhost:4173 PLAYWRIGHT_MODULE=/absolute/path/to/playwright/index.mjs node scripts/verify-search-accessibility.mjs` against the built preview. It checks selection linkage, changed and empty results, responsive instances, dismissal, recent results, and navigation. Omit `BASE_URL` to reproduce against production.

Remove the patch and lifecycle hooks when an [upstream Vocs Search](https://github.com/wevm/vocs/blob/main/src/react/internal/Search.tsx) release provides unique result/listbox IDs and `aria-activedescendant`, then update the pinned version and run the keyboard/browser regressions. Published 2.10.3 still lacked this relationship when checked on 2026-10-18.

## Deploy

`alchemy.run.ts` deploys `dist/public` as a static-assets Worker on `vibelang.sh`, with state in Cloudflare.

```bash
npm run deploy:plan
npm run deploy
```

Both need `CLOUDFLARE_ACCOUNT_ID`, plus `CLOUDFLARE_API_TOKEN` with Workers Scripts edit and Account Secrets Store Edit access. Alchemy's state store keeps its own credentials in the account's Secrets Store and reads them through a temporary Worker binding. Secrets Store Read allows metadata access but cannot create that binding; [Cloudflare requires Edit for bindings](https://developers.cloudflare.com/secrets-store/access-control/).

If planning fails with `Unauthorized: Authentication error`, check the token's access to the selected account's Secrets Store. A token without metadata access fails at `GET /accounts/{account_id}/secrets_store/stores` before a deployment plan can be generated.
