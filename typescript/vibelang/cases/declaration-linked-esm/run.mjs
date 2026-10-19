import { runSync } from "effect/Effect";
import { fn, obj } from "./out/main.js";
console.log(JSON.stringify([runSync(fn(41)), runSync(obj.f(42))]));
