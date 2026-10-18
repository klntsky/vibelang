<script lang="ts">
  import { runtime } from "./runtime.ts"
  import { quoteCart } from "./cart.effeck"

  const quotes = runtime.runPromise(quoteCart())
</script>

<main>
  <h1>Cart</h1>
  {#await quotes}
    <p>Loading…</p>
  {:then lines}
    {#each lines as line (line)}
      <p>{line}</p>
    {/each}
  {/await}
</main>
