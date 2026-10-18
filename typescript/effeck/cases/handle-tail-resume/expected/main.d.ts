export declare function one(version: string): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<"no" | "yes", never, never>;
export declare function two(version: string): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<"no" | "yes", never, never>;
export declare function three(version: string): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<"no" | "stopped" | "yes", never, never>;
export declare function main(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<{
    one: string;
    two: string;
    three: string;
}, never, never>;
