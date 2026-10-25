# VibeLang

[![npm](https://img.shields.io/npm/v/vibelang)](https://www.npmjs.com/package/vibelang)
[![license](https://img.shields.io/badge/license-MIT%20%2B%20Apache--2.0-blue)](#license)

Write [Effect](https://effect.website) programs in TypeScript syntax, with typed errors and services the compiler checks.

[Get started](https://vibelang.sh) · [Cheat sheet](https://vibelang.sh/vibelang-features/115-cheat-sheet) · [Contribute](CONTRIBUTING.md)

> [!WARNING]
> VibeLang is v0. The language will change in major, breaking ways. It is not suitable for production use.

## Why VibeLang

Effect gives you typed failures, dependency injection, concurrency and resource management. VibeLang gives those programs direct syntax: a call returns its value, and its errors and required services stay in its type.

Here is one program in both languages: a typed failure, an injected service, two concurrent lookups and a fallback.

**VibeLang**

```vibe
error UserNotFound { id: string }
service Users {
  find(id: string): string!UserNotFound
}

const UsersLive = Layer.succeed(Users, {
  find(id: string) {
    if (id !== "42") throw new UserNotFound({ id })
    return "Ada"
  }
})

export function main() {
  provide UsersLive {
    const [name, fallback] = await.all [
      Users.find("42"),
      Users.find("7") catch "Guest"
    ]
    Console.log(name, fallback)
  }
}
```

**Effect 4 TypeScript**

```ts
import { Console, Context, Data, Effect, Layer } from "effect"

class UserNotFound extends Data.TaggedError("UserNotFound")<{ id: string }> {}
class Users extends Context.Service<Users, {
  find: (id: string) => Effect.Effect<string, UserNotFound>
}>()("Users") {}

const UsersLive = Layer.succeed(Users, {
  find: (id: string) => id === "42"
    ? Effect.succeed("Ada")
    : Effect.fail(new UserNotFound({ id }))
})

const main = Effect.gen(function* () {
  const users = yield* Users
  const [name, fallback] = yield* Effect.all([
    users.find("42"),
    users.find("7").pipe(Effect.catch(() => Effect.succeed("Guest")))
  ], { concurrency: "unbounded" })
  yield* Console.log(name, fallback)
}).pipe(Effect.provide(UsersLive))

Effect.runPromise(main)
```

**116 tokens vs 202 tokens: 42.6% on this program; [11.4% across the 44 cheat-sheet pairs](docs/user/src/vibelang-features/117-token-counts.md)** (fewer tokens, o200k_base). Both print `Ada Guest`. The counts cover the complete source files, including TypeScript's imports and runner, which VibeLang supplies. It is one example, not a general ratio. [Reproduce the counts and both runs](.smithers/token-compare/README.md). [Token counts](docs/user/src/vibelang-features/117-token-counts.md) measures the skill agents load, the docs and every cheat-sheet pair.

`service` declares a dependency and `provide` supplies it. `await.all` runs calls concurrently, and `catch` handles the declared failure. `Console` and `Layer` are in scope without imports. A contract reads `Result!Error!Services`, and the compiler infers the parts you leave out.

The compiler checks `.vibe` and `.ts` files together and emits JavaScript and TypeScript declarations, so TypeScript code can import VibeLang modules.

## Start here

```sh
bun create vibelang@latest my-app
cd my-app
bun run start
bun run test
```

With npm, run `npm create vibelang@latest my-app`. You need Node 26 or later and Bun 1.4 or later. `bun run start` prints:

```text
Hello, VibeLang!
Recovered from EmptyName
```

Replace `src/main.vibe` with this program and run `bun run start` again:

```vibe
error EmptyName {}
function greet(name: string): string!EmptyName {
  if (name === "") throw new EmptyName()
  return `Hello, ${name}!`
}
export function main() {
  Console.log(greet("VibeLang"))
  const recovered = greet("") catch "Hello, stranger!"
  Console.log(recovered)
}
```

```text
Hello, VibeLang!
Hello, stranger!
```

To add VibeLang to an existing project, see [Installation](https://vibelang.sh/getting-started/004-installation).

## Editors

The `vibelang` package ships a VS Code and Cursor extension. From your project, after installing dependencies:

```sh
code --install-extension node_modules/vibelang/editor/vibelang.vsix
```

Use `cursor` instead of `code` for Cursor. The extension isn't on the marketplaces yet. It runs the project's compiler as a language server for diagnostics, hovers and completions. [Editor setup](editors/README.md) covers Neovim, Zed and other LSP clients.

## Learn more

- [Guides and reference](https://vibelang.sh)
- [Syntax cheat sheet](https://vibelang.sh/vibelang-features/115-cheat-sheet): every form beside the Effect code it replaces
- [VibeLang for Effect users](https://vibelang.sh/vibelang-features/114-vibelang-for-effect-users)
- [Specification](docs/design/src/000-intro.md)
- [Examples](examples/README.md): services, an HTTP API, retries and races, streams, React, Vue and Svelte

## Help build it

Report a bug with a minimal reproduction, or propose an idea, in [Issues](https://github.com/smithersai/vibelang/issues/new/choose). A maintainer accepts issues for discussion, labels each with an expected timeline and commitment, and links a proof of concept as a draft pull request titled `poc: #<n>` that you can check out and run. [CONTRIBUTING.md](CONTRIBUTING.md) explains the process, and [MAINTAINERS.md](MAINTAINERS.md) covers triage and releases. Report vulnerabilities privately through [SECURITY.md](.github/SECURITY.md).

## License

[MIT](LICENSE), except the TypeScript fork in [`typescript/`](typescript/) and the compiler packages built from it, `vibelang` and `vibelang-<platform>`, which use the [Apache License 2.0](typescript/LICENSE.txt) and keep its [notices](typescript/NOTICE.txt).

<sub>1,520 tokens (o200k_base, js-tiktoken 1.0.21), counted by the [docs flow](.smithers/flows/docs/flow.ts).</sub>
