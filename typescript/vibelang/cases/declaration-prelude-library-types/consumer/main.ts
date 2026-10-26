import { Context, Effect } from "effect"
import { decodeName, done, makeContext } from "../out/main.js"
// Each parameter keeps its type: an unbound name in the declaration file
// would be an error here, and an any would let the @ts-expect-error lines pass.
const zero: number = done(null)
// @ts-expect-error a bigint is not JSON
done(1n)
const name: Effect.Effect<string, unknown> = decodeName("ada", { errors: "all" })
// @ts-expect-error errors is "first" or "all"
decodeName("ada", { errors: "some" })
const Port = Context.Service<"Port", number>("Port")
const context: Context.Context<"Port"> = makeContext(Port, 8080)
// @ts-expect-error the service of Port is a number
makeContext(Port, "8080")
export { zero, name, context }
