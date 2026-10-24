export declare const make: (name: string) => import("effect/Effect").Effect<{
    name: string;
}, TypeError, never>;
export declare const pick: (name: string) => import("effect/Effect").Effect<string, TypeError, never>;
export declare const count: () => import("effect/Stream").Stream<number, never, never>;
