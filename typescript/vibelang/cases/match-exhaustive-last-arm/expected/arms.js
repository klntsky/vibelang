import * as Effect from "effect/Effect";
import * as Option from "effect/Option";
import { $is, TaggedError } from "vibelang-runtime";
import * as Effect_1 from "effect/Effect";
import * as runtime from "vibelang-runtime";
export class NotFound extends TaggedError("NotFound") {
}
export class Timeout extends TaggedError("Timeout") {
}
export class Denied extends TaggedError("Denied") {
}
export function phaseLabel$direct(phase) {
    {
        {
            if (phase
                === "draft") {
                return "Draft";
            }
        }
        {
            if (phase
                === "review") {
                return "Review";
            }
        }
        {
            {
                return "Published";
            }
        }
    }
}
// After return.
export function phaseLabel(...args) { return Effect_1.sync(() => phaseLabel$direct(...args)); }
phaseLabel[runtime.$direct] = phaseLabel$direct;
export function width$direct(phase) {
    const label = (() => {
        {
            if (phase
                === "draft") {
                return 5;
            }
        }
        {
            if (phase
                === "review") {
                return 6;
            }
        }
        {
            {
                return 9;
            }
        }
    })();
    return label;
}
// Inside an expression, where the match runs in its own function.
export function width(...args) { return Effect_1.sync(() => width$direct(...args)); }
width[runtime.$direct] = width$direct;
export function record$direct(phase, log) {
    $match1: {
        {
            if (phase
                === "draft") {
                log.push("draft");
                break $match1;
            }
        }
        {
            if (phase
                === "review") {
                log.push("review");
                break $match1;
            }
        }
        {
            {
                log.push("published");
                break $match1;
            }
        }
    }
}
// As a statement.
export function record(...args) { return Effect_1.sync(() => record$direct(...args)); }
record[runtime.$direct] = record$direct;
export function describe$direct(failure) {
    {
        {
            if ($is(NotFound, failure) && typeof failure === "object" && failure
                !== null && "id" in
                failure) {
                let id = failure.id;
                return `no user ${id}`;
            }
        }
        {
            if ($is(Timeout, failure)) {
                return "retry";
            }
        }
        {
            {
                let user = failure.user;
                return `denied ${user}`;
            }
        }
    }
}
// The last arm still binds user and n.
export function describe(...args) { return Effect_1.sync(() => describe$direct(...args)); }
describe[runtime.$direct] = describe$direct;
export function greeting$direct(name) {
    {
        {
            if (Option.isNone(name)) {
                return "Hello";
            }
        }
        {
            {
                let n = name.value;
                return `Hello, ${n}`;
            }
        }
    }
}
export function greeting(...args) { return Effect_1.sync(() => greeting$direct(...args)); }
greeting[runtime.$direct] = greeting$direct;
// In an Effect function, whose arms run Effects.
export function announce(arg_1) {
    return Effect_1.fnUntraced(function* announce(phase) {
        $match2: {
            {
                if (phase
                    === "draft") {
                    yield* Effect.log("draft");
                    break $match2;
                }
            }
            {
                if (phase
                    === "review") {
                    yield* Effect.log("review");
                    break $match2;
                }
            }
            {
                {
                    yield* Effect.log("published");
                    break $match2;
                }
            }
        }
        return phase;
    }).apply(this, arguments);
}
