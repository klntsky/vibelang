import type { Effect } from "@effectscript/runtime";
declare const UserNotFound_base: new <A extends Record<string, any> = {}>(args: import("effect/Types", { with: { "resolution-mode": "import" } }).VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause", { with: { "resolution-mode": "import" } }).YieldableError & {
    readonly _tag: "UserNotFound";
} & Readonly<A>;
export declare class UserNotFound extends UserNotFound_base<{
    readonly id: string;
}> {
}
declare const Invalid_base: new <A extends Record<string, any> = {}>(args: import("effect/Types", { with: { "resolution-mode": "import" } }).VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause", { with: { "resolution-mode": "import" } }).YieldableError & {
    readonly _tag: "Invalid";
} & Readonly<A>;
export declare class Invalid extends Invalid_base<{
    readonly raw: string;
}> {
}
export interface User {
    readonly id: string;
    readonly name: string;
}
export declare function find(id: string): Effect<User, UserNotFound, never>;
export declare function wide(id: string): Effect<string, UserNotFound | Invalid, never>;
export declare function answer(): Effect<number, never, never>;
export declare function point(): Effect<{
    readonly x: number;
    readonly kind: "point";
}, never, never>;
export declare function greeting(id: string): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<string, UserNotFound, never>;
export declare function safe(id: string): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<string, never, never>;
export declare function named(id: string): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<{
    id: string;
    name: string;
}, UserNotFound, never>;
export declare function everything(id: string): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<User, UserNotFound, never>;
export declare const twice: (n: number) => number;
export declare function main(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<{
    find: string;
    wide: string;
    answer: number;
    point: "point";
    greeting: string;
    safe: string;
    named: string;
    everything: string;
    twice: number;
}, Invalid | UserNotFound, never>;
export {};
