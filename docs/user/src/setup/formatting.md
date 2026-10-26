# Format with dprint

[dprint](https://dprint.dev) formats `.vibe` files through the `vibelang` package's dprint plugin. The plugin writes each `.vibe` file as TypeScript, has dprint's TypeScript plugin format it, and writes the result back as VibeLang, so `.vibe` and `.ts` files follow the same `typescript` settings of the project's dprint configuration. Install the packages with [Add VibeLang to an existing project](existing-project.md) first.

## 1. Install dprint

Use dprint 0.64.3 or later, the minimum supported version for this configuration. dprint 0.50.2 cannot resolve `npm:vibelang/dprint-plugin.json`. The alpha review verified it with 0.64.3.

```sh
npm install --save-dev dprint@0.64.3
```

## 2. Configure dprint.json

Add `vibe` to `includes` and the plugin `npm:vibelang/dprint-plugin.json` to `plugins`. If the project has no `dprint.json`, create this one:

```json
{
  "includes": ["**/*.{ts,vibe}"],
  "excludes": ["**/node_modules", "**/dist"],
  "lineWidth": 120,
  "indentWidth": 2,
  "typescript": {
    "semiColons": "asi",
    "quoteStyle": "alwaysDouble"
  },
  "plugins": [
    "https://plugins.dprint.dev/typescript-0.93.4.wasm@cdbf74066391bdb11ddaef9ea95299636a97d637f5a9bf7f121d584875647c7a",
    "npm:vibelang/dprint-plugin.json"
  ]
}
```

dprint finds the plugin in `node_modules/vibelang` from `dprint.json`'s directory, so the configuration needs no checksum for it. The plugin's executable is the native compiler: the manifest names the platform package that holds it, such as `vibelang-linux-x64`, with that package's tarball checksum; dprint downloads the tarball from the npm registry once, checks it, and caches it. List the TypeScript plugin too: the VibeLang plugin formats with it. The VibeLang plugin has no settings; put style settings under `typescript`.

## 3. Format and check

```sh
npx dprint fmt
npx dprint check
```

`dprint fmt` rewrites unformatted files. `dprint check` changes nothing: it prints a diff per unformatted file and exits with status 20, so a CI step or `lint` script fails on a file that needs formatting. Run `npx dprint check` again after `npx dprint fmt`: it prints nothing and exits with status 0.

## What the formatter changes

Everything dprint's TypeScript plugin changes in a `.ts` file: indentation, line breaks at `lineWidth`, quotes, semicolons, trailing commas, and import order. VibeLang's own forms keep their shape:

| Form | Formatted as |
| --- | --- |
| `A!E!R`, `mutable`, patterns after `when` and `is` | As written; line breaks around them follow the surrounding code |
| `f(args) { ... }`, `do { ... }`, `fork { ... }` | The block as a function body, as TypeScript formats `f(args)(() => { ... })` |
| `match (x) { when P: v; ... }` | Arms as object-literal properties: one line when written on one line and they fit, else one per line, each ending in `;` |
| `provide X { ... }`, `uninterruptible { ... }` | The block as a block statement |
| `x catch y`, `x \|> f(%)` | Like a method call and the `\|\|` operator: a line break before `catch` or `\|>` stays |
| `fork x`, `join x`, `throw x`, `await.all x` | Like a prefix operator such as `void` |
| `error`, `schema` | The body as an object type: one line when written on one line and it fits |
| `service`, `control` | The body as a class body, one member per line |
| `try { } handle A.b(x) { } catch (e) { } finally { }` | Each clause as `} catch (e) {`, after the block before it |
| `module text { ... }`, `from text`, `import text` | The body exactly as the same statements in a `.ts` file, imports sorted, indented inside the braces; the module's name stays a name after `from` and `import` |

Comments and JSDoc stay in place. Formatting is idempotent: a formatted file doesn't change when formatted again.

## What the formatter refuses

The plugin reports the file and leaves it unchanged when:

- the file has a syntax error, reported as `vibelang format: line:column: message`, or `vibelang format: line:column: module text: message` in a module body;
- a comment sits inside a VibeLang form's own tokens, such as between `f(args)` and its block or between two clauses of a `try` statement; move it into the block or before the statement;
- formatting would change the program, a plugin bug: report it with the file.

It keeps the superseded `layer` declaration and the retired `select` statement exactly as written; rewrite them as `Layer.effect(...) { ... }` and `await.race`.

## From a VibeLang checkout

`sh scripts/build-vibelang.sh` builds the compiler and writes the plugin beside it: `packages/compiler/dprint-plugin.json`, which names a zip archive of the compiler in `packages/compiler/native/`. A project whose `node_modules/vibelang` links to the checkout formats with the freshly built compiler. `vibelang --write-dprint-plugin <directory>` writes the plugin for another copy of the package.

## The editor

The language server's own formatting, `textDocument/formatting`, is TypeScript's formatter and ignores dprint's settings. Format with dprint: its editor extensions and `dprint lsp` run the same plugin.
