package vibelang

import "testing"

func TestBangTypes(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"function find(id: string): Effect<string, NotFound, never>", "function find(id: string): string!NotFound!"},
		{"Effect<number, never, never>", "number!!"},
		{"Effect<User, never, Db>", "User!!Db"},
		{"Effect<User>", "User!!"},
		{"Effect<User, NotFound>", "User!NotFound!"},
		{"type Scoped = Effect.Effect<string, NotFound, Scope>", "type Scoped = string!NotFound!Scope"},
		{`import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<string, A | B, never>`, "string!A | B!"},
		{"ReadonlyArray<Effect<number, never, never>>", "ReadonlyArray<number!!>"},
		{"const f: () => Effect<void, never, never>", "const f: () => void!!"},
		{"Effect<() => void, E, R>", "(() => void)!E!R"},
		{"Effect<A, { code: number; }, never>", "A!({ code: number; })!"},
		{"Effect<{ a: number; }, E, never>", "{ a: number; }!E!"},
		{"Effect<Effect<A, E1, R1>, E2, R2>", "(A!E1!R1)!E2!R2"},
		{"Effect<A, E, R> | undefined", "(A!E!R) | undefined"},
		{"Effect<A, E, R>[]", "(A!E!R)[]"},
		{"Map<string, Effect<A, E, never>>", "Map<string, A!E!>"},
		{"MyEffect<A, B, C>", "MyEffect<A, B, C>"},
		{`Effect<"a>b", E, never>`, `"a>b"!E!`},
	} {
		if got := bangTypes(tc.in); got != tc.want {
			t.Errorf("bangTypes(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
