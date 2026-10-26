import * as Data from "effect/Data";
import * as Effect from "effect/Effect";
import * as runtime from "vibelang-runtime";
const Delivery = Data.taggedEnum();
function describe(delivery) {
    {
        const $match2 = delivery;
        {
            if (Delivery.$is("Waiting")($match2)) {
                return "waiting";
            }
        }
        {
            {
                let tracking = $match2.tracking;
                return tracking;
            }
        }
    }
}
function isSent(value) {
    return (Delivery.$is("Sent")(value));
}
export function main$direct() {
    return [describe(Delivery.Waiting()), describe(Delivery.Sent({ tracking: "parcel-8" })), isSent(Delivery.Sent({ tracking: "x" })), isSent({ tracking: "x" })];
}
export function main(...args) { return Effect.sync(() => main$direct(...args)); }
main[runtime.$direct] = main$direct;
runtime.$entry(import.meta.main, import.meta.url, main, () => runtime.$runMain(Effect.sync(main)));
