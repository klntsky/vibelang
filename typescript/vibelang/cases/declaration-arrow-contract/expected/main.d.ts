import type { Effect } from "vibelang-runtime";
export declare const Digest: import("effect/Schema").brand<import("effect/Schema").String, "Digest">;
export type Digest = typeof Digest.Type;
declare const Empty_base: new <A extends Record<string, any> = {}>(args: import("effect/Types").VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause").YieldableError & {
    readonly _tag: "Empty";
} & Readonly<A>;
export declare class Empty extends Empty_base<{}> {
}
export declare const parse: (input: string) => Effect.Effect<Digest, Empty, never>;
export declare const parseExpression: (input: string) => Effect.Effect<Digest, Empty, never>;
export declare const codec: {
    parse(input: string): Effect.Effect<Digest, Empty, never>;
};
export declare function parseDeclaration(input: string): Effect.Effect<Digest, Empty, never>;
export declare function main(): Effect.Effect<string, Empty, never>;
export {};
