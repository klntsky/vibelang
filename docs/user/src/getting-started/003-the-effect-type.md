# The Effect Type

`Effect<A, E, R>` is the contract of a computation: its success value, expected errors, and required services. EffectScript infers this contract for every function, including small functions that only calculate a value.

## Success, failure, and services

```ts
class MissingRate extends TaggedError("MissingRate")<{
  readonly currency: string
}> {}

abstract class Rates {
  abstract lookup(currency: string): Effect<number, MissingRate, never>
}

function convert(amount: number, currency: string) {
  return amount * Rates.lookup(currency)
}
// Effect<number, MissingRate, Rates>
```

`A` is `number`, `E` is `MissingRate`, and `R` is `Rates`. A service method states its complete contract because it has no body to infer from. Using that method also adds the service itself to the caller's requirements.

| Channel | Meaning of special types |
| --- | --- |
| `A` | `void` returns no useful value; `never` never succeeds. |
| `E` | `never` has no expected failure; defects and interruption remain possible. |
| `R` | `never` needs no additional services or control operations. |

Errors and services from several calls accumulate as unions. Catching a failure removes that handled failure from `E`; providing a service removes that requirement from `R`. Write a full annotation when you want the compiler to check a promise, such as `Effect<number, never, Rates>`.

## Check selected channels

```ts
// compiler-gap: D57 Return contracts must check written channels and infer channels written infer Name.
function rate(currency: string): Effect<number, infer E, Rates> {
  return Rates.lookup(currency)
}
// Checks number and Rates; infers MissingRate.

function rateOrDie(currency: string): Effect<number, never, infer R> {
  return Effect.orDie(() => Rates.lookup(currency))
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
```

In a return annotation on a function with a body, `infer` followed by any name infers that channel from the body. Other slots are promises the compiler checks. The name is documentation only and is not in scope in the body or other slots. `infer _` is allowed, including `Effect<infer _, infer _, infer _>` to infer all three channels.

Write all three slots. `Effect<number>` in this position reports ES2003: "Did you mean never or inferred?" The quick fixes offer `Effect<number, never, never>` to check for no failures or requirements, and `Effect<number, infer E, infer R>` to infer them. A bare return annotation such as `: number` is also ES2003.

This applies only to function return annotations with bodies in `.es`. In parameter types, aliases, abstract members, and `.ts` files, `Effect<number>` keeps TypeScript's defaults: `Effect<number, never, never>`. Those positions do not support this channel inference. A callback parameter without an Effect contract, such as `(n: number) => string`, still accepts any contract.

## Description and execution

```ts
const conversion = () => convert(30, "EUR")

function displayedPrice() {
  const amount = conversion()
  return amount.toFixed(2)
}
```

Creating `conversion` does no work. Calling it runs the computation within the current program and gives you its success value. A call does not start a new fiber; runners start execution at the application edge.

An imported Effect value is also callable. For example, `Clock.currentTimeMillis` from `effect` is a description, and `Clock.currentTimeMillis()` reads the clock inside a function. The same call rule runs Effect-returning library functions.

The result type must be `Effect` itself for a call to run it automatically. `Config`, `Exit`, Context keys, and yieldable error instances carry additional structure and stay data when returned. Call such a value to run it; `new` only constructs it.

## Extract inferred channel types

```ts
// convert.es
export function double(amount: number) {
  return amount * 2
}
// Effect<number, never, never>
```

```ts
// TypeScript (.ts)
// inspect.ts: imports the emitted EffectScript module
import { Effect } from "effect"
import { double } from "./convert.js"

type Contract = ReturnType<typeof double>
type Value = Effect.Success<Contract>    // number
type Failure = Effect.Error<Contract>    // never
type Services = Effect.Services<Contract> // never
```

TypeScript consumers see exported EffectScript functions as functions returning ordinary Effects. `Effect.Success`, `Effect.Error`, and `Effect.Services` extract the channels from those inferred return types. These utilities inspect types without executing the program.

## Coming from Effect

`Effect<A, E, R>` is available globally in `.es` files. A local call gives `A`; preserve a program with a function value when you need to compose or store it.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/the-effect-type)
