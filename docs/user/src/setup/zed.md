# Zed

Zed defines languages through extensions, so these settings treat `.effeck` files as TypeScript and replace the binary of Zed's TypeScript server, `vtsls`, with `effeck`. Zed then sends `.effeck` files with the language id `typescript`, which the server accepts, and the server also serves the project's `.ts` files. These settings are untested: Zed cannot run headlessly, so nothing has checked them in Zed.

## 1. Find the compiler's absolute path

From the project directory, run:

```sh
echo "$PWD/node_modules/.bin/effeck"
"$PWD/node_modules/.bin/effeck" --version
```

The first line is the path to use in step 2. The second command prints `Effeck 0.0.1 (TypeScript 7.1.0-dev)`.

## 2. Add the project settings

Put these settings in the project's `.zed/settings.json`, so other TypeScript projects keep `vtsls`. Replace the `path` value with the path from step 1. If the file exists, merge these keys into it.

```json
{
  "file_types": {
    "TypeScript": ["effeck"]
  },
  "lsp": {
    "vtsls": {
      "binary": {
        "path": "/absolute/path/to/project/node_modules/.bin/effeck",
        "arguments": ["--lsp", "--stdio"]
      }
    }
  },
  "languages": {
    "TypeScript": {
      "language_servers": ["vtsls", "!typescript-language-server", "..."]
    }
  }
}
```

The path must be absolute. Tell the user to open the project in Zed and check that a `.effeck` file shows diagnostics and hovers.
