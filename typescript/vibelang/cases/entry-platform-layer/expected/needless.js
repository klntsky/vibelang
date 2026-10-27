import * as Console from "effect/Console";
import * as Effect from "effect/Effect";
import * as runtime from "vibelang-runtime";
import { NodeRuntime as NodeRuntime } from "@effect/platform-node";
export function main() {
    return Effect.fnUntraced(function* main() {
        yield* Console.log("needs no service");
    }).apply(this, arguments);
}
runtime.$entry(import.meta.main, import.meta.url, main, () => NodeRuntime.runMain(main()));
