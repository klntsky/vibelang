import type { Effect } from "vibelang-runtime";
export interface SearchApi {
    find(query: string): Effect<string>;
}
export interface Search {
    readonly "~vibelang/service"?: "decl-service-type-dts/main#Search";
}
declare const Search_base: import("vibelang-runtime").ServiceKey<Search, SearchApi>;
export declare abstract class Search extends Search_base {
    static readonly layer: import("effect/Layer").Layer<Search, never, never>;
}
export interface Prefix {
    readonly "~vibelang/service"?: "decl-service-type-dts/main#Prefix";
}
declare const Prefix_base: import("vibelang-runtime").ServiceKey<Prefix, string>;
export declare abstract class Prefix extends Prefix_base {
}
export interface Version {
    readonly "~vibelang/service"?: "decl-service-type-dts/main#Version";
}
declare const Version_base: import("vibelang-runtime").ReferenceKey<number>;
export declare abstract class Version extends Version_base {
    static readonly default: number;
}
export declare function greet(query: string): import("effect/Effect").Effect<string, never, Prefix | Search>;
