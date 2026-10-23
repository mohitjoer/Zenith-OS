"""A small shell parser: enough to list the commands a script would run.

It is not a shell. It reads POSIX sh and bash well enough to answer one question:
which words are in command position? Everything else (variables, arithmetic,
heredoc bodies, case patterns, test expressions) is skipped, not evaluated.

Expansions that cannot be known statically become DYN (a NUL), the same
placeholder jslex uses, so a QML string with `${x}` holes parses the same way as
a script with `$x` holes. A command word that contains DYN cannot be resolved and
is reported as dynamic rather than guessed at.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field

from jslex import DYN, line_of, line_starts

_IDENT_FN = re.compile(r"[A-Za-z_][A-Za-z0-9_.:+-]*")
_ASSIGN = re.compile(r"[A-Za-z_][A-Za-z0-9_]*(\[[^\]]*\])?\+?=")

# Words that start or close a compound command and are not themselves commands.
_LEADERS = {"if", "then", "elif", "else", "do", "while", "until", "!", "{", "time", "coproc"}
_TERMINAL_WORDS = {"fi", "done", "}", "esac", "in"}


@dataclass
class Word:
    text: str
    dyn: bool
    line: int


@dataclass
class Cmd:
    words: list[Word]
    line: int


@dataclass
class Parsed:
    cmds: list[Cmd] = field(default_factory=list)
    functions: set[str] = field(default_factory=set)


class _Lexer:
    def __init__(self, src: str, base_line: int, subs: list[Cmd], funcs: set[str]):
        self.s = src
        self.n = len(src)
        self.i = 0
        self.starts = line_starts(src)
        self.base = base_line - 1
        self.tokens: list[tuple[str, object]] = []
        self.heredocs: list[tuple[str, bool]] = []
        self.subs = subs
        self.funcs = funcs

    # ---------------------------------------------------------------- helpers
    def line(self, pos: int | None = None) -> int:
        return self.base + line_of(self.starts, self.i if pos is None else pos)

    def _sub(self, inner: str, at: int) -> None:
        p = parse_shell(inner, self.line(at))
        self.subs.extend(p.cmds)
        self.funcs.update(p.functions)

    def _match_paren(self, i: int) -> int:
        """s[i] is the "(" after "$"; return the index of its matching ")"."""
        s, n = self.s, self.n
        depth = 0
        j = i
        while j < n:
            c = s[j]
            if c == "\\":
                j += 2
                continue
            if c == "'":
                k = s.find("'", j + 1)
                j = n if k < 0 else k + 1
                continue
            if c == '"':
                j += 1
                while j < n and s[j] != '"':
                    j += 2 if s[j] == "\\" else 1
                j += 1
                continue
            if c == "(":
                depth += 1
            elif c == ")":
                depth -= 1
                if depth == 0:
                    return j
            j += 1
        return n

    def _match_brace(self, i: int) -> int:
        s, n = self.s, self.n
        depth = 0
        j = i
        while j < n:
            c = s[j]
            if c == "\\":
                j += 2
                continue
            if c == "'":
                k = s.find("'", j + 1)
                j = n if k < 0 else k + 1
                continue
            if c == '"':
                j += 1
                while j < n and s[j] != '"':
                    j += 2 if s[j] == "\\" else 1
                j += 1
                continue
            if c == "{":
                depth += 1
            elif c == "}":
                depth -= 1
                if depth == 0:
                    return j
            j += 1
        return n

    # ----------------------------------------------------------------- pieces
    def _dollar(self, buf: list[str]) -> bool:
        """Handle "$" at s[i]; append to buf; return True when it made a DYN."""
        s, n = self.s, self.n
        i = self.i
        nxt = s[i + 1] if i + 1 < n else ""
        if nxt == "(":
            if s.startswith("$((", i):
                j = self._match_paren(i + 1)
                self.i = j + 1
            else:
                j = self._match_paren(i + 1)
                self._sub(s[i + 2:j], i + 2)
                self.i = j + 1
            buf.append(DYN)
            return True
        if nxt == "{":
            j = self._match_brace(i + 1)
            self.i = j + 1
            buf.append(DYN)
            return True
        if nxt == "'":
            j = i + 2
            out = []
            while j < n and s[j] != "'":
                if s[j] == "\\" and j + 1 < n:
                    esc = s[j + 1]
                    out.append({"n": "\n", "t": "\t", "r": "\r", "e": "\x1b"}.get(esc, esc))
                    j += 2
                else:
                    out.append(s[j])
                    j += 1
            buf.append("".join(out))
            self.i = j + 1
            return False
        if nxt == '"':
            self.i = i + 1
            return self._dquote(buf)
        if nxt and (nxt.isalnum() or nxt in "_@*#?$!-"):
            j = i + 1
            if nxt.isalpha() or nxt == "_":
                while j < n and (s[j].isalnum() or s[j] == "_"):
                    j += 1
            else:
                j += 1
            self.i = j
            buf.append(DYN)
            return True
        buf.append("$")
        self.i = i + 1
        return False

    def _backtick(self, buf: list[str]) -> None:
        s, n = self.s, self.n
        j = self.i + 1
        out = []
        while j < n and s[j] != "`":
            if s[j] == "\\" and j + 1 < n and s[j + 1] in "`\\$":
                out.append(s[j + 1])
                j += 2
            else:
                out.append(s[j])
                j += 1
        self._sub("".join(out), self.i + 1)
        self.i = j + 1
        buf.append(DYN)

    def _dquote(self, buf: list[str]) -> bool:
        """s[i] is the opening double quote."""
        s, n = self.s, self.n
        self.i += 1
        dyn = False
        while self.i < n:
            c = s[self.i]
            if c == '"':
                self.i += 1
                return dyn
            if c == "\\" and self.i + 1 < n:
                d = s[self.i + 1]
                if d in '"$`\\':
                    buf.append(d)
                    self.i += 2
                elif d == "\n":
                    self.i += 2
                else:
                    buf.append("\\")
                    self.i += 1
                continue
            if c == "$":
                dyn = self._dollar(buf) or dyn
                continue
            if c == "`":
                self._backtick(buf)
                dyn = True
                continue
            buf.append(c)
            self.i += 1
        return dyn

    def _word(self) -> Word | None:
        s, n = self.s, self.n
        start = self.i
        buf: list[str] = []
        dyn = False
        while self.i < n:
            c = s[self.i]
            if c in " \t\n;&|<>)":
                break
            if c == "(":
                head = "".join(buf)
                if head and _ASSIGN.fullmatch(head):
                    j = self._match_paren(self.i)
                    self.i = j + 1
                    buf.append(DYN)
                    dyn = True
                    continue
                break
            if c == "\\":
                if self.i + 1 < n:
                    if s[self.i + 1] == "\n":
                        self.i += 2
                        continue
                    buf.append(s[self.i + 1])
                    self.i += 2
                else:
                    self.i += 1
                continue
            if c == "'":
                j = s.find("'", self.i + 1)
                if j < 0:
                    j = n
                buf.append(s[self.i + 1:j])
                self.i = j + 1
                continue
            if c == '"':
                dyn = self._dquote(buf) or dyn
                continue
            if c == "$":
                dyn = self._dollar(buf) or dyn
                continue
            if c == "`":
                self._backtick(buf)
                dyn = True
                continue
            buf.append(c)
            self.i += 1
        if self.i == start:
            return None
        text = "".join(buf)
        return Word(text, dyn or DYN in text, self.line(start))

    def _redirect_target(self) -> None:
        while self.i < self.n and self.s[self.i] in " \t":
            self.i += 1
        self._word()

    def _heredoc_body(self) -> None:
        """Consume the bodies of pending heredocs; s[i] is just past a newline."""
        s, n = self.s, self.n
        for delim, strip in self.heredocs:
            while self.i < n:
                j = s.find("\n", self.i)
                line = s[self.i:] if j < 0 else s[self.i:j]
                self.i = n if j < 0 else j + 1
                check = line.lstrip("\t") if strip else line
                if check == delim:
                    break
        self.heredocs = []

    # ------------------------------------------------------------------- main
    def run(self) -> list[tuple[str, object]]:
        s, n = self.s, self.n
        while self.i < n:
            c = s[self.i]
            if c in " \t":
                self.i += 1
            elif c == "\n":
                self.tokens.append(("OP", "NL"))
                self.i += 1
                if self.heredocs:
                    self._heredoc_body()
            elif c == "\\" and self.i + 1 < n and s[self.i + 1] == "\n":
                self.i += 2
            elif c == "#":
                j = s.find("\n", self.i)
                self.i = n if j < 0 else j
            elif c == ";":
                if s.startswith(";;", self.i):
                    self.tokens.append(("OP", ";;"))
                    self.i += 3 if s.startswith(";;&", self.i) else 2
                elif s.startswith(";&", self.i):
                    self.tokens.append(("OP", ";;"))
                    self.i += 2
                else:
                    self.tokens.append(("OP", ";"))
                    self.i += 1
            elif c == "&":
                if s.startswith("&&", self.i):
                    self.tokens.append(("OP", "&&"))
                    self.i += 2
                elif s.startswith("&>", self.i):
                    self.i += 3 if s.startswith("&>>", self.i) else 2
                    self._redirect_target()
                else:
                    self.tokens.append(("OP", "&"))
                    self.i += 1
            elif c == "|":
                if s.startswith("||", self.i):
                    self.tokens.append(("OP", "||"))
                    self.i += 2
                elif s.startswith("|&", self.i):
                    self.tokens.append(("OP", "|"))
                    self.i += 2
                else:
                    self.tokens.append(("OP", "|"))
                    self.i += 1
            elif c == "(":
                if s.startswith("((", self.i):
                    j = s.find("))", self.i + 2)
                    j = n if j < 0 else j
                    self.i = min(n, j + 2)
                    self.tokens.append(("W", Word("((", True, self.line())))
                else:
                    self.tokens.append(("OP", "("))
                    self.i += 1
            elif c == ")":
                self.tokens.append(("OP", ")"))
                self.i += 1
            elif c in "<>":
                if s.startswith("<(", self.i) or s.startswith(">(", self.i):
                    j = self._match_paren(self.i + 1)
                    self._sub(s[self.i + 2:j], self.i + 2)
                    self.i = j + 1
                    self.tokens.append(("W", Word(DYN, True, self.line())))
                elif s.startswith("<<<", self.i):
                    self.i += 3
                    self._redirect_target()
                elif s.startswith("<<", self.i):
                    strip = s.startswith("<<-", self.i)
                    self.i += 3 if strip else 2
                    while self.i < n and s[self.i] in " \t":
                        self.i += 1
                    w = self._word()
                    if w is not None:
                        self.heredocs.append((w.text, strip))
                else:
                    self.i += 1
                    if self.i < n and s[self.i] in ">&|<":
                        self.i += 1
                    self._redirect_target()
            else:
                w = self._word()
                if w is None:
                    self.i += 1
                    continue
                if self.i < n and s[self.i] in "<>" and (
                    w.text.isdigit() or re.fullmatch(r"\{[A-Za-z_][A-Za-z0-9_]*\}", w.text)
                ):
                    continue    # `2>file` or `{fd}<file`: a descriptor, not a word
                self.tokens.append(("W", w))
                if w.text == "[[" and self._at_command_start():
                    m = re.compile(r"(?:^|(?<=[\s]))\]\](?=$|[\s;&|)])", re.M).search(s, self.i)
                    self.i = n if m is None else m.end()
        return self.tokens

    def _at_command_start(self) -> bool:
        # tokens[-1] is the "[[" word itself.
        if len(self.tokens) < 2:
            return True
        kind, val = self.tokens[-2]
        if kind == "OP":
            return True
        return isinstance(val, Word) and val.text in _LEADERS


def _segments(tokens: list[tuple[str, object]], funcs: set[str]) -> list[tuple[list[Word], str]]:
    out: list[tuple[list[Word], str]] = []
    cur: list[Word] = []
    i = 0
    n = len(tokens)
    while i < n:
        kind, val = tokens[i]
        if kind == "W":
            w: Word = val  # type: ignore[assignment]
            if (
                not cur
                and not w.dyn
                and _IDENT_FN.fullmatch(w.text)
                and i + 2 < n
                and tokens[i + 1] == ("OP", "(")
                and tokens[i + 2] == ("OP", ")")
            ):
                funcs.add(w.text)
                i += 3
                continue
            cur.append(w)
        else:
            out.append((cur, str(val)))
            cur = []
        i += 1
    if cur:
        out.append((cur, "NL"))
    return out


def parse_shell(src: str, base_line: int = 1) -> Parsed:
    """Parse shell source into the simple commands it would run."""
    result = Parsed()
    subs: list[Cmd] = []
    toks = _Lexer(src, base_line, subs, result.functions).run()
    segments = _segments(toks, result.functions)

    case_stack: list[bool] = []
    for words, term in segments:
        if case_stack and case_stack[-1]:
            first = words[0].text if words else ""
            if first == "esac" and not words[0].dyn:
                case_stack.pop()
                if term == ";;" and case_stack:
                    case_stack[-1] = True
            elif term == ")":
                case_stack[-1] = False
            continue

        words = list(words)
        while words:
            head = words[0]
            if _ASSIGN.match(head.text) is not None:
                words.pop(0)
            elif not head.dyn and head.text in _LEADERS:
                words.pop(0)
            elif not head.dyn and head.text == "function" and len(words) > 1:
                result.functions.add(words[1].text)
                del words[:2]
            else:
                break
        if not words:
            if term == ";;" and case_stack:
                case_stack[-1] = True
            continue

        head = words[0]
        if not head.dyn and head.text in ("for", "select"):
            continue
        if not head.dyn and head.text == "case":
            case_stack.append(term != ")")
            continue
        if not head.dyn and head.text == "esac":
            if case_stack:
                case_stack.pop()
            if term == ";;" and case_stack:
                case_stack[-1] = True
            continue
        if not head.dyn and head.text in _TERMINAL_WORDS:
            if term == ";;" and case_stack:
                case_stack[-1] = True
            continue
        result.cmds.append(Cmd(words, head.line))
        if term == ";;" and case_stack:
            case_stack[-1] = True

    result.cmds.extend(subs)
    return result
