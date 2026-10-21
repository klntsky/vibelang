import { readFileSync } from "node:fs";
console.log(readFileSync(new URL("./out/index.d.ts", import.meta.url), "utf8").includes(".vibe"));
