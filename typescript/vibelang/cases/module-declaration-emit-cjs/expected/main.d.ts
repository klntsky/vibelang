import type { Effect } from "vibelang-runtime";
import type { Shape } from "./main.vibe.shapes.js";
import "./main.vibe.setup.js";
export { clean as tidy, join as pathJoin, prefix as "two words", prefix as "__proto__", default as Counter } from "./main.vibe.text.js";
export * as textNs from "./main.vibe.text.js";
export * from "./main.vibe.extras.js";
export declare function describe(shape: Shape): Effect.Effect<string, never, never>;
export declare function program(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<void, never, never>;
