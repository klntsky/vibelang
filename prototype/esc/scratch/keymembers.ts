import ts from "typescript"
const file = "./prototype/esc/scratch/km.ts"
const src = `import { $service } from "../../../packages/runtime/src/index.ts"\ninterface S { m(): void }\nexport class S extends $service<S>("k") {}\nexport const probe = S\n`
const host = ts.createCompilerHost({})
const og = host.getSourceFile
host.getSourceFile = (f, l, ...r) => (f === file ? ts.createSourceFile(f, src, l, true) : og.call(host, f, l, ...r))
const oe = host.fileExists
host.fileExists = (f) => f === file || oe(f)
const p = ts.createProgram([file], { strict: true, target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, moduleResolution: ts.ModuleResolutionKind.Bundler, allowImportingTsExtensions: true, noEmit: true, skipLibCheck: true, types: [] }, host)
const c = p.getTypeChecker()
const sf = p.getSourceFile(file)!
const decl = (sf.statements[3] as ts.VariableStatement).declarationList.declarations[0]
const t = c.getTypeAtLocation(decl.name)
console.log(c.getPropertiesOfType(t).map((s) => s.name).filter((n) => n !== "prototype").join(", "))
