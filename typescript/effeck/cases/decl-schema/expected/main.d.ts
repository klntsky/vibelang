import type { Schema } from "effeck-runtime";
export interface Address extends Schema.Schema.Type<typeof Address> {
}
export declare const Address: import("effect/Schema", { with: { "resolution-mode": "import" } }).Struct<{
    readonly city: import("effect/Schema", { with: { "resolution-mode": "import" } }).String;
    readonly zip: import("effect/Schema", { with: { "resolution-mode": "import" } }).optional<import("effect/Schema", { with: { "resolution-mode": "import" } }).String>;
}>;
export interface User extends Schema.Schema.Type<typeof User> {
}
export declare const User: import("effect/Schema", { with: { "resolution-mode": "import" } }).Struct<{
    readonly id: import("effect/Schema", { with: { "resolution-mode": "import" } }).String;
    readonly name: import("effect/Schema", { with: { "resolution-mode": "import" } }).String;
    readonly nickname: import("effect/Schema", { with: { "resolution-mode": "import" } }).optional<import("effect/Schema", { with: { "resolution-mode": "import" } }).String>;
    readonly role: import("effect/Schema", { with: { "resolution-mode": "import" } }).Literals<readonly ["admin", "member"]>;
    readonly tags: import("effect/Schema", { with: { "resolution-mode": "import" } }).$Array<import("effect/Schema", { with: { "resolution-mode": "import" } }).String>;
    readonly home: import("effect/Schema", { with: { "resolution-mode": "import" } }).Struct<{
        readonly city: import("effect/Schema", { with: { "resolution-mode": "import" } }).String;
        readonly zip: import("effect/Schema", { with: { "resolution-mode": "import" } }).optional<import("effect/Schema", { with: { "resolution-mode": "import" } }).String>;
    }>;
    readonly score: import("effect/Schema", { with: { "resolution-mode": "import" } }).Tuple<readonly [import("effect/Schema", { with: { "resolution-mode": "import" } }).Number, import("effect/Schema", { with: { "resolution-mode": "import" } }).String]>;
    readonly counts: import("effect/Schema", { with: { "resolution-mode": "import" } }).$Record<import("effect/Schema", { with: { "resolution-mode": "import" } }).String, import("effect/Schema", { with: { "resolution-mode": "import" } }).Number>;
    readonly verified: import("effect/Schema", { with: { "resolution-mode": "import" } }).Union<readonly [import("effect/Schema", { with: { "resolution-mode": "import" } }).Boolean, import("effect/Schema", { with: { "resolution-mode": "import" } }).Null]>;
    readonly login: import("effect/Schema", { with: { "resolution-mode": "import" } }).Struct<{
        readonly at: import("effect/Schema", { with: { "resolution-mode": "import" } }).Number;
    }>;
}>;
export declare function main(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<{
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
