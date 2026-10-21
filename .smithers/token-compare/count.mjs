import { readFileSync } from "node:fs"
import { getEncoding } from "js-tiktoken"
const encoder = getEncoding("o200k_base")
const counts = Object.fromEntries(["users.vibe", "users.ts"].map(file =>
  [file, encoder.encode(readFileSync(new URL(file, import.meta.url), "utf8")).length]
))
console.log(JSON.stringify({ tokenizer: "js-tiktoken@1.0.21 / o200k_base", ...counts,
  fewerPercent: Number((100 * (1 - counts["users.vibe"] / counts["users.ts"])).toFixed(1)) }, null, 2))
