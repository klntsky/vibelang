import type { Effect } from "vibelang-runtime";
import { Prompt } from "effect/cli";
export declare const plainBody: (pick: boolean) => Effect.Effect<Prompt.Prompt<string>, never, never>;
export declare const effectBody: (pick: boolean) => Effect.Effect<Prompt.Prompt<string>, never, never>;
export declare const noOperation: () => Effect.Effect<number, never, never>;
