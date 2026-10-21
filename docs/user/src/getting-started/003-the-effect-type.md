# The Effect Type

`Effect<A, E, R>` is the contract of a computation: its success value, expected errors, and required services. VibeLang infers it for every function, even one that only calculates a value.

## Success, failure, and services

```vibe
// run: Console.log(Effect.provide(convert(30, "EUR"), Layer.succeed(Rates, { lookup: (_currency) => 2 })))
error MissingRate {
  currency: string
}

service Rates {
  lookup(currency: string): number!MissingRate
}

function convert(amount: number, currency: string) {
  return amount * Rates.lookup(currency)
}
// number!MissingRate!Rates
// Output: 60
```

`A` is `number`, `E` is `MissingRate`, and `R` is `Rates`. A service method states its complete contract because it has no body to infer from; using it also adds the service itself to the caller's requirements.

| Channel | Meaning of special types |
| --- | --- |
| `A` | `void` returns no useful value; `never` never succeeds. |
| `E` | `never` has no expected failure; defects and interruption remain possible. |
| `R` | `never` needs no additional services or control operations. |

Errors and services from several calls accumulate as unions. Catching a failure removes it from `E`; providing a service removes it from `R`. Write a full annotation, such as `number!!Rates`, to have the compiler check a promise.

## Write a contract

```vibe
// run: Console.log(Effect.provide(safeRate("EUR"), Layer.succeed(Rates, { lookup: (_currency) => 2 })))
function rateOf(currency: string): number!MissingRate!Rates {
  return Rates.lookup(currency)
}

function safeRate(currency: string): number!!Rates {
  return Effect.orDie(() => Rates.lookup(currency))
}

function anyRate(currency: string): number!MissingRate {
  return Rates.lookup(currency)
}
// Dependencies inferred: number!MissingRate!Rates
// Output: 2
```

`.vibe` code writes `Effect<A, E, R>` as `Result!Error!Dependencies`. An empty slot is `never`: `number!MissingRate!` cannot need a service, and `number!!` neither fails nor needs one. Dependencies left out entirely, as in `anyRate`, are inferred from the body on a function with a body, and are `never` on a declaration without one, such as a service member. A union belongs to its slot: `string!MissingRate | Offline!Rates` fails with either error. Parenthesize a function type, a conditional type, or an object type after a `!`; an object type's `{` would start the function's body.

Hovers show contracts this way, and declaration files keep `Effect.Effect<A, E, R>` for TypeScript callers. `Effect<A, E, R>` is accepted anywhere a type is.

## Check selected channels

```vibe
// run: Console.log(Effect.provide(checkedConversion(30, "EUR"), Layer.succeed(Rates, { lookup: (_currency) => 2 })))
function rate(currency: string): Effect<number, infer E, Rates> {
  return Rates.lookup(currency)
}
// Checks number and Rates; infers MissingRate.

function rateOrDie(currency: string): Effect<number, never, infer R> {
  return Effect.orDie(Rates.lookup(currency))
}
// Checks number and never; infers Rates.

function label(currency: string): Effect<infer A, MissingRate, Rates> {
  return Rates.lookup(currency).toFixed(2)
}
// Infers string; checks MissingRate and Rates.

function checkedConversion(amount: number, currency: string): Effect<number, infer E, infer R> {
  return convert(amount, currency)
}
// Checks number; infers MissingRate and Rates.
// Output: 60
```

In a return annotation on a function with a body, `infer` followed by any name infers that channel from the body; the other slots are promises the compiler checks. The name is documentation only, not in scope in the body or other slots. `infer _` is allowed, as in `Effect<infer _, infer _, infer _>`, which infers all three channels.

Write all three slots. `Effect<number>` here reports EK2003: "Did you mean never or inferred?" It suggests `number!!`, to check for no failures or requirements, or removing the annotation to infer them. A plain return annotation such as `: number` is EK2003 on a function whose body runs an Effect; a function that compiles to plain JavaScript keeps it.

This applies only to return annotations of `.vibe` functions with bodies. In parameter types, aliases, abstract members, and `.ts` files, `Effect<number>` keeps TypeScript's defaults, `Effect<number, never, never>`, and channel inference is unsupported. A callback parameter without an Effect contract, such as `(n: number) => string`, still accepts any contract.

## Description and execution

```vibe
// run: Console.log(Effect.provide(displayedPrice(), Layer.succeed(Rates, { lookup: (_currency) => 2 })))
const conversion = () => convert(30, "EUR")

function displayedPrice() {
  const amount = conversion()
  return amount.toFixed(2)
}
// Output: 60.00
```

Creating `conversion` does no work. Calling it runs the computation within the current program and returns its success value. A call starts no new fiber; runners start execution at the application edge.

An imported Effect value is callable too: `Clock.currentTimeMillis` from `effect` is a description, and `Clock.currentTimeMillis()` reads the clock inside a function. Effect-returning library functions run by the same call rule.

A call runs its result automatically only when the result type is `Effect` itself. `Config`, `Exit`, Context keys, and yieldable error instances carry additional structure and stay data when returned. Call such a value to run it; `new` only constructs it.

## Extract inferred channel types

```vibe
// convert.vibe
// run: Console.log(double(30))
export function double(amount: number) {
  return amount * 2
}
// number!!
// Output: 60
```

```ts
// TypeScript (.ts)
// inspect.ts: imports the emitted VibeLang module
import { Effect } from "effect"
import { double } from "./convert.js"

type Contract = ReturnType<typeof double>
type Value = Effect.Success<Contract>    // number
type Failure = Effect.Error<Contract>    // never
type Services = Effect.Services<Contract> // never
```

TypeScript consumers see exported VibeLang functions as returning ordinary Effects. `Effect.Success`, `Effect.Error`, and `Effect.Services` extract the channels from those inferred return types without executing the program.

## Coming from Effect

`Effect<A, E, R>` is global in `.vibe` files. Written with its type arguments, as `Effect.Effect<A, E, R>`, it draws EK4833, whose quick fix writes `A!E!R` ([TypeScript-style Effect code](012-devtools.md#typescript-style-effect-code)). A local call gives `A`; keep a program as a function value to compose or store it.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/the-effect-type)
