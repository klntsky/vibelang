import * as Effect_1 from "effect/Effect";
import { Console, Data, Effect } from "effect";
class NotFound extends Data.TaggedError("NotFound") {
}
const names = ["ada", "grace", "barbara"];
// No annotations: lookup is Effect<string, NotFound, never>.
function lookup(arg_1) {
    return Effect_1.fnUntraced(function* lookup(id) {
        if (id >= names.length)
            yield* Effect.fail(new NotFound({ id }));
        return names[id];
    }).apply(this, arguments);
}
// main is Effect<void, NotFound, never>: lookup's error flows into it.
export function main() {
    return Effect_1.fnUntraced(function* main() {
        yield* Console.log(`found ${yield* lookup(1)}`);
        const missing = yield* lookup(7); // fails, so main stops here
        yield* Console.log(`not printed: ${missing}`);
    }).apply(this, arguments);
}
// The module top level is not inside a program, so a runner starts one: it
// receives a program, a function with no parameters (D7, D38).
Effect.runPromise(Effect_1.suspend(() => Effect.catch(Effect_1.suspend(() => main()), (e) => Effect_1.suspend(() => { return Console.log(`caught ${e._tag} ${e.id}`); }))));
