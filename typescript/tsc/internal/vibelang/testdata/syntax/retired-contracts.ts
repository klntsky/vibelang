function read(id: string): Effect.Effect<string, Gone, Users> { return Users.find(id); }
type Program = Effect.Effect<number, never, never>;
type Errors = Effect.Effect<string, Gone, never>;
type Requirements = Effect.Effect<boolean, never, Users>;
