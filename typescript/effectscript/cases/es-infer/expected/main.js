import * as Effect_1 from "effect/Effect";
import { Console, Data, Effect } from "effect";
class NotFound extends Data.TaggedError("NotFound") {
}
const names = ["ada", "grace", "barbara"];
// No annotations: lookup is Effect<string, NotFound, never>.
const lookup = Effect_1.fnUntraced(function* (id) {
    if (id >= names.length)
        yield* Effect.fail(new NotFound({ id }));
    return names[id];
});
// main is Effect<void, NotFound, never>: lookup's error flows into it.
export const main = Effect_1.fnUntraced(function* () {
    yield* Console.log(`found ${yield* lookup(1)}`);
    const missing = yield* lookup(7); // fails, so main stops here
    yield* Console.log(`not printed: ${missing}`);
});
// The module top level is not inside a program, so main() is an Effect here.
Effect.runPromise(Effect.catch(main(), Effect_1.fnUntraced(function* (e) { return yield* Console.log(`caught ${e._tag} ${e.id}`); })));
