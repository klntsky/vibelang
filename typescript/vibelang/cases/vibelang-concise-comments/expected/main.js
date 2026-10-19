import * as Effect_1 from "effect/Effect";
import * as runtime from "vibelang-runtime";
// A concise arrow body that the lowering turns into a block keeps the
// comment after it outside the block, and the file keeps its last comment.
import { Effect } from "effect";
class Settings extends runtime.$service("vibelang-concise-comments/main#Settings") {
}
function findName(arg_1) {
    return Effect_1.fnUntraced(function* findName(id) {
        return id === 1 ? yield* Effect.succeed("ada") : yield* Effect.succeed("grace");
    }).apply(this, arguments);
}
export const greeting = Effect_1.fnUntraced(function* greeting() { return (yield* Settings).greeting; });
// Effect<string, never, Settings>
export const nameLength = Effect_1.fnUntraced(function* nameLength() { return (yield* findName(1)).length; });
// Effect<number, never, never>
