// Show what the single pass saw for one file: the neutral text, every dual selection the checker
// recorded (with the candidate it was made against), and the intrinsic decisions.
//
//   bun prototype/single-pass/harness/inspect.ts ch01-first-look.es [--text]
import path from "node:path"
import ts from "typescript"
import { ESC } from "./replica.ts"
import { runSinglePass } from "./single-pass.ts"

const name = process.argv[2]
const S = runSinglePass([path.join(ESC, "examples", name)])
if (process.argv.includes("--text")) for (const t of S.neutralText.values()) console.log(t)
for (const u of S.units) {
  for (const d of u.emitted.duals) {
    const m = S.modes.get(d.es)
    const line = u.file.sf.getLineAndCharacterOfPosition(d.es.getStart(u.file.sf)).line + 1
    if (m) console.log(`dual#${d.id} L${line} ${d.es.getText(u.file.sf).replace(/\s+/g, " ").slice(0, 60)} -> ${m.mode} (ctx ${m.ctx}, via ${m.via}, candidates seen ${m.candidates})`)
  }
}
for (const [call, f] of S.facts.calls) {
  if (f.type !== "plain" || f.calleeEffectValue || f.serviceReceiver) console.log(`call ${call.getText().replace(/\s+/g, " ").slice(0, 60)}: ${f.type}${f.calleeEffectValue ? " effect-value" : ""}${f.serviceReceiver ? " service" : ""}`)
}
for (const [e, c] of S.facts.convs) console.log(`conv ${e.getText()}: ${JSON.stringify(c)}`)
for (const a of S.anomalies) console.log(`anomaly ${a.kind}: ${a.detail}`)
console.log(`check ${S.checkMs.toFixed(0)} ms, ${S.tsErrors} TypeScript diagnostics in the neutral text`)
void ts
