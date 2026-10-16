import * as Effect_1 from "effect/Effect";
import * as runtime from "@effectscript/runtime";
// A concise arrow body that the lowering turns into a block keeps the
// comment after it outside the block, and the file keeps its last comment.
import { Effect } from "effect";
class Settings extends runtime.$service("es-concise-comments/main#Settings") {
}
const findName = Effect_1.fnUntraced(function* findName(id) {
    return id === 1 ? yield* Effect.succeed("ada") : yield* Effect.succeed("grace");
});
export const greeting = Effect_1.fnUntraced(function* greeting() { return (yield* Settings).greeting; });
// Effect<string, never, Settings>
export const nameLength = Effect_1.fnUntraced(function* nameLength() { return (yield* findName(1)).length; });
// Effect<number, never, never>
