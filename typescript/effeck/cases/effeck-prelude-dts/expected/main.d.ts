import type { Effect, Scope } from "@effeck/runtime";
export { NotFound } from "./users.js";
export declare const one: Effect<number>;
export declare const scoped: Effect<void, never, Scope>;
export declare function main(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<string, import("./users.js").NotFound, never>;
