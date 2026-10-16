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
