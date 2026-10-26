import type { Data } from "effect";
/** A parcel on its way. */
export declare const Delivery: Data.TaggedEnum.Constructor<Delivery>;
/** A parcel on its way. */
export type Delivery = Data.TaggedEnum<{
    Waiting: {};
    /** Handed to the carrier. */
    Sent: {
        readonly tracking: string;
        readonly carrier?: string;
    };
    Rejected: {
        readonly reason: string;
        readonly note?: string;
    };
}>;
export interface LoadStateDefinition extends Data.TaggedEnum.WithGenerics<2> {
    readonly taggedEnum: LoadState<this["A"], this["B"]>;
}
export declare const LoadState: {
    readonly [Tag in LoadStateDefinition["taggedEnum"]["_tag"]]: <A, B>(args: Data.TaggedEnum.Args<Data.TaggedEnum.Kind<LoadStateDefinition, A, B>, Tag>) => Data.TaggedEnum.Value<Data.TaggedEnum.Kind<LoadStateDefinition, A, B>, Tag>;
} & Data.TaggedEnum.GenericMatchers<LoadStateDefinition>;
export type LoadState<A, E> = Data.TaggedEnum<{
    Idle: {};
    Loaded: {
        readonly value: A;
    };
    Failed: {
        readonly error: E;
    };
}>;
export declare function describe(delivery: Delivery): import("effect/Effect").Effect<string, never, never>;
export declare function main(): import("effect/Effect").Effect<{
    described: string[];
    loads: number[];
    sent: boolean;
    same: boolean;
    different: boolean;
    matched: string;
    guard: boolean[];
    note: string;
    fromObject: string[];
}, never, never>;
