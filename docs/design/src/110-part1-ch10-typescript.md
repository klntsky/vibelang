### 10. Working with TypeScript

EffectScript uses the TypeScript compiler and shares its types, modules, and Effect. A project can mix `.ts` and `.es` files freely: existing Effect code stays in TypeScript, and new code can be written in either.

#### Using TypeScript Effect code from EffectScript

A TypeScript function that returns an `Effect` is called like any EffectScript function. The call runs it, and its A, E, and R join the caller's contract unchanged.

```ts ts
// users-repo.ts
export const findUser = Effect.fn("findUser")(function* (id: string) {
  const db = yield* Database
  const rows = yield* db.query("select * from users where id = $1", [id])
  if (rows.length === 0) return yield* new UserNotFound({ id })
  return toUser(rows[0])
})
// (id: string) => Effect<User, UserNotFound | DbError, Database>
```

```ts es
// page.es
import { findUser } from "./users-repo.ts"

export function title(id: string) {
  return findUser(id).name.toUpperCase()
}
// Effect<string, UserNotFound | DbError, Database>
```

Generic, overloaded, and aliased TypeScript functions keep their signatures. Services written in TypeScript as `Context.Service` classes or values are used exactly like abstract service classes: `Database.query(…)` works on either. A TypeScript function whose R contains a control operation is the one exception (ES4202, Chapter 9).

#### Generator-style code stays in TypeScript

Existing `Effect.gen` and `Effect.fn` code keeps working in `.ts` files, and EffectScript calls it as above. It cannot be pasted into a `.es` file unchanged, because there `function*` declares a producer (Chapter 8), not a JavaScript generator:

```ts es
const findName = (id: string) => Effect.gen(function* () {
  const users = yield* Users
  const user = yield* users.find(id)
  return user.name
})
```

```text
error ES1002: Effect.gen expects a JavaScript generator, but in EffectScript function* declares a producer.
  Write the body as an ordinary function: remove Effect.gen, function*, and yield*.
    function findName(id: string) { return Users.find(id).name }
```

The same holds for `Option.gen`, `Result.gen`, and any other API that takes a generator. `async` functions and `await` are not part of EffectScript either: a call already waits. Converting a function is mostly deletion, as every chapter of Part I shows, and the editor refactor remains a design target (Chapter 28).

#### Using EffectScript code from TypeScript

To TypeScript, an exported unannotated or Effect-contract function returns an `Effect`, and every exported producer is a function that returns a `Stream`. The compiler writes that into the declaration file:

```ts dts
// page.d.ts
export declare const title: (id: string) => Effect.Effect<string, UserNotFound | DbError, Database>
```

```ts ts
// server.ts
import { Effect } from "effect"
import { title } from "./page.js"

const program = title("42").pipe(Effect.provide(DatabaseLive))   // nothing has run yet
```

Calling an EffectScript function from TypeScript builds a program and runs nothing, like calling an `Effect.fn` function. That holds even for functions that compile to plain JavaScript (Chapter 15): TypeScript sees an unannotated `add` as `(a: number, b: number) => Effect<number>`, and EffectScript callers in the same compilation use the fast path. An eligible helper with a plain annotation, such as `: number`, instead keeps a plain JavaScript signature and runs when TypeScript calls it (D67). TypeScript imports `./page.js`, which the compiler resolves to `page.es` the way it resolves `.js` to `.ts`.

#### Code that is not Effect

A function that does not return an `Effect` is foreign code. Nothing in its type says whether it throws, what it reads, or whether it touches the outside world, so each kind of callee has a rule:

```ts es
import { slugify } from "./slug.ts"          // carries /** @effectscript trusted */

function publish(draft: Draft) {
  const meta = JSON.parse(draft.frontMatter)  // a built-in: SyntaxError joins E
  const slug = slugify(meta.title)            // trusted: its @throws adds InvalidSlug
  const words = draft.body.split(/\s+/).length
  return Articles.save({ slug, words, ...meta })
}
// Effect<Article, SyntaxError | InvalidSlug | SaveError, Articles>
```

| Callee | Rule | Diagnostic |
| --- | --- | --- |
| A function returning `Effect`, from EffectScript or TypeScript | Calls run it | |
| A JavaScript built-in: `String`, `Array`, `Math`, `JSON`, `Number`, `Object`, `Map`, `Set`, `RegExp`, deterministic `Date` parsing | Ships with a trusted contract: a control class, and which exceptions are expected failures (`JSON.parse` fails with `SyntaxError`) | |
| An export of `effect` or an `@effect/*` package that does not return an `Effect`, and the functions and values it returns, such as a `Brand` constructor or the function `Match.exhaustive` returns | Trusted: `sync`, and its exceptions are defects, as inside `Effect.gen` today | |
| A TypeScript function tagged `/** @effectscript trusted */` | Trusted: `sync`; each `@throws {T}` names an expected failure; any other exception is a defect | |
| A platform API: `node:*` modules and browser globals such as `fetch`, `setTimeout`, `process`, `console`, `localStorage` | Not callable from `.es` files; the error names the Effect module to use (Chapter 11) | ES4402 |
| A clock or randomness read: `Date.now()`, `new Date()`, `Math.random()`, `crypto.randomUUID()` | Use `Clock`, `DateTime`, and `Random`, so tests control them | ES4403 |
| Any other plain TypeScript or JavaScript function | Needs an explicit boundary or a trusted contract | ES4401 |

A trusted contract is declared once, on the TypeScript declaration:

```ts ts
// slug.ts
/**
 * @effectscript trusted
 * @throws {InvalidSlug} when the title has no letters or digits
 */
export function slugify(title: string): string { … }
```

The compiler trusts the tag; it cannot check a TypeScript body. `@throws {InvalidSlug}` makes `InvalidSlug` an expected failure that joins the caller's E; any other exception from `slugify` is a defect. Each `@throws` names a class exported by a module the `.es` caller imports; otherwise the call is ES4401 (D101). The generated code tests `instanceof InvalidSlug`. The tag cannot be put on a function that returns a Promise or an `Effect`. To trust a function from a package you cannot edit, re-export it from a `.ts` file you own, with the tag.

ES4401 applies where a fiber runs the code. Module initialization and its plain callbacks passed to TypeScript, such as `describe` bodies and route handlers, run ordinary JavaScript calls (D100). An exception at module top level fails the import. Trusted failures stay defects inside plain callbacks, while they make free lambdas and named EffectScript functions use the Effect form (D101).

Only built-ins that parse outside text declare expected failures: `JSON.parse` declares `SyntaxError`, and `decodeURI` and `decodeURIComponent` declare `URIError`. Other built-in exceptions are defects (D101).

An explicit boundary says what an exception means at the call. These are Effect's own constructors, unchanged:

```ts es
function parseLegacy(text: string) {
  return Effect.try({
    try: () => legacyParse(text),                 // plain TypeScript, allowed inside the boundary
    catch: (cause) => new LegacyFormatError({ cause })
  })
}
// Effect<LegacyDoc, LegacyFormatError, never>
```

`Effect.try` and `Effect.tryPromise` turn exceptions and rejections into a typed error; `Effect.sync` and `Effect.promise` make them defects; `Effect.callback` adapts a callback API (Chapter 6). Inside their callbacks, plain TypeScript calls are allowed, because the boundary says what an exception means. Platform APIs (ES4402) and clock reads (ES4403) are rejected there too: they belong behind an Effect module.

An exception that no contract lists is a defect, and an EffectScript `catch` does not see it, unlike a JavaScript `catch` inside `Effect.gen` (Chapter 3). That includes the exceptions of Effect's "cannot fail here" functions, such as `Option.getOrThrow` and `Schema.decodeUnknownSync`, which `effect`'s trusted contracts classify as defects.

What a boundary cannot do is set by JavaScript, not by EffectScript. It cannot discover a dependency the function reaches through a global or a module variable, it cannot cancel synchronous JavaScript once it has started, and it cannot let a synchronous callback pause. Effect code today has the same limits.

This is a policy change from the original direction, which adapted every plain call automatically, with unknown errors and unknown requirements. The premise is kept: foreign code is untrusted until something vouches for it. What changes is where the vouching happens: once, explicitly, at a boundary or on a declaration, so an unprovidable `unknown` never enters a requirement and an `unknown` error never spreads to every caller. Pure helpers written in `.es` get inferred contracts and compile to plain JavaScript (Chapter 15), so most project code never needs a boundary at all.

#### Promises

EffectScript has no `await`: a call already waits for the callee. A Promise from a TypeScript library is waited on with `Effect.tryPromise`, which says what a rejection means and passes an `AbortSignal` so interruption can cancel the request. This code is identical to the Effect code you write today:

```ts es
import { createCharge } from "./payments-sdk.ts"   // (amount: number, signal: AbortSignal) => Promise<Charge>

function charge(cartId: string, amount: number) {
  return Effect.tryPromise({
    try: (signal) => createCharge(amount, signal),
    catch: (cause) => new ChargeFailed({ cartId, cause })
  })
}
// Effect<Charge, ChargeFailed, never>
```

HTTP itself goes through Effect's `HttpClient`, not `fetch` (Chapter 11).

#### Callbacks into plain JavaScript

A JavaScript function such as `Array.prototype.map` calls its callback synchronously, so the callback cannot pause. Callbacks that only compute are fine, and so is an EffectScript function passed by name if it compiles to plain JavaScript. A callback that calls a service is rejected, with the fix in the message:

```ts es
const trimmed = ids.map((id) => id.trim())         // fine
const users = ids.map((id) => Users.find(id))      // rejected
```

```text
error ES4301: this callback can pause (Users.find), but Array.prototype.map calls it
  synchronously. Use Effect.forEach(ids, (id) => Users.find(id)) or a for...of loop.
```

Effect's own combinators, such as `Effect.forEach`, expect callbacks that return Effects, so EffectScript callbacks work there. Chapter 16 gives the rules for higher-order functions.

#### Effect 3

EffectScript targets Effect 4 only. Effect 3 values do not run on the Effect 4 runtime. Code that must call an Effect 3 library wraps it as foreign code, for example with `Effect.tryPromise` around `Effect3.runPromise`.
