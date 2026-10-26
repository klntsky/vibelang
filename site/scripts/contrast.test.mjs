import { test } from 'node:test'
import { checkSiteContrast } from './contrast.mjs'

// Run "bun run build" and "bunx playwright install chromium" in site/ first.
test('built home and docs meet contrast requirements in both themes', { timeout: 120_000 }, checkSiteContrast)
