package vibelangmapper

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestDataAccessNativeAnnotations accepts written native boundaries even when
// their declarations live elsewhere, while retaining unsafe method diagnostics.
func TestDataAccessNativeAnnotations(t *testing.T) {
	for _, source := range []string{
		`import type { Todo } from "./todos.vibe"; function f(todo: Todo) { return todo.done }`,
		`import type { Todo } from "./todos.vibe"; function f(todo: Todo | undefined) { return todo?.done }`,
		`import type { Todo } from "./todos.vibe"; function f(todos: readonly Todo[]) { return todos.keys }`,
		`import type { Todo } from "./todos.vibe"; function f(todos: ReadonlyArray<Todo>) { return todos.values }`,
		`import type { Todo } from "./todos.vibe"; function f(todos: readonly Todo[]) { return todos[0].done }`,
		`import type { Todo } from "./todos.vibe"; function f(todos: ReadonlyArray<Todo>) { return todos[0]?.done }`,
		`import type { Todo } from "./todos.vibe"; function f(value: unknown) { return (value as Todo).done }`,
		`import type { Todo } from "./todos.vibe"; function f(value: unknown) { return (<Todo>value)?.done }`,
		`import type { Todo } from "./todos.vibe"; function f(value: unknown) { return (value as readonly Todo[]).values }`,
		`import type { Todo } from "./todos.vibe"; function f(value: unknown) { return (value as ReadonlyArray<Todo>)[0].done }`,
		`function f(value: import("./todos.vibe").Todo) { return value.done }`,
		`import type { Todo } from "./todos.vibe"; type Item = Todo; function f(value: Item) { return value.done }`,
		`interface Todo { done: boolean }; function f(todo: Todo) { return todo.done }`,
		`function f(todo: { done: boolean }) { return todo.done }`,
		`function f() { return { done: true }.done }`,
	} {
		t.Run(source, func(t *testing.T) {
			p := Project(source, testContext)
			assert.Equal(t, len(p.Diagnostics), 0, "%v", p.Diagnostics)
		})
	}
	for _, name := range []string{"done", "size", "end", "timed", "request", "keys", "values", "head"} {
		for _, access := range []string{"value." + name, "value?." + name} {
			p := Project(`import { value } from "./opaque.js"; export const field = `+access, testContext)
			assert.Equal(t, len(p.Diagnostics), 1)
			assert.Equal(t, p.Diagnostics[0].Code, int32(CodeDataMethod))
			assert.Assert(t, strings.Contains(p.Diagnostics[0].Message, "const value: { "+name+": unknown }"), "%s", p.Diagnostics[0].Message)
			assert.Assert(t, !strings.Contains(p.Diagnostics[0].Message, "module function"), "%s", p.Diagnostics[0].Message)
		}
		p := Project(`import type { Native } from "./native.js"; function f(value: Native) { return value.`+name+` }`, testContext)
		assert.Equal(t, len(p.Diagnostics), 0, "%v", p.Diagnostics)
	}
	for _, source := range []string{
		`import { get } from "./opaque.js"; export function f() { const value = get(); return value.map() }`,
		`import { get } from "./opaque.js"; export function f() { const value = get(); return value?.map() }`,
		`import { get } from "./opaque.js"; export const value = get(); export const method = value.map`,
		`function f(value: unknown) { return value.done }`,
		`function f(value: any) { return value?.done }`,
		`function f(value: Effect.Effect<number>) { return value.map }`,
		`function f(value: Effect<number>) { return value?.map() }`,
		`import type { Effect as Program } from "effect/Effect"; function f(value: Program<number>) { return value.map }`,
		`type Program = Effect.Effect<number>; function f(value: Program) { return value.map }`,
		`function f(value: Option.Option<number>) { return value?.isSome() }`,
		`import type { Option } from "effect/Option"; function f(value: Option<number>) { return value.isSome }`,
		`function f(value: import("effect/Effect").Effect<number>) { return value.map }`,
		`function f(value: unknown) { return (value as Effect.Effect<number>).map }`,
		`function f(values: readonly Option.Option<number>[]) { return values[0].isSome }`,
		`function f(values: ReadonlyArray<Effect.Effect<number>>) { return values[0]?.map() }`,
	} {
		p := Project(source, testContext)
		assert.Equal(t, len(p.Diagnostics), 1, "%s: %v", source, p.Diagnostics)
		assert.Equal(t, p.Diagnostics[0].Code, int32(CodeDataMethod))
	}
}
