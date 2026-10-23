import { Effect } from "effect"
import { shop } from "./shop.vibe"
// script.vibe exports a main, which a bundle of this module never runs (D135).
import "./script.vibe"

Effect.runPromise(shop()).then((lines) => console.log(lines.join("\n")))
