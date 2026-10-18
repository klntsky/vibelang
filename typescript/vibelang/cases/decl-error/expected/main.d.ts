declare const NotFound_base: new <A extends Record<string, any> = {}>(args: import("effect/Types", { with: { "resolution-mode": "import" } }).VoidIfEmpty<{ readonly [P in keyof A as P extends "_tag" ? never : P]: A[P]; }>) => import("effect/Cause", { with: { "resolution-mode": "import" } }).YieldableError & {
    readonly _tag: "NotFound";
} & Readonly<A>;
export declare class NotFound extends NotFound_base<{
    readonly id: string;
}> {
}
export declare function main(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<{
    timeout: void | string;
    missing: void | string;
    wrapped: number;
    label: string;
}, never, never>;
export {};
