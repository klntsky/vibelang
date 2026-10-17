import type { Chunk, Option } from "effect";
export declare function firstOf(values: Chunk.Chunk<number>): Option.Option<number>;
export declare function main(): import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<{
    doubled: number;
    size: number;
    first: number;
    id: number;
    twice: number;
    isArray: boolean;
    text: string;
    local: string;
}, never, never>;
