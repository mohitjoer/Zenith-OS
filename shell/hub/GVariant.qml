pragma Singleton
pragma ComponentBehavior: Bound

// The dialect the control tool speaks.
//
// Every value the Hub reads and writes is a GVariant literal in text form, and
// the two directions do not spell the same value the same way. A string is
// quoted on the way out of `gsettings` and a boolean is not. An enum's nick
// arrives quoted and has to be written bare, because the tool compares it
// against the schema's nicks and a quoted nick is not a nick.
//
// So the spelling is stated once, here, and every value goes through it. An
// editor that guesses is an editor whose writes are refused, and a refused
// write is a line on stderr that nobody reads.

import Quickshell

Singleton {
    id: gvariant

    // The value as plain text, with the quotes and the escapes taken off.
    //
    // A value that is not a literal is returned unchanged. The export's
    // numbers and booleans are not quoted, and a caller asking for the text
    // of one of those wants the text it already has.
    function unquote(text) {
        if (text === undefined || text === null) return ""
        const trimmed = String(text).trim()
        if (trimmed.length < 2) return trimmed

        const first = trimmed.charAt(0)
        const last = trimmed.charAt(trimmed.length - 1)
        if ((first === "'" && last === "'") || (first === '"' && last === '"')) {
            return unescapeLiteral(trimmed.substring(1, trimmed.length - 1), first)
        }
        return trimmed
    }

    // The value as a GVariant string literal.
    //
    // Double quotes when the value contains an apostrophe and no double
    // quote, because that is the spelling GLib itself writes, and a list
    // built out of mixed spellings is a list nobody can read.
    function quote(text) {
        const value = text === undefined || text === null ? "" : String(text)
        if (value.indexOf("'") >= 0 && value.indexOf('"') < 0) {
            return '"' + escapeLiteral(value, '"') + '"'
        }
        return "'" + escapeLiteral(value, "'") + "'"
    }

    // The elements of a GVariant tuple of strings.
    //
    // The tuple is parsed rather than split, because a comma inside a quoted
    // element belongs to the element and an apostrophe inside one belongs to
    // the element. Splitting on the comma mangles both, and the mangled value
    // is what gets written back.
    function parseList(text) {
        const items = []
        if (text === undefined || text === null) return items

        const source = String(text).trim()
        if (source.length < 2) return items
        if (source.charAt(0) !== "[" || source.charAt(source.length - 1) !== "]") {
            return items
        }

        const body = source.substring(1, source.length - 1)
        let i = 0
        while (i < body.length) {
            // The separators between the elements. The comma is consumed
            // here rather than where an element ends, so that an element
            // that is not a literal cannot leave the scan where it was.
            while (i < body.length && (isSpace(body.charAt(i))
                    || body.charAt(i) === ",")) i++
            if (i >= body.length) break

            const quoteChar = body.charAt(i)
            if (quoteChar !== "'" && quoteChar !== '"') {
                // A bare element. GVariant does not write one, but a value
                // that reached the Hub from somewhere else might.
                let bare = ""
                while (i < body.length && body.charAt(i) !== ",") {
                    bare += body.charAt(i)
                    i++
                }
                items.push(bare.trim())
                continue
            }

            i++
            let element = ""
            while (i < body.length && body.charAt(i) !== quoteChar) {
                if (body.charAt(i) === "\\" && i + 1 < body.length) {
                    element += body.charAt(i) + body.charAt(i + 1)
                    i += 2
                    continue
                }
                element += body.charAt(i)
                i++
            }
            i++
            items.push(unescapeLiteral(element, quoteChar))
        }
        return items
    }

    // The elements as a GVariant tuple.
    //
    // The tuple is written whole, because that is what it is: one value, and
    // not a list of values the tool has to be told apart.
    function formatList(items) {
        const quoted = []
        for (let i = 0; i < items.length; i++) {
            quoted.push(quote(items[i]))
        }
        return "[" + quoted.join(", ") + "]"
    }

    // The value as a number, or the fallback when it is not one.
    function toNumber(text, fallback) {
        const value = parseInt(text, 10)
        return isNaN(value) ? fallback : value
    }

    function isSpace(character) {
        return character === " " || character === "\t"
            || character === "\n" || character === "\r"
    }

    // The escapes inside a literal, written out.
    //
    // The backslash first, because a backslash that is escaped after the
    // others has already been doubled by the time it is looked at.
    function escapeLiteral(value, quoteChar) {
        let out = ""
        for (let i = 0; i < value.length; i++) {
            const character = value.charAt(i)
            const code = value.charCodeAt(i)

            if (character === "\\") out += "\\\\"
            else if (character === quoteChar) out += "\\" + character
            else if (character === "\n") out += "\\n"
            else if (character === "\t") out += "\\t"
            else if (character === "\r") out += "\\r"
            else if (character === "\b") out += "\\b"
            else if (character === "\f") out += "\\f"
            else if (code < 0x20) {
                out += "\\u" + ("0000" + code.toString(16)).slice(-4)
            } else out += character
        }
        return out
    }

    // The escapes inside a literal, read back.
    function unescapeLiteral(body, quoteChar) {
        let out = ""
        for (let i = 0; i < body.length; i++) {
            const character = body.charAt(i)
            if (character !== "\\") {
                out += character
                continue
            }

            i++
            const next = i < body.length ? body.charAt(i) : ""
            if (next === "n") out += "\n"
            else if (next === "t") out += "\t"
            else if (next === "r") out += "\r"
            else if (next === "b") out += "\b"
            else if (next === "f") out += "\f"
            else if (next === "u" || next === "U") {
                const width = next === "u" ? 4 : 8
                const digits = body.substr(i + 1, width)
                out += String.fromCharCode(parseInt(digits, 16))
                i += width
            } else out += next
        }
        return out
    }
}
