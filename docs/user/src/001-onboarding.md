# Welcome to Effeck

Effeck is a language for building reliable TypeScript in the AI era, built on [Effect](https://effect.website). It gives Effect a simple, readable syntax that compiles to TypeScript, so your team can ship production-ready systems with first-class access to the TypeScript ecosystem.

Effeck is v0. The language will change in major, breaking ways, and it is not ready for production use.

## Why a language

**Effect makes TypeScript reliable.** A TypeScript signature says what a function returns, but not what can fail or what it depends on. Effect puts all three in one type, `Effect<Success, Error, Requirements>`, and the compiler checks it. Retries, timeouts, concurrency, resource cleanup, and tracing come with it.

**Effect pays for it in syntax.** Effect is a library, so it builds every feature out of what TypeScript already has: generators and `yield*` to run steps in order, `pipe` to compose them, and classes to declare errors and services. The code reads like a separate language written in library calls.

**Effeck makes it a real language.** Each Effect idea gets its own syntax. Calls run in order, `throw` fails with a typed error, `catch` handles it, `service` declares a dependency, and `fork` starts a fiber. The compiler builds the same Effect program you would have written by hand.

## The same program, twice

A service that finds users, a typed error, and a fallback. On the left is Effect; on the right is Effeck.

```ts effect
class UserNotFound extends Schema.TaggedError<UserNotFound>()("UserNotFound", {
  id: Schema.String
}) {}

class Users extends Context.Service<Users, {
  find(id: string): Effect.Effect<User, UserNotFound>
}>()("app/Users") {}

const greet = Effect.fn("greet")(function* (id: string) {
  const users = yield* Users
  const user = yield* users.find(id).pipe(
    Effect.catchTag("UserNotFound", () => Effect.succeed({ name: "guest" }))
  )
  return `Hello, ${user.name}`
})
```

```ts
interface User { id: string; name: string }

error UserNotFound { id: string }

service Users {
  find(id: string): User!UserNotFound
}

function greet(id: string) {
  const name = Users.find(id).name catch "guest"
  return `Hello, ${name}`
}
// string!!Users
```

The comment under `greet` is the type your editor shows: `string!!Users` is Effeck's spelling of `Effect<string, never, Users>`. It returns a `string`, cannot fail, and needs the `Users` service. The compiler infers all three from the body, and TypeScript code that imports `greet` sees the Effect type. The Effeck version is about half the size: 71 tokens against 143.

## TypeScript stays one step away

A new language usually leaves its host's ecosystem behind. Effeck keeps it in two ways:

1. **It compiles to TypeScript.** Effeck's compiler is TypeScript's compiler, extended. It checks `.effeck` and `.ts` files together and emits JavaScript and `.d.ts` files, so npm packages, your `.ts` files, and every Effect library work unchanged, and TypeScript code calls Effeck functions as ordinary Effect functions.
2. **You can opt out to plain TypeScript.** `sync`, `try`, and `callback` take a plain TypeScript function, and a `.ts` file can sit beside any `.effeck` file.

```ts
function slug(title: string) {
  return sync((title) => {
    // Plain TypeScript: nothing in here is an Effect.
    let s = title.trim().toLowerCase()
    s = s.replaceAll(/[^a-z0-9]+/g, "-")
    return s
  }, title)
}
// string!!
```

A hatch's function is self-contained: it reads the `.effeck` file's values only as the arguments passed after it, here `title`.

## Effect's features, in Effeck syntax

Everything Effect does, Effeck does, because the output is Effect. The common features have their own syntax:

| Effect feature | Effeck |
| --- | --- |
| Effect types | `User!NotFound!Users`: returns `User`, fails with `NotFound`, needs `Users` |
| Typed errors | `error NotFound { id: string }`, `throw new NotFound({ id })`, `x catch fallback` |
| Dependency injection | `service Users { ... }`, `provide UsersLive { ... }` |
| Structured concurrency | `fork`, `join`, `await.all [a(), b()]`, `for concurrent(8) (const id of ids)` |
| Retries and timeouts | `work() \|> retry(%, policy) \|> timeout(%, "1 second")` |
| Resource safety | `using conn = connect()`, `finally` |
| Tracing | `withSpan("checkout") { ... }` |
| Schema validation | `schema User { id: string; role: "admin" \| "member" }` |
| Pattern matching | `match (e) { when NotFound: ...; }` |

Streams, configuration, and Effect's HTTP, SQL, CLI, and AI packages need no syntax of their own: call them as you would in TypeScript. The [cheat sheet](effeck-features/115-cheat-sheet.md) puts every Effeck form beside the Effect code it replaces.

## Built for the AI era

When coding agents write a growing share of your code, the hard part moves from writing code to trusting it. Effect's answer is to make failures and dependencies visible to the compiler. Effeck keeps that and makes the code worth reading:

- **The compiler checks what agents write.** Every function's type lists what it can fail with and what it needs, and code that breaks a declared type does not compile, so an agent gets precise feedback to act on.
- **People can review what agents write.** Effeck code is about half the size of the Effect code it replaces, and it reads as plain TypeScript.
- **Agents write less.** Fewer tokens per program means less to generate and less context to read back.

## Try it

```sh
bun create effeck my-app
cd my-app
bun run dev
```

Then open the folder in VS Code or Cursor and install the editor extension that comes with the compiler: `code --install-extension node_modules/effeck/editor/effeck.vsix`. [Installation](getting-started/004-installation.md) covers existing projects.

## Where to go next

- **Coming from Effect:** the [cheat sheet](effeck-features/115-cheat-sheet.md) shows every change, and [Effeck for Effect users](effeck-features/114-effeck-for-effect-users.md) explains the rules.
- **New to Effect:** start with [The Effect Type](getting-started/003-the-effect-type.md), then [Creating Effects](getting-started/005-creating-effects.md) and [Expected Errors](error-management/014-expected-errors.md).
- **The full language:** the [specification](../../design/src/000-intro.md) defines every rule.
