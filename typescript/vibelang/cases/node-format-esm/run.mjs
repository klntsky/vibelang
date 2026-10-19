import { here, ready } from "./out/main.js";
if (!here().endsWith("/out/main.js") || ready !== 42) throw Error("ESM format");
console.log("esm");
