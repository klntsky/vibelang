function f(value: unknown) { {
    const $match2 = value;
    {
        if ($match2 === 1) {
            return "one";
        }
    }
    {
        if (typeof $match2 === "string") {
            let text = $match2;
            return text;
        }
    }
    return "other";
} }
