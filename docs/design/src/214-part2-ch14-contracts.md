### 14. Contracts and inference

#### What the compiler infers

An eligible function with a plain return annotation keeps its plain type (D67, Chapter 15). An unannotated function, or one with an Effect return annotation, gets a contract `Effect<A, E, R>`:

- **A** is its return type, inferred and widened as TypeScript infers one: `return "ok"` gives `string`.
- **E** is every error it `throw`s, plus every error its callees can fail with, plus the expected failures named by the trusted contracts it calls, minus what its `catch` blocks handle.
- **R** is every service and control operation it uses, plus its callees' requirements, minus what it provides or handles itself, plus `Scope` if a resource it acquires without `using` outlives it (Chapter 6).

```ts es
function quoteCart(cartId: string) {
  const cart = Carts.load(cartId)              // E: CartNotFound   R: Carts
  if (cart.items.length === 0) throw new EmptyCart({ cartId })
  try {
    return Pricing.quote(cart)                 // E: PricingDown    R: Pricing
  } catch (e) {                                // e: PricingDown
    return estimate(cart)                      // a pure VibeLang helper
  }
}
// Effect<Quote, CartNotFound | EmptyCart, Carts | Pricing>
```

`PricingDown` is caught, so it is not in E, but `Pricing` stays in R: catching an error does not remove the need for the service.

Precisely, E is the type of the expected failures in a cause made only of expected failures. A cause that also holds a defect or an interruption can carry `Fail` reasons outside E, because `catch` passes such a cause through untouched (Chapter 17); no VibeLang `catch` recovers it, and only cause-level tools see those reasons.

#### Writing contracts

`.vibe` code writes a contract as TypeScript writes an Effect type, `Effect<A, E, R>`, wherever a type can appear: return annotations, service members, parameters, and aliases (D308). `Effect` is the prelude's name for effect's `Effect.Effect`, and a type argument left out means what it means in TypeScript:

| Written | Means |
| --- | --- |
| `Effect<User, NotFound, Db>` | Succeeds with `User`, can fail with `NotFound`, needs `Db` |
| `Effect<User, NotFound>` | `Effect<User, NotFound, never>`: needs nothing |
| `Effect<User, never, Db>` | Cannot fail, needs `Db` |
| `Effect<User>` | `Effect<User, never, never>`: cannot fail and needs nothing |
| `Effect<User, NotFound, infer R>` on a function with a body | R comes from the body (D57) |

A contract in a `.vibe` file means exactly what the same type means in a `.ts` file. `Effect.Effect<A, E, R>` is the same type; EK4833 suggests the prelude's shorter name. Hovers and messages about `.vibe` code write contracts as `Effect<…>` and leave out a trailing `never`, as TypeScript's defaults allow: `Effect<string, never, never>` shows as `Effect<string>`. Declaration files write the same types, through the runtime's `Effect`, an alias of effect's `Effect.Effect` with the same defaults, so TypeScript callers see Effect's own type.

D127's shorthand, `Result!Error!Dependencies`, is retired (D308). The parser still reads it, so a file written with it checks, and each one is EK1014. Its quick fix writes the Effect type the shorthand stood for, and `vibelang -p tsconfig.json --fix` rewrites a whole project:

| Retired | Rewritten |
| --- | --- |
| `User!NotFound!Db` | `Effect<User, NotFound, Db>` |
| `User!NotFound!` | `Effect<User, NotFound>` |
| `User!!Db` | `Effect<User, never, Db>` |
| `User!!` | `Effect<User>` |
| `User!NotFound`, the return annotation of a function with a body | `Effect<User, NotFound, infer R>` |
| `User!NotFound` anywhere else | `Effect<User, NotFound>` |

#### Checking a written contract

The compiler checks a body against its written contract:

```ts es
export function welcome(id: string): Effect<string, UserNotFound | MailError, Users> {
  const user = Users.find(id)
  Mailer.send(user.email, "Welcome")
  return user.name
}
```

```text
error EK2002: welcome needs Mailer, but its contract says R = Users.
  welcome.vibe:3:3   Mailer.send(user.email, "Welcome")
  Provide Mailer inside welcome, or add it: Effect<string, UserNotFound | MailError, Users | Mailer>.
```

| Check | Rule | Diagnostic |
| --- | --- | --- |
| Success | The returned value must be assignable to A | TypeScript's |
| Errors | Every error that can escape must be in E; `Effect<A, never, R>` allows `catch` inside and rejects any error that escapes | EK2001 |
| Requirements | Every requirement that remains must be in R | EK2002 |
| Width | A contract may be wider than the body needs, never narrower | EK2001, EK2002 |

A call to a function with a complete written contract contributes that contract's E and R, even when the body needs less (D73).

The contract has exactly three channels. Whether a function can pause is a separate fact the compiler tracks, its control class (Chapter 15).

#### Declarations without bodies

Service members, control operations, `declare function`, and fields typed as functions have no body to infer from, so each states its contract as an Effect type, and `infer` has nothing to infer there. A bare return type on a service or control method is EK2101, never a silent `Effect<A>` (D93, D308). A type argument left out is `never`, as everywhere: `find(id: string): Effect<User, NotFound>` can fail with `NotFound` and needs nothing.

```ts es
service Rates {
  lookup(currency: string): number
}
```

```text
error EK2101: Rates.lookup has no body to infer from, so its contract must be written as an Effect type.
  rates.vibe:2:38   abstract lookup(currency: string): number
  Write Effect<number, Errors, Dependencies>; use Effect<number> if it cannot fail and needs nothing.
```

A service's value members are not calls, so they take any type: `abstract readonly baseUrl: string`. Callback parameter types are the one exception to written contracts: there a missing contract means "any contract", not an empty one (Chapter 16).

#### Partial annotations

In the return annotation of a function with a body, a channel written `infer Name` comes from the body; every other channel is checked, and a channel left out is `never`, as TypeScript's defaults make it (D57, D308).

```ts es
function find(id: string): Effect<User, infer E> {
  return Effect.fail(new UserNotFound({ id }))
}
// Checks A = User and R = never; infers E = UserNotFound.

function load(id: string): Effect<User, never, infer R> {
  return Effect.orDie(Users.find(id))
}
// Checks A = User and E = never; infers R = Users.

function plan(id: string): Effect<infer A, UserNotFound, Users> {
  return Users.find(id).name
}
// Infers A = string; checks E = UserNotFound and R = Users.

function greeting(id: string): Effect<string, infer E, infer R> {
  return `Hello, ${Users.find(id).name}`
}
// Checks A = string; infers E = UserNotFound and R = Users.

function inferred(id: string): Effect<infer _, infer _, infer _> {
  return Users.find(id)
}
// Effect<User, UserNotFound, Users>, as with no annotation.
```

The name after `infer` is documentation only, not in scope in any channel or in the body. `infer _` is allowed. In `Effect<infer Value, infer Value, infer Value>`, the repeated name does not connect the channels.

A channel left out is checked, never inferred: `Effect<User>` promises that the function cannot fail and needs nothing, so a body that returns `Users.find(id)` under it is EK2001 for `UserNotFound` and EK2002 for `Users`. To check only A, write `Effect<User, infer E, infer R>`. A plain return annotation such as `: User` is EK2003 when a non-method function's body runs an Effect, and when any function's throw can escape, since a plain function never fails (D282); an eligible plain helper keeps that annotation and compiles to one plain function (D67). Methods, including those implementing JavaScript protocols, are exempt from EK2003 for running Effects under plain annotations (D74); their expected type and body determine whether they lower to Effects (D92).

`infer` applies only in the return annotation of a VibeLang function with a body. Everywhere else, in parameter types, type aliases, service members, and `.ts` files, an Effect type means what TypeScript means by it, `never` defaults included. A callback parameter without an Effect contract still accepts any contract (D12, Chapter 16).

#### Recursion

A recursive Effect function's E and R come from a fixpoint over its strongly connected component of the call graph. Cached catch-variable or return types can require a written contract before those channels are known (EK2102, D71):

```ts es
function totalSize(path: string) {
  const entry = Files.stat(path)
  if (!entry.isDirectory) return entry.size
  let total = 0
  for (const child of Files.list(path)) total += totalSize(`${path}/${child}`)
  return total
}
// Effect<number, IOError, Files>
```

TypeScript cannot compute this alone: to it, the generated recursive function is an implicit `any` (TS7022). The compiler computes the fixpoint itself, inside the component only (D52): it checks the component's bodies, then rechecks only those bodies with the E and R found so far, until E and R stop growing. Rows only grow and are finite, so the iteration ends, and no function outside the component is checked twice. The recheck matters for mutual recursion:

```ts es
error Locked { path: string }

function visitDir(path: string) {
  for (const child of Files.list(path)) visitEntry(`${path}/${child}`)
}

function visitEntry(path: string) {
  if (path.endsWith(".lock")) throw new Locked({ path })
  if (Files.stat(path).isDirectory) visitDir(path)
}
// both: Effect<void, IOError | Locked, Files>
```

Checked once in source order, `visitDir` meets `visitEntry` before `visitEntry` has a contract and misses `Locked`; the recheck adds it. The prototype compiler reaches the same contracts by re-checking the whole program (`prototype/esc/FINDINGS.md`, finding 7). The single-pass experiment found this case, in its `isEven` and `isOdd` probe, the only one needing a second check (`prototype/single-pass/RESULTS.md`). A follows TypeScript's rule: when A depends on the function's own result, the contract must be written (EK2102).

A contract may write A and leave E or R to the fixpoint, as `Effect<number, Neg, infer R>` leaves R (D57). A call of the function in its own body that stands for its value, such as an operand, a condition, a receiver, an index or an unannotated initializer, has type A while the fixpoint runs:

```ts es
function fact(n: number): Effect<number, Neg, infer R> {
  if (n < 0) throw new Neg({ n })
  if (n <= 1) return 1
  return n * fact(n - 1)
}
// Effect<number, Neg>
```

Where a call of itself could hold the program instead, as an argument or a logical operand can, or where the function is generic, TypeScript needs the contract before the fixpoint has its channels, so the contract must write every channel; EK2102's note gives it, such as `Effect<number, Neg, Audit>`.

EK2102 applies to functions that run Effects. Pure recursion keeps TypeScript's TS7023 when its return type depends on itself; a plain annotation fixes it (D72). The current checker also needs contracts in some cached-type cycles that the fixpoint model alone would solve. A cycle through a callback uses the checker's single type for that call, and generic-call inference does not fill `infer` channels in a lambda (F3's known gaps).
