# EffectScript app

Install local tooling first with `sh scripts/setup.sh` in the EffectScript
repository. Keep that checkout available: this project depends on its packages
through absolute `file:` paths written when setup installs the template.
After updating the checkout, rerun setup there, then run
`bun update @effectscript/runtime @effectscript/compiler @effectscript/vite` here to install
the new packages.

```sh
bun install
bun run build
bun run start
bun run dev
bun run test
```

`start` prints `Hello, EffectScript!` and `Recovered from EmptyName`. `dev`
builds once, then runs `esc --watch` and Node's output watcher together. Press
Ctrl+C to stop both. Vitest runs `src/**/*.test.es` through `@effectscript/vite` and
`@effect/vitest`. The plugin checks the project with `esc`, serves compiled
JavaScript from `node_modules/.effectscript/`, and maps assertion stacks to
the original `.es` lines. `test` needs no prior build. The Node server keeps
its compiler and output watchers for `dev`.

Edit `src/main.es` for the service and layer, and `src/greeting.es` for a typed
failure. Import local `.es` source files; the compiler emits `.js` imports.
Source maps connect Node debugging to those sources.

Install the `.vsix` printed by setup with `code --install-extension <path>`
(or `cursor --install-extension <path>`), then open this directory in your editor.
The extension starts this project's compiler as the language server.
