# Guidelines

Run a VibeLang application through its platform runtime, keep its lifetime in `main`, and make callback arguments explicit. These habits keep shutdown predictable and prevent accidental argument forwarding when composing functions.

## Running application entry points

```vibe
// main.vibe
export function main() {
  try {
    log("worker started")
    while (true) {
      sleep("2 seconds")
      log("checking work")
    }
  } finally {
    log("worker stopped")
  }
}
// node main.js runs main with NodeRuntime.runMain
```

Export `main` from the entry module. When node starts the module, the compiled JavaScript passes `main()` to `NodeRuntime.runMain`, which starts it and coordinates interruption and shutdown; importing the module starts nothing. The `finally` block runs when the worker stops, including on interruption. Acquire application resources with `using` inside `main`, and provide the application layer once around the main program so its services share one lifetime.

`Effect.runPromise` is useful when another host owns execution and needs a Promise. For a standalone process, prefer the platform runner's signal handling and shutdown behavior. Module-level calls may build values or register an entry point, but ordinary application work belongs inside functions.

Call runners only where no fiber runs the code: at the module's top level or in a plain framework callback registered from there. A runner inside code a fiber runs is EK1005; call the program or use `fork` there. Tests in `.vibe` use `it.effect`; Promise-style tests stay in `.ts`.

## Selecting a platform runtime

```vibe
// main.vibe
import { BunRuntime } from "@effect/platform-bun"

function main() {
  log("running on Bun")
}

BunRuntime.runMain(() => main())
```

Choose the runtime matching the host and install that platform package. The application still uses Effect's services rather than directly calling Node.js modules or browser globals.

| Host | Runtime | Import |
| --- | --- | --- |
| Node.js | `NodeRuntime.runMain` | `@effect/platform-node` |
| Bun | `BunRuntime.runMain` | `@effect/platform-bun` |
| Browser | `BrowserRuntime.runMain` | `@effect/platform-browser` |

## Making argument forwarding explicit

```vibe
// run: Console.log(JSON.stringify(parseDecimalTokens(["10", "10", "10"])))
function parseDecimalTokens(tokens: ReadonlyArray<string>) {
  return tokens.map((text) => Number.parseInt(text, 10))
}
// parseDecimalTokens(["10", "10", "10"]) returns [10, 10, 10].

function printValues(values: ReadonlyArray<number>) {
  forEach(values, (value) => log(value), { discard: true })
}
// Output: [10,10,10]
```

Write a lambda that names the arguments you intend to pass. Passing `Number.parseInt` directly to array `map` would forward the array index as its radix argument. Explicit lambdas also preserve overload selection and make generic inference easier to inspect.

Use `Effect.forEach` or an ordinary loop when the callback performs Effect work. Array `map` accepts only callbacks eligible for plain JavaScript compilation: no Effect operations and no expected failures. A composition helper does not change that boundary.

## Coming from Effect

Application code uses ordinary calls, `try`/`finally`, and `using`. Keep `runMain` at the outer boundary, and pass it `main()`: the runner receives the call by name, so the entry program has not already run.
