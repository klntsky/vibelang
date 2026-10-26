# VibeLang project

`bun run build`; `bun run test`. https://vibelang.sh/llms.txt

Read `node_modules/vibelang/skills/vibelang/SKILL.md` before .vibe edits.

## Essentials

Functions are Effects; calls run in order, without `yield*`. Contracts are inferred; `Effect<A>` means `Effect<A, never, never>`.

```vibe
error NotFound { id: string }

service Users {
  name(id: string): Effect<string, NotFound>
}

function nameOf(id: string): Effect<string, never, Users> {
  return Users.name(id)
    |> timeout(%, "1 second")
    |> retry(%, { times: 3, schedule: Schedule.exponential("100 millis") })
    catch "unknown"
}

export function names(ids: ReadonlyArray<string>) {
  return forEach(ids, nameOf, { concurrency: 8 })
}
```

- `throw new NotFound({ id })` fails; `x catch (e) => y` or `try { } catch (e) { }` recovers; `match (e) { when NotFound: ...; default: throw e; }` picks error classes.
- Import `"./user.vibe"` with its extension. Effect's root modules need no import.
- From `.ts`, import `"./user.vibe"`, run its Effect with `Effect.runPromise` from `effect`, and include `src/**/*.ts` in tsconfig.
- `provide UsersLive { ... }` supplies a service; `UsersLive = Layer.succeed(Users, { name(id) { ... } })`.
- `await.all [a(), b()]` runs programs together; in `x |> f(%)`, `%` is `x`.
- Bodies take TypeScript: `let`, loops, `typeof`, regex, Array/Map methods.
- `using h = Effect.acquireRelease(() => open(), (h) => close(h))` frees `h` at block end, failure or interrupt. An interrupted start still runs to the first yield: call `Effect.yieldNow()` first if `open()` must not run.
- A plain TypeScript call needs `sync((x) => f(x), x)`, or `try((x) => f(x), (error: unknown) => new E(), x)` if it can throw.
- Tests: `it.effect("name") { assert.strictEqual(a, b) }` from `@effect/vitest` uses a test clock (`sleep` stalls); `it.live` uses real time. Vitest hides `console.log` under agents; assert instead.

<!-- 594 tokens (o200k_base, js-tiktoken 1.0.21). Generated: smithersai/vibelang .smithers/flows/docs/flow.ts -->
