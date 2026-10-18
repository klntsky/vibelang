# Documentation checks

From the repository root, install `experiments` and `docs/tools` dependencies
with `bun install` in each directory, build `packages/runtime` with `bun run
build`, and build the native compiler:

```sh
cd typescript/tsc
CGO_ENABLED=0 GOFLAGS=-trimpath go build -o ../built/local/tsc ./cmd/tsc
```

Then run one command from the repository root:

```sh
bun docs/tools/check-snippets.ts
```

The checker extracts `vibe` and `ts` fences under `docs/user`, except the
built site in `docs/user/site`. The fence decides the language: a `vibe`
block compiles as a `.vibe` module and a `ts` block as a `.ts` module. It
skips `ts effect` blocks, Effect TypeScript shown for comparison, and `ts`
blocks whose first line is `// TypeScript (.ts)` and that name no file. It
creates one temporary project per page and runs `vibelang --noEmit` once per
page. Each block gets its own module. A filename comment in either of the
first two lines, such as `// launch-options.ts` after `// TypeScript (.ts)` or
`// main.vibe`, names that file and makes it available to relative imports.
Its extension must match the fence, or the checker stops with an error. `.ts`
companions keep TypeScript semantics and must import their own dependencies.
The temporary project links the local runtime and Vite plugin packages.
Repeated captions name independent examples, so those blocks keep numbered
filenames. Unnamed blocks also use numbered files, such as `block-3.vibe`. Imports
and declarations from earlier examples are available in later blocks; a new
name shadows the earlier example's name. Blocks that deliberately fail or opt
out do not supply declarations to later blocks. Local imports with no fixture
remain fragments. By default, no snippet executes. `--run` also checks declared output (see below).

| Status | Meaning |
| --- | --- |
| `ok` | No unexpected errors; all declared expected codes occur |
| `fragment` | Only TS2304, TS2552, or TS2307 for a relative import; or an explicit opt-out |
| `compiler-gap` | A checked example has a documented compiler disagreement |
| `fail` | Another error, a missing expected diagnostic, or a stale gap marker |

Use a header comment before the code:

- `// expect-error: EK2001` declares an intentionally invalid example. List
  several codes separated by spaces. Unexpected diagnostics still fail.
- `// fragment` opts out of classification for incomplete syntax or a prose
  sketch. The compiler still checks the block.
- `// compiler-gap: <reason>` retains the intended language example while the
  compiler disagrees. Record the page, first code line, diagnostics, and rule
  in `docs/user/notes/compiler-gaps.md`. A marker becomes stale when the block
  has no unexpected substantive error and every expected code occurs. Stale
  markers fail, so resolved gaps cannot silently accumulate.

`--compiler path` (or `VIBELANG_BINARY`) selects a compiler. The default is
`typescript/built/local/tsc`, then `vibelang` on PATH. `--filter text` selects pages,
`--json path` writes diagnostics with Markdown line numbers, and `--keep`
preserves temporary projects for debugging. Compiler crashes, missing runtime
builds, and project configuration errors exit 2. Snippet failures exit 1.
Temporary files are removed on success and failure unless `--keep` is set.

```sh
bun test docs/tools/check-snippets.test.ts docs/tools/first-run.test.ts docs/tools/hovers.test.ts
bun docs/tools/render-user.ts
```

## Running examples

```sh
bun docs/tools/check-snippets.ts --run
bun docs/tools/check-snippets.ts --run --filter getting-started/
```

Declare stdout with an `output` fence immediately after a checked code fence
(blank lines are allowed), or put one `// Output: text` comment per output line
inside the code block. An empty `// Output:` asserts no stdout. Do not use both
forms for one block. Other output fence labels (`out`, `text`) do not opt in.

```vibe
// run: Console.log(greet("Ada"))
function greet(name: string) { return `Hello, ${name}!` }
// Output: Hello, Ada!
```

`// run: <expression>` is an optional header comment. It runs that expression
inside a generated VibeLang entry function, with the block's declarations and
imports in scope. Without it, Node runs the emitted block as the entry module:
export `function main()` or include an explicit top-level runner. Markers never
cause a function to run in the published example itself.

The `check:snippets` package script and the pre-deploy snippet check both use
`--run`. Both also test the locally scaffolded starter, including Vitest without
a prior build, and compare its stdout to the `out` fences in the getting-started,
installation, and new-project guides. It also checks the compiler version shown
in the existing-project guide. Shell commands are not executed by the snippet checker.

With `--run`, the checker emits marked blocks and their dependencies with the
selected compiler, then runs each entry separately with Node and the linked
local runtime. It compares stdout exactly, normalizing CRLF to LF and removing
one final newline from each side. Spaces, ordering, and extra lines matter.
Nonzero exits, execution timeouts (10 seconds by default, changed with
`--timeout milliseconds`), compile errors, and output mismatches fail the check.
Marked fragments, expected-error blocks, and compiler gaps fail instead of
silently skipping execution. Unmarked blocks keep their compile-only checks.
Reports include execution status and captured stdout/stderr for marked blocks.
Use `Console.log` for stable output. Logs with timestamps, randomness, network
access, and interactive programs need deterministic fixtures before opting in.
`--keep` retains emitted files and compiler logs for debugging.

## Compiler hovers

scripts/deploy-docs.sh builds hovers by default with the 0.0.1 release compiler, so popups show what readers' compiler shows. `VIBELANG_DOCS_HOVERS=0` deploys without them. The tool records hover request failures in each page's cache entry and prints examples so language-server defects remain visible.

`hovers.ts` asks the VibeLang language server for the hovers and diagnostics of
the same blocks and caches them for a renderer:

```sh
bun docs/tools/hovers.ts
```

It builds the projects that the snippet checker builds, with the same
`--compiler`, `--root`, and `--filter` options, and opens them in one
`vibelang --lsp --stdio` session, four pages at a time. It writes one JSON file
per page under `.cache/docs-hovers` (`--cache` selects another directory). Each
block's entry is a Twoslash result: the block's `code` as the fence holds it,
and `nodes` in the shape of `twoslash-protocol`:

| Node | Fields |
| --- | --- |
| `hover` | `start`, `length`, `line`, `character`, `target`, `text` (the quick info), `docs` |
| `error` | the same position fields, `code` such as `EK2001` or `TS2307`, `level` (`error` or `warning`), `text` |

Positions count UTF-16 code units from 0 within the block. `docs` keeps
VibeLang's notes, such as "Runs here and returns `number`.", and the first
paragraph of the declaration's JSDoc. Links to `file:///` URIs keep only their
text, and paths into the temporary project or `node_modules` become relative.
The tool refuses to write a page whose entry still contains anything that
`prepare-site.ts` refuses to publish (`leaks.ts`).

A page's entry is reused while its blocks, the compiler binary, the lockfiles,
the runtime's declarations, and these tools are unchanged; otherwise the
whole page is refreshed, because later blocks import earlier ones. Pages
that no longer exist lose their entries unless `--filter` is set. A hover
request that the server answers with an error is a language server defect:
the entry keeps it under `failures`, and every run prints one example of each.
Concurrent hover requests can render a contextually typed property's owner
generically or instantiated, such as `Key<T>` or `Key<string>`, so a cold run
can differ from the last one in a few nodes. Upstream tsgo does the same.

`render-user.ts` builds the one docs site in `docs/user/site`: the user pages
from `docs/user/src` and one page per specification chapter from
`docs/design/src` under `spec/`, in the order of the sidebar in
`docs/user/inventory.md`. `spec/index.html` repeats the specification's first
page, so `/spec/` still opens the specification. Before rendering, it runs the
kernel law suite in `experiments` and refuses to build if any test fails; the
suite also supplies the specification's `{{testCount}}`, `{{testFiles}}`, and
`{{kernelLines}}` facts. It checks every local link's page and heading id. A
specification link to a heading on another chapter's page, written as a bare
fragment such as `#20-the-machine`, and "Chapter 20" in specification prose
both link to that page. A link to a repository file that is not a page keeps
its text and drops the link.

Each page is also written as Markdown beside its HTML, at the same path with
`.md`, with links to pages rewritten to their `.md` copies on
`https://vibelang.sh`. `index.md` and `spec/index.md` repeat the first pages.
`llms.txt` lists every copy in the llmstxt.org format, one section per sidebar
view, with a page's sidebar group as its note.

## What smithers.sh serves for VibeLang

```sh
bun docs/tools/check-smithers-content.ts
```

Checks the files smithers.sh reads from VibeLang's `main`, and the vibelang.sh hero. It exits 1 on any problem.

- `.smithers/instructions/app.md`, the guest chat's whole knowledge base:
  - it stays under 24 KiB;
  - every link resolves to a docs page source or a repository file;
  - every VibeLang block passes `check-snippets.ts`;
  - the install block equals what `create-vibelang` prints.
- `.smithers/home.json`: only blocks smithers.sh renders, and every markdown path exists.
- `.smithers/flows/convert/prompt.md`: matches `build-prompt.mjs`'s output.
- `docs/site/converter/hero.json`: equals golden case 01, with matching hashes.

It needs the same compiler and runtime build as `check-snippets.ts`. `bun test check-smithers-content.test.ts` covers the link, section and home rules.

## Converter content

`bun run --cwd docs/tools check:content` checks the app instructions, repository
home, generated converter prompt and hero receipt. It also rejects held-out
golden source in the prompt or worked examples, after normalizing whitespace
and comments between tokens. The converter prompt uses `examples/` only;
all 20 cases in `golden/`, including the hero, remain held out.

Run the overlap regression tests with:

```sh
node --test .smithers/flows/convert/check-heldout.test.mjs
```
