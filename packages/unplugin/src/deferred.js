// vibelang-unplugin/deferred: the plugin for a config that is evaluated where
// the bundle runs. Infrastructure-as-code tools such as Alchemy declare a
// Worker's bundler options in the Worker's own module, so the Worker evaluates
// them too; a static import of vibelang-unplugin there would bundle the
// compiler backend, which spawns processes, into the Worker. This module
// imports nothing. deferred() returns a promise-like value that loads the
// plugin only when a bundler awaits it.

// The bundlers that await promises in their plugin list.
const entries = { rolldown: "./rolldown.js", rollup: "./rollup.js", vite: "./vite.js" }

/** @type {import('./deferred.d.ts').deferred} */
export function deferred(bundler, options) {
  const entry = entries[bundler]
  if (entry === undefined) {
    throw new TypeError(
      `vibelang-unplugin/deferred: ${bundler} does not await plugins; use one of ${Object.keys(entries).join(", ")}, or import vibelang-unplugin/${bundler}`
    )
  }
  let plugin
  // A computed specifier: a bundler that bundles this module leaves the import alone.
  const load = () => (plugin ??= import(new URL(entry, import.meta.url).href).then((module) => module.default(options)))
  // A promise in shape, so a config typed for promised plugins accepts it, that starts only when awaited.
  return {
    then: (accept, reject) => load().then(accept, reject),
    catch: (reject) => load().catch(reject),
    finally: (settle) => load().finally(settle),
    [Symbol.toStringTag]: "Promise"
  }
}

export default deferred
