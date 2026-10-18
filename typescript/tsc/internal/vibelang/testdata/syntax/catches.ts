function f() { return $catchExpression(() => read(), (e) => 0); }
function g() { try {
    return read();
}
catch ($caught) {
    if ($isFailure<Gone>($caught)) {
        const e = $caught;
        return e.id;
    }
    else
        throw $caught;
}
finally {
    const exit = $exit();
    write(exit);
} }
