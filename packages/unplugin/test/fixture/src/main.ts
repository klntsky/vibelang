import { Effect } from "effect"
import { shop } from "./shop.vibe"

Effect.runPromise(shop()).then((lines) => console.log(lines.join("\n")))
