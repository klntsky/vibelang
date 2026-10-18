import { expect, test } from "bun:test"
import { readFileSync } from "node:fs"
import { join, resolve } from "node:path"

const repo = resolve(import.meta.dir, "../..")

test("published Markdown and llms.txt link to the VibeLang docs domain", async () => {
  const child = Bun.spawn(["bun", join(import.meta.dir, "render-user.ts")], {
    cwd: repo, env: { ...process.env, VIBELANG_DOCS_HOVERS: "0", VIBELANG_LANDING: "0" },
    stdout: "pipe", stderr: "pipe"
  })
  const [stdout, stderr, exitCode] = await Promise.all([
    new Response(child.stdout).text(), new Response(child.stderr).text(), child.exited
  ])
  expect({ exitCode, failure: exitCode ? stdout + stderr : "" }).toEqual({ exitCode: 0, failure: "" })
  const site = join(repo, "docs/user/site")
  const llms = readFileSync(join(site, "llms.txt"), "utf8")
  expect(llms).toContain("https://vibelang.sh/setup.md")
  expect(llms).toContain("https://vibelang.sh/getting-started/003-the-effect-type.md")
  expect(llms).not.toContain("https://effeck.dev/")
  const onboarding = readFileSync(join(site, "001-onboarding.md"), "utf8")
  expect(onboarding).toContain("https://vibelang.sh/getting-started/004-installation.md")
  expect(onboarding).not.toContain("https://effeck.dev/")
}, 60000)
