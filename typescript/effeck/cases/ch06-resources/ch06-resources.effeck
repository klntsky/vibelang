// Chapter 6. Resources.
import { Scope } from "effect"

class IOError extends TaggedError("IOError")<{ readonly path: string }> {}
class ParseError extends TaggedError("ParseError")<{ readonly text: string }> {}

interface FileHandle {
  readonly path: string
  readAll(): Effect<string, IOError, never>
  close(): Effect<void, never, never>
}

interface Report {
  readonly title: string
  readonly lines: number
}

interface Diff {
  readonly same: boolean
  readonly left: number
  readonly right: number
}

interface Connection {
  readonly id: number
  open: boolean
}

// Files.open returns a scoped resource: Effect<FileHandle, IOError, Scope>.
abstract class Files {
  abstract open(path: string): Effect<FileHandle, IOError, Scope.Scope>
}

// FileSystem.open returns an unscoped handle; openFile below makes it a resource.
abstract class FileSystem {
  abstract open(path: string): Effect<FileHandle, IOError, never>
}

abstract class Database {
  abstract connect(): Effect<Connection, never, Scope.Scope>
}

abstract class Logs {
  abstract open(): Effect<FileHandle, IOError, Scope.Scope>
}

const events: string[] = []

const disk = new Map<string, string>([
  ["/reports/q3.txt", "Q3 report\nline 2\nline 3"],
  ["/var/log/app-mon.log", "a\nb\nc"],
  ["/var/log/app-tue.log", "a\nb\nd"]
])

function memoryFile(path: string) {
  const file: FileHandle = {
    path,
    readAll: () => {
      const text = disk.get(path)
      if (text === undefined) throw new IOError({ path })
      return text
    },
    close: () => {
      events.push(`close ${path}`)
    }
  }
  return file
}

// using

function parseReport(text: string) {
  const lines = text.split("\n")
  if (lines[0].length === 0) throw new ParseError({ text })
  const report: Report = { title: lines[0], lines: lines.length }
  return report
}

function readReport(path: string) {
  using file = Files.open(path)
  return parseReport(file.readAll())
}
// Effect<Report, IOError | ParseError, Files>

// Defining a resource

function openFile(path: string) {
  return Effect.acquireRelease(
    () => FileSystem.open(path),      // acquire
    (file) => file.close()            // release: always runs, exactly once
  )
}
// Effect<FileHandle, IOError, FileSystem | Scope>

function openLog(day: string) {
  return Files.open(`/var/log/app-${day}.log`)    // no `using`: the caller owns the file
}
// Effect<FileHandle, IOError, Files | Scope>

function diffLines(left: string, right: string) {
  const diff: Diff = { same: left === right, left: left.split("\n").length, right: right.split("\n").length }
  return diff
}

function compareLogs(a: string, b: string) {
  using left = openLog(a)
  using right = openLog(b)
  return diffLines(left.readAll(), right.readAll())
}
// Effect<Diff, IOError, Files>; both files close when compareLogs returns

// D14 (Codex review R8): only `using` acquisitions belong to the block. The connection is acquired
// without `using`, so it belongs to the caller and must still be open when openAndLog returns.

function openAndLog() {
  const connection = Database.connect() // caller-owned: no `using`
  using log = Logs.open()
  return connection
}

function connectionOutlivesBlock() {
  return Effect.scoped(() => {
    const connection = openAndLog()
    events.push(`after openAndLog: open=${connection.open}`)
    return connection.open
  })
}

// "Adapting a callback API" with suspend is reference program 4 (ref4-adapter.ts).

const FilesTest = Layer.succeed(Files, {
  open: (path) => Effect.acquireRelease(
    () => {
      events.push(`open ${path}`)
      return memoryFile(path)
    },
    (file) => file.close()
  )
})

const FileSystemTest = Layer.succeed(FileSystem, {
  open: (path) => {
    events.push(`open ${path}`)
    return memoryFile(path)
  }
})

const DatabaseTest = Layer.succeed(Database, {
  connect: () => Effect.acquireRelease(
    () => {
      events.push("connect")
      const connection: Connection = { id: 1, open: true }
      return connection
    },
    (connection) => {
      connection.open = false
      events.push("disconnect")
    }
  )
})

const LogsTest = Layer.succeed(Logs, {
  open: () => Effect.acquireRelease(
    () => {
      events.push("open log")
      return memoryFile("/var/log/app.log")
    },
    () => {
      events.push("close log")
    }
  )
})

function scenario() {
  const report = readReport("/reports/q3.txt")
  const opened = Effect.scoped(() => openFile("/reports/q3.txt").path)
  const diff = compareLogs("mon", "tue")
  const outlived = connectionOutlivesBlock()
  return { report, opened, diff, outlived, events }
}

// expect-result: {"report":{"title":"Q3 report","lines":3},"opened":"/reports/q3.txt","diff":{"same":false,"left":3,"right":3},"outlived":true,"events":["open /reports/q3.txt","close /reports/q3.txt","open /reports/q3.txt","close /reports/q3.txt","open /var/log/app-mon.log","open /var/log/app-tue.log","close /var/log/app-tue.log","close /var/log/app-mon.log","connect","open log","close log","after openAndLog: open=true","disconnect"]}
export function main() {
  return Effect.provide(() => scenario(), Layer.mergeAll(FilesTest, FileSystemTest, DatabaseTest, LogsTest))
}
