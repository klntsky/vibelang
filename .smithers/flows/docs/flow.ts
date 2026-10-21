import { Action, Flow } from "@smthrs/flow";
import { Node } from "@smthrs/plan";
import { Effect, Schema } from "effect";
import { render, type Sources } from "./render.ts";

const payload = { sources: Schema.Record(Schema.String, Schema.String) };
const success = Schema.Record(Schema.String, Schema.String);
const implementationVersion = "docs-render/v2";

/** Renders every generated file from project.ts and the repository files it reads. */
export const Render = Action.make("docs/Render", { implementationVersion, payload, success });
export const layer = Render.toLayer(({ sources }) => Effect.sync(() => render(sources as Sources)), { implementationVersion });

export default Flow.make("docs", {
  description: "Generate CONTRIBUTING, MAINTAINERS and the agent files from project.ts, and count each doc's tokens",
  capabilities: [],
  effects: { reads: [], writes: [] },
  payload,
  success,
  body: Node.capture({ action: Render.name, implementationVersion }, (input) => Render.call(input))
});
