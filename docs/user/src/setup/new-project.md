# Start a new project

Create a project from the Effeck starter. The starter builds with the `effeck` CLI and tests with Vitest through `effeck-vite`, so it needs no build page. Run each command, then compare its output with the expected output.

## 1. Create the project

Choose a directory name that does not exist yet, or an empty directory. With Bun, run:

```sh
bun create effeck my-app
```

With npm, run:

```sh
npm create effeck@latest my-app
```

The output begins:

```out
Created my-app with Effeck 0.0.1.
Installing dependencies with bun...
```

With npm, the second line names `npm`. The command copies the starter into `my-app`, pins `effeck`, `effeck-runtime`, and `effeck-vite` to its own version, and installs dependencies with the package manager that ran it. It refuses a directory that is not empty, and it never runs `git init`. With `--no-install`, it skips the install; then run `bun install` or `npm install` in `my-app` before step 2.

## 2. Build and run

```sh
cd my-app
bun run start
```

With npm, run `npm run start`. The output ends:

```out
Hello, Effeck!
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

`bun run dev` rebuilds and restarts the program on every save, and runs until Ctrl+C stops it. Do not run it in a session that waits for each command to exit.

## Next

Set up the editor: [VS Code and Cursor](vscode.md), [Neovim](neovim.md), [Zed](zed.md), or [other editors](other-editors.md). Then verify the setup with step 4 of [Set up Effeck](../setup.md).
