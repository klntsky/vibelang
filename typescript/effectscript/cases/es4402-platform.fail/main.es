// expect: ES4402
// D5, D6: a Node.js or browser API is not usable from an .es file, even
// inside a boundary; the error names the Effect module that replaces it.
import { readFile } from "node:fs/promises"
import fs from "fs"
import type { Server } from "node:http"
import { Effect } from "effect"

export function load(path: string) {
  return Effect.tryPromise(() => readFile(path, "utf8"))
}

export function exists(path: string) {
  return Effect.sync(() => fs.existsSync(path))
}

export function get(url: string) {
  return Effect.tryPromise(() => fetch(url))
}

export function later() {
  return Effect.promise(() => new Promise((resolve) => setTimeout(resolve, 10)))
}

export function log(message: string) {
  console.log(message)
}

export function port() {
  return process.env.PORT
}

export function save(key: string, value: string) {
  localStorage.setItem(key, value)
}

// Allowed: a type from a platform module, and web standards that touch
// nothing outside the program.
export function serve(server: Server) {
  return server
}

export function host(url: string) {
  return new URL(url).host
}

export function size(text: string) {
  return new TextEncoder().encode(text).length
}
