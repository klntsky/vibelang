// Reads the docs sidebar from the "V4 sidebar in order" section of
// docs/user/inventory.md. markdown.ts builds the site in this order, and
// token-counts.ts counts the pages in it.
import { relative, resolve } from "node:path"

// group is the bold sidebar label a page is indented under, if any. view is
// the ### heading the page is listed under.
export type Page = { path: string; label: string; group?: string; view: string }
export type NavItem = { kind: "view" | "group"; label: string } | { kind: "page"; page: Page }

// Page keys are site paths with a .md extension. Specification chapters live under spec/.
export function pageKeys(repo: string) {
  const src = resolve(repo, "docs/user/src")
  const design = resolve(repo, "docs/design/src")
  return {
    src,
    design,
    fileOf: (key: string) => key.startsWith("spec/") ? resolve(design, key.slice(5)) : resolve(src, key),
    keyOf: (file: string) =>
      file.startsWith(design + "/") ? "spec/" + relative(design, file) : file.startsWith(src + "/") ? relative(src, file) : undefined
  }
}

export function readSidebar(inventory: string, repo: string): { navigation: NavItem[]; order: Page[] } {
  const { keyOf } = pageKeys(repo)
  const root = resolve(repo, "docs/user")
  const lines = inventory.split("\n")
  const start = lines.findIndex(line => /^## V4 sidebar in order\s*$/.test(line))
  if (start < 0) throw new Error('inventory.md: missing "V4 sidebar in order" section')
  const navigation: NavItem[] = []
  const order: Page[] = []
  let lastView = ""
  let lastGroup: string | undefined
  for (const line of lines.slice(start + 1)) {
    if (/^## /.test(line)) break
    const view = /^### (.+)$/.exec(line)
    const group = /^- \*\*(.+)\*\*\s*$/.exec(line)
    const page = /^\s*- \[([^\]]+)\]\(((?:src|\.\.\/design\/src)\/[^)]+\.md)\)/.exec(line)
    if (view) navigation.push({ kind: "view", label: lastView = view[1]! })
    else if (group) navigation.push({ kind: "group", label: lastGroup = group[1]! })
    else if (page) {
      const path = keyOf(resolve(root, page[2]!))
      if (!path) throw new Error(`inventory.md: page outside docs/user/src and docs/design/src: ${page[2]}`)
      if (order.some(p => p.path === path)) throw new Error(`inventory.md: duplicate page: ${path}`)
      const entry = { path, label: page[1]!, group: /^\s/.test(line) ? lastGroup : undefined, view: lastView }
      order.push(entry)
      navigation.push({ kind: "page", page: entry })
    } else if (line.trim().startsWith("-")) throw new Error(`inventory.md: unrecognized sidebar entry: ${line.trim()}`)
  }
  if (!order.length) throw new Error("inventory.md: sidebar contains no pages")
  return { navigation, order }
}
