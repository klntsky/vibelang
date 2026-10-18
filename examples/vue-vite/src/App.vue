<script setup lang="ts">
import { onMounted, shallowRef } from "vue"
import { runtime } from "./runtime.ts"
import { loadBoard, type Board } from "./todos.vibe"

const board = shallowRef<Board>()
onMounted(async () => {
  board.value = await runtime.runPromise(loadBoard())
})
</script>

<template>
  <main>
    <h1>Todos</h1>
    <p v-if="board === undefined">Loading…</p>
    <template v-else>
      <ul>
        <li v-for="todo in board.todos" :key="todo.id">
          {{ todo.done ? "[x]" : "[ ]" }} {{ todo.title }}
        </li>
      </ul>
      <p v-for="message in board.messages" :key="message">{{ message }}</p>
    </template>
  </main>
</template>
