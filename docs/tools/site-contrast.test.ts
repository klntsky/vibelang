import { test, expect } from "bun:test"
import { resolve } from "node:path"

test("built site passes browser contrast checks", async () => {
  const site = resolve(import.meta.dir, "../../site")
  // Exercise the production build in the release validator's docs/tools step.
  // Install Chromium once with: cd site && bunx playwright install chromium
  for (const command of [
    ["npm", "ci", "--no-audit", "--no-fund"],
    ["bun", "run", "build"],
    ["bun", "run", "test:contrast"],
  ]) {
    // Bun tests set NODE_ENV=test; React would embed development RSC stacks.
    // Keep npm ci in the test environment so it installs devDependencies.
    const env = { ...process.env, ...(command.includes("build") ? { NODE_ENV: "production" } : {}) }
    console.log("site contrast stage:", command.join(" "), "NODE_ENV=" + (env.NODE_ENV ?? "unset"))
    const child = Bun.spawn(command, { cwd: site, env, stdout: "pipe", stderr: "pipe" })
    const [stdout, stderr, code] = await Promise.all([
      new Response(child.stdout).text(), new Response(child.stderr).text(), child.exited,
    ])
    expect({ command, code, output: code === 0 ? "" : stdout + stderr }).toEqual({ command, code: 0, output: "" })
    if (command.includes("test:contrast")) console.log(stdout)
  }
}, 300_000)
