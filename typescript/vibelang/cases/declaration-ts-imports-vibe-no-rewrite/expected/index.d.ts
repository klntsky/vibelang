export { find, NotFound } from "./User.js";
export declare const lookup: (id: string) => import("effect/Effect").Effect<string, import("./User.js").NotFound, never>;
export declare const later: () => Promise<typeof import("./User.js")>;
