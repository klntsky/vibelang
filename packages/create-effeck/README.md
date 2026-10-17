# create-effeck

> **Warning:** Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use.

Create an [Effeck](https://effeck.pages.dev) application: TypeScript syntax in
which every function is an Effect.

```sh
bun create effeck my-app
# or
npm create effeck@latest my-app
```

The command copies a starter app into `my-app`, pins `effeck`,
`@effeck/runtime`, and `@effeck/vite` to its own version, and installs
dependencies with the package manager that ran it. It refuses a directory that
is not empty, and it never runs `git init`.

| Option | Effect |
| --- | --- |
| `--no-install` | Copy the app without installing dependencies. |
| `--no-git` | Accepted for compatibility with `bun create`; nothing changes. |

Then run the app:

```sh
cd my-app
bun run start    # build and run src/main.effeck
bun run dev      # rebuild and restart on every save
bun run test     # run the .effeck tests with Vitest
```

With npm, use `npm run start`, `npm run dev`, and `npm run test`.

## Editor support

The `effeck` package ships the VS Code extension. After the install, run one of:

```sh
code --install-extension node_modules/effeck/editor/effeck.vsix
cursor --install-extension node_modules/effeck/editor/effeck.vsix
```

The extension starts the project's own compiler as the language server.

## Requirements

Node 26 or later. The starter app depends on `effect@4.0.5` exactly, the
version the Effeck runtime is built against.
