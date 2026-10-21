declare const FsError_base: import("effect/Schema").Class<FsError, import("effect/Schema").TaggedStruct<string, {
    readonly method: import("effect/Schema").String;
    readonly path: import("effect/Schema").optional<import("effect/Schema").String>;
}>, import("effect/Cause").YieldableError>;
/** The base of a family of file system failures. */
export declare class FsError extends FsError_base {
}
declare const NotFound_base: import("effect/Schema").Class<NotFound, import("effect/Schema").Struct<{
    readonly method: import("effect/Schema").String;
    readonly path: import("effect/Schema").optional<import("effect/Schema").String>;
    _tag: import("effect/Schema").tag<"NotFound">;
    attempts: import("effect/Schema").Number;
}>, FsError> & Pick<{}, never>;
export declare class NotFound extends NotFound_base {
}
declare const Locked_base: import("effect/Schema").Class<Locked, import("effect/Schema").TaggedStruct<"Locked", {
    readonly holder: import("effect/Schema").String;
}>, import("effect/Cause").YieldableError>;
export declare class Locked extends Locked_base {
    describe(): string;
}
export declare function main(): import("effect/Effect").Effect<{
    tag: "NotFound";
    name: string;
    encoded: {
        readonly method: string;
        readonly path?: string | undefined;
        readonly _tag: "NotFound";
        readonly attempts: number;
    };
    decoded: {
        tag: "NotFound";
        method: string;
        family: boolean;
    };
    base: {
        tag: string;
        encoded: {
            readonly _tag: string;
            readonly method: string;
            readonly path?: string | undefined;
        };
    };
    isNotFound: boolean;
    routed: string;
    locked: string;
}, never, never>;
export {};
