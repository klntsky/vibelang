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

The checker extracts `ts` fences under `docs/user`, except blocks whose first
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
bun test docs/tools/check-snippets.test.ts
bun docs/tools/render-user.ts
```

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
