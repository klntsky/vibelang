// TypeScript and JavaScript callers see main as a function returning an Effect.
import { Effect } from "effect"
import { main } from "./out/main.js"

await Effect.runPromise(main())
