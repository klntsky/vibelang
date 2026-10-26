import type { Effect } from "vibelang-runtime";
import type { Shape } from "./main.vibe.shapes.js";
import "./main.vibe.setup.js";
export { clean as tidy } from "./main.vibe.text.js";
export * from "./main.vibe.extras.js";
export declare function describe(shape: Shape): Effect<string>;
export declare function main(): import("effect/Effect").Effect<void, never, never>;
