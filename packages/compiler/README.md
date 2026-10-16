# @effectscript/compiler

Build the native compiler from the repository root:

```sh
sh scripts/build-esc.sh
packages/compiler/bin/esc --version
```

The Node launcher forwards arguments, standard streams, signals, and the native
process's exit status. It loads `native/esc` relative to its own package, so it
works from another directory. Set `ESC_BINARY` to use another native binary.
The local setup uses absolute `file:` dependencies; no registry publication or
global package registration is required. Rebuild after changing compiler source,
then run `bun install --force` in existing projects to refresh their package copy.
