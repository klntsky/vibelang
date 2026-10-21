// Module declarations lower inline (D303): each module the file imports
// becomes a function, called once before the file's own code, whose result
// holds the module's exports. Its imports move to the file's top level as
// namespace imports. A module the file imports only types from, and one it
// never imports, emit nothing; each module's declarations are a declaration
// file of their own, main.vibe.<name>.d.ts.
import * as moduleRuntime_1 from "vibelang-runtime";
import * as more_ts_1 from "./more.js";
const text_1 = (() => {
    let Mode;
    (function (Mode) {
        Mode[Mode["Trim"] = 0] = "Trim";
        Mode[Mode["Upper"] = 1] = "Upper";
    })(Mode || (Mode = {}));
    function clean(value, mode = Mode.Trim) {
        const trimmed = value.trim();
        return mode === Mode.Upper ? trimmed.toUpperCase() : (0, more_ts_1.twice)(trimmed);
    }
    function area(shape) {
        return shape.width * shape.height;
    }
    const label = `${more_ts_1.more}:`;
    class default_1 {
        count = 0;
        increment() {
            return ++this.count;
        }
    }
    return moduleRuntime_1.$freeze({
        __proto__: null,
        double: more_ts_1.twice,
        moreModule: more_ts_1,
        Mode: Mode,
        clean: clean,
        area: area,
        prefix: label,
        default: default_1
    });
})();
const config_1 = await (async () => {
    // A module's top level may await; the file then awaits the module.
    const level = await Promise.resolve(3);
    return moduleRuntime_1.$freeze({
        __proto__: null,
        level: level
    });
})();
const setup_1 = (() => {
    console.log("setup ran");
    return moduleRuntime_1.$freeze({
        __proto__: null
    });
})();
const extras_1 = (() => {
    const one = 1;
    function two() {
        return 2;
    }
    const default_2 = "not re-exported";
    return moduleRuntime_1.$freeze({
        __proto__: null,
        one: one,
        two: two,
        default: default_2
    });
})();
const { default: Counter, clean, Mode } = text_1;
const tools = text_1;
const { level } = config_1;
const tidy_1 = text_1.clean;
export { tidy_1 as tidy };
const one_1 = extras_1.one, two_1 = extras_1.two;
export { one_1 as one, two_1 as two };
import * as Console from "effect/Console";
import * as Effect_1 from "effect/Effect";
import * as runtime from "vibelang-runtime";
export function describe$direct(shape) {
    return `${shape.width}x${shape.height}`;
}
export function describe(...args) { return Effect_1.sync(() => describe$direct(...args)); }
describe[runtime.$direct] = describe$direct;
export function main() {
    return Effect_1.fnUntraced(function* main() {
        const counter = yield* Effect_1.sync(() => new Counter());
        const counted = yield* Effect_1.sync(((counter) => counter.increment()).bind(undefined, counter));
        yield* Console.log(JSON.stringify({
            cleaned: yield* Effect_1.sync(() => clean(" ab ")),
            upper: yield* Effect_1.sync(() => clean(" ab ", Mode.Upper)),
            doubled: yield* Effect_1.sync(() => tools.double("x")),
            prefix: tools.prefix,
            more: tools.moreModule.more,
            area: yield* Effect_1.sync(() => tools.area({ width: 2, height: 3 })),
            level,
            counted,
            described: describe$direct({ width: 1, height: 2 }),
        }));
    }).apply(this, arguments);
}
if (import.meta.main)
    runtime.$runMain(main());
