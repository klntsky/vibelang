function read(id: string): Effect<string, Gone, Users> { return Users.find(id); }
function partial(id: string): Effect<string, infer E, infer R> { return Users.find(id); }
type Program = Effect<number>;
type Errors = Effect<string, Gone>;
type Requirements = Effect<boolean, never, Users>;
