import { mkdirSync, symlinkSync } from "node:fs"
import { Cause, Effect, Exit } from "effect"
// Link the emitted workspace package after compilation, as a workspace manager would.
mkdirSync(new URL("./node_modules/@smthrs", import.meta.url), { recursive: true })
try { symlinkSync("../../out/workspace", new URL("./node_modules/@smthrs/twin", import.meta.url)) } catch (e) { if (e.code !== "EEXIST") throw e }
const { succeeds, defects } = await import("./out/main.js")
const { marker } = await import("./out/workspace/index.js")
const exit = Effect.runSyncExit(defects())
console.log(JSON.stringify([Effect.runSync(succeeds()), Exit.isFailure(exit) && Cause.hasDies(exit.cause) && Cause.squash(exit.cause) === marker]))
