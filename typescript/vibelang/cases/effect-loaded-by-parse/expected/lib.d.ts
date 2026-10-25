export declare const line: (asked: string) => import("effect/Effect").Effect<any, SyntaxError, never>;
export declare const component: (text: string) => import("effect/Effect").Effect<string, URIError, never>;
export declare const global: (text: string) => import("effect/Effect").Effect<unknown, SyntaxError, never>;
