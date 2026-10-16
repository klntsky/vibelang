import * as Effect from "effect/Effect";
import { Console } from "effect";
// A function that makes a call becomes an Effect. Calls do not run yet, so
// the Console.log below builds an Effect and drops it.
export const answer = Effect.fnUntraced(function* () {
    Console.log("computing the answer");
    return 42;
});
export const half = Effect.fnUntraced(function* (n) {
    Console.log(`halving ${n}`);
    return n / 2;
});
