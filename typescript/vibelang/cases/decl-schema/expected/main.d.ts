import type { Schema } from "vibelang-runtime";
export interface Address extends Schema.Schema.Type<typeof Address> {
}
export declare const Address: import("effect/Schema").Struct<{
    readonly city: import("effect/Schema").String;
    readonly zip: import("effect/Schema").optional<import("effect/Schema").String>;
}>;
export interface User extends Schema.Schema.Type<typeof User> {
}
export declare const User: import("effect/Schema").Struct<{
    readonly id: import("effect/Schema").String;
    readonly name: import("effect/Schema").String;
    readonly nickname: import("effect/Schema").optional<import("effect/Schema").String>;
    readonly role: import("effect/Schema").Literals<readonly ["admin", "member"]>;
    readonly tags: import("effect/Schema").$Array<import("effect/Schema").String>;
    readonly home: import("effect/Schema").Struct<{
        readonly city: import("effect/Schema").String;
        readonly zip: import("effect/Schema").optional<import("effect/Schema").String>;
    }>;
    readonly score: import("effect/Schema").Tuple<readonly [import("effect/Schema").Number, import("effect/Schema").String]>;
    readonly counts: import("effect/Schema").$Record<import("effect/Schema").String, import("effect/Schema").Number>;
    readonly verified: import("effect/Schema").Union<readonly [import("effect/Schema").Boolean, import("effect/Schema").Null]>;
    readonly login: import("effect/Schema").Struct<{
        readonly at: import("effect/Schema").Number;
    }>;
}>;
export declare function main(): import("effect/Effect").Effect<{
    a: string | {
        readonly id: string;
        readonly name: string;
        readonly nickname?: string | undefined;
        readonly role: "admin" | "member";
        readonly tags: readonly string[];
        readonly home: {
            readonly city: string;
            readonly zip?: string | undefined;
        };
        readonly score: readonly [number, string];
        readonly counts: {
            readonly [x: string]: number;
        };
        readonly verified: boolean | null;
        readonly login: {
            readonly at: number;
        };
    };
    b: string | {
        readonly id: string;
        readonly name: string;
        readonly nickname?: string | undefined;
        readonly role: "admin" | "member";
        readonly tags: readonly string[];
        readonly home: {
            readonly city: string;
            readonly zip?: string | undefined;
        };
        readonly score: readonly [number, string];
        readonly counts: {
            readonly [x: string]: number;
        };
        readonly verified: boolean | null;
        readonly login: {
            readonly at: number;
        };
    };
    c: boolean;
}, never, never>;
