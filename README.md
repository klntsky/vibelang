# EffectScript

A language for writing Effect programs: TypeScript syntax in which every function is an Effect, built on one-shot delimited continuations over Effect 4's fiber runtime.

## Quickstart

From this checkout, install the local compiler, runtime, and Bun project template:

```sh
sh scripts/setup.sh
bun create effectscript my-app --no-git
cd my-app
bun run build
bun run start
bun run test
```

Setup requires Go, Bun, and Node on `PATH`. The tested toolchain is Go 1.26
(with automatic Go 1.27 download), Bun 1.4.1, and Node 26.10. Nothing needs to
be published to npm. Keep this checkout available: scaffolded projects use
absolute `file:` dependencies on its compiler and Vite packages and on the
runtime tarball that setup packs. Rerun setup after updating the checkout, then
run `bun update @effectscript/runtime @effectscript/compiler @effectscript/vite` in
existing projects to install the new packages.

`bun run dev` builds once, watches `.es` sources with `esc --watch`, and runs
Node with `--watch` on the output. `bun run test` compiles `.es` tests before
running Vitest with `@effect/vitest`. The starter demonstrates an abstract-class
service, a layer, a tagged failure, and `try`/`catch` recovery.

Setup builds the editor extension when `editors/vscode` is available and prints
its exact `.vsix` path. Install it with `code --install-extension <path>` or
`cursor --install-extension <path>`, then open your project directory.

During the parallel build, setup reports missing runtime or editor packages.
The end-to-end check fails at the first incomplete step with its diagnostics:

```sh
sh scripts/e2e.sh
```

See [Installation](docs/user/src/getting-started/004-installation.md) and
[Getting Started](docs/user/src/013-getting-started.md) for the full workflow.

## Before you write your first file

- Every function in a `.es` file is an Effect, and a call runs it:
  `const todo = Todos.get(id)` is a `Todo`. Pass work to an Effect operator as
  a function with no parameters:
  `Effect.retry(() => send(message), Schedule.exponential("100 millis"))`.
- `Effect`, `Layer`, `Schema`, `Scope`, `TaggedError`, `Control`, `pipe`,
  `wire`, `fork`, `handle`, `resume`, `resumeWith`, and `abandon` need no
  import. Import other source files by their source name,
  `import { Todos } from "./todos.es"`; the compiler emits `.js` imports.
- A module's top level only declares. A call there that would run a program
  is ES1001, including `Effect.succeed(1)`. Start the program once with a
  runner: `NodeRuntime.runMain(() => main())`.
- Plain functions from other packages need a boundary, such as
  `Effect.try(() => parse(text))`, or a `/** @effectscript trusted */`
  re-export from a `.ts` file (ES4401). Effect's packages are trusted,
  including `assert` and `expect` from `@effect/vitest`: import them from
  `@effect/vitest`, not from `vitest`.
- `console.log`, `fetch`, timers, and `Date.now()` are errors in `.es` code
  (ES4402, ES4403). Use `Effect.log` or `Console`, `Effect.sleep`, `Clock`,
  and Effect's HTTP modules: `effect/http` with `NodeHttpServer` from
  `@effect/platform-node`.
- A `Disposable`'s `[Symbol.dispose]` runs synchronously and cannot run an
  Effect (ES4301). Acquire a resource whose release logs or waits with
  `Effect.acquireRelease`, and hold it with `using`.
- The user docs mirror Effect's pages and add an EffectScript features section:
  [control operations and handlers](docs/user/src/effectscript-features/111-control-operations-and-handlers.md),
  [producers](docs/user/src/effectscript-features/112-producers.md),
  [building an HTTP API](docs/user/src/effectscript-features/113-building-an-http-api.md),
  and [EffectScript for Effect users](docs/user/src/effectscript-features/114-effectscript-for-effect-users.md).
- A route handler's services are request requirements. `HttpRouter.serve`
  turns them into the server layer's inputs, which `wire` provides; a test
  that calls `HttpRouter.toHttpEffect` provides them with
  `HttpRouter.provideRequest(layer)(routes)`.

| Path | What it is |
| --- | --- |
| `docs/design/` | The spec (`effectscript-design.html`, built from `src/` by `docs/tools/render.ts`) and the decision log (`DECISIONS.md`) |
| `docs/user/` | End-user docs that mirror Effect 4's documentation page for page, in original EffectScript writing; build with `docs/tools/render-user.ts` |
| `docs/research/` | Research behind the design: Effect 4's runtime, continuation prior art, the TypeScript compiler, TC39 proposals, usage censuses, feature coverage |
| `docs/reviews/` | Independent reviews of the design |
| `experiments/` | The continuation kernel prototype on `effect@4.0.5` and its law suite (`bun test`); the design build refuses to run if any test fails |
| `prototype/esc/` | A prototype compiler on the TypeScript API that lowers `.es` files to Effect TypeScript; the executable reference for lowering |
| `prototype/single-pass/` | The experiment that settled single-pass lowering (D56) on a patched TypeScript 5.9 checker |
| `typescript/` | The compiler, `esc`: microsoft/TypeScript 7 (Go) with the EffectScript commits on top; `typescript/EFFECTSCRIPT.md` describes them |
| `artifacts/` | The earlier design-space proposal, kept for history; superseded by `docs/design/` |

```sh
cd experiments && bun install && bun test          # kernel laws
cd docs/tools && bun install && bun render.ts      # build the spec
bun prototype/esc/examples/run-all.ts              # prototype compiler examples
cd typescript && sh effectscript/smoke.sh          # build esc and run its cases
```

`experiments/scripts/v3-wakeup-chain.mjs` needs Effect 3.22.3 unpacked in `experiments/scratch-agent-a/v3pkg/` (`npm pack effect@3.22.3`), which is not checked in.
