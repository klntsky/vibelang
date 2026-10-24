import type { Effect } from "vibelang-runtime";
declare const NotFound_base: new <A extends Record<string, any> = {}>(args: import("effect/Types").VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause").YieldableError & {
    readonly _tag: "NotFound";
} & Readonly<A>;
declare class NotFound extends NotFound_base<{
    readonly id: string;
}> {
}
export declare function mapAll<A, B, E = never, R = never>(items: ReadonlyArray<A>, f: (a: A) => Effect<B, E, R>): import("effect/Effect").Effect<B[], E, R>;
export declare function mapSafe<A, B>(items: ReadonlyArray<A>, f: (a: A) => Effect<B, never, never>): import("effect/Effect").Effect<B[], never, never>;
export declare function withRetry<A, E = never, R = never>(times: number, program: () => Effect<A, E, R>): import("effect/Effect").Effect<A, E, R>;
export declare function main(): import("effect/Effect").Effect<{
    names: string[];
    sizes: number[];
    safe: number[];
    retried: number;
    caught: string;
}, NotFound, never>;
export {};
