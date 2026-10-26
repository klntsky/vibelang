class Gone extends TaggedError("Gone")<{
    readonly id: string;
}> {
}
abstract class Users {
    abstract find(id: string): Effect<string, Gone>;
}
abstract class Ask extends Control {
    abstract answer(): Effect<boolean>;
}
interface Ticket extends Schema.Schema.Type<typeof Ticket> {
}
const Ticket = Schema.Struct({
    id: Schema.String, count: Schema.Number,
});
