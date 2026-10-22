import * as Option from "effect/Option";
import { $noMatch } from "vibelang-runtime";
import * as Effect from "effect/Effect";
import * as runtime from "vibelang-runtime";
// expect-result: {"phases":["Draft","Review","Published"],"widths":[5,6,9],"logged":["draft","review","published"],"failures":["no user 7","retry","denied Ada"],"greetings":["Hello","Hello, Ada"],"announced":"published","text":"text: hi"}
// A match without default that D209 proves exhaustive runs its last arm
// without testing it (D294), or with a check that adds no branch when an arm
// tests a class: no final if and no $noMatch, after return, inside an
// expression, as a statement, and in an Effect function, and the last arm
// still binds its names (arms.vibe, expected/arms.js). Over a type
// that is not a finite union, a match without default keeps its tests and
// $noMatch, which throws the proposal's TypeError (D209), as text does here.
import { announce, Denied, describe, greeting, NotFound, phaseLabel, record, Timeout, width } from "./arms.js";
import { record$direct, phaseLabel$direct, width$direct, describe$direct, greeting$direct } from "./arms.js";
function text$direct(value) {
    {
        {
            if (typeof value === "string") {
                let s = value;
                return `text: ${s}`;
            }
        }
        return $noMatch(value);
    }
}
function text(...args) { return Effect.sync(() => text$direct(...args)); }
text[runtime.$direct] = text$direct;
export function main() {
    return Effect.fnUntraced(function* main() {
        const phases = ["draft", "review", "published"];
        const logged = [];
        for (const phase of phases)
            record$direct(phase, logged);
        return {
            phases: phases.map(phaseLabel$direct),
            widths: phases.map(width$direct),
            logged,
            failures: [describe$direct(new NotFound({ id: "7" })), describe$direct(new Timeout()), describe$direct(new Denied({ user: "Ada" }))],
            greetings: [greeting$direct(Option.none()), greeting$direct(Option.some("Ada"))],
            announced: yield* announce("published"),
            text: text$direct("hi")
        };
    }).apply(this, arguments);
}
if (import.meta.main)
    runtime.$runMain(main());
