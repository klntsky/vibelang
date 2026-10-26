import type { Effect } from "vibelang-runtime";
declare const NotFound_base: new <A extends Record<string, any> = {}>(args: import("effect/Types").VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause").YieldableError & {
    readonly _tag: "NotFound";
} & Readonly<A>;
declare class NotFound extends NotFound_base<{
    readonly id: string;
}> {
}
interface Users {
    find(id: string): Effect.Effect<string, NotFound, never>;
    count(): Effect<number>;
    readonly "~vibelang/service"?: "contract-spellings-mixed/main#Users";
}
declare const Users_base: import("vibelang-runtime").ServiceKey<Users>;
declare abstract class Users extends Users_base {
}
type Lookup = (id: string) => Effect<string, NotFound, Users>;
type Counted = Effect.Effect<number, never, never>;
export declare function terse(id: string): Effect.Effect<string, NotFound, Users>;
export declare function spelled(id: string): Effect<string, NotFound, Users>;
export declare function guest(lookup: Lookup, id: string): Effect.Effect<string, never, Users>;
export declare function add(program: Counted, extra: Effect<number>): Effect<number, never, Users>;
export declare function main(): import("effect/Effect").Effect<{
    terse: string;
    spelled: string;
    guest: string;
    count: number;
    sum: number;
}, NotFound, never>;
export {};
