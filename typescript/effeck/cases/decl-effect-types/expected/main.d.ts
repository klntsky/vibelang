import type { Effect, Scope } from "effeck-runtime";
declare const NotFound_base: new <A extends Record<string, any> = {}>(args: import("effect/Types", { with: { "resolution-mode": "import" } }).VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause", { with: { "resolution-mode": "import" } }).YieldableError & {
    readonly _tag: "NotFound";
} & Readonly<A>;
declare class NotFound extends NotFound_base<{
    readonly id: string;
}> {
}
declare const Timeout_base: new <A extends Record<string, any> = {}>(args: import("effect/Types", { with: { "resolution-mode": "import" } }).VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause", { with: { "resolution-mode": "import" } }).YieldableError & {
    readonly _tag: "Timeout";
} & Readonly<A>;
declare class Timeout extends Timeout_base<{}> {
}
export declare function find(id: string): Effect.Effect<string, NotFound, never>;
export declare function lookup(id: string): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<string, NotFound | Timeout, never>;
export declare function count(): Effect.Effect<number, never, never>;
export type Finder = (id: string) => Effect.Effect<string, NotFound, never>;
export type Scoped = Effect.Effect<string, NotFound, Scope>;
export type Values = ReadonlyArray<Effect.Effect<number, never, never>>;
export declare function main(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<{
    found: string;
    slow: string;
    missing: string;
    count: number;
    plain: string;
}, NotFound | Timeout, never>;
export {};
