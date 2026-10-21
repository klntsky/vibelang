// The flat config the tests lint .vibe files with: the rules Smithers lints
// its TypeScript packages with (eslint.package.js, eslint.jsdoc.js), minus
// those that resolve files on disk or need type information.
import js from "@eslint/js"
import importPlugin from "eslint-plugin-import"
import jsdoc from "eslint-plugin-jsdoc"
import unicorn from "eslint-plugin-unicorn"
import tseslint from "typescript-eslint"
import vibelang from "../../eslint.mjs"

const exported = [
  "ExportNamedDeclaration[declaration]",
  "ExportDefaultDeclaration[declaration.type='FunctionDeclaration']",
  "ExportDefaultDeclaration[declaration.type='ClassDeclaration']"
]
const lacking = (tag) => `JsdocBlock:not(*:has(JsdocTag[tag=${tag}]))`

export const config = tseslint.config(
  js.configs.recommended,
  {
    files: ["**/*.vibe"],
    extends: [tseslint.configs.recommended],
    languageOptions: { parser: vibelang },
    plugins: { import: importPlugin, jsdoc, unicorn },
    rules: {
      "@typescript-eslint/array-type": ["error", { default: "generic", readonly: "generic" }],
      "@typescript-eslint/consistent-type-imports": ["error", { fixStyle: "inline-type-imports" }],
      "@typescript-eslint/no-import-type-side-effects": "error",
      "@typescript-eslint/no-unused-vars": ["error", { argsIgnorePattern: "^_", varsIgnorePattern: "^_" }],
      "import/no-duplicates": "error",
      "import/no-empty-named-blocks": "error",
      "no-console": "error",
      "no-unneeded-ternary": "error",
      "no-useless-concat": "error",
      "unicorn/no-abusive-eslint-disable": "error",
      "unicorn/prefer-array-flat-map": "error",
      "jsdoc/require-jsdoc": ["error", {
        require: {
          ArrowFunctionExpression: false, ClassDeclaration: false, ClassExpression: false,
          FunctionDeclaration: false, FunctionExpression: false, MethodDefinition: false
        },
        contexts: exported
      }],
      "jsdoc/require-description": ["error", { contexts: exported, exemptedBy: ["inheritdoc", "private"] }],
      "jsdoc/no-restricted-syntax": ["error", {
        contexts: [
          ...exported.map((context) => ({ context, comment: lacking("since"), message: "Every exported declaration needs `@since`." })),
          ...exported.map((context) => ({ context, comment: lacking("category"), message: "Every exported declaration needs `@category`." }))
        ]
      }],
      "jsdoc/check-tag-names": ["error", { definedTags: ["category", "since"] }],
      "no-restricted-syntax": ["error", {
        selector: "AssignmentPattern > CallExpression > MemberExpression[object.name='process'][property.name='cwd']",
        message: "No ambient default."
      }, {
        selector: "VibeCatchExpression > Literal.handler[value='']",
        message: "Do not recover to an empty string."
      }]
    }
  }
)
