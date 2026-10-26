package vibelang

import "testing"

func TestEffectText(t *testing.T) {
	for _, tc := range []struct {
		channels []string
		want     string
	}{
		{[]string{"string", "never", "never"}, "Effect<string>"},
		{[]string{"string", "", ""}, "Effect<string>"},
		{[]string{"string"}, "Effect<string>"},
		{[]string{"never", "never", "never"}, "Effect<never>"},
		{[]string{"User", "NotFound", "never"}, "Effect<User, NotFound>"},
		{[]string{"User", "NotFound"}, "Effect<User, NotFound>"},
		{[]string{"User", "never", "Db"}, "Effect<User, never, Db>"},
		{[]string{"User", "", "Db"}, "Effect<User, never, Db>"},
		{[]string{"User", "NotFound", "Db"}, "Effect<User, NotFound, Db>"},
		{[]string{"User", "NotFound", "infer R"}, "Effect<User, NotFound, infer R>"},
		{[]string{"() => void", "A | B", "never"}, "Effect<() => void, A | B>"},
	} {
		if got := effectText(tc.channels...); got != tc.want {
			t.Errorf("effectText(%q) = %q, want %q", tc.channels, got, tc.want)
		}
	}
}

func TestEffectTypes(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"function find(id: string): Effect<string, NotFound, never>", "function find(id: string): Effect<string, NotFound>"},
		{"Effect<number, never, never>", "Effect<number>"},
		{"Effect<User, never, Db>", "Effect<User, never, Db>"},
		{"Effect<User>", "Effect<User>"},
		{"Effect<never>", "Effect<never>"},
		{"Effect<never, NotFound, Db>", "Effect<never, NotFound, Db>"},
		{"type Scoped = Effect.Effect<string, NotFound, Scope>", "type Scoped = Effect<string, NotFound, Scope>"},
		{`import("effect/Effect", { with: { "resolution-mode": "import" } }).Effect<string, A | B, never>`, "Effect<string, A | B>"},
		{"ReadonlyArray<Effect<number, never, never>>", "ReadonlyArray<Effect<number>>"},
		{"const f: () => Effect<void, never, never>", "const f: () => Effect<void>"},
		{"Effect<() => void, E, R>", "Effect<() => void, E, R>"},
		{"Effect<A, { code: number; }, never>", "Effect<A, { code: number; }>"},
		{"Effect<Effect<A, never, never>, E2, never>", "Effect<Effect<A>, E2>"},
		{"Effect<A, E, never> | undefined", "Effect<A, E> | undefined"},
		{"Effect<A, never, never>[]", "Effect<A>[]"},
		{"Map<string, Effect.Effect<A, E, never>>", "Map<string, Effect<A, E>>"},
		{"MyEffect<A, never, never>", "MyEffect<A, never, never>"},
		{`Effect<"a>b", E, never>`, `Effect<"a>b", E>`},
	} {
		if got := effectTypes(tc.in); got != tc.want {
			t.Errorf("effectTypes(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
