// The format of .smithers/home.json, as the Smithers app reads it: a copy of
// the schemas and the reader in Smithers' @smthrs/views (Home.ts), which is
// not published. Keep the two in step: a home that this reader accepts must
// decode with smithers.sh's, and the reverse.
//
// A home is data, never code. Blocks sit in up to 8 sections, each with an
// audience; a bare `blocks` array is one section for everyone. Every string is
// bounded, and one that carries an HTML tag is refused (a `code` block's source
// excepted). A block of a kind the reader doesn't know is skipped and counted;
// any other block that breaks its schema makes the whole file invalid.
import * as Result from "effect/Result"
import * as Schema from "effect/Schema"

export const maximumTitleLength = 120
export const maximumTextLength = 4096
export const maximumUrlLength = 2048
export const maximumBlocks = 32
export const maximumSections = 8
export const maximumEntries = 12
export const maximumStats = 6
export const maximumActions = 3
export const maximumLineLength = 280
export const maximumCodeLength = 4096
export const htmlPattern = /<\/?[A-Za-z!?]/

const noHtml = Schema.makeFilter<string>(
  (text) => htmlPattern.test(text) ? "must not contain HTML; blocks are declared values, never markup" : true
)
const Title = Schema.NonEmptyString.check(Schema.isMaxLength(maximumTitleLength), Schema.isPattern(/^[^\r\n]*$/), noHtml)
const Prose = Schema.NonEmptyString.check(Schema.isMaxLength(maximumTextLength), noHtml)
const Url = Schema.NonEmptyString.check(Schema.isMaxLength(maximumUrlLength), Schema.isPattern(/^https?:\/\/[^\s<>"']+$/i))
const RepositoryPath = Schema.NonEmptyString.check(
  Schema.isMaxLength(1024),
  Schema.makeFilter<string>((value) =>
    value.startsWith("/") || value.includes("\\") ||
      value.split("/").some((part) => part === "" || part === "." || part === "..") ||
      /[%:?#]/.test(value) || [...value].some((character) => character.charCodeAt(0) < 0x20)
      ? "must be a relative in-repository path"
      : true
  )
)
const FlowName = Schema.NonEmptyString.check(Schema.isPattern(/^[a-z0-9_-]+(?:[./][a-z0-9_-]+)*$/))
const Line = Schema.NonEmptyString.check(Schema.isMaxLength(maximumLineLength), Schema.isPattern(/^[^\r\n]*$/), noHtml)
const Code = Schema.NonEmptyString.check(Schema.isMaxLength(maximumCodeLength), Schema.isPattern(/^[^\u0000]*$/))
const Language = Schema.NonEmptyString.check(Schema.isPattern(/^[a-z][a-z0-9+#-]{0,19}$/))
const IsoDate = Schema.String.check(
  Schema.makeFilter<string>((value) => {
    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
    if (match === null) return "must be a date, YYYY-MM-DD"
    const [year, month, day] = [Number(match[1]), Number(match[2]), Number(match[3])]
    const date = new Date(Date.UTC(year, month - 1, day))
    return date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day
      ? true
      : "must be a date that exists"
  })
)
const Link = Schema.Struct({ label: Title, url: Url })
export const Audience = Schema.Literals(["everyone", "contributor", "maintainer"])
export type Audience = typeof Audience.Type
export const Contributors = Schema.Literals(["role", "anyone"])
export type Contributors = typeof Contributors.Type
const optionalTitle = Schema.optional(Title)
const Card = Schema.Struct({ title: Title, text: Schema.optional(Line), label: optionalTitle, url: Schema.optional(Url) })
const Release = Schema.Struct({ name: Title, url: Url, date: IsoDate, summary: Schema.optional(Line) })

export const Block = Schema.Union([
  Schema.Struct({ type: Schema.Literal("text"), title: optionalTitle, text: Prose }),
  Schema.Struct({ type: Schema.Literal("links"), title: optionalTitle, links: Schema.NonEmptyArray(Link) }),
  Schema.Struct({ type: Schema.Literal("flows"), title: optionalTitle }),
  Schema.Struct({ type: Schema.Literal("prompt"), flow: Schema.optional(FlowName), title: optionalTitle, placeholder: optionalTitle }),
  Schema.Struct({ type: Schema.Literal("markdown"), path: RepositoryPath, title: optionalTitle }),
  Schema.Struct({ type: Schema.Literal("stack"), title: optionalTitle }),
  Schema.Struct({ type: Schema.Literal("app"), flow: FlowName, title: Title, picture: Schema.Literals(["issue", "review", "wiki", "schedule"]) }),
  Schema.Struct({ type: Schema.Literal("view"), flow: FlowName }),
  Schema.Struct({
    type: Schema.Literal("hero"),
    title: Title,
    tagline: Schema.optional(Line),
    text: Schema.optional(Line),
    install: Schema.optional(Line),
    actions: Schema.optional(Schema.NonEmptyArray(Link).check(Schema.isMaxLength(maximumActions))),
    announcement: Schema.optional(Link)
  }),
  Schema.Struct({ type: Schema.Literal("code"), title: optionalTitle, file: optionalTitle, language: Schema.optional(Language), code: Code }),
  Schema.Struct({ type: Schema.Literal("cards"), title: optionalTitle, cards: Schema.NonEmptyArray(Card).check(Schema.isMaxLength(maximumEntries)), more: Schema.optional(Link) }),
  Schema.Struct({ type: Schema.Literal("stats"), title: optionalTitle, stats: Schema.NonEmptyArray(Schema.Struct({ value: Title, label: Title })).check(Schema.isMaxLength(maximumStats)) }),
  Schema.Struct({ type: Schema.Literal("releases"), title: optionalTitle, releases: Schema.NonEmptyArray(Release).check(Schema.isMaxLength(maximumEntries)), more: Schema.optional(Link) }),
  Schema.Struct({ type: Schema.Literal("issues"), title: optionalTitle }),
  Schema.Struct({ type: Schema.Literal("runs"), title: optionalTitle }),
  Schema.Struct({ type: Schema.Literal("action"), flow: FlowName, title: Title, key: Schema.optional(Schema.String.check(Schema.isPattern(/^[a-z]$/))) })
])
export type Block = typeof Block.Type

export const kinds: ReadonlyArray<Block["type"]> = [
  "text", "links", "flows", "prompt", "markdown", "stack", "app", "view",
  "hero", "code", "cards", "stats", "releases", "issues", "runs", "action"
]
const SectionHeading = Schema.Struct({ title: optionalTitle, audience: Schema.optional(Audience) })

export interface Section {
  readonly title?: string
  readonly audience: Audience
  readonly blocks: ReadonlyArray<Block>
}
export type Invalid = "json" | "shape" | "sections" | "section" | "blocks" | "block"
/** A skipped block: its section's index, its index there, and its unknown type. */
export type Skipped = { readonly section: number; readonly block: number; readonly type: string }
export type Read =
  | { readonly _tag: "Declared"; readonly sections: ReadonlyArray<Section>; readonly contributors: Contributors; readonly skipped: ReadonlyArray<Skipped> }
  | { readonly _tag: "Invalid"; readonly reason: Invalid; readonly at?: { readonly section: number; readonly block?: number } }

const known = new Set<unknown>(kinds)
const decodeBlock = Schema.decodeUnknownResult(Block)
const decodeHeading = Schema.decodeUnknownResult(SectionHeading)
const isContributors = Schema.is(Contributors)
const isObject = (value: unknown): value is Readonly<Record<string, unknown>> =>
  typeof value === "object" && value !== null && !Array.isArray(value)
const unknownKind = (value: unknown): string | undefined => {
  const type = isObject(value) ? value.type : undefined
  return typeof type === "string" && !known.has(type) ? type : undefined
}

/** Reads home.json's text as @smthrs/views' Home.read does, and also says where a file is invalid and which blocks it skipped. */
export function read(text: string): Read {
  let document: unknown
  try {
    document = JSON.parse(text)
  } catch {
    return { _tag: "Invalid", reason: "json" }
  }
  if (!isObject(document)) return { _tag: "Invalid", reason: "shape" }
  const { blocks, sections, contributors = "role" } = document
  if (!isContributors(contributors)) return { _tag: "Invalid", reason: "shape" }
  const declared: ReadonlyArray<unknown> | undefined = Array.isArray(blocks) && sections === undefined
    ? [{ blocks }]
    : Array.isArray(sections) && blocks === undefined
    ? sections
    : undefined
  if (declared === undefined) return { _tag: "Invalid", reason: "shape" }
  if (declared.length > maximumSections) return { _tag: "Invalid", reason: "sections" }
  let count = 0
  for (const [s, section] of declared.entries()) {
    if (!isObject(section) || !Array.isArray(section.blocks)) return { _tag: "Invalid", reason: "section", at: { section: s } }
    count += section.blocks.length
  }
  if (count > maximumBlocks) return { _tag: "Invalid", reason: "blocks" }
  const out: Array<Section> = []
  const skipped: Array<Skipped> = []
  for (const [s, section] of (declared as ReadonlyArray<Readonly<Record<string, unknown>>>).entries()) {
    const heading = decodeHeading(section)
    if (Result.isFailure(heading)) return { _tag: "Invalid", reason: "section", at: { section: s } }
    const drawn: Array<Block> = []
    for (const [b, entry] of (section.blocks as ReadonlyArray<unknown>).entries()) {
      const type = unknownKind(entry)
      if (type !== undefined) {
        skipped.push({ section: s, block: b, type })
        continue
      }
      const decoded = decodeBlock(entry)
      if (Result.isFailure(decoded)) return { _tag: "Invalid", reason: "block", at: { section: s, block: b } }
      drawn.push(decoded.success)
    }
    const { title, audience = "everyone" } = heading.success
    out.push(title === undefined ? { audience, blocks: drawn } : { title, audience, blocks: drawn })
  }
  return { _tag: "Declared", sections: out, contributors, skipped }
}
