## Essentials

Read this before writing VibeLang. Every function is an Effect. Calls run in order; there is no `yield*`, `Effect.gen` or `Effect.fn`, and `function*` declares a value producer, not a sequence. A function's type is `Success!Errors!Requirements`: `string!NotFound` can fail with `NotFound`, `string!!` cannot fail, and the compiler infers what you leave out.

```vibe
error NotFound { id: string }
error Timeout

interface User { id: string; name: string }

function getUser(id: string): User!NotFound {
  if (id === "") throw new NotFound({ id })
  return { id, name: "Ada" }
}

function greet(id: string) {
  return `Hello, ${getUser(id).name}` catch (e) => `No user ${e.id}`
}

function label(id: string) {
  try {
    return getUser(id).name
  } catch (error) {
    return match (error) {
      when NotFound: "missing";
      default: throw error;
    }
  }
}
```

`error` declares a typed error class; `throw` adds it to the error channel. `x catch (e) => y` and `try { } catch { }` recover expected failures; `match` with `when` selects error classes, and a `match` without `default` must cover every failure. `x ?? throw new E()` fails in expression position.

```vibe
service Users {
  find(id: string): User!NotFound
}

const UsersLive = Layer.succeed(Users, {
  find(id) { return getUser(id) }
})

export function main() {
  provide UsersLive {
    const [ada, other] = await.all [Users.find("42"), Users.find("7")]
    const shout = ada.name |> %.toUpperCase()
    Console.log(shout, other.name)
  }
}
```

`service` declares a dependency; call its members directly, and `provide` a layer to satisfy it. `await.all [...]` and `await.race [...]` run programs concurrently. In `x |> f(%)`, `%` stands for `x`.

Effect's root modules (`Option`, `Schedule`, `Console`, `Stream`, ...) are in scope without imports, and their data types have methods (`maybe.isSome()`). A plain TypeScript function needs a boundary: `sync(() => f(x))` when it cannot throw, `try(() => f(x), (error: unknown) => new MyError())` when it can, or a `/** @vibelang trusted */` declaration. Tests use `@effect/vitest`: `it.effect("name") { ... }` with `assert`. Under a coding agent, vitest hides `console.log` output from passing tests, so check values with `assert` rather than by logging them. The pages below cover each form; the [cheat sheet](vibelang-features/115-cheat-sheet.md) maps every Effect construct to its VibeLang form.
