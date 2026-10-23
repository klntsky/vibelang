import * as Effect_1 from "effect/Effect";
import * as runtime from "vibelang-runtime";
import { Config, Console } from "effect";
import { findName } from "./users.js";
// Pure: no call runs an Effect, so it stays a plain JavaScript function
// that Array.prototype.map can call.
function double(n) {
    return n * 2;
}
// Console.log returns an Effect, so calling it runs it, and greet becomes
// an Effect itself.
function greet(arg_1) {
    return Effect_1.fnUntraced(function* greet(name) {
        yield* Console.log(`hello, ${name}`);
        return name.length;
    }).apply(this, arguments);
}
// An arrow function that runs an Effect becomes one too. Its body is a single
// call, so it returns that call's Effect, suspended until it runs, without a
// generator.
const shout = (text) => Effect_1.suspend({ shout: () => Console.log(text.toUpperCase()) }.shout);
export function main() {
    return Effect_1.fnUntraced(function* main() {
        for (const id of [0, 1]) {
            const length = yield* greet(`user ${id}`); // a .vibe Effect: runs
            yield* Console.log(`greeted with ${length} letters`);
        }
        yield* shout("calls run in order");
        const total = (yield* greet("a")) + (yield* greet("bc")); // operands run left to right
        yield* Console.log(`total ${total}`);
        yield* Console.log(`doubled: ${[1, 2, 3].map(double).join(", ")}`); // plain calls stay plain
        // Config extends Effect, but only Effect itself runs: a Config is data (design D24).
        const port = Config.Port("PORT");
        yield* Console.log(`config is data: ${Config.isConfig(port)}`);
        // findName returns Effect<string>, so the call runs it and name is a string.
        const name = yield* findName(2);
        yield* Console.log(`found ${name.toUpperCase()}`);
    }).apply(this, arguments);
}
runtime.$entry(import.meta.main, import.meta.url, main, () => runtime.$runMain(main()));
