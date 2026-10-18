### 15. Which functions compile to plain JavaScript

Unannotated functions and functions with Effect annotations expose Effect contracts, but a function like `add` should cost what a JavaScript call costs. A helper with a plain return annotation keeps a plain function type (D67). The compiler emits a plain JavaScript function whenever the difference is unobservable.

```ts es
function double(n: number) {
  return n * 2
}

function area(w: number, h: number) {
  return double(w) * double(h) * Settings.scale()
}
```

```ts gen
export function double$direct(n: number): number {              // Effeck callers use this
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

A function compiles to plain JavaScript when all three hold:

1. **It performs no Effect operation at all.** Its body calls only plain-JavaScript Effeck functions, trusted built-ins, and trusted functions; it calls no service, control operation, `Effect` function, or boundary hatch such as `sync`. Its control class is `pure` or `sync`.
2. **Its E is `never`.** A typed failure leaving a plain function would cross JavaScript frames as an exception, and v0 bridges no exception across a call. Exceptions become typed failures only where they are raised: at the call of a trusted function, whose listed exceptions are converted there (Chapter 26), and inside the plain function's own `try`, whose `catch` handles them (below). Bridging across calls is experiment G15.
3. **Its contract, if it has one, writes E and R as `never`.** A caller sees the contract, not the body (Chapter 14), so `function answer(): number!Gone!Needed { return 1 }` is an Effect: its callers can fail with `Gone` and need `Needed`, and EK2001 and EK2002 hold them to it. `number!!` keeps the plain path, and a channel written `infer Name` counts what the body raises.

The test is "can it reach an Effect operation", not "are E and R empty". `Effect.sleep("1 second")` has E = `never` and R = `never`, and it pauses the fiber. So the compiler tracks a separate fact for each function, its **control class**:

| Control class | Meaning | Examples | Plain JavaScript |
| --- | --- | --- | --- |
| `pure` | No observable effect; cannot pause | `double`, string formatting, `Math.max` | Yes |
| `sync` | Synchronous work you can observe; cannot pause | Mutating an object, `Map.prototype.set`, a trusted TypeScript function | Yes; the order and count of the work are kept |
| `suspends` | May pause the fiber, or crosses an Effect boundary | `Effect.sleep`, `sync(…)`, any service method, any imported Effect | No |
| `captures` | May reach a handler (Chapter 9) | `Approval.approve`, `yield` in a producer | No |

`pure` is assigned only to Effeck functions the compiler analyzed and to built-ins on the compiler's list; a trusted TypeScript function is `sync` at most. `pure` permits one thing beyond `sync`: inlining the body at a call. It never permits removing a call whose result is unused, reordering calls, or merging two calls into one.

#### Why this is safe

- **Nothing runs early when building an Effect.** For an unannotated or Effect-contract helper, the TypeScript-facing form is `Effect.sync(() => …)`, never `Effect.succeed(double$direct(n))`. Building a program does no work, even work that looks free. A pure function can still throw a defect (`(undefined as any).x`), and that must happen when the program runs, not when it is built.
- **No point where another fiber could run disappears.** A plain call runs inside its caller's current step. Every Effect operation is such a point, including an `Effect.sync` body, so a function containing one is not eligible. Inlining a callee that performs no operation removes none.
- **Order is kept.** Generated code evaluates in source order. In `area`, both `double` calls run before `Settings.scale()`, as the source says, even though only the service call can pause.
- **Public types follow annotations.** An unannotated or Effect-contract helper exposes `(...) => Effect<A, E, R>` and gets a `$direct` shortcut. An eligible helper annotated `: A` exposes `(...) => A` and emits one plain function (D67). Plain calls run inline even at module top level (D65). The compiler does not yet emit `$direct` declaration metadata, so callers compiled against declarations use the correct, slower Effect form (Chapter 27).

#### Regions inside a plain function

A plain JavaScript function may contain `try`, `catch`, and `finally`, and they compile to JavaScript's own statements. Nothing inside can pause or be interrupted, so JavaScript's semantics match the machine's. The generated `catch` rethrows anything that is not one of the region's expected failures, so defects still pass through:

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
    return JSON.parse(text).port as number
  } catch (e) {
    if (!(e instanceof SyntaxError)) throw e     // a defect: not caught
    return 8080
  }
}
```

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

`Equal.Equal` declares `[Equal.symbol](that): boolean` and `Hash.Hash` declares `[Hash.symbol](): number`: plain functions, which `Equal.equals` and `HashSet` call synchronously. A method that implements such a member compiles to a plain JavaScript method when it is eligible under the rule above, and is EK4301 otherwise. Effeck callers call it as a plain method, because its declared type is not an Effect. A method that implements a service member keeps compiling to an Effect function, because the service declares an `Effect` (D55).

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

`sumTo` is plain JavaScript, recursion included, so it runs on the JavaScript stack like any JavaScript function: fast, and a `RangeError` defect past the host's limit, about 10,000 frames on Node and 40,000 on Bun (F9). Eligibility is decided per strongly connected component of the call graph: the functions of a component compile to plain JavaScript when each one would with the calls inside the component counted as plain calls, and otherwise all of them compile to frames (D56). `walk` calls a service, so it runs on heap frames and recurses without limit: 100,001 nested calls run without overflow (T6a), at about 110 ns per frame (Chapter 25). A pure helper costs what JavaScript costs; only functions that can pause pay for frames.

#### Higher-order functions

A function that calls a callback it receives compiles to frames in v0, because its control class depends on the callback's. `mapAll` in Chapter 16 runs a loop of frames whether its callback pauses or not. Compiling a plain-JavaScript copy per call site, or per class of callback, is a later optimization that changes no rule here.
