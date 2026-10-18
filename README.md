# VibeLang

A language for building reliable TypeScript in the AI era, built on [Effect](https://effect.website). VibeLang gives Effect a simple, readable syntax that compiles to TypeScript, with first-class access to the TypeScript ecosystem.

> [!WARNING]
> VibeLang is v0. The language will change in major, breaking ways. It is not suitable for production use.

Docs: [effeck.dev](https://effeck.dev), with the guides and, under [effeck.dev/spec/](https://effeck.dev/spec/), the specification.

## A short tour

```ts
error NotFound { id: string }

interface User { id: string; name: string }

service Users {
  find(id: string): User!NotFound
}

const UsersLive = Layer.succeed(Users, {
  find(id) {
    if (id !== "42") throw new NotFound({ id })
    return { id, name: "Ada" }
  }
})

function greet(id: string) {
  return `Welcome, ${Users.find(id).name}` catch (e) => `No user ${e.id}`
}

export function main() {
  provide UsersLive {
    const [ada, missing] = await.all [greet("42"), greet("7")]
    const first = await.race [greet("42"), do { sleep("1 second"); "timeout" }]
    const shout = ada |> %.toUpperCase()
    Console.log(shout, missing, first)
  }
}
```

Every function is an Effect: `greet` has the contract `string!!Users` (no expected failures, because `catch` handles `NotFound`). Calls run where they stand, `throw` fails with a typed error, `provide` supplies a layer, and `await.all` and `await.race` run programs concurrently. The [cheat sheet](https://effeck.dev/vibelang-features/115-cheat-sheet) shows every form next to the Effect code it replaces.

## Quickstart

```sh
bun create vibelang my-app
cd my-app
bun run start
bun run test
```

With npm, run `npm create vibelang@latest my-app`. You need Node 26 or later and
Bun 1.4 or later. The command copies a starter, pins `vibelang`,
`vibelang-runtime`, and `vibelang-vite` to one version, and installs dependencies;
`--no-install` skips installation. The compiler is a native binary from a
per-platform package for macOS, Linux, and Windows on arm64 and x64. Version
0.0.1 is tested on macOS on Apple silicon and on Linux on x64.

`bun run start` prints `Hello, VibeLang!` and `Recovered from EmptyName`.
`bun run dev` builds once, watches `.vibe` sources with `vibelang --watch`, and
runs Node with `--watch` on the output. `bun run test` compiles `.vibe` tests
before running Vitest with `@effect/vitest`. The starter demonstrates an
`service`, a `Layer.effect` layer, an `error`, and `try`/`catch` recovery.

To add VibeLang to an existing project, see
[Installation](docs/user/src/getting-started/004-installation.md).

## Editor setup

```sh
code --install-extension node_modules/vibelang/editor/vibelang.vsix
# Or, for Cursor:
cursor --install-extension node_modules/vibelang/editor/vibelang.vsix
```

Run the command from the project directory after installing dependencies, then
open the project. The extension runs the project's compiler as the language
server, `vibelang --lsp --stdio`, for diagnostics, hovers, and completions. It is
not on the VS Code Marketplace or Open VSX yet. For Neovim, Zed, and other LSP
clients, see [editors/](editors/README.md).

## Before you write your first file

- Every function in a `.vibe` file is an Effect, and a call runs it:
  `const todo = Todos.get(id)` is a `Todo`. Where an Effect operator expects a
  program, pass the call itself; it is passed by name and runs when the
  operator runs it:
  `Effect.retry(send(message), Schedule.exponential("100 millis"))`. A program
  of several statements is a function with no parameters, `() => { … }`.
- Declare an expected failure with `error NotFound { id: string }`, a tagged
  error class with readonly fields; `throw new NotFound({ id })` adds it to
  the function's errors. `catch (e)` receives expected failures; test which one
  with `match (e) { when NotFound: …; default: throw e; }` or `e is NotFound`.
  In an expression, `x catch fallback` and `x catch (e) => …` recover the same
  failures.
- Declare a struct schema and its type with `schema User { id: string; role: "admin" | "member" }`;
  a field may name another schema.
- Members of interfaces and object types are readonly unless written
  `mutable`: `interface Cart { id: string; mutable items: Array<string> }`.
- Write a contract as `Result!Error!Dependencies`: `User!NotFound!Db` is
  `Effect<User, NotFound, Db>`, an empty slot is `never` (`User!!`), and
  dependencies left out of a function's return type are inferred from its body.
- Declare a service with `service Users { find(id: string): User!NotFound }`
  and call `Users.find(id)`; `control` declares a control operation the same way.
- Implement a service with `Layer.effect(Users) { setup; return { find(id) { ... } } }`;
  the setup runs once, when the layer builds, and `Layer.succeed(Users, { ... })` needs none.
- Provide a layer to a block with `provide AppLive { ... }`; a `return` inside
  returns from the enclosing function. `provide Clock = testClock { ... }`
  provides one service, and layers and services mix in one list.
- Run programs concurrently with the TC39 await operations, which behave as
  the `Promise` methods: `const [a, b] = await.all [left(), right()]`,
  `await.race [...]`, `await.any [...]`, `await.allSettled [...]`. Start
  background work with `fork work()` and wait for the fiber with `join`;
  `for concurrent(8) (const x of xs)` runs a loop body in parallel. A plain
  `await` is an error with a quick fix: `join` for a fiber, `d.await()` for a
  Deferred, and a call already waits.
- Answer a control operation with a handle clause:
  `try { return release(v) } handle Approval.approve(request) { resume(true) }`.
  A `return` in the try block or a clause returns from the enclosing function.
- `Effect`, `Layer`, `Schema`, `Scope`, `TaggedError`, `Control`, `pipe`,
  `wire`, `fork`, `handle`, `resume`, `resumeWith`, `abandon`, the boundary
  hatches `sync`, `try`, and `callback`, the runners, and the Effect
  functions `sleep`, `log`, `retry`, `timeout`, `all`, `forEach`, `race`, and
  a few more need no import, and neither does any other module of `effect`'s root, such as
  `Console`, `Option`, and `Stream`. Import other source files by their source name,
  `import { Todos } from "./todos.vibe"`; the compiler emits `.js` imports.
- A value of one of Effect's data types takes its module's functions as
  methods: `maybe.map(f).getOrElse(() => 0)` calls `Option.map` and
  `Option.getOrElse`, and `stream.filter(p).take(3)` calls `Stream.filter` and
  `Stream.take`. Arrays, strings, and objects keep JavaScript's methods.
- Chain operators with the pipe operator: `x |> f(%, a)`, where `%` stands
  for `x`. Where `%` is an argument that expects a program, `x` is passed by
  name: `fetchUser(id) |> retry(%, policy) |> timeout(%, "1 second")` retries
  and times the call. Elsewhere `x` runs once and `%` is its value.
- A module's top level only declares. A call there that would run a program
  is EK1001, including `Effect.succeed(1)`. Export `function main()` from the
  entry module: `node dist/main.js` runs it with `NodeRuntime.runMain`, and
  importing the module runs nothing. A runner you call at the top level,
  `NodeRuntime.runMain(main())`, replaces that.
- Plain functions from other packages need a boundary, such as
  `try((text) => parse(text), text)`, or a `/** @vibelang trusted */`
  re-export from a `.ts` file (EK4401). Effect's packages are trusted,
  including `assert` and `expect` from `@effect/vitest`: import them from
  `@effect/vitest`, not from `vitest`.
- `console.log`, `fetch`, timers, and `Date.now()` are errors in `.vibe` code
  (EK4402, EK4403). Use `Effect.log` or `Console`, `Effect.sleep`, `Clock`,
  and Effect's HTTP modules: `effect/http` with `NodeHttpServer` from
  `@effect/platform-node`.
- A `Disposable`'s `[Symbol.dispose]` runs synchronously and cannot run an
  Effect (EK4301). Acquire a resource whose release logs or waits with
  `Effect.acquireRelease`, and hold it with `using`.
- The user docs mirror Effect's pages and add a VibeLang features section:
  [control operations and handlers](docs/user/src/vibelang-features/111-control-operations-and-handlers.md),
  [producers](docs/user/src/vibelang-features/112-producers.md),
  [building an HTTP API](docs/user/src/vibelang-features/113-building-an-http-api.md),
  and [VibeLang for Effect users](docs/user/src/vibelang-features/114-vibelang-for-effect-users.md).
- A route handler's services are request requirements. `HttpRouter.serve`
  turns them into the server layer's inputs, which `wire` provides; a test
  that calls `HttpRouter.toHttpEffect` provides them with
  `HttpRouter.provideRequest(layer)(routes)`.

## Develop VibeLang

Work on the compiler, runtime, editor, or docs from this checkout. Setup
requires Go, Bun, and Node on `PATH`. The tested toolchain is Go 1.26 (with
automatic Go 1.27 download), Bun 1.4.1, and Node 26.10.

```sh
sh scripts/setup.sh
bun create vibelang my-app --no-git
```

Setup builds the native compiler into `packages/compiler/native/vibelang`, packs
the runtime, builds the editor extension and prints its `.vsix` path, and
installs a local template into `~/.bun-create/vibelang`. `bun create vibelang`
prefers that local template over the npm package, so projects created after
setup use absolute `file:` dependencies on this checkout's compiler and Vite
packages and on the runtime tarball that setup packs. Keep the checkout
available. Rerun setup after updating it, then run
`bun update vibelang-runtime vibelang vibelang-vite` in those projects. To scaffold
from npm instead, run `npm create vibelang@latest`, or remove
`~/.bun-create/vibelang`.

The end-to-end check runs setup, scaffolding, installation, the build, the
program, and its tests in a temporary project, and fails at the first
incomplete step:

```sh
sh scripts/e2e.sh
```

| Path | What it is |
| --- | --- |
| `docs/design/` | The spec, one Markdown file per chapter in `src/`, and the decision log (`DECISIONS.md`); the docs site renders each chapter as a page |
| `docs/user/` | End-user docs that mirror Effect 4's documentation page for page, in original VibeLang writing; `docs/tools/render-user.ts` builds them and the spec into one site |
| `docs/research/` | Research behind the design: Effect 4's runtime, continuation prior art, the TypeScript compiler, TC39 proposals, usage censuses, feature coverage |
| `docs/reviews/` | Independent reviews of the design |
| `experiments/` | The continuation kernel prototype on `effect@4.0.5` and its law suite (`bun test`); the docs build refuses to run if any test fails |
| `prototype/esc/` | A prototype compiler on the TypeScript API, from before the rename, that lowers `.es` files (the old extension) to Effect TypeScript; the executable reference for lowering |
| `prototype/single-pass/` | The experiment that settled single-pass lowering (D56) on a patched TypeScript 5.9 checker |
| `typescript/` | The compiler, `vibelang`: microsoft/TypeScript 7 (Go) with the VibeLang commits on top; `typescript/VIBELANG.md` describes them |
| `artifacts/` | The earlier design-space proposal, kept for history; superseded by `docs/design/` |

```sh
cd experiments && bun install && bun test          # kernel laws
cd docs/tools && bun install && bun render-user.ts # build the docs site and the spec
bun prototype/esc/examples/run-all.ts              # prototype compiler examples
cd typescript && sh vibelang/smoke.sh                # build vibelang and run its cases
bun docs/tools/check-snippets.ts                   # compile every user-doc code block
bun docs/tools/check-smithers-content.ts           # check app.md, home.json, the convert prompt and hero
node --test scripts/brand-gate.test.mjs            # no retired brand spelling outside the gate's exemptions
sh scripts/check-examples.sh                       # build and run the examples
sh scripts/deploy-docs.sh                          # publish the docs on Cloudflare
```

To run any of these on a Linux build host instead of this machine, set `ES_REMOTE_HOST` (and `ES_REMOTE_KEY`) and prefix the command with `scripts/remote.sh` from the directory it runs in, for example `cd typescript && ../scripts/remote.sh sh vibelang/smoke.sh --status`.

`experiments/scripts/v3-wakeup-chain.mjs` needs Effect 3.22.3 unpacked in `experiments/scratch-agent-a/v3pkg/` (`npm pack effect@3.22.3`), which is not checked in.
