import { Effect } from "effect"
import { shop } from "./shop.effeck"

Effect.runPromise(shop()).then((lines) => console.log(lines.join("\n")))
