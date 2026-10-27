import * as Console from "effect/Console";
import * as Effect from "effect/Effect";
import * as runtime from "vibelang-runtime";
import { NodeServices as NodeServices } from "@effect/platform-node";
import { NodeRuntime as NodeRuntime } from "@effect/platform-node";
import { FileSystem } from "effect/FileSystem";
export function main() {
    return Effect.fnUntraced(function* main() {
        yield* Console.log(yield* (yield* FileSystem).readFileString("effect.txt"));
    }).apply(this, arguments);
}
runtime.$entry(import.meta.main, import.meta.url, main, () => NodeRuntime.runMain(Effect.provide(main(), NodeServices.layer)));
