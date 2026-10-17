### 2. Functions and calls

#### Every function is an Effect

There is one kind of function. Some can fail, some need services, some pause; the compiler works out which from the body, and records it in the function's contract `Effect<A, E, R>`.

```ts es
function add(a: number, b: number) {
  return a + b
}
// Effect<number, never, never>

function greeting(id: string) {
  const user = Users.find(id)
  return `Hello, ${user.name}`
}
// Effect<string, UserNotFound, Users>
```

`add` performs no Effect operation and cannot fail, so the compiler emits a plain JavaScript function for it: calling `add(1, 2)` costs what any JavaScript call costs (Chapter 15). `greeting` can fail and needs `Users`, so it compiles to an Effect.

#### Calling runs the function

A call runs the function and gives you its success value. If the callee fails, the caller fails with the same error, unless it catches it (Chapter 3). There is no `yield*` and no `await`.

```ts effect
const userName = Effect.fn("userName")(function* (id: string) {
  const users = yield* Users
  const user = yield* users.find(id)
  const name = user.name.trim()
  yield* Effect.log(`found ${name}`)
  return name
})
```

```ts es
function userName(id: string) {
  const name = Users.find(id).name.trim()
  Effect.log(`found ${name}`)
  return name
}
```

Where Effect code sequences with `Effect.map`, `flatMap`, `andThen`, or `tap`, Effeck uses the value and then the next statement.

#### Control flow is ordinary JavaScript

Loops, conditions, early returns, and local variables work as they do in any function, with effectful calls anywhere inside them.

```ts effect
const notifyAll = Effect.fn("notifyAll")(function* (ids: ReadonlyArray<string>) {
  const users = yield* Users
  let sent = 0
  for (const id of ids) {
    const user = yield* users.find(id)
    if (!user.subscribed) continue
    yield* welcome(id)
    sent++
  }
  return sent
})
```

```ts es
function notifyAll(ids: ReadonlyArray<string>) {
  let sent = 0
  for (const id of ids) {
    const user = Users.find(id)
    if (!user.subscribed) continue
    welcome(id)
    sent++
  }
  return sent
}
// Effect<number, UserNotFound | MailError, Users | Mailer>
```

The loop runs one `welcome` at a time, in order. Running several at once is always explicit (Chapter 7).

#### Tracing

`Effect.fn("welcome")` gives every call a span. Effeck functions compile to untraced frames by default, because a traced frame costs 10 to 20 times more (Chapter 24). Add a span where you want one, with Effect's own `withSpan`:

```ts es
function welcomeTraced(id: string) {
  return Effect.withSpan(() => welcome(id), "welcome")
}
```

#### Writing the contract yourself

Most functions leave the contract to inference. Write it when you want the compiler to check a promise, typically on exported functions. The annotation is the whole computation, the way `Promise<string>` is on an `async` function: the `return` is checked against `string`.

```ts es
export function greeting(id: string): Effect<string, UserNotFound, Users> {
  const user = Users.find(id)
  return `Hello, ${user.name}`
}
```

A contract may be wider than what the body does, never narrower:

```ts es
export function greeting(id: string): Effect<string, never, Users> {
  return `Hello, ${Users.find(id).name}`
}
```

```text
error EK2001: UserNotFound can escape greeting, but its contract says E = never.
  greeting.effeck:2:22   return `Hello, ${Users.find(id).name}`
                                      ^^^^^^^^^^^^^ Users.find can fail with UserNotFound
  Catch it inside greeting, or change the contract to Effect<string, UserNotFound, Users>.
```

To check selected channels, write `infer` followed by a name in each channel the compiler should infer (D57):

```ts es
function greeting(id: string): Effect<string, infer E, infer R> {
  return `Hello, ${Users.find(id).name}`
}
// Effect<string, UserNotFound, Users>
```

Here A is checked, and E and R come from the body. `Effect<string, never, infer R>` instead promises no expected failures while inferring services. All three slots must be present: `Effect<string>` in a return annotation on a function with a body is EK2003, with fixes for explicit `never` or explicit inference ([Chapter 14](#14-contracts-and-inference)). The name after `infer` is documentation only; it introduces no type variable.

#### Calling TypeScript

Effeck calls TypeScript functions directly. What a call does depends on what the function returns:

```ts es
import { loadAvatar } from "./avatars.ts"    // (id: string) => Effect<Avatar, AvatarError, Storage>
import { formatName } from "./format.ts"     // (user: User) => string, declared trusted

function profileCard(id: string) {
  const user = Users.find(id)
  return {
    title: formatName(user),                 // a trusted plain function: an ordinary call
    avatar: loadAvatar(user.id)              // returns an Effect, so the call runs it
  }
}
// Effect<{ title: string; avatar: Avatar }, UserNotFound | AvatarError, Users | Storage>
```

A TypeScript function that returns an `Effect` is treated exactly like an Effeck function: calling it runs it. A TypeScript function that returns anything else is foreign code, and nothing says what its exceptions mean, so the compiler asks you to say it once. `formatName` carries a trusted contract, a one-line tag on its declaration (Chapter 10). Without one, the call is an error that offers the fixes:

```text
error EK4401: formatName is a plain TypeScript function, and nothing says what its exceptions mean.
  profile-card.effeck:7:12   title: formatName(user),
  Declare it trusted once:  add /** @effeck trusted */ to formatName in format.ts
  Or bound this call:       Effect.try({ try: () => formatName(user), catch: (cause) => new FormatError({ cause }) })
  Or move it into a .effeck file, where its contract is inferred.
```

Effect's own modules (`Option`, `Array`, `Schedule`, and the rest) and JavaScript's built-ins (`String`, `Math`, `JSON`, and the rest) need none of this: their contracts ship with the compiler. Node.js and browser APIs are replaced by Effect's modules (Chapter 11).
