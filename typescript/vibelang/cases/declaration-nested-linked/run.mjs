import assert from "node:assert/strict";
import { symlinkSync } from "node:fs";
// The emitted consumer retains its dependency's package specifier.
symlinkSync(new URL("./consumer/node_modules", import.meta.url), new URL("./out/consumer/node_modules", import.meta.url));
const { store } = await import("./out/main.js");
assert.equal(store.revision, "r1");
console.log(store.revision);
