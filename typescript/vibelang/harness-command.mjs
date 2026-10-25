import { spawn } from "node:child_process"

// A case owns its command history, so concurrent workers never mix evidence.
export class CaseCommands {
  records = []

  run(command, argv, { phase, timeout, ...options }) {
    return new Promise((resolve) => {
      const started = Date.now()
      const record = {
        phase, command, argv, cwd: options.cwd ?? process.cwd(),
        startedAt: new Date(started).toISOString(),
        timeoutMs: timeout, deadline: new Date(started + timeout).toISOString(),
        exitCode: null, signal: null, timedOut: false,
      }
      this.records.push(record)
      const child = spawn(command, argv, { ...options, stdio: ["ignore", "pipe", "pipe"] })
      let stdout = ""
      let stderr = ""
      child.stdout.on("data", (d) => (stdout += d))
      child.stderr.on("data", (d) => (stderr += d))
      const timer = setTimeout(() => {
        record.timedOut = true
        child.kill("SIGKILL")
      }, timeout)
      child.on("error", (error) => {
        record.error = error.message
        record.errorCode = error.code
      })
      child.on("close", (code, signal) => {
        clearTimeout(timer)
        Object.assign(record, {
          finishedAt: new Date().toISOString(),
          exitCode: record.error ? null : code, signal,
          stdoutTail: stdout.slice(-16_384), stderrTail: stderr.slice(-16_384),
        })
        resolve({ code: record.errorCode ?? (signal ? `killed by ${signal}` : code), stdout, stderr })
      })
    })
  }
}
