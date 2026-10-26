import type { LanguageRegistration } from 'shiki'
import { defineConfig } from 'vocs/config'
import vibeGrammar from './vibe.tmLanguage.json' with { type: 'json' }
import { sidebar } from './sidebar.ts'
import { specSidebar } from './spec-sidebar.ts'

// The JSON's inferred type is narrower than Shiki's grammar type, hence the cast.
const vibe: LanguageRegistration = {
  ...(vibeGrammar as unknown as LanguageRegistration),
  name: 'vibe',
  aliases: ['vibelang'],
}

export default defineConfig({
  accentColor: 'light-dark(#646cff, #a5a9ff)',
  baseUrl: 'https://vibelang.sh',
  banner:
    'VibeLang is in early alpha. The language will change in major, breaking ways. It is not suitable for production use.',
  checkDeadlinks: true,
  codeHighlight: {
    // ```vibe blocks use the grammar from VibeLang's editor extension
    // (editors/vscode/syntaxes/VibeLang.tmLanguage.json in the language repo).
    langs: [
      'bash',
      'json',
      'jsx',
      'lua',
      'markdown',
      'md',
      'mdx',
      'sh',
      'svelte',
      'ts',
      'tsx',
      'vue',
      'zsh',
      vibe,
    ],
  },
  colorScheme: 'light dark',
  description:
    'VibeLang gives Effect a simple, readable syntax that compiles to TypeScript, with a typed boundary to TypeScript and npm.',
  ogImageUrl: 'https://vibelang.sh/social-card.png',
  head: {
    meta: {
      ogImageWidth: '1200',
      ogImageHeight: '630',
      ogImageAlt: 'VibeLang — Effect, as a language',
    },
  },
  iconUrl: '/icon.svg',
  logoUrl: {
    light: '/logo-light.svg',
    dark: '/logo-dark.svg',
  },
  renderStrategy: 'full-static',
  sidebar: [...sidebar, ...specSidebar],
  socials: [{ icon: 'github', link: 'https://github.com/smithersai/vibelang' }],
  title: 'VibeLang',
  titleTemplate: '%s | VibeLang',
  topNav: [
    { text: 'Welcome', link: '/001-onboarding' },
    { text: 'Guides', link: '/013-getting-started' },
    { text: 'Cheat sheet', link: '/vibelang-features/115-cheat-sheet' },
    { text: 'Specification', link: '/spec/' },
    { text: 'GitHub', link: 'https://github.com/smithersai/vibelang', external: true },
  ],
})
