export function nickEscape(s) {
    return s.replaceAll(/(\[|]|<|>| |\?)/g, "--")
}