# Effeck app

> **Warning:** Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use.

```sh
bun run start
bun run dev
bun run test
```

This project was created from the Effeck starter. If its dependencies are not
installed yet, for example after `--no-install`, run `bun install` first.

`start` builds the project and prints `Hello, Effeck!` and
`Recovered from EmptyName`. `dev` builds once, then runs `effeck --watch` and
Node's output watcher together; press Ctrl+C to stop both. Vitest runs
`src/**/*.test.effeck` through `@effeck/vite` and `@effect/vitest`. The plugin
checks the project with `effeck`, serves compiled JavaScript from
`node_modules/.effeck/`, and maps assertion stacks to the original `.effeck`
lines. `test` needs no prior build.

Edit `src/main.effeck` for the service and layer, and `src/greeting.effeck` for
a typed failure. Import local `.effeck` source files; the compiler emits `.js`
imports. Source maps connect Node debugging to those sources.

## Editor

```sh
code --install-extension node_modules/effeck/editor/effeck.vsix
# Or, for Cursor:
cursor --install-extension node_modules/effeck/editor/effeck.vsix
```

Then open this directory. The extension starts this project's compiler as the
language server.

## Versions

`effeck`, `@effeck/runtime`, and `@effeck/vite` share one version. Update them
together:

```sh
bun add @effeck/runtime@latest
bun add -d effeck@latest @effeck/vite@latest
```

Keep `effect`, `@effect/platform-node`, and `@effect/vitest` at 4.0.5: the
runtime requires that exact Effect version.

If this project came from a local Effeck checkout through `scripts/setup.sh`,
its Effeck packages are `file:` paths into that checkout. Rerun setup there
after updating the checkout, then run
`bun update @effeck/runtime effeck @effeck/vite` here.
