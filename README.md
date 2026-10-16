# EffectScript

A language for writing Effect programs: TypeScript syntax in which every function is an Effect, built on one-shot delimited continuations over Effect 4's fiber runtime.

| Path | What it is |
| --- | --- |
| `docs/design/` | The spec (`effectscript-design.html`, built from `src/` by `docs/tools/render.ts`) and the decision log (`DECISIONS.md`) |
| `docs/user/` | End-user docs that mirror Effect 4's documentation page for page, in original EffectScript writing; build with `docs/tools/render-user.ts` |
| `docs/research/` | Research behind the design: Effect 4's runtime, continuation prior art, the TypeScript compiler, TC39 proposals, usage censuses, feature coverage |
| `docs/reviews/` | Independent reviews of the design |
| `experiments/` | The continuation kernel prototype on `effect@4.0.5` and its law suite (`bun test`); the design build refuses to run if any test fails |
| `prototype/esc/` | A prototype compiler on the TypeScript API that lowers `.es` files to Effect TypeScript |
| `artifacts/` | The earlier design-space proposal, kept for history; superseded by `docs/design/` |

The fork of the TypeScript compiler lives in its own repository, `../effectscript-ts`.

```sh
cd experiments && bun install && bun test          # kernel laws
cd docs/tools && bun install && bun render.ts      # build the spec
bun prototype/esc/examples/run-all.ts              # prototype compiler examples
```

`experiments/scripts/v3-wakeup-chain.mjs` needs Effect 3.22.3 unpacked in `experiments/scratch-agent-a/v3pkg/` (`npm pack effect@3.22.3`), which is not checked in.
