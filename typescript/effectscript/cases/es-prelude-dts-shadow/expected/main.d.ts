import type { Effect as Effect_1 } from "@effectscript/runtime";
import { Effect } from "effect";
declare const Missing_base: new <A extends Record<string, any> = {}>(args: import("effect/Types", { with: { "resolution-mode": "import" } }).VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause", { with: { "resolution-mode": "import" } }).YieldableError & {
    readonly _tag: "Missing";
} & Readonly<A>;
export declare class Missing extends Missing_base<{
    readonly id: number;
}> {
}
export interface Names {
    lookup(id: number): Effect_1<string, Missing, never>;
    readonly "~esc/service"?: "es-prelude-dts-shadow/main#Names";
}
declare const Names_base: import("@effectscript/runtime").ServiceKey<Names>;
export declare abstract class Names extends Names_base {
}
export declare const NamesLive: import("effect/Layer", { with: { "resolution-mode": "import" } }).Layer<Names, never, never>;
export declare function fallback(): Effect_1<string, never, never>;
export declare function greet(id: number): Effect.Effect<string, Missing, Names>;
export {};
