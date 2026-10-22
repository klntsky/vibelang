/**
 * The file's header.
 */
import type { Schema } from "vibelang-runtime";
/** The codes. */
export type Code = typeof Code.Type;
/** The codes. */
export declare const Code: import("effect/Schema").Literals<readonly ["a", "b"]>;
/** A user. */
export interface User extends Schema.Schema.Type<typeof User> {
}
/** A user. */
export declare const User: import("effect/Schema").Struct<{
    /** The id. */
    readonly id: import("effect/Schema").String;
    /** The nickname. */
    readonly nick: import("effect/Schema").optional<import("effect/Schema").String>;
    /** The address. */
    readonly home: import("effect/Schema").Struct<{
        /** The city. */
        readonly city: import("effect/Schema").String;
    }>;
}>;
/** A name. */
export type Name = typeof Name.Type;
/** A name. */
export declare const Name: import("effect/Schema").String;
/** Nothing. */
export type Nothing = typeof Nothing.Type;
/** Nothing. */
export declare const Nothing: import("effect/Schema").Null;
/** After the schemas. */
export declare const after = 1;
export declare function main(): import("effect/Effect").Effect<{
    code: "a" | "b";
    user: {
        readonly id: string;
        readonly nick?: string | undefined;
        readonly home: {
            readonly city: string;
        };
    };
}, import("effect/Schema").SchemaError, never>;
