# Effeck examples

Each project has exact Effect 4.0.5 dependencies, `.effeck` sources in `src`,
Node ESM output in `dist`, and a short README with an Effect TypeScript equivalent.

| Example | Shows |
| --- | --- |
| [hello](hello/) | One function and one log; an exported `main` runs when node starts the module. |
| [users](users/) | Abstract services, layers, `wire`, typed failures, and test layers. |
| [http-api](http-api/) | A schema-validated HTTP route, conflict errors, and socket-free route tests. |
| [file-tree](file-tree/) | Inferred recursion, platform services, a producer, and concurrent reads. |
| [resilience](resilience/) | Retry schedules, deadlines, races, block-owned fibers, and virtual-time tests. |
| [approval](approval/) | One release program under automatic and scripted control handlers. |
| [pipeline](pipeline/) | A producer flowing through Stream operators and a `for...of` consumer. |
| [react-vite](react-vite/) | A React component that runs an Effeck service lookup through a `ManagedRuntime` and renders its recovered error. |
| [vue-vite](vue-vite/) | A Vue single-file component over an Effeck layer whose setup holds the todo list. |
| [svelte-vite](svelte-vite/) | A Svelte 5 `{#await}` block over quotes that `await.all` runs concurrently. |

Set up the repository's local compiler and runtime first using the repository
setup instructions. The manifests install `effeck` from
`../../packages/compiler` and `effeck-runtime` from the tarball that setup
packs, `../../packages/runtime/effeck-runtime.tgz`, so the runtime and the
example load one copy of `effect`. Run repository setup before installing the
examples, and after rerunning it, run `bun update effeck-runtime` in an
example you installed before.
Then, in an example directory, run `bun install`,
`bun run build`, and `bun run start`. The users, http-api, and resilience
projects also have `bun run test`; Vitest compiles their `.effeck` sources
through `effeck-vite`, with source maps for assertion stacks.
Run file-tree from its own directory to find the bundled fixtures.

From the repository root, `scripts/check-examples.sh` compiles every project
and compares the six finite programs with their output fixtures. It compiles
http-api without starting its persistent server. Set `EFFECK=/path/to/effeck` to
select a compiler; otherwise it uses `effeck` on PATH, then `typescript/built/local/tsc`.
Each project gets one status line. A failure returns exit code 1 and leaves
diagnostics in `.cache/check-examples/`. Execution has a 15-second bound.

The executable language reference provides a separate check during compiler work:

```sh
(cd experiments && bun install)
(cd packages/runtime && bun install)
ln -s ../../experiments/node_modules prototype/esc/node_modules # once
# Install dependencies in each example directory with bun install.
bun scripts/check-examples-prototype.ts
```

The prototype check compiles all seven main modules, runs each exported `main`,
compares finite output,
and runs the six `it.effect` tests. It creates disposable `.prototype` directories
inside each example and writes each project's diagnostics to `.prototype/check.log`.
Project sources use `.effeck` imports per D31; the compiler rewrites them to `.js`.
It runs a main module through a TypeScript harness, except hello, whose module
already contains its runner. The CLI's `run` command would run hello twice.
File-tree's main runs, but the prototype widens recursive E and R to `unknown`;
checking its Effeck runner then reports EK3201. Its contract stays inferred
as specified by D52 instead of adding annotations to work around the prototype.

The setup lane provides `sh scripts/setup.sh` to build the local compiler and
runtime. Both local package directories must exist before installing an example;
otherwise `bun install` reports their missing `package.json` files.

The initial real-compiler snapshot on `main` at `79130dda` rejects all seven projects:

| Example | First missing support |
| --- | --- |
| hello | A runner argument written as a zero-argument program function. |
| users | Prelude types, abstract service lookup, and layer composition. |
| http-api | The Schema/TaggedError prelude and abstract services. |
| file-tree | Methods called through imported FileSystem and Path service keys. |
| resilience | The Effect/TaggedError prelude and program arguments. |
| approval | Control declarations, handlers, and the prelude. |
| pipeline | Producer-to-Stream conversion and Stream iteration. |

These are compiler targets. `noEmitOnError` prevents running partially emitted code.
Rerun the checker after compiler lanes land; the snapshot above remains historical.
