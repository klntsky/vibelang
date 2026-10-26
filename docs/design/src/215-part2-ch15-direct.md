### 15. Which functions compile to plain JavaScript

Unannotated and Effect-annotated functions expose Effect contracts, but a function like `add` should cost what a JavaScript call costs. A helper with a plain return annotation keeps a plain function type (D67), and the compiler emits a plain JavaScript function whenever the difference is unobservable.

```ts es
function double(n: number) {
  return n * 2
}

function area(w: number, h: number) {
  return double(w) * double(h) * Settings.scale()
}
```

```ts gen
export function double$direct(n: number): number {              // VibeLang callers use this
  return n * 2
}
export const double = (n: number): Effect.Effect<number> =>     // TypeScript callers use this
  Effect.sync(() => double$direct(n))

export const area = Effect.fnUntraced(function* (w: number, h: number) {
  const left = double$direct(w) * double$direct(h)              // source order: before the service call
  const scale = yield* (yield* Settings).scale()                // a service call: may pause
  return left * scale
})
```

#### The rule

A function with a plain contract, from an explicit non-Effect return annotation or a plain callback type its context gives it, is JavaScript because it never fails and needs nothing (D282). `throw` is a typed failure there as everywhere outside a hatch: one that can escape is EK2003 under an annotation and EK4301 for a callback, and the function may throw what its own `catch` handles. Calls of TypeScript code, including callback parameters and trusted or untrusted imports, are ordinary calls needing no trust boundary; their exceptions propagate unchanged and become defects where they reach an effectful caller. Inside a `try` whose `catch` belongs to the function, a call's listed failures are typed, and the catch receives them with their types. Its `.d.ts` keeps the plain return type. Running an Effect under a plain contract is EK2003.

For an unannotated function declaration or an Effect-contract function, the compiler can provide a plain `$direct` variant when all three hold:

1. **It performs no Effect operation at all.** Its body calls only plain-JavaScript VibeLang functions, trusted built-ins, and trusted functions; it calls no service, control operation, `Effect` function, or boundary hatch such as `sync`. Its control class is `pure` or `sync`.
2. **Its E is `never`.** A typed failure leaving a plain function would cross JavaScript frames as an exception, and v0 bridges no exception across a call. In Effect code, a trusted function's listed exceptions become typed failures at its call (Chapter 26). A plain function's own catch receives them as typed failures (D54, D282); outside its `try` they propagate as JavaScript exceptions, and an exception from code with no contract passes through that catch and becomes a defect where it reaches an Effect caller. Bridging across calls is experiment G15.
3. **Its contract, if it has one, writes E and R as `never`.** A caller sees the contract, not the body (Chapter 14), so `function answer(): number!Gone!Needed { return 1 }` is an Effect: its callers can fail with `Gone` and need `Needed`, and EK2001 and EK2002 hold them to it. `number!!` keeps the plain path, and a channel written `infer Name` counts what the body raises.

The test is "can it reach an Effect operation", not "are E and R empty": `Effect.sleep("1 second")` has E = R = `never`, and it pauses the fiber. So the compiler tracks a separate fact per function, its **control class**:

| Control class | Meaning | Examples | Plain JavaScript |
| --- | --- | --- | --- |
| `pure` | No observable effect; cannot pause | `double`, string formatting, `Math.max` | Yes |
| `sync` | Observable synchronous work; cannot pause | Mutating an object, `Map.prototype.set`, a trusted TypeScript function | Yes; the work's order and count are kept |
| `suspends` | May pause the fiber, or crosses an Effect boundary | `Effect.sleep`, `sync(…)`, any service method, any imported Effect | No |
| `captures` | May reach a handler (Chapter 9) | `Approval.approve`, `yield` in a producer | No |

`pure` is assigned only to VibeLang functions the compiler analyzed and to built-ins on the compiler's list; a trusted TypeScript function is `sync` at most. `pure` permits one thing beyond `sync`, inlining the body at a call, and never permits removing a call whose result is unused, reordering calls, or merging two calls into one.

#### Why this is safe

- **Nothing runs early when building an Effect.** For an unannotated or Effect-contract helper, the TypeScript-facing form is `Effect.sync(() => …)`, never `Effect.succeed(double$direct(n))`. Building a program does no work, even work that looks free. A pure function can still throw a defect (`(undefined as any).x`), and that must happen when the program runs, not when it is built.
- **No point where another fiber could run disappears.** A plain call runs inside its caller's current step. Every Effect operation is such a point, including an `Effect.sync` body, so a function containing one is not eligible. Inlining a callee that performs no operation removes none.
- **Order is kept.** Generated code evaluates in source order: in `area`, both `double` calls run before `Settings.scale()`, though only the service call can pause.
- **Public types follow annotations.** An unannotated or Effect-contract helper exposes `(...) => Effect<A, E, R>` and gets a `$direct` shortcut. An eligible helper annotated `: A` exposes `(...) => A` and emits one plain function (D67). Plain calls run inline even at module top level (D65). The compiler does not yet emit `$direct` declaration metadata, so callers compiled against declarations use the correct, slower Effect form (Chapter 27).

#### Regions inside a plain function

A plain JavaScript function may contain `try`, `catch`, and `finally`; they compile to JavaScript's own statements. Nothing inside can pause or be interrupted, so JavaScript's semantics match the machine's. Its catch still receives typed failures only (D54, D282): a throw becomes `throw $expected(e)`, a trusted call inside the try `$trustedPlain(…)`, and the catch starts with `e = $caught(e)`, which rethrows anything else, a defect. `JSON.parse`'s shipped contract lists `SyntaxError`, so this catch receives it:

```ts es
function readPort(text: string) {
  try {
    return JSON.parse(text).port as number
  } catch (e) {                                // e: SyntaxError
    return 8080
  }
}
```

```ts gen
export function readPort$direct(text: string): number {
  try {
    return runtime.$trustedPlain(() => JSON.parse(text), [SyntaxError]).port as number
  } catch (e) {
    e = runtime.$caught(e)
    return 8080
  }
}
```

A `TypeError` from reading `.port` of `null` is a defect: `$caught` rethrows it, and it reaches the caller unchanged.

Any `using` declaration makes a function compile to frames, whether its initializer acquires an Effect resource or supplies an ordinary Disposable (D54, D94).

#### Methods that implement a plain-function member

```ts es
class Money implements Equal.Equal {
  constructor(readonly cents: number, readonly currency: string) {}

  [Equal.symbol](that: Equal.Equal): boolean {
    return that instanceof Money && that.cents === this.cents && that.currency === this.currency
  }

  [Hash.symbol](): number {
    return Hash.combine(Hash.number(this.cents), Hash.string(this.currency))
  }
}
```

`Equal.Equal` declares `[Equal.symbol](that): boolean` and `Hash.Hash` declares `[Hash.symbol](): number`: plain functions that `Equal.equals` and `HashSet` call synchronously. A method implementing such a member compiles to a plain JavaScript method when eligible under the rule above, and is EK4301 otherwise. VibeLang callers call it as a plain method, because its declared type is not an Effect. A method implementing a service member still compiles to an Effect function, because the service declares an `Effect` (D55).

#### Recursion

```ts es
function sumTo(n: number): number!! {
  return n === 0 ? 0 : n + sumTo(n - 1)
}

function walk(node: TreeNode): number!!Nodes {
  let total = node.size
  for (const child of Nodes.children(node)) total += walk(child)
  return total
}
```

`sumTo` is plain JavaScript, recursion included, so it runs on the JavaScript stack: fast, and a `RangeError` defect past the host's limit, about 10,000 frames on Node and 40,000 on Bun (F9). Eligibility is decided per strongly connected component of the call graph: a component's functions compile to plain JavaScript when each would with calls inside the component counted as plain, and otherwise all compile to frames (D56). `walk` calls a service, so it runs on heap frames and recurses without limit: 100,001 nested calls run without overflow (T6a), at about 110 ns per frame (Chapter 25). A pure helper costs what JavaScript costs; only functions that can pause pay for frames.

#### Higher-order functions

An Effect-contract function that calls a callback it receives compiles to frames in v0, because its control class depends on the callback's. `mapAll` in Chapter 16 runs a loop of frames whether its callback pauses or not. Compiling a plain-JavaScript copy per call site, or per class of callback, is a later optimization that changes no rule here.
