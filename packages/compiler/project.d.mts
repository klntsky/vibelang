export interface Diagnostic {
  readonly file?: string
  readonly line?: number
  readonly column?: number
  readonly category: "error" | "warning"
  readonly code: string
  text: string
}

export interface Output {
  readonly code: string
  readonly map: { sources: Array<string>; sourceRoot: string; [key: string]: unknown }
}

export interface Result {
  /** Each .vibe source, by listed and real path, to its JavaScript and source map. */
  readonly outputs: Map<string, Output>
  /** Project source files the result depends on. */
  readonly inputs: Array<string>
  /** Diagnostics that blocked emission; outputs is then empty. */
  readonly errors: Array<Diagnostic>
  /** Diagnostics of a successful compile. */
  readonly warnings: Array<Diagnostic>
}

export interface Loaded {
  /** The tsconfig whose compile served the module. */
  readonly project: string
  readonly code: string
  readonly map?: Output["map"]
  readonly inputs: Array<string>
  /** The compile's errors. The module's code throws when there are any. */
  readonly errors: Array<Diagnostic>
  /** The compile's warnings. */
  readonly warnings: Array<Diagnostic>
  /** True for the first module loaded from this compile, so a bundler reports its diagnostics once. */
  readonly first: boolean
  /** Set when the project does not include the module. */
  readonly error?: false | { readonly message: string; readonly id: string }
}

export function findCompiler(root: string, compiler?: string): string
export function parseOutput(output: string, root: string): { files: Array<string>; diagnostics: Array<Diagnostic> }
export function formatDiagnostic(diagnostic: Diagnostic): string
export function projectBackend(options: { root: string; compiler: string }): { compile(project: string): Promise<Result> }
export function compileProjectSync(options: { project: string; root?: string; compiler?: string }): Result
export function nearestProject(file: string): string | undefined
export function projects(options: {
  root: string
  project: string
  compiler: string
  debug?: (text: string) => void
}): {
  projectOf(file: string): string
  invalidate(): void
  load(file: string): Promise<Loaded>
}
