# Release notes

VibeLang is v0, so a new version can change the language in breaking ways. Read
the notes for every version between yours and the new one before you update, and
update `vibelang`, `vibelang-runtime`, and `vibelang-vite` together.

## 0.0.2

Eleven packages: `vibelang`, six native compilers
(`vibelang-{darwin,linux,win32}-{arm64,x64}`), `vibelang-runtime`,
`vibelang-vite`, `vibelang-unplugin`, and `create-vibelang`. Windows and
`vibelang-unplugin` are new in this release.

### Changes that can break code that compiled on 0.0.1

Each change is a compile error that names the problem. None changes what
compiled code does without telling you.

#### Platform globals through `globalThis` are EK4402

`process.env` was already EK4402. `globalThis.process.env`,
`globalThis.fetch`, `globalThis.crypto.subtle`, `globalThis.location`,
`globalThis.navigator`, and the other platform globals now are too.

```vibe
// expect-error: EK4402
function home() {
  return globalThis.process.env.HOME
}
```

Read configuration with `Config` from Effect:

```vibe
function homeDir() {
  return Config.String("HOME")()
}
```

Otherwise, read the global in a TypeScript module the project trusts, such as
`platformEnv.ts`:

```ts
// TypeScript (.ts)
/** @vibelang trusted */
export const processEnv = () => globalThis.process?.env
```

#### A call typed as a union of Effects runs where it stands

A call to a function that returns `Effect<A> | Effect<never, E>` now runs the
program, as `yield* decide(flag)` does in TypeScript. In 0.0.1 it held the
program without saying so.

```vibe
error Rejected

function decide(flag: boolean): Effect.Effect<number> | Effect.Effect<never, Rejected> {
  return flag ? Effect.succeed(1) : Effect.fail(new Rejected())
}
```

Code that worked around 0.0.1 by holding the program and then calling it no
longer compiles:

```vibe
// expect-error: TS2349
function total(flag: boolean) {
  const held = decide(flag)
  return held() + 1
}
```

Delete the workaround and use the value:

```vibe
function total(flag: boolean) {
  const value = decide(flag)
  return value + 1
}
// number!Rejected!
```

A function annotated with a union of Effects whose branches return decided
Exits, such as `cond ? Exit.fail(e) : Exit.void`, runs the chosen Exit.

#### `Request` is the web-standard class

Web-standard globals keep their names, as JavaScript's built-ins do: `Request`,
`Response`, `Headers`, `URL`, `URLSearchParams`, `TextEncoder`, `TextDecoder`,
`AbortController`, `AbortSignal`, `Blob`, `FormData`, `structuredClone`, and
`fetch`. Of Effect's root names only `Request` collides, so `Request.tagged`
without an import is now TS2339 or TS2702. Import Effect's module:

```vibe
import { Request } from "effect"

interface Lookup extends Request.Request<string> {
  _tag: "Lookup"
  id: string
}

const Lookup = Request.tagged<Lookup>("Lookup")
```

#### A `const` with a declared Effect type holds the program

`const p: Effect.Effect<A, E> = make()` keeps the program that `make()` returns
cold instead of running it, at any scope. When the call returns a program that
produces a program, as `Effect.cached(...)` does, the annotation must name the
outer program, or the compiler reports TS2322 or TS2375. Annotate the outer type,
`Effect.Effect<Effect.Effect<A, E>>`, or drop the annotation to run the call.

#### A throw expression can't be an operator's left operand (EK1010)

`throw cond ? a : b` in expression position threw `cond` and discarded the rest.
It is now EK1010. Write `throw (cond ? a : b)`. A statement that starts with
`throw` is unchanged.

#### A curried pipe stage after a call keeps the call

`x |> f(args)(%)` becomes `x.pipe(f(args))` only when `x` calls nothing. When a
call builds the left side, bind it first: `const s = Schema.NullOr(Schema.String)`,
then `s |> f(args)(%)`.

### Fixes

- An Effect subtype returned at an Effect boundary runs. A value that the
  contract's written success type accepts is returned as data; for example,
  `keep(command): ChildProcess.Command!never!Scope` returns the command.
- A discarded structured Effect call is diagnosed (EK4301). A program cast and
  piped with `|>` is passed by name.
- Callback cleanup waits for async and Effect finalizers on interrupt. Copied
  declaration types import prelude names.
- A `function*` annotated `Generator<...>` stays a JavaScript generator.
- Function-valued service implementations and contract-annotated arrows keep
  their programs.
- Nested Effect function declarations hoist, so a call above the declaration
  works.
- The checker no longer overflows the stack on a shorthand discriminant property
  under a union context.
- By-name tail programs, conditional by-name arms, and optional program fields
  run as written.
- Diagnostics: inline boundary callbacks, `Effect.fn` span advice, non-adjustable
  EK option policies, and `.d.ts` JSDoc that matches tsc.

### Known issues

- A throwing local arrow with no contract, inside a function with a plain return
  annotation, reports EK2003 on the outer annotation. Annotate the local arrow,
  for example `(value: number): void => { ... }`.
- EK4802 can warn on a local annotated `Effect<Effect<...>>` that returns its
  program unrun. The emitted code is correct.
- Programs that by-name lowering wraps spend extra Effect operations.
- A recursive `Schema.Class` whose `Schema.suspend` callback writes an Effect
  contract, `(): Schema.Codec<Crate, CrateWire>!! => Crate`, reports TS2506 and
  TS2740. Annotate the plain return type instead,
  `(): Schema.Codec<Crate, CrateWire> => Crate`. Both compile to the same
  JavaScript.
