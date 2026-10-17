## Part I. The language by example

Each chapter shows something you do in Effect today and the same thing in Effeck. Effeck code uses Effect's own modules unchanged; what changes is that you stop writing generators, `yield*`, and most combinators.

Three conventions for the snippets. A snippet shown without an enclosing function sits inside one, because a module's top level cannot run a program (Chapter 13). Names such as `User`, `guest`, and `MailerTest` are declared nearby, and helpers such as `toUser` are Effeck functions. Imports of Effect modules outside the prelude (`Schedule`, `Stream`, `Fiber`, and so on) are left out.

### 1. A first look

Here is a small but complete program: two services, two typed errors, a function that uses them, and a `main` that runs it.

#### The program

```ts effect
import { Context, Data, Effect } from "effect"

class UserNotFound extends Data.TaggedError("UserNotFound")<{
  readonly id: string
}> {}

class MailError extends Data.TaggedError("MailError")<{
  readonly reason: string
}> {}

class Users extends Context.Service<Users, {
  find(id: string): Effect.Effect<User, UserNotFound>
}>()("app/Users") {}

class Mailer extends Context.Service<Mailer, {
  send(to: string, body: string): Effect.Effect<void, MailError>
}>()("app/Mailer") {}

export const welcome = Effect.fn("welcome")(function* (id: string) {
  const users = yield* Users
  const mailer = yield* Mailer
  const user = yield* users.find(id)
  yield* mailer.send(user.email, `Welcome, ${user.name}!`)
  return user.name
})
```

```ts es
class UserNotFound extends TaggedError("UserNotFound")<{
  readonly id: string
}> {}

class MailError extends TaggedError("MailError")<{
  readonly reason: string
}> {}

abstract class Users {
  abstract find(id: string): Effect<User, UserNotFound, never>
}

abstract class Mailer {
  abstract send(to: string, body: string): Effect<void, MailError, never>
}

export function welcome(id: string) {
  const user = Users.find(id)
  Mailer.send(user.email, `Welcome, ${user.name}!`)
  return user.name
}
```

Four things changed:

- **`welcome` is a plain function.** No `Effect.fn`, no generator, no `yield*`. Calling `Users.find(id)` runs it and gives you the `User`; if it fails, `welcome` fails with the same error.
- **A service is an abstract class.** `abstract class Users` replaces the `Context.Service` class: no key string and no `Context` import. You call `Users.find(id)` directly instead of first getting the service with `yield* Users` (Chapter 4).
- **The contract is inferred.** `welcome` is `Effect<string, UserNotFound | MailError, Users | Mailer>`: it succeeds with a `string`, can fail with either error, and needs both services. You write the contract only when you want the compiler to check it (Chapter 2).
- **Errors are the same classes.** `TaggedError` is Effect's `Data.TaggedError`, in scope without an import.

#### What is in scope

Every `.effeck` file starts with a small prelude in scope, so the program above has no imports:

| Name | What it is |
| --- | --- |
| `Effect` | Effect's `Effect` module, and the type `Effect<A, E, R>` |
| `Schema`, `Layer` | Effect's modules, unchanged |
| `Scope` | Effect's `Scope` module, and the type `Scope`, which is Effect's `Scope.Scope` |
| `TaggedError` | Effect's `Data.TaggedError` |
| `Control` | The base class of control operations (Chapter 9) |
| `pipe`, `wire`, `fork`, `handle`, `resume`, `resumeWith`, `abandon` | Effeck's built-ins, each introduced where it is first used |

Everything else is imported from `effect` as usual. An import or a local declaration with the same name shadows a prelude name, as it would shadow any global. The prelude is what most Effect files import anyway: `Effect` appears in 77-81% of the Effect files in two large codebases (Smithers, 2,149 files; opencode, 1,031), `Schema` in 39-44%, `Layer` in 34-44%, and no other module in more than 22%. `Scope` is there because contracts name it whenever a resource outlives a function (Chapter 6). `Option` and `Array` stay imports because they would shadow the browser's `Option` and JavaScript's `Array`.

#### Running it

```ts es
// main.effeck
import { NodeRuntime } from "@effect/platform-node"
import { AppLive } from "./app.effeck"
import { welcome } from "./welcome.effeck"

function main() {
  const name = Effect.provide(() => welcome("42"), AppLive)
  Effect.log(`sent a welcome email to ${name}`)
}

NodeRuntime.runMain(() => main())
```

`Effect.provide` supplies the services `welcome` needs from an application Layer (Chapter 4 builds `AppLive`). The `() =>` in `() => welcome("42")` is how you hand a program to Effect without running it yet (Chapter 5). The last line hands `main` to Node's runner, which is the only place a program starts.

#### What the compiler produces

Effeck compiles to TypeScript. This is the output for `welcome`:

```ts gen
export const welcome = Effect.fnUntraced(function* (id: string) {
  const users = yield* Users
  const user = yield* users.find(id)
  const mailer = yield* Mailer
  yield* mailer.send(user.email, `Welcome, ${user.name}!`)
  return user.name
})
```

```ts dts
export declare const welcome: (id: string) =>
  Effect.Effect<string, UserNotFound | MailError, Users | Mailer>
```

TypeScript code that imports `welcome` sees an ordinary function returning an `Effect`. Each service class compiles to an interface for its methods and a `Context.Key` class (Chapter 4), so a TypeScript program can provide `Users` with an ordinary Layer, and Effeck can use services written in TypeScript.
