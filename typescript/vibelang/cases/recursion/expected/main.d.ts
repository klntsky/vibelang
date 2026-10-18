import type { Effect } from "vibelang-runtime";
declare const Even_base: new <A extends Record<string, any> = {}>(args: import("effect/Types", { with: { "resolution-mode": "import" } }).VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause", { with: { "resolution-mode": "import" } }).YieldableError & {
    readonly _tag: "Even";
} & Readonly<A>;
export declare class Even extends Even_base<{
    readonly n: number;
}> {
}
declare const Odd_base: new <A extends Record<string, any> = {}>(args: import("effect/Types", { with: { "resolution-mode": "import" } }).VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause", { with: { "resolution-mode": "import" } }).YieldableError & {
    readonly _tag: "Odd";
} & Readonly<A>;
export declare class Odd extends Odd_base<{
    readonly n: number;
}> {
}
export declare function isOdd(n: number): Effect<boolean, Even | Odd, never>;
export declare function isEven(n: number): Effect<boolean, Even | Odd, never>;
export declare function parity(n: number): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<"Even" | "Odd" | "even" | "odd", never, never>;
declare const Missing_base: new <A extends Record<string, any> = {}>(args: import("effect/Types", { with: { "resolution-mode": "import" } }).VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause", { with: { "resolution-mode": "import" } }).YieldableError & {
    readonly _tag: "Missing";
} & Readonly<A>;
export declare class Missing extends Missing_base<{
    readonly path: string;
}> {
}
export declare function totalSize(path: string): Effect<number, Missing, never>;
export declare function main(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<{
    parity: string[];
    size: number;
}, Missing, never>;
export {};
