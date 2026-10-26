// Does a fiber suspended on Deferred.await keep the process alive?
import { Deferred, Effect } from "effect"
const t0 = Date.now()
process.on("exit", (code) => console.log(`process exit code=${code} after ~${Date.now() - t0}ms; promise settled=${settled}`))
let settled = false
Effect.runPromise(Effect.flatMap(Deferred.make<void>(), Deferred.await)).then(() => { settled = true })
