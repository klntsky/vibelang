import { Greeting } from "./Greeting.tsx"

export function App() {
  return (
    <main>
      <h1>Users</h1>
      <Greeting id="42" />
      <Greeting id="404" />
    </main>
  )
}
