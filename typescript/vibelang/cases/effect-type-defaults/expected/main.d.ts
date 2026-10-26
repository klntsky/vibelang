import type { Effect } from "vibelang-runtime";
declare const Gone_base: new <A extends Record<string, any> = {}>(args: import("effect/Types").VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause").YieldableError & {
    readonly _tag: "Gone";
} & Readonly<A>;
declare class Gone extends Gone_base<{
    readonly id: string;
}> {
}
interface Users {
    find(id: string): Effect<string, Gone>;
    count(): Effect<number>;
    all(): Effect<ReadonlyArray<string>, never, never>;
    readonly "~vibelang/service"?: "effect-type-defaults/main#Users";
}
declare const Users_base: import("vibelang-runtime").ServiceKey<Users>;
declare abstract class Users extends Users_base {
}
export declare function plain(): Effect<number>;
export declare function failing(id: string): Effect<string, Gone>;
export declare function found(id: string): Effect<string, Gone, Users>;
export declare function guest(id: string): Effect<string, never, Users>;
export declare function inferred(id: string): import("effect/Effect").Effect<string, Gone, Users>;
export declare function onlyErrors(id: string): import("effect/Effect").Effect<string, Gone, never>;
export declare function partial(id: string): import("effect/Effect").Effect<string, Gone, Users>;
export declare function run(program: Effect<number>): Effect<number>;
export declare function main(): import("effect/Effect").Effect<{
    plain: number;
    failing: string;
    found: string;
    guest: string;
    inferred: string;
    onlyErrors: string;
    partial: string;
    count: number;
    all: readonly string[];
    ran: number;
}, Gone, never>;
export {};
