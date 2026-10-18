function f(value: unknown) { {
    {
        if (value === 1) {
            return "one";
        }
    }
    {
        if (typeof value === "string") {
            let text = value;
            return text;
        }
    }
    return "other";
} }
