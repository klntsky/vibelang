# Effeck

A language for writing Effect programs: TypeScript syntax in which every function is an Effect, built on one-shot delimited continuations over Effect 4's fiber runtime.

> [!WARNING]
> Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use.

Docs: [effeck.willcory10.workers.dev](https://effeck.willcory10.workers.dev) for the guides and [effeck.willcory10.workers.dev/spec/](https://effeck.willcory10.workers.dev/spec/) for the specification.

## Quickstart

```sh
bun create effeck my-app
cd my-app
bun run start
bun run test
```

With npm, run `npm create effeck@latest my-app`. You need Node 26 or later and
Bun 1.4 or later. The command copies a starter, pins `effeck`,
`effeck-runtime`, and `effeck-vite` to one version, and installs dependencies;
`--no-install` skips installation. The compiler is a native binary from a
per-platform package for macOS, Linux, and Windows on arm64 and x64. Version
0.0.1 is tested on macOS on Apple silicon and on Linux on x64.

`bun run start` prints `Hello, Effeck!` and `Recovered from EmptyName`.
`bun run dev` builds once, watches `.effeck` sources with `effeck --watch`, and
runs Node with `--watch` on the output. `bun run test` compiles `.effeck` tests
before running Vitest with `@effect/vitest`. The starter demonstrates an
abstract-class service, a layer, a tagged failure, and `try`/`catch` recovery.

To add Effeck to an existing project, see
[Installation](docs/user/src/getting-started/004-installation.md).

## Editor setup

```sh
code --install-extension node_modules/effeck/editor/effeck.vsix
# Or, for Cursor:
cursor --install-extension node_modules/effeck/editor/effeck.vsix
```

Run the command from the project directory after installing dependencies, then
open the project. The extension runs the project's compiler as the language
server, `effeck --lsp --stdio`, for diagnostics, hovers, and completions. It is
not on the VS Code Marketplace or Open VSX yet. For Neovim, Zed, and other LSP
clients, see [editors/](editors/README.md).

## Before you write your first file

- Every function in a `.effeck` file is an Effect, and a call runs it:
  `const todo = Todos.get(id)` is a `Todo`. Where an Effect operator expects a
  program, pass the call itself; it is passed by name and runs when the
  operator runs it:
  `Effect.retry(send(message), Schedule.exponential("100 millis"))`. A program
  of several statements is a function with no parameters, `() => { … }`.
- Declare an expected failure with `error NotFound { id: string }`, a tagged
  error class with readonly fields; `throw new NotFound({ id })` adds it to
  the function's errors, and `catch (e: NotFound) { ... }` handles only it.
- Write a contract as `Result!Error!Dependencies`: `User!NotFound!Db` is
  `Effect<User, NotFound, Db>`, an empty slot is `never` (`User!!`), and
  dependencies left out of a function's return type are inferred from its body.
- Declare a service with `service Users { find(id: string): User!NotFound }`
  and call `Users.find(id)`; `control` declares a control operation the same way.
- Implement a service with `layer UsersLive: Users { find(id) { ... } }`;
  statements before the members run once, when the layer builds.
- Provide a layer to a block with `with AppLive { ... }`; a `return` inside
  returns from the enclosing function.
- `Effect`, `Layer`, `Schema`, `Scope`, `TaggedError`, `Control`, `pipe`,
  `wire`, `fork`, `handle`, `resume`, `resumeWith`, `abandon`, the boundary
  hatches `sync`, `try`, and `callback`, the runners, and the Effect
  functions `sleep`, `log`, `retry`, `timeout`, `all`, `forEach`, `race`, and
  a few more need no import, and neither does any other module of `effect`'s root, such as
  `Console`, `Option`, and `Stream`. Import other source files by their source name,
  `import { Todos } from "./todos.effeck"`; the compiler emits `.js` imports.
- A value of one of Effect's data types takes its module's functions as
  methods: `maybe.map(f).getOrElse(() => 0)` calls `Option.map` and
  `Option.getOrElse`, and `stream.filter(p).take(3)` calls `Stream.filter` and
  `Stream.take`. Arrays, strings, and objects keep JavaScript's methods.
- A module's top level only declares. A call there that would run a program
  is EK1001, including `Effect.succeed(1)`. Start the program once with a
  runner, which receives `main()` by name: `NodeRuntime.runMain(main())`.
- Plain functions from other packages need a boundary, such as
  `try(parse(text))`, or a `/** @effeck trusted */`
  re-export from a `.ts` file (EK4401). Effect's packages are trusted,
  including `assert` and `expect` from `@effect/vitest`: import them from
  `@effect/vitest`, not from `vitest`.
- `console.log`, `fetch`, timers, and `Date.now()` are errors in `.effeck` code
  (EK4402, EK4403). Use `Effect.log` or `Console`, `Effect.sleep`, `Clock`,
  and Effect's HTTP modules: `effect/http` with `NodeHttpServer` from
  `@effect/platform-node`.
- A `Disposable`'s `[Symbol.dispose]` runs synchronously and cannot run an
  Effect (EK4301). Acquire a resource whose release logs or waits with
  `Effect.acquireRelease`, and hold it with `using`.
- The user docs mirror Effect's pages and add an Effeck features section:
  [control operations and handlers](docs/user/src/effeck-features/111-control-operations-and-handlers.md),
  [producers](docs/user/src/effeck-features/112-producers.md),
  [building an HTTP API](docs/user/src/effeck-features/113-building-an-http-api.md),
  and [Effeck for Effect users](docs/user/src/effeck-features/114-effeck-for-effect-users.md).
- A route handler's services are request requirements. `HttpRouter.serve`
  turns them into the server layer's inputs, which `wire` provides; a test
  that calls `HttpRouter.toHttpEffect` provides them with
  `HttpRouter.provideRequest(layer)(routes)`.

## Develop Effeck

Work on the compiler, runtime, editor, or docs from this checkout. Setup
requires Go, Bun, and Node on `PATH`. The tested toolchain is Go 1.26 (with
automatic Go 1.27 download), Bun 1.4.1, and Node 26.10.

```sh
sh scripts/setup.sh
bun create effeck my-app --no-git
```

Setup builds the native compiler into `packages/compiler/native/effeck`, packs
the runtime, builds the editor extension and prints its `.vsix` path, and
installs a local template into `~/.bun-create/effeck`. `bun create effeck`
prefers that local template over the npm package, so projects created after
setup use absolute `file:` dependencies on this checkout's compiler and Vite
packages and on the runtime tarball that setup packs. Keep the checkout
available. Rerun setup after updating it, then run
`bun update effeck-runtime effeck effeck-vite` in those projects. To scaffold
from npm instead, run `npm create effeck@latest`, or remove
`~/.bun-create/effeck`.

The end-to-end check runs setup, scaffolding, installation, the build, the
program, and its tests in a temporary project, and fails at the first
incomplete step:

```sh
sh scripts/e2e.sh
```

| Path | What it is |
| --- | --- |
| `docs/design/` | The spec (`effeck-design.html`, built from `src/` by `docs/tools/render.ts`) and the decision log (`DECISIONS.md`) |
| `docs/user/` | End-user docs that mirror Effect 4's documentation page for page, in original Effeck writing; build with `docs/tools/render-user.ts` |
| `docs/research/` | Research behind the design: Effect 4's runtime, continuation prior art, the TypeScript compiler, TC39 proposals, usage censuses, feature coverage |
| `docs/reviews/` | Independent reviews of the design |
| `experiments/` | The continuation kernel prototype on `effect@4.0.5` and its law suite (`bun test`); the design build refuses to run if any test fails |
| `prototype/esc/` | A prototype compiler on the TypeScript API, from before the rename, that lowers `.es` files (the old extension) to Effect TypeScript; the executable reference for lowering |
| `prototype/single-pass/` | The experiment that settled single-pass lowering (D56) on a patched TypeScript 5.9 checker |
| `typescript/` | The compiler, `effeck`: microsoft/TypeScript 7 (Go) with the Effeck commits on top; `typescript/EFFECK.md` describes them |
| `artifacts/` | The earlier design-space proposal, kept for history; superseded by `docs/design/` |

```sh
cd experiments && bun install && bun test          # kernel laws
cd docs/tools && bun install && bun render.ts      # build the spec
bun prototype/esc/examples/run-all.ts              # prototype compiler examples
cd typescript && sh effeck/smoke.sh                # build effeck and run its cases
bun docs/tools/check-snippets.ts                   # compile every user-doc code block
sh scripts/check-examples.sh                       # build and run the examples
sh scripts/deploy-docs.sh                          # publish the docs on Cloudflare
```

To run any of these on a Linux build host instead of this machine, set `ES_REMOTE_HOST` (and `ES_REMOTE_KEY`) and prefix the command with `scripts/remote.sh` from the directory it runs in, for example `cd typescript && ../scripts/remote.sh sh effeck/smoke.sh --status`.

`experiments/scripts/v3-wakeup-chain.mjs` needs Effect 3.22.3 unpacked in `experiments/scratch-agent-a/v3pkg/` (`npm pack effect@3.22.3`), which is not checked in.
