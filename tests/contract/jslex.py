"""Just enough JavaScript/QML lexing to find the argument arrays of commands.

The contract checkers never evaluate QML. They need two things: to know which
characters are code (a quoted `kitty` in a comment is not an invocation), and to
read an array literal such as `["sh", "-c", `echo ${x}`]` into elements, keeping
the literal parts and marking the dynamic ones. This module is only that.

A dynamic part is represented by DYN (a NUL) inside the element text, so a
consumer can tell `zenith-${name}` from `zenith-shell` without a second type.
"""

from __future__ import annotations

import re
from dataclasses import dataclass

DYN = "\0"

_ESCAPES = {"n": "\n", "t": "\t", "r": "\r", "b": "\b", "f": "\f", "v": "\v", "0": "\0"}
_IDENT = re.compile(r"[A-Za-z0-9_$]")

# After one of these a "/" starts a regular expression rather than dividing.
_REGEX_AFTER_WORD = {
    "return", "typeof", "case", "in", "of", "delete", "void", "throw", "new",
    "else", "do", "instanceof",
}


@dataclass
class Elem:
    """One element of an array literal.

    kind is "lit" for a fully literal string, "mixed" for a string with dynamic
    parts (text carries DYN where they were), and "dyn" for anything else.
    """

    kind: str
    text: str
    pos: int


def unescape(body: str) -> str:
    """Resolve the escapes of a JS string body. Unknown escapes keep the char."""
    if "\\" not in body:
        return body
    out = []
    i = 0
    n = len(body)
    while i < n:
        c = body[i]
        if c != "\\" or i + 1 >= n:
            out.append(c)
            i += 1
            continue
        d = body[i + 1]
        if d in _ESCAPES:
            out.append(_ESCAPES[d])
            i += 2
        elif d == "x" and re.fullmatch(r"[0-9a-fA-F]{2}", body[i + 2:i + 4]):
            out.append(chr(int(body[i + 2:i + 4], 16)))
            i += 4
        elif d == "u" and re.fullmatch(r"[0-9a-fA-F]{4}", body[i + 2:i + 6]):
            out.append(chr(int(body[i + 2:i + 6], 16)))
            i += 6
        elif d == "\n":
            i += 2
        else:
            out.append(d)
            i += 2
    return "".join(out)


def _skip_string(text: str, i: int) -> int:
    """text[i] is a quote; return the index just past the closing quote."""
    q = text[i]
    i += 1
    n = len(text)
    while i < n:
        c = text[i]
        if c == "\\":
            i += 2
            continue
        if c == q or c == "\n":
            return i + 1
        i += 1
    return n


def _skip_template(text: str, i: int) -> int:
    """text[i] is a backtick; return the index just past the closing one."""
    i += 1
    n = len(text)
    while i < n:
        c = text[i]
        if c == "\\":
            i += 2
            continue
        if c == "`":
            return i + 1
        if c == "$" and text.startswith("${", i):
            i = _skip_braces(text, i + 1)
            continue
        i += 1
    return n


def _skip_braces(text: str, i: int) -> int:
    """text[i] is "{"; return the index just past its matching "}"."""
    depth = 0
    n = len(text)
    while i < n:
        c = text[i]
        if c in "\"'":
            i = _skip_string(text, i)
            continue
        if c == "`":
            i = _skip_template(text, i)
            continue
        if c == "/" and i + 1 < n and text[i + 1] in "/*":
            i = _skip_comment(text, i)
            continue
        if c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return i + 1
        i += 1
    return n


def _skip_comment(text: str, i: int) -> int:
    if text.startswith("//", i):
        j = text.find("\n", i)
        return len(text) if j < 0 else j
    j = text.find("*/", i + 2)
    return len(text) if j < 0 else j + 2


def _skip_regex(text: str, i: int) -> int:
    """text[i] is the opening "/" of a regex literal."""
    n = len(text)
    i += 1
    in_class = False
    while i < n:
        c = text[i]
        if c == "\\":
            i += 2
            continue
        if c == "\n":
            return i
        if in_class:
            if c == "]":
                in_class = False
        elif c == "[":
            in_class = True
        elif c == "/":
            i += 1
            while i < n and text[i].isalpha():
                i += 1
            return i
        i += 1
    return n


def _regex_allowed(text: str, i: int) -> bool:
    """Whether a "/" at text[i] opens a regex, judged from what precedes it."""
    j = i - 1
    while j >= 0 and text[j] in " \t\r\n":
        j -= 1
    if j < 0:
        return True
    c = text[j]
    if c in ")]":
        return False
    if _IDENT.match(c):
        k = j
        while k >= 0 and _IDENT.match(text[k]):
            k -= 1
        return text[k + 1:j + 1] in _REGEX_AFTER_WORD
    if c == "}":
        return False
    return True


def code_view(text: str) -> str:
    """The text with comments, strings, templates and regexes blanked.

    Offsets and newlines are preserved, so a match in the view is a match in the
    source. Trigger searches run on the view so they cannot fire inside a string.
    """
    out = list(text)
    n = len(text)
    i = 0

    def blank(a: int, b: int) -> None:
        for k in range(a, min(b, n)):
            if out[k] != "\n":
                out[k] = " "

    while i < n:
        c = text[i]
        if c in "\"'":
            j = _skip_string(text, i)
            blank(i, j)
            i = j
        elif c == "`":
            j = _skip_template(text, i)
            blank(i, j)
            i = j
        elif c == "/" and i + 1 < n and text[i + 1] in "/*":
            j = _skip_comment(text, i)
            blank(i, j)
            i = j
        elif c == "/" and _regex_allowed(text, i):
            j = _skip_regex(text, i)
            blank(i, j)
            i = j
        else:
            i += 1
    return "".join(out)


def template_text(text: str, i: int) -> tuple[str, bool, int]:
    """Read the template literal at text[i]; return (text, has_dynamic, end)."""
    end = _skip_template(text, i)
    out = []
    dyn = False
    j = i + 1
    stop = end - 1 if end > i + 1 and text[end - 1] == "`" else end
    while j < stop:
        c = text[j]
        if c == "\\":
            out.append(unescape(text[j:j + 2]))
            j += 2
            continue
        if c == "$" and text.startswith("${", j):
            k = _skip_braces(text, j + 1)
            out.append(DYN)
            dyn = True
            j = k
            continue
        out.append(c)
        j += 1
    return "".join(out), dyn, end


def _split_top(text: str, start: int) -> tuple[int, list[tuple[int, int]]]:
    """Walk from start at depth 0, cutting at each top-level ",".

    Returns (index of the bracket that closed the walk, [(a, b) element spans]).
    Used to read the elements of an array literal: the walk ends at the "]" that
    closes it.
    """
    n = len(text)
    i = start
    depth = 0
    spans: list[tuple[int, int]] = []
    seg = start
    while i < n:
        c = text[i]
        if c in "\"'":
            i = _skip_string(text, i)
            continue
        if c == "`":
            i = _skip_template(text, i)
            continue
        if c == "/" and i + 1 < n and text[i + 1] in "/*":
            i = _skip_comment(text, i)
            continue
        if c == "/" and _regex_allowed(text, i):
            i = _skip_regex(text, i)
            continue
        if c in "([{":
            depth += 1
        elif c in ")]}":
            if depth == 0:
                spans.append((seg, i))
                return i, spans
            depth -= 1
        elif c == "," and depth == 0:
            spans.append((seg, i))
            seg = i + 1
        i += 1
    spans.append((seg, n))
    return n, spans


def _classify(text: str, a: int, b: int) -> Elem | None:
    """Classify the element expression text[a:b]."""
    seg = text[a:b]
    if not seg.strip():
        return None
    if re.match(r"\s*\.\.\.", code_view(seg)):
        return Elem("dyn", DYN, a)

    # Tokenise into string pieces and "+" joins; anything else makes it dynamic.
    pieces: list[str] = []
    dyn = False
    i = 0
    n = len(seg)
    expect_operand = True
    while i < n:
        c = seg[i]
        if c.isspace():
            i += 1
            continue
        if c == "/" and i + 1 < n and seg[i + 1] in "/*":
            i = _skip_comment(seg, i)
            continue
        if expect_operand:
            if c in "\"'":
                j = _skip_string(seg, i)
                pieces.append(unescape(seg[i + 1:j - 1]))
                i = j
            elif c == "`":
                body, d, j = template_text(seg, i)
                pieces.append(body)
                dyn = dyn or d
                i = j
            else:
                # An identifier chain, call, parenthesised or other expression:
                # scan to the next top-level "+" and call the whole thing dynamic.
                j = _scan_operand(seg, i)
                if j == i:
                    return Elem("dyn", DYN, a)
                operand = seg[i:j].strip()
                if re.fullmatch(r"\d+(\.\d+)?", operand):
                    pieces.append(operand)
                else:
                    pieces.append(DYN)
                    dyn = True
                i = j
            expect_operand = False
        else:
            if c == "+":
                expect_operand = True
                i += 1
            else:
                return Elem("dyn", DYN, a)
    if expect_operand and pieces:
        return Elem("dyn", DYN, a)
    if not pieces:
        return None
    joined = "".join(pieces)
    if not dyn:
        return Elem("lit", joined, a)
    if all(p == DYN for p in pieces):
        return Elem("dyn", DYN, a)
    return Elem("mixed", joined, a)


def _scan_operand(seg: str, i: int) -> int:
    """Advance over one non-string operand, stopping at a top-level "+" or end."""
    n = len(seg)
    depth = 0
    j = i
    while j < n:
        c = seg[j]
        if c in "\"'":
            j = _skip_string(seg, j)
            continue
        if c == "`":
            j = _skip_template(seg, j)
            continue
        if c in "([{":
            depth += 1
        elif c in ")]}":
            depth -= 1
        elif c == "+" and depth == 0:
            # A "++" or a unary context is not a join; treat any "+" as a join.
            break
        elif c in "?:=<>!&|" and depth == 0:
            # A ternary, comparison or logical expression: the whole element is
            # dynamic, so swallow the rest of it.
            return n
        j += 1
    return j


def parse_array(text: str, i: int) -> tuple[list[Elem], int]:
    """Parse the array literal opening at text[i] == "["; return (elements, end)."""
    end, spans = _split_top(text, i + 1)
    elems: list[Elem] = []
    for a, b in spans:
        e = _classify(text, a, b)
        if e is not None:
            elems.append(e)
    return elems, end + 1


def line_starts(text: str) -> list[int]:
    starts = [0]
    for m in re.finditer("\n", text):
        starts.append(m.end())
    return starts


def line_of(starts: list[int], pos: int) -> int:
    """1-based line number of text offset pos."""
    lo, hi = 0, len(starts)
    while lo < hi:
        mid = (lo + hi) // 2
        if starts[mid] <= pos:
            lo = mid + 1
        else:
            hi = mid
    return lo
