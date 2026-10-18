import { useEffect, useState } from "react"
import { runtime } from "./runtime.ts"
import { greet } from "./users.vibe"

export function Greeting({ id }: { readonly id: string }) {
  const [text, setText] = useState<string>()
  useEffect(() => {
    // Aborting interrupts the program when the component unmounts or id changes.
    const controller = new AbortController()
    runtime.runPromise(greet(id), { signal: controller.signal }).then(setText, (cause) => {
      if (!controller.signal.aborted) throw cause
    })
    return () => controller.abort()
  }, [id])
  return <p>{text ?? "Loading…"}</p>
}
