# Zed

Zed defines languages through extensions, so these settings treat `.vibe` files as TypeScript and replace the binary of Zed's TypeScript server, `vtsls`, with `vibelang`. Zed then sends `.vibe` files with the language id `typescript`, which the server accepts, and the server also serves the project's `.ts` files. The server starts with these settings in Zed 0.195.3, but diagnostic requests fail. That version sends `previousResultId: null`, which VibeLang rejects with `InvalidParams` (-32602). Use `npx vibelang --noEmit` in the terminal to check the project.

## 1. Find the compiler's absolute path

From the project directory, run:

```sh
echo "$PWD/node_modules/.bin/vibelang"
"$PWD/node_modules/.bin/vibelang" --version
```

The first line is the path to use in step 2. The second command prints `VibeLang 0.0.2 (TypeScript 7.1.0-dev)`.

## 2. Add the project settings

Put these settings in the project's `.zed/settings.json`, so other TypeScript projects keep `vtsls`. Replace the `path` value with the path from step 1. If the file exists, merge these keys into it.

```json
{
  "file_types": {
    "TypeScript": ["vibe"]
  },
  "lsp": {
    "vtsls": {
      "binary": {
        "path": "/absolute/path/to/project/node_modules/.bin/vibelang",
        "arguments": ["--lsp", "--stdio"]
      }
    }
  },
  "languages": {
    "TypeScript": {
      "language_servers": ["vtsls"]
    }
  }
}
```

The path must be absolute. The `language_servers` list selects only this server, so another TypeScript server does not report the same files. Open the project in Zed to use the server, and run `npx vibelang --noEmit` for diagnostics until the client sends diagnostic requests without the null field.
