# Start a new project

Create a project from the VibeLang starter. It builds with the `vibelang` CLI and tests with Vitest through `vibelang-vite`, so it needs no build page. Compare each command's output with the expected output.

## 1. Create the project

Choose a new directory name or an empty directory. With Bun, run:

```sh
bun create vibelang@latest my-app
```

With npm, run:

```sh
npm create vibelang@latest my-app
```

The starter prints:

```out
Created my-app with VibeLang 0.0.5.
Installing dependencies with bun...
```

With npm, the second line names `npm`. The command copies the starter into `my-app`, pins `vibelang`, `vibelang-runtime`, and `vibelang-vite` to its own version, and installs dependencies with the package manager that ran it. It refuses a non-empty directory and never runs `git init`. `--no-install` skips installation; npm needs `--` before starter options:

```sh
bun create vibelang my-app --no-install
npm create vibelang@latest my-app -- --no-install
```

Then run `bun install` or `npm install` in `my-app` before step 2.

## 2. Build and run

```sh
cd my-app
bun run start
```

With npm, run `npm run start`. The output ends:

```out
Hello, VibeLang!
Recovered from EmptyName
```

## 3. Run the tests

```sh
bun run test
```

With npm, run `npm run test`. The output includes:

```out
 Test Files  1 passed (1)
      Tests  2 passed (2)
```

`bun run dev` rebuilds and restarts the program on every save until Ctrl+C stops it. Don't run it in a session that waits for each command to exit.

## Next

Set up the editor: [VS Code and Cursor](vscode.md), [Neovim](neovim.md), [Zed](zed.md), or [other editors](other-editors.md). Then verify with step 4 of [Set up VibeLang](../setup.md).
