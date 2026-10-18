# Schema Compilers

Effect's experimental Schema compilers work behind the ordinary Schema APIs in VibeLang. They generate JavaScript for supported parsing operations while leaving unsupported work to the interpreter. Measure the change on your schemas before relying on a performance improvement. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Understanding optimized validation paths

```vibe
// schemas/Parcel.vibe
// run: Console.log(JSON.stringify(readParcel({ reference: "pkg-94", count: 2 }))); try { readParcel({ reference: "pkg-94", count: "2" }) } catch (error) { Console.log(error._tag) }
import { Console } from "effect"
export const Parcel = Schema.Struct({
  reference: Schema.String,
  count: Schema.Int
})

function readParcel(input: unknown) {
  return Schema.decodeUnknownEffect(Parcel)(input)
}
```

```output
{"reference":"pkg-94","count":2}
SchemaError
```

A compiler installs optimized operations for exact schema ASTs. The decoder or constructor can try a synchronous fast path; success returns immediately, while failure falls through to the detailed parser that produces normal issues. Guards can return false directly because they do not need detailed failures.

Compilation must be active for the relevant AST and operation before a consumer captures its parser. Checks and property getters can run twice after a failed fast path, so keep them deterministic and free of side effects. The compiler avoids fast paths that would replay defaults, class construction, transformations, middleware, or other effectful work unsafely.

## Enabling just-in-time compilation and fallback

```vibe
// main.vibe
// run: Console.log(JSON.stringify(receiveCompiled({ reference: "pkg-94", count: 2 }))); try { receiveCompiled({}) } catch (error) { Console.log(error.message) }
import "effect/schema/SchemaJITCompiler/enable"

const CompiledParcel = Schema.Struct({
  reference: Schema.String,
  count: Schema.Int
})

function receiveCompiled(input: unknown) {
  return Schema.decodeUnknownEffect(CompiledParcel)(input)
}
```

```output
{"reference":"pkg-94","count":2}
Missing key
  at ["reference"]
```

Put the enabling import before code that creates or runs parsers. Importing the compiler namespace alone does not enable it. Operations compile lazily on first use, without eagerly executing every check or default.

```vibe
// run: enableSelectedDirections(); Console.log(Schema.decodeEffect(Schema.FiniteFromString)("2")); Console.log(Schema.is(Schema.toType(Schema.FiniteFromString))(2)); Console.log(Schema.encodeEffect(Schema.FiniteFromString)(2))
import { SchemaJITCompiler } from "effect/schema"

function enableSelectedDirections() {
  try(() => {
    SchemaJITCompiler.enable(Schema.FiniteFromString.ast)
    SchemaJITCompiler.enable(SchemaAST.toType(Schema.FiniteFromString.ast))
    SchemaJITCompiler.enable(SchemaAST.flip(Schema.FiniteFromString.ast))
  })
}
```

```output
2
true
2
```

Selective enabling applies to the exact AST and dependencies reached while parsing it. The original AST decodes, the type projection validates and constructs decoded values, and the flipped AST encodes. Global enabling handles those roots as operations encounter them.

JIT uses dynamic function construction. When the engine or Content Security Policy blocks it, or generation or initialization fails, parsing stays interpreted. An exception thrown by a compiled parser that is already running is an ordinary parser exception, not a compiler-initialization failure that triggers a retry.

## Generating and loading ahead-of-time validators

```ts
// TypeScript (.ts)
// build-schema-aot.ts
// run: return Effect.flatMap(buildSchemas(), (result) => Console.log(`Prepared ${result.schemas} schema exports`))
import { fileURLToPath } from "node:url"
import { Console, Effect } from "effect"
import { NodeFileSystem, NodePath } from "@effect/platform-node"
import * as Build from "effect/schema/SchemaAOTCompiler/Build"

export const buildSchemas = () => Effect.provide(
  Build.build({
    modules: {
      "./schemas/Parcel.js": () => import("./schemas/Parcel.js")
    },
    baseUrl: import.meta.url,
    outFile: fileURLToPath(new URL("./generated/parcel-validators.js", import.meta.url)),
    operations: ["decode", "encode", "is", "make"]
  }),
  [NodeFileSystem.layer, NodePath.layer]
)
```

```output
Prepared 1 schema exports
```

```vibe
// build.vibe
// run: main()
import { buildSchemas } from "./build-schema-aot.ts"

export function main() {
  const result = buildSchemas()
  Console.log(`Prepared ${result.schemas} schema exports`)
}
```

```output
Prepared 1 schema exports
```

The build Effect finds directly exported schema values in the listed modules and writes a self-installing ES module. The module keys resolve relative to `baseUrl`; each loader must load the module named by its key. The TypeScript helper keeps Promise-returning dynamic module loaders at the foreign-code boundary. Build requires FileSystem and Path; this helper provides the Node layers. The output URL is relative to the emitted helper, so a helper in `dist/` writes `dist/generated/parcel-validators.js` regardless of the working directory. A relative `outFile` string instead resolves from the process working directory.

For a project with these source files under `src/` and JavaScript output in `dist/`, compile the build entry separately so the application does not need the generated module yet:

```json
{
  "extends": "./tsconfig.json",
  "files": ["src/build.vibe"],
  "include": []
}
```

Save this as `tsconfig.aot.json`, then run `vibelang -p tsconfig.aot.json` and `node dist/build.js`. The compiler follows the build entry's imports to the helper and schema module. Exporting `main` makes the build entry executable. For the application compilation, set `compilerOptions.rootDirs` to `["src", "dist"]` so its generated-module import resolves in `dist/`. Then compile and run the application.

Decoding is the default operation. Add encoding, guards, and construction only when needed. Nested dependencies do not need direct exports unless a separate target is required for an otherwise undiscovered graph.

```vibe
// fragment
// application.vibe
import "./generated/parcel-validators.js"
import { Parcel } from "./schemas/Parcel.vibe"

function receive(input: unknown) {
  return Schema.decodeUnknownEffect(Parcel)(input)
}
```

Load the generated module before using the schemas and ensure the bundler preserves its installation side effect. AOT code uses static functions and does not import the source generator or construct functions dynamically. Regenerate it whenever schema definitions or the Effect version change; it reads callbacks and symbols from matching runtime ASTs.

## Using lower-level compiler APIs

```vibe
// run: Console.log(generateSource().includes("export function install"))
import * as SchemaAOTCompiler from "effect/schema/SchemaAOTCompiler"

function generateSource() {
  return try(() => SchemaAOTCompiler.compile([
    { ast: Parcel.ast, operations: ["decode"] },
    { ast: SchemaAST.toType(Parcel.ast), operations: ["is", "make"] },
    { ast: SchemaAST.flip(Parcel.ast), operations: ["decode"] }
  ]))
}
```

```output
true
```

The low-level compiler returns source text and leaves file writing and installation to you. Save that source as a module and call its exported `install` with the same ASTs in exactly the same order. The application build API performs the projections and installation wiring automatically.

| Goal | Target AST | Low-level operation |
| --- | --- | --- |
| Decode | original `.ast` | `decode` |
| Encode | `SchemaAST.flip(ast)` | `decode` |
| Validate decoded value | `SchemaAST.toType(ast)` | `is` |
| Construct decoded value | `SchemaAST.toType(ast)` | `make` |

The low-level API names directional parsing `decode`, including the flipped encoding path. The ordered AST array is part of the install contract; changing its order can connect a parser to the wrong schema. `SchemaCompiler.set` is the lower-level registry entry point for a custom compiled implementation.

## Specialized operations, registries, startup behavior, and limits

```vibe
// run: Console.log(JSON.stringify(parseNormally({ reference: "pkg-94", count: 2 }))); try { parseNormally({ reference: "pkg-94", count: 2, extra: true }) } catch (error) { Console.log(error.message) }
function parseNormally(input: unknown) {
  return Schema.decodeUnknownEffect(Parcel)(input, {
    errors: "all", onExcessProperty: "error"
  })
}
```

```output
{"reference":"pkg-94","count":2}
Expected no excess property
  at ["extra"]
```

Supported primitives, objects, arrays, tuples, unions, and template literals can use generated validation. Struct and homogeneous-array decoding have specialized loops, and pure fixed-struct and homogeneous-array construction can use fast paths. Transformations, middleware, and declarations often keep interpreted callbacks while eligible nested schemas compile.

JIT resolves suspended schemas lazily. AOT does not evaluate suspend thunks, so schemas reached only through those thunks remain interpreted unless installed as separate targets. Large or unsupported graphs keep their interpreter paths.

The interpreter, JIT, AOT, and `SchemaCompiler.set` share a registry keyed by exact AST identity; it stores parser functions, not parsed values. A consumer that already captured an entry keeps it after a replacement; later consumers can see the new entry. Install compilers during startup, before parser creation and use, and remember that each operation initializes independently.

Runtime parse options, including concurrency, still apply. Proxy inputs and modified built-in object behavior are outside the optimization contract. Compilers optimize parsing without giving check callbacks permission to pause, fail through arbitrary channels, or perform observable work twice.

[Upstream page](https://effect.website/docs/v4/schema/compilers)
