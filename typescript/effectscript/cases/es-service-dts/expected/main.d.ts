import { Effect, Layer } from "effect";
export interface User {
    readonly id: string;
    readonly name: string;
}
export interface Users {
    find(id: string): Effect.Effect<User, never, never>;
    readonly source: string;
    readonly "~esc/service"?: "es-service-dts/main#Users";
}
declare const Users_base: import("@effectscript/runtime").ServiceKey<Users>;
export declare abstract class Users extends Users_base {
    static readonly layer: Layer.Layer<Users, never, never>;
}
export interface Flags {
    readonly beta: boolean;
    readonly "~esc/service"?: "es-service-dts/main#Flags";
}
declare const Flags_base: import("@effectscript/runtime").ReferenceKey<Flags>;
export declare abstract class Flags extends Flags_base {
    static readonly default: Flags;
}
export declare function greet(id: string): Effect.Effect<string, never, Users>;
