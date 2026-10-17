# Hello

The smallest Node.js Effeck program: one function and one log.
`Console.log` runs when called. The module exports `main`, so running its
JavaScript with `node dist/main.js` runs `main` with `NodeRuntime.runMain`.

From this directory, with `effeck` on `PATH`:
```sh
bun install
bun run build
bun run start
```

Expected output: `Hello, Effeck!`.
The key function in Effect TypeScript is:
```ts
import { Console, Effect } from "effect"
const main = Effect.fnUntraced(function* () {
  yield* Console.log("Hello, Effeck!")
})
```

The compiler ends `main.js` with `if (import.meta.main) NodeRuntime.runMain(main())`,
so importing the module runs nothing.
