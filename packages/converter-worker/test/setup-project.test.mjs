import assert from "node:assert/strict"
import { spawnSync } from "node:child_process"
import { mkdirSync, mkdtempSync, cpSync, readFileSync, writeFileSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { fileURLToPath } from "node:url"
import { test } from "node:test"

test("fixture setup derives compiler tarball names and Effect from release manifests", t => {
  const scratch = mkdtempSync(join(tmpdir(), "vibelang-worker-setup-"))
  t.after(() => rmSync(scratch, {recursive:true,force:true}))
  for (const path of ["packages/compiler", "packages/runtime", "packages/converter-worker/test", "packages/converter-worker/project/node_modules/vibelang-linux-x64/bin", "bin", "tarballs"]) mkdirSync(join(scratch,path),{recursive:true})
  cpSync(new URL("./setup-project.sh",import.meta.url),join(scratch,"packages/converter-worker/test/setup-project.sh"))
  writeFileSync(join(scratch,"packages/converter-worker/project/node_modules/vibelang-linux-x64/bin/vibelang"),"fixture")
  writeFileSync(join(scratch,"packages/compiler/package.json"),JSON.stringify({version:"9.8.7"}))
  writeFileSync(join(scratch,"packages/runtime/package.json"),JSON.stringify({peerDependencies:{effect:"4.2.1"}}))
  const record = join(scratch,"npm-args.json")
  writeFileSync(join(scratch,"bin/npm"),"#!"+process.execPath+"\nrequire('node:fs').writeFileSync("+JSON.stringify(record)+",JSON.stringify(process.argv.slice(2)))\n",{mode:0o755})
  const result = spawnSync("sh",[join(scratch,"packages/converter-worker/test/setup-project.sh"),join(scratch,"tarballs")],{encoding:"utf8",env:{...process.env,PATH:join(scratch,"bin")+":"+process.env.PATH}})
  assert.equal(result.status,0,result.stdout+result.stderr)
  const args = JSON.parse(readFileSync(record,"utf8"))
  for (const name of ["vibelang","vibelang-runtime","vibelang-linux-x64"]) assert.ok(args.includes(join(scratch,"tarballs",name+"-9.8.7.tgz")),args.join(" "))
  assert.ok(args.includes("effect@4.2.1"),args.join(" "))
})
