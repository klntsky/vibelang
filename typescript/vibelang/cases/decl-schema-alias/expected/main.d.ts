export type Code = typeof Code.Type;
export declare const Code: import("effect/Schema").Literals<readonly ["root_missing", "read_failed"]>;
export type Port = typeof Port.Type;
export declare const Port: import("effect/Schema").Number;
export declare function main(): import("effect/Effect").Effect<{
    code: string;
    invalid: string;
    described: string;
    port: boolean;
    id: boolean[];
    tags: readonly string[];
    names: boolean;
    pair: readonly [string, number];
    counts: {
        readonly [x: string]: number;
    };
    point: {
        readonly x: number;
        readonly y?: number | undefined;
    };
    home: {
        readonly city: string;
    };
    status: boolean[];
    flag: boolean;
    version: number;
}, import("effect/Schema").SchemaError, never>;
