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

The checker extracts `ts` fences under `docs/user`, except the built site in
`docs/user/site` and blocks whose first
line is `// TypeScript (.ts)` and has no filename caption. It creates one
temporary project per page and runs `effeck --noEmit` once per page. Each block
gets its own module. A filename comment in either of the first two lines,
such as `// launch-options.ts` after `// TypeScript (.ts)` or `// main.effeck`,
names that file and makes it available to relative imports. Named `.ts`
companions keep TypeScript semantics and must import their own dependencies.
The temporary project links the local runtime and Vite plugin packages.
Repeated captions name independent examples, so those blocks keep numbered
filenames. Unnamed Effeck blocks also use numbered `.effeck` files. Imports
and declarations from earlier examples are available in later blocks; a new
name shadows the earlier example's name. Blocks that deliberately fail or opt
out do not supply declarations to later blocks. Local imports with no fixture
remain fragments. No snippet executes.

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

`--compiler path` (or `EFFECK_BINARY`) selects a compiler. The default is
`typescript/built/local/tsc`, then `effeck` on PATH. `--filter text` selects pages,
`--json path` writes diagnostics with Markdown line numbers, and `--keep`
preserves temporary projects for debugging. Compiler crashes, missing runtime
builds, and project configuration errors exit 2. Snippet failures exit 1.
Temporary files are removed on success and failure unless `--keep` is set.

```sh
bun test docs/tools/check-snippets.test.ts docs/tools/hovers.test.ts
bun docs/tools/render-user.ts
```

## Compiler hovers

`hovers.ts` asks the Effeck language server for the hovers and diagnostics of
the same blocks and caches them for a renderer:

```sh
bun docs/tools/hovers.ts
```

It builds the projects that the snippet checker builds, with the same
`--compiler`, `--root`, and `--filter` options, and opens them in one
`effeck --lsp --stdio` session, four pages at a time. It writes one JSON file
per page under `.cache/docs-hovers` (`--cache` selects another directory). Each
block's entry is a Twoslash result: the block's `code` as the fence holds it,
and `nodes` in the shape of `twoslash-protocol`:

| Node | Fields |
| --- | --- |
| `hover` | `start`, `length`, `line`, `character`, `target`, `text` (the quick info), `docs` |
| `error` | the same position fields, `code` such as `EK2001` or `TS2307`, `level` (`error` or `warning`), `text` |

Positions count UTF-16 code units from 0 within the block. `docs` keeps
Effeck's notes, such as "Runs here and returns `number`.", and the first
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
`https://effeck.dev`. `index.md` and `spec/index.md` repeat the first pages.
`llms.txt` lists every copy in the llmstxt.org format, one section per sidebar
view, with a page's sidebar group as its note.

## What smithers.sh serves for Effeck

```sh
bun docs/tools/check-smithers-content.ts
```

Checks the files smithers.sh reads from Effeck's `main`, and the effeck.dev hero. It exits 1 on any problem.

- `.smithers/instructions/app.md`, the guest chat's whole knowledge base:
  - it stays under 24 KiB;
  - every link resolves to a docs page source or a repository file;
  - every Effeck block passes `check-snippets.ts`;
  - the install block equals what `create-effeck` prints.
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
