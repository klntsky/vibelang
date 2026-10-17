# effeck

> **Warning:** Effeck is v0. The language will change in major, breaking ways. It is not suitable for production use.

Build the native compiler from the repository root:

```sh
sh scripts/build-effeck.sh
packages/compiler/bin/effeck --version
```

The Node launcher forwards arguments, standard streams, signals, and the native
process's exit status. It loads `native/effeck` relative to its own package, so it
works from another directory. Set `EFFECK_BINARY` to use another native binary.
The local setup uses absolute `file:` dependencies; no registry publication or
global package registration is required. A project links `native/effeck` from
this package, so it uses a rebuilt compiler without reinstalling.
