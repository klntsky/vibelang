import type { Effect, Scope } from "vibelang-runtime";
export { NotFound } from "./users.js";
export declare const one: Effect<number>;
export declare const scoped: Effect<void, never, Scope>;
export declare function main(): import("effect/Effect").Effect<string, import("./users.js").NotFound, never>;
