export declare function one(version: string): import("effect/Effect").Effect<"no" | "yes", never, never>;
export declare function two(version: string): import("effect/Effect").Effect<"no" | "yes", never, never>;
export declare function three(version: string): import("effect/Effect").Effect<"no" | "stopped" | "yes", never, never>;
export declare function main(): import("effect/Effect").Effect<{
    one: string;
    two: string;
    three: string;
}, never, never>;
