import { registerHooks } from "node:module"

const stubs = {
  "vibelang-runtime/experimental/Process": "./stubs/experimental/Process.js",
  "vibelang-runtime/experimental/platform-node": "./stubs/experimental/platform-node/index.js",
}

registerHooks({
  resolve(specifier, context, nextResolve) {
    const stub = stubs[specifier]
    return stub === undefined ? nextResolve(specifier, context) : { url: new URL(stub, import.meta.url).href, shortCircuit: true }
  },
})
