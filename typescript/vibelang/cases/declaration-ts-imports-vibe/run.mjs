import { readFileSync } from "node:fs";
import { runSync } from "effect/Effect";
import { find, later, prefix } from "./out/index.js";
// The declaration names the emitted module, which a consumer resolves; no .vibe file ships beside it.
const declaration = readFileSync(new URL("./out/index.d.ts", import.meta.url), "utf8");
console.log(JSON.stringify([prefix, runSync(find("ada")), typeof (await later()).find, declaration.includes(".vibe")]));
