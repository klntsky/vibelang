# Welcome to Effeck

Effeck is TypeScript in which every function is an [Effect](https://effect.website). You write ordinary functions; the compiler turns them into Effect programs, with typed errors, dependency injection, and safe concurrency, and no `yield*`, `pipe`, or generators.

## The same program, twice

A service that finds users, a typed error, and a fallback. On the left is Effect today; on the right is Effeck.

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

Both compile to the same kind of program. The comment under `greet` is the type your editor shows on hover: it returns a `string`, cannot fail, and needs the `Users` service. The compiler infers all three from the body.

## What you get

- **Plain code.** A call runs; statements run in order; `throw` fails; `try`, `catch`, and `finally` work as you expect. No generators, `yield*`, or `pipe`.
- **Every function's type tells the truth.** `User!UserNotFound!Db` reads as: returns a `User`, can fail with `UserNotFound`, needs `Db`. The compiler infers it and checks it everywhere.
- **All of Effect.** Effeck compiles to ordinary Effect 4 JavaScript. Effect's libraries, your `.ts` files, and any npm package work unchanged, and TypeScript code can call Effeck functions.
- **Less to write and read.** Effect's ideas get first-class syntax: `error`, `service`, `provide`, `fork` and `join`, `match`, `await.race`, and the `|>` pipe. The Effeck half of the example above is about half the size of the Effect half.

## Why use it

Use Effeck when you want what Effect gives you, errors in the type, services injected and swappable in tests, structured concurrency, resources that always close, tracing, without writing Effect's plumbing by hand.

- **If you use Effect today**, Effeck removes the ceremony and keeps the runtime, the libraries, and the mental model.
- **If you write TypeScript and Effect looked like too much**, Effeck lets you start with plain functions and add typed errors and services as you need them.
- **If AI agents write your code**, Effeck is shorter to generate, and its types reject unhandled failures and missing dependencies at compile time.

Effeck is v0: expect breaking changes, and do not use it in production yet.

## Try it

```sh
bun create effeck my-app
cd my-app
bun run dev
```

Then open the folder in VS Code or Cursor and install the editor extension that comes with the compiler: `code --install-extension node_modules/effeck/editor/effeck.vsix`. [Installation](getting-started/004-installation.md) covers existing projects.

## Where to go next

- **Coming from Effect:** the [cheat sheet](effeck-features/115-cheat-sheet.md) puts every Effeck form beside the Effect code it replaces.
- **New to Effect:** start with [The Effect Type](getting-started/003-the-effect-type.md), then [Creating Effects](getting-started/005-creating-effects.md) and [Expected Errors](error-management/014-expected-errors.md).
- **The full language:** [Effeck for Effect users](effeck-features/114-effeck-for-effect-users.md) explains the rules, and the specification defines them.
