import type { Effect } from "effeck-runtime";
declare const Gone_base: new <A extends Record<string, any> = {}>(args: import("effect/Types", { with: { "resolution-mode": "import" } }).VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause", { with: { "resolution-mode": "import" } }).YieldableError & {
    readonly _tag: "Gone";
} & Readonly<A>;
declare class Gone extends Gone_base<{}> {
}
interface Needed {
    readonly id: string;
    readonly "~effeck/service"?: "contract-wider-callers/main#Needed";
}
declare const Needed_base: import("effeck-runtime").ServiceKey<Needed>;
declare abstract class Needed extends Needed_base {
}
export declare function longForm(): Effect<number, Gone, Needed>;
export declare function shortForm(): Effect.Effect<number, Gone, Needed>;
export declare function plain(): Effect.Effect<number, never, never>;
export declare function callerOfLong(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<number, Gone, Needed>;
export declare function callerOfShort(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<number, Gone, Needed>;
export declare function callerOfPlain(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<number, never, never>;
export declare function callerOfMethod(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<number, Gone, Needed>;
export declare function main(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<number[], Gone, never>;
export {};
