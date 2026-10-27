// Static JSON imports work in Node, Bun, and browser bundlers without Node APIs.
import effectPackage from "effect/package.json" with { type: "json" }

export const PINNED_EFFECT_VERSION = "4.0.5"

if (effectPackage.version !== PINNED_EFFECT_VERSION) {
  throw new Error(
    `vibelang-runtime@0.0.10 requires effect@${PINNED_EFFECT_VERSION}; found effect@${effectPackage.version}. ` +
    `Install the supported version with: npm install effect@${PINNED_EFFECT_VERSION}`
  )
}
