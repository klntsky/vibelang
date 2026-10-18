import { describe, expect, test } from "bun:test"
import { homeProblems, linkTarget, links, sectionBlocks } from "./check-smithers-content.ts"

describe("links in app.md", () => {
  test("map vibelang.sh pages to their sources", () => {
    expect(linkTarget("https://vibelang.sh/001-onboarding.html")).toEqual({ file: "docs/user/src/001-onboarding.md" })
    expect(linkTarget("https://vibelang.sh/spec/103-part1-ch03-errors.html")).toEqual({ file: "docs/design/src/103-part1-ch03-errors.md" })
    expect(linkTarget("https://vibelang.sh/")).toEqual({ ok: true })
  })
  test("map effeck.dev pages to their sources", () => {
    expect(linkTarget("https://effeck.dev/001-onboarding.html")).toEqual({ file: "docs/user/src/001-onboarding.md" })
    expect(linkTarget("https://effeck.dev/error-management/014-expected-errors.html")).toEqual({ file: "docs/user/src/error-management/014-expected-errors.md" })
    expect(linkTarget("https://effeck.dev/spec/103-part1-ch03-errors.html")).toEqual({ file: "docs/design/src/103-part1-ch03-errors.md" })
    expect(linkTarget("https://effeck.dev/")).toEqual({ ok: true })
  })
  test("map smithers.sh blobs to repository files", () => {
    expect(linkTarget("https://smithers.sh/effeck/effeck/blob/main/README.md")).toEqual({ file: "README.md" })
    expect(linkTarget("https://smithers.sh/effeck/effeck/issues")).toEqual({ ok: true })
  })
  test("refuse other hosts and plain http", () => {
    expect(linkTarget("https://example.com/x").error).toBeDefined()
    expect(linkTarget("http://effeck.dev/001-onboarding.html").error).toBe("not https")
    expect(linkTarget("https://effeck.dev/001-onboarding").error).toBeDefined()
  })
  test("are collected once, without trailing punctuation", () => {
    expect(links("See https://effeck.dev/a.html. And (https://effeck.dev/a.html), https://effeck.dev/b.html")).toEqual(["https://effeck.dev/a.html", "https://effeck.dev/b.html"])
  })
})

test("sectionBlocks reads only the named section", () => {
  const md = "## Install\n\n```sh\nnpm a\n```\n\n## Other\n\n```sh\nnpm b\n```\n"
  expect(sectionBlocks(md, "Install", "sh")).toEqual(["npm a"])
})

describe("home.json", () => {
  const exists = (path: string) => path === "README.md"
  test("accepts the blocks smithers.sh renders", () => {
    const doc = JSON.stringify({ blocks: [{ type: "text", text: "x" }, { type: "prompt", title: "Ask" }, { type: "links", links: [{ label: "Docs", url: "https://effeck.dev/" }] }, { type: "markdown", path: "README.md" }] })
    expect(homeProblems(doc, exists)).toEqual([])
  })
  test("refuses a missing markdown file, a bad path and an unknown block", () => {
    expect(homeProblems(JSON.stringify({ blocks: [{ type: "markdown", path: "docs/site/fixed-by-agents.md" }] }), exists)[0]).toContain("missing")
    expect(homeProblems(JSON.stringify({ blocks: [{ type: "markdown", path: "../x.md" }] }), exists)[0]).toContain("invalid")
    expect(homeProblems(JSON.stringify({ blocks: [{ type: "tiles" }] }), exists)[0]).toContain("unknown type")
  })
  test("refuses an empty or oversized document", () => {
    expect(homeProblems(JSON.stringify({ blocks: [] }), exists)).toEqual(["home.json needs 1 to 32 blocks"])
    expect(homeProblems("not json", exists)).toEqual(["home.json is not JSON"])
  })
})
