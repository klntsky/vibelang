### 14. Contracts and inference

#### What the compiler infers

Each function gets a contract `Effect<A, E, R>`:

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
    return estimate(cart)                      // a pure EffectScript helper
  }
}
// Effect<Quote, CartNotFound | EmptyCart, Carts | Pricing>
```

`PricingDown` is caught, so it is not in E. `Pricing` is still in R: catching an error does not remove the need for the service.

Precisely, E is the type of the expected failures in a cause made only of expected failures. A cause that also holds a defect or an interruption can carry `Fail` reasons outside E, because `catch` passes such a cause through untouched (Chapter 17); no EffectScript `catch` recovers it, and only cause-level tools see those reasons.

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
error ES2002: welcome needs Mailer, but its contract says R = Users.
  welcome.es:3:3   Mailer.send(user.email, "Welcome")
  Provide Mailer inside welcome, or add it: Effect<string, UserNotFound | MailError, Users | Mailer>.
```

| Check | Rule | Diagnostic |
| --- | --- | --- |
| Success | The returned value must be assignable to A | TypeScript's |
| Errors | Every error that can escape must be in E; `Effect<A, never, R>` allows `catch` inside and rejects any error that escapes | ES2001 |
| Requirements | Every requirement that remains must be in R | ES2002 |
| Width | A contract may be wider than the body needs, never narrower | ES2001, ES2002 |

The contract has exactly three channels. Whether a function can pause is a separate fact the compiler tracks, its control class (Chapter 15), not part of the contract.

#### Declarations without bodies

Service members, control operations, `declare function`, and fields typed as functions have no body to infer from, so each states its full contract. A bare return type is an error, never a silent `Effect<A, never, never>`:

```ts es
abstract class Rates {
  abstract lookup(currency: string): number
}
```

```text
error ES2101: Rates.lookup has no body to infer from, so its contract must be written in full.
  rates.es:2:38   abstract lookup(currency: string): number
  Write Effect<number, E, R>; use Effect<number, never, never> if it cannot fail and needs nothing.
```

A service's value members are not calls, so they take any type: `abstract readonly baseUrl: string`. Callback parameter types are the one exception to full contracts, and there a missing contract means "any contract", not an empty one (Chapter 16).

#### Partial annotations

You will often want to fix one channel and leave the others to inference, for example to promise that a function never fails without listing its services. The requirement is settled: each channel can be inferred or declared independently, and a channel you leave out is inferred, never assumed to be `never`. The spelling is the owner's call (O7, Chapter 29). Until it is chosen, a contract is written with all three channels or not at all, so a return annotation on a function with a body that is not a full `Effect<A, E, R>`, such as `: number`, is ES2003.

#### Recursion

A recursive function's E and R come from a fixpoint over its strongly connected component of the call graph, so they never need an annotation:

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

Checked once in source order, `visitDir` meets `visitEntry` before `visitEntry` has a contract and misses `Locked`; the recheck adds it. The prototype compiler reaches the same contracts by re-checking the whole program (`prototype/esc/FINDINGS.md`, finding 7). The single-pass experiment found this case, in its `isEven` and `isOdd` probe, to be the only one that needs a second check (`prototype/single-pass/RESULTS.md`). A follows TypeScript's rule: when A depends on the function's own result, the contract must be written (ES2102).
