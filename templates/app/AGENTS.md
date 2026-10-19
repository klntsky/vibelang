# VibeLang project

This app is written in VibeLang: `.vibe` files are Effect programs in TypeScript syntax. `bun run build` type-checks and compiles to `dist/`; `bun run test` runs `src/**/*.test.vibe` with Vitest. Full docs: https://effeck.dev/llms.txt

## Essentials

Every function is an Effect: calls run in order, with no `yield*` or `Effect.gen`. `A!E` is a result and its failures, `A!!` cannot fail; omitted parts are inferred.

```vibe
error NotFound { id: string }

service Users {
  name(id: string): string!NotFound
}

function nameOf(id: string): string!!Users {
  return Users.name(id)
    |> timeout(%, "1 second") // each attempt
    |> retry(%, { times: 3, schedule: Schedule.exponential("100 millis") })
    catch (e) => "unknown"
}

export function names(ids: ReadonlyArray<string>) {
  return forEach(ids, nameOf, { concurrency: 8 })
}
```

- `throw new NotFound({ id })` fails; `x catch (e) => y` or `try { } catch (e) { }` recovers; `match (e) { when NotFound: ...; default: throw e; }` picks error classes.
- Import a sibling module with its extension: `import { User } from "./user.vibe"`. Effect's root modules (`Option`, `Schedule`, `Console`) need no import.
- `provide UsersLive { ... }` supplies a service; `UsersLive = Layer.succeed(Users, { name(id) { ... } })`.
- `await.all [a(), b()]` runs programs together; in `x |> f(%)`, `%` is `x`.
- Function bodies take plain TypeScript: `let`, loops, `typeof`, regex, Array/Map methods.
- `using h = Effect.acquireRelease(() => open(), (h) => close(h))` frees `h` at block end, failure or interrupt. An interrupted start still runs to the first yield: call `Effect.yieldNow()` first if `open()` must not run.
- A plain TypeScript call needs `sync(() => f(x))`, or `try(() => f(x), (error: unknown) => new E())` if it can throw.
- Tests: `it.effect("name") { assert.strictEqual(a, b) }` from `@effect/vitest` uses a test clock (`sleep("1 second")` stalls); `it.live` uses real time. Under an agent, vitest hides `console.log`; assert instead.
