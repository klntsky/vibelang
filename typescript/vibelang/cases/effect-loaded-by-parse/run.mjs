import { Effect } from "effect"
import { component, global, line } from "./out/lib.js"

const settle = (program) => Effect.runSync(Effect.catch(program, (e) => Effect.succeed(`failed: ${e.name}`)))
console.log(JSON.stringify([settle(line("[1]")), settle(line("{")), settle(component("a%20b")), settle(component("%")), settle(global("2"))]))
