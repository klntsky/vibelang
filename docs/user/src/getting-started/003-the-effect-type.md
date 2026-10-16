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

Partial annotations must infer omitted channels rather than set them to `never`. Until the partial-annotation spelling is chosen, write all three channels or leave the annotation off. A bare return annotation such as `: number` is ES2003.

> **Not designed yet.** How do you write an annotation that fixes only selected channels and leaves the others inferred?

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
