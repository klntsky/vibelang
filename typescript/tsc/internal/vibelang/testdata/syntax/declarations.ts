class Gone extends TaggedError("Gone")<{
    readonly id: string;
}> {
}
abstract class Users {
    abstract find(id: string): Effect.Effect<string, Gone, never>;
}
abstract class Ask extends Control {
    abstract answer(): Effect.Effect<boolean, never, never>;
}
interface Ticket extends Schema.Schema.Type<typeof Ticket> {
}
const Ticket = Schema.Struct({
    id: Schema.String, count: Schema.Number,
});
