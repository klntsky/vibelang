import { Effect, Stream } from "effect"
import { count, make, pick } from "./out/lib.js"

const settle = (program) => Effect.runSync(Effect.catch(program, (e) => Effect.succeed(`failed: ${e.message}`)))
console.log(JSON.stringify([settle(make("ada")), settle(make("")), settle(pick("bob")), settle(pick(""))]))
console.log(JSON.stringify(Effect.runSync(Stream.runCollect(count()))))
