# Hello

The smallest Node.js EffectScript program: one function and one log.
`Console.log` runs when called; the runner starts `main` at the module boundary.

From this directory, with `esc` on `PATH`:
```sh
bun install
bun run build
bun run start
```

Expected output: `Hello, EffectScript!`.
The key function in Effect TypeScript is:
```ts
import { Console, Effect } from "effect"
const main = Effect.fnUntraced(function* () {
  yield* Console.log("Hello, EffectScript!")
})
```

`NodeRuntime.runMain(main())` is the TypeScript entry point.
The prototype CLI also invokes exported `main`, so its `run` command logs twice.
Compile with the prototype and execute its emitted module once to check output.
