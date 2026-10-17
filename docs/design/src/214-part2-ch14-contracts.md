### 14. Contracts and inference

#### What the compiler infers

An unannotated function or a function with an Effect return annotation gets a contract `Effect<A, E, R>`. An eligible function with a plain return annotation keeps its plain type (D67, Chapter 15):

- **A** is the type of what it returns, inferred and widened as TypeScript infers a function's return type: `return "ok"` gives `string`.
- **E** is every error it `throw`s, plus every error its callees can fail with, plus the expected failures named by the trusted contracts it calls, minus what its `catch` blocks handle.
- **R** is every service and control operation it uses, plus its callees' requirements, minus what it provides or handles itself, plus `Scope` if a resource it acquires without `using` outlives it (Chapter 6).

```ts es
function quoteCart(cartId: string) {
  const cart = Carts.load(cartId)              // E: CartNotFound   R: Carts
  if (cart.items.length === 0) throw new EmptyCart({ cartId })
  try {
    return Pricing.quote(cart)                 // E: PricingDown    R: Pricing
  } catch (e) {                                // e: PricingDown
    return estimate(cart)                      // a pure Effeck helper
  }
}
// Effect<Quote, CartNotFound | EmptyCart, Carts | Pricing>
```

`PricingDown` is caught, so it is not in E. `Pricing` is still in R: catching an error does not remove the need for the service.

Precisely, E is the type of the expected failures in a cause made only of expected failures. A cause that also holds a defect or an interruption can carry `Fail` reasons outside E, because `catch` passes such a cause through untouched (Chapter 17); no Effeck `catch` recovers it, and only cause-level tools see those reasons.

#### Checking a written contract

When you write the contract, the compiler checks the body against it:

```ts es
export function welcome(id: string): Effect<string, UserNotFound | MailError, Users> {
  const user = Users.find(id)
  Mailer.send(user.email, "Welcome")
  return user.name
}
```

```text
error EK2002: welcome needs Mailer, but its contract says R = Users.
  welcome.effeck:3:3   Mailer.send(user.email, "Welcome")
  Provide Mailer inside welcome, or add it: Effect<string, UserNotFound | MailError, Users | Mailer>.
```

| Check | Rule | Diagnostic |
| --- | --- | --- |
| Success | The returned value must be assignable to A | TypeScript's |
| Errors | Every error that can escape must be in E; `Effect<A, never, R>` allows `catch` inside and rejects any error that escapes | EK2001 |
| Requirements | Every requirement that remains must be in R | EK2002 |
| Width | A contract may be wider than the body needs, never narrower | EK2001, EK2002 |

A call to a function with a complete written contract contributes that contract's E and R, even when the body needs less (D73).

The contract has exactly three channels. Whether a function can pause is a separate fact the compiler tracks, its control class (Chapter 15), not part of the contract.

#### Declarations without bodies

Service members, control operations, `declare function`, and fields typed as functions have no body to infer from, so each states its contract without `infer`. A bare return type is an error, never a silent `Effect<A, never, never>`. Service and control methods require all three Effect type arguments; `Effect<A>` is EK2101 there (D93, amending D57):

```ts es
abstract class Rates {
  abstract lookup(currency: string): number
}
```

```text
error EK2101: Rates.lookup has no body to infer from, so its contract must be written in full.
  rates.effeck:2:38   abstract lookup(currency: string): number
  Write Effect<number, E, R>; use Effect<number, never, never> if it cannot fail and needs nothing.
```

A service's value members are not calls, so they take any type: `abstract readonly baseUrl: string`. Callback parameter types are the one exception to full contracts, and there a missing contract means "any contract", not an empty one (Chapter 16).

#### Partial annotations

In the return annotation of a function with a body, write all three channel slots. A slot written `infer` followed by a name comes from the body; every other slot is checked (D57).

```ts es
function find(id: string): Effect<User, infer E, never> {
  return Effect.fail(new UserNotFound({ id }))
}
// Checks A = User and R = never; infers E = UserNotFound.

function load(id: string): Effect<User, never, infer R> {
  return Effect.orDie(() => Users.find(id))
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

The name after `infer` is documentation only and is not in scope in any channel or in the body. `infer _` is allowed. In `Effect<infer Value, infer Value, infer Value>`, the repeated name does not connect the channels.

Omission is an error in this position. Neither `Effect<User>` nor `Effect<User, UserNotFound>` says whether an absent channel should be checked as `never` or inferred. A plain return annotation such as `: User` is EK2003 only when a non-method function's body runs an Effect. An eligible plain helper keeps that annotation and compiles to one plain function (D67). Methods are exempt from EK2003 for plain annotations, including methods implementing JavaScript protocols (D74); their expected type and body determine whether they lower to Effects (D92).

```ts es
function find(id: string): Effect<User> {
  return Users.find(id)
}
```

```text
error EK2003: Effect<User> leaves contract channels unspecified. Did you mean never or inferred?
  Write Effect<User, never, never> to check E and R as never.
  Or write Effect<User, infer E, infer R> to infer E and R from the body.
```

These are two distinct quick fixes. The first can then report EK2001 or EK2002 when the body fails or needs services. The second keeps those channels inferred.

This rule applies only to the return annotation of an Effeck function with a body. Parameter types, type aliases, and `.ts` files keep TypeScript's meaning of `Effect<User>`, including its `never` defaults. Service and control methods instead require a complete contract (D93). They do not gain channel inference from this syntax. A callback parameter without an Effect contract still accepts any contract (D12, Chapter 16).

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

TypeScript cannot compute this on its own: to TypeScript, the generated recursive function is an implicit `any` (TS7022). The compiler computes the fixpoint itself, inside the component only (D52). It checks the component's bodies, then rechecks those bodies alone, with the E and R found so far, until E and R stop growing. Rows only grow and are finite, so the iteration ends, and no function outside the component is checked twice. Mutual recursion is where the recheck matters:

```ts es
class Locked extends TaggedError("Locked")<{ readonly path: string }> {}

function visitDir(path: string) {
  for (const child of Files.list(path)) visitEntry(`${path}/${child}`)
}

function visitEntry(path: string) {
  if (path.endsWith(".lock")) throw new Locked({ path })
  if (Files.stat(path).isDirectory) visitDir(path)
}
// both: Effect<void, IOError | Locked, Files>
```

Checked once in source order, `visitDir` meets `visitEntry` before `visitEntry` has a contract and misses `Locked`; the recheck adds it. The prototype compiler reaches the same contracts by re-checking the whole program (`prototype/esc/FINDINGS.md`, finding 7). The single-pass experiment found this case, in its `isEven` and `isOdd` probe, to be the only one that needs a second check (`prototype/single-pass/RESULTS.md`). A follows TypeScript's rule: when A depends on the function's own result, the contract must be written (EK2102).

EK2102 applies to functions that run Effects. Pure recursion keeps TypeScript's TS7023 when its return type depends on itself; a plain annotation fixes it (D72). The current checker also needs contracts in some cached-type cycles that the fixpoint model alone would solve. A cycle through a callback uses the checker's single type for that call, and generic-call inference does not fill `infer` channels in a lambda (F3's known gaps).
