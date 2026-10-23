"""Find every external command the tree invokes, without running anything.

Sources: QML and JS (`command: [...]`, `execDetached`, `Spawn.run`, `sh -c`
strings), Hyprland and hypr* configuration (`exec`, `exec-once`, `bind ... exec`),
shell scripts, Go (`exec.Command`) and desktop entries. Every source reduces to
one shape: a list of words, some of them dynamic, which `unwrap` then walks
through the wrappers a command line hides behind (`flock`, `env`, a terminal's
`-e`, `sh -c`) until it reaches the program that is actually executed.
"""

from __future__ import annotations

import fnmatch
import os
import re
from dataclasses import dataclass, field

import jslex
import shparse
from jslex import DYN, Elem
from shparse import Word

# ------------------------------------------------------------------- vocabularies

# Commands the shell interprets itself. They need no provider when they appear in
# shell text; in an argv list (QML `command: [...]`) there is no shell, so these
# are skipped only when the context is a shell.
SHELL_BUILTINS = {
    ".", ":", "[", "[[", "]", "]]", "((", "alias", "bg", "bind", "break", "builtin", "caller",
    "cd", "compgen", "complete", "compopt", "continue", "declare", "dirs", "disown", "echo",
    "enable", "eval", "exec", "exit", "export", "false", "fc", "fg", "getopts", "hash", "help",
    "history", "jobs", "kill", "let", "local", "logout", "mapfile", "popd", "printf", "pushd",
    "pwd", "read", "readarray", "readonly", "return", "set", "shift", "shopt", "source",
    "suspend", "test", "times", "trap", "true", "type", "typeset", "ulimit", "umask",
    "unalias", "unset", "wait", "command", "time", "then", "do", "done", "fi", "esac",
    "else", "elif", "if", "while", "until", "for", "case", "select", "function", "in", "{", "}", "!",
}

SHELLS = {"sh", "bash", "dash", "zsh", "ksh", "ash"}

# Terminal emulators whose `-e` (or equivalent) introduces the program to run.
TERMINALS = {
    "kitty": ("-e", "--"), "ghostty": ("-e",), "foot": ("-e", "--"), "alacritty": ("-e",),
    "xterm": ("-e",), "konsole": ("-e",), "x-terminal-emulator": ("-e",),
    "gnome-terminal": ("--",), "terminator": ("-x", "-e"), "tilix": ("-e",),
    "wezterm": ("--",), "xdg-terminal-exec": (),
}

# Options that take a value, per wrapper, so the wrapped command is found after them.
_FLAG_WITH_VALUE = {
    "env": {"-u", "--unset", "-C", "--chdir", "-S"},
    "nice": {"-n", "--adjustment"},
    "ionice": {"-c", "-n", "-p", "-P", "-u", "--class", "--classdata"},
    "stdbuf": {"-i", "-o", "-e"},
    "timeout": {"-k", "--kill-after", "-s", "--signal"},
    "flock": {"-w", "--timeout", "-E", "--conflict-exit-code"},
    "systemd-run": {"-p", "--property", "-E", "--setenv", "-u", "--unit", "-r", "--description",
                    "--slice", "-M", "-H", "--working-directory", "-G"},
    "pkexec": {"--user"},
    "sudo": {"-u", "-g", "-h", "-p", "-C", "-D", "-R", "-T", "-r", "-t"},
    "xargs": {"-n", "-L", "-P", "-I", "-d", "-s", "-E", "-a", "-l"},
}
_PLAIN_WRAPPERS = {
    "nohup", "setsid", "time", "nice", "ionice", "stdbuf", "dbus-launch",
    "dbus-run-session", "unbuffer", "chrt", "taskset",
}

# Probes: the argument is the name of something being looked for, not run.
PROBES = {"which", "type", "hash", "command"}

DEFAULT_EXCLUDES = [
    "shell/vendor/**",
    "shell/i18n/**",
    "shell/modules/ii/**",
    "shell/modules/common/**",
    "shell/services/**",
    "shell/Blobs-src/**",
    "shell/legacy-shell.qml",
    "shell/**/qa/**",
    "shell/shell_test.sh",
    "shell/*/install.sh",
    "**/*_test.sh",
    "**/*.test.mjs",
    "**/*.test.js",
    "**/*_test.go",
    "**/testdata/**",
    "**/__pycache__/**",
    "tests/**",
]

SCAN_ROOTS = ["shell", "session", "system/helpers", "desktop", "system/hardware", "apps", "release/mkdeb"]


@dataclass
class Invocation:
    name: str                 # program name; "" when dynamic
    path: str                 # the path it was invoked by when not a bare name, else ""
    args: list[Word]
    file: str
    line: int
    via: str
    dynamic: bool = False
    src: str = ""             # source text of a dynamic command word, for reports
    in_shell: bool = False    # the command line was read by a shell (builtins apply)


@dataclass
class ScanResult:
    invocations: list[Invocation] = field(default_factory=list)
    probes: dict[str, set[str]] = field(default_factory=dict)            # file -> probed names
    files_scanned: int = 0
    errors: list[str] = field(default_factory=list)


# ----------------------------------------------------------------------- unwrap


def _words_from_elems(elems: list[Elem], starts: list[int]) -> list[Word]:
    out = []
    for e in elems:
        line = jslex.line_of(starts, e.pos)
        out.append(Word(e.text, e.kind != "lit", line))
    return out


def _skip_flags(args: list[Word], with_value: set[str]) -> int:
    """Index of the first word that is not an option (or the value of one)."""
    i = 0
    while i < len(args):
        w = args[i]
        if w.dyn:
            return i
        t = w.text
        if t == "--":
            return i + 1
        if t.startswith("-") and len(t) > 1:
            i += 2 if t in with_value else 1
            continue
        break
    return i


def _wrapped_index(base: str, args: list[Word]) -> int | None:
    """Where the wrapped command starts in args, or None when this is not a wrapper."""
    if base == "env":
        i = _skip_flags(args, _FLAG_WITH_VALUE["env"])
        # NAME=value assignments: the name is literal even when the value is not.
        while i < len(args) and re.match(r"[A-Za-z_][A-Za-z0-9_]*=", args[i].text):
            i += 1
        return i
    if base == "exec":
        i = 0
        while i < len(args) and not args[i].dyn and args[i].text in ("-c", "-l"):
            i += 1
        if i < len(args) and not args[i].dyn and args[i].text == "-a":
            i += 2
        return i
    if base == "command":
        i = 0
        while i < len(args) and not args[i].dyn and args[i].text == "-p":
            i += 1
        return i
    if base in _PLAIN_WRAPPERS:
        return _skip_flags(args, _FLAG_WITH_VALUE.get(base, set()))
    if base == "timeout":
        return _skip_flags(args, _FLAG_WITH_VALUE["timeout"]) + 1  # past the duration
    if base == "flock":
        return _skip_flags(args, _FLAG_WITH_VALUE["flock"]) + 1  # past the lock file
    if base in ("systemd-run", "sudo", "pkexec", "xargs"):
        return _skip_flags(args, _FLAG_WITH_VALUE[base])
    return None


def _shell_script_arg(args: list[Word]) -> Word | None:
    """For `sh -c SCRIPT`, the SCRIPT word."""
    for k, w in enumerate(args):
        if w.dyn:
            continue
        t = w.text
        if re.fullmatch(r"-[A-Za-z]*c[A-Za-z]*", t) and k + 1 < len(args):
            return args[k + 1]
        if not t.startswith("-"):
            break
    return None


def _emit(out: ScanResult, name: str, path: str, args: list[Word], file: str, line: int,
          via: str, in_shell: bool) -> None:
    out.invocations.append(Invocation(name, path, args, file, line, via, False, "", in_shell))


def unwrap(
    words: list[Word],
    out: ScanResult,
    file: str,
    line: int,
    via: str,
    in_shell: bool,
    src: str = "",
    depth: int = 0,
) -> None:
    """Append the programs `words` ends up running to out.invocations."""
    if not words or depth > 8:
        return
    head = words[0]
    if head.dyn:
        # A command word with an expansion in it cannot be resolved. A path with a
        # literal tail ("${dir}/tool.sh") is still reported, as dynamic.
        out.invocations.append(
            Invocation("", "", words[1:], file, line, via, True, src or head.text.replace(DYN, "{}"), in_shell)
        )
        return

    path = head.text if "/" in head.text else ""
    base = os.path.basename(head.text) if path else head.text
    args = words[1:]

    if in_shell and base in SHELL_BUILTINS and not path:
        if base == "eval" and args and not args[0].dyn:
            _recurse_shell(args[0].text, out, file, args[0].line or line, via, depth)
        elif base in ("exec", "command", "time") and args:
            if base == "command" and not args[0].dyn and args[0].text in ("-v", "-V"):
                return
            idx = _wrapped_index(base, args)
            if idx is not None and idx < len(args):
                unwrap(args[idx:], out, file, line, via, in_shell, src, depth + 1)
        return

    _emit(out, base, path, args, file, line, via, in_shell)

    if base in SHELLS:
        script = _shell_script_arg(args)
        if script is not None and script.text != DYN:
            _recurse_shell(script.text, out, file, script.line or line, via, depth)
        return

    if base in PROBES and not path:
        return

    if base == "hyprctl" and len(args) >= 3 and not args[0].dyn and args[0].text == "dispatch" \
            and not args[1].dyn and args[1].text == "exec":
        if args[2].text != DYN:
            _recurse_shell(args[2].text, out, file, args[2].line or line, via, depth)
        return

    if base in TERMINALS:
        flags = TERMINALS[base]
        if base == "xdg-terminal-exec":
            unwrap(args[_skip_flags(args, set()):], out, file, line, via, False, src, depth + 1)
            return
        for k, w in enumerate(args):
            if not w.dyn and w.text in flags:
                unwrap(args[k + 1:], out, file, line, via, False, src, depth + 1)
                break
        return

    if base == "flock":
        idx = _wrapped_index(base, args)
        if idx is not None and idx + 1 < len(args) and not args[idx].dyn \
                and args[idx].text in ("-c", "--command"):
            # `flock FILE -c 'command'`: the command is a shell string.
            if args[idx + 1].text != DYN:
                _recurse_shell(args[idx + 1].text, out, file, line, via, depth)
            return
        # otherwise `flock FILE CMD...` is the plain wrapper handled below

    if base == "watch":
        i = _skip_flags(args, {"-n", "--interval", "-d"})
        if i < len(args) and args[i].text != DYN:
            _recurse_shell(" ".join(w.text for w in args[i:]), out, file, line, via, depth)
        return

    idx = None if path else _wrapped_index(base, args)
    if idx is not None and idx < len(args):
        unwrap(args[idx:], out, file, line, via, False, src, depth + 1)


def _recurse_shell(script: str, out: ScanResult, file: str, line: int, via: str, depth: int) -> None:
    if depth > 6:
        return
    parsed = shparse.parse_shell(script, line)
    collect_shell(parsed, out, file, via if via.endswith("-sh") else via + "-sh", depth + 1)


def collect_shell(parsed: shparse.Parsed, out: ScanResult, file: str, via: str, depth: int = 0) -> None:
    for cmd in parsed.cmds:
        head = cmd.words[0]
        if not head.dyn and head.text in parsed.functions:
            continue
        unwrap(cmd.words, out, file, cmd.line, via, True, "", depth)


# -------------------------------------------------------------------- QML / JS

_TRIGGERS = [
    re.compile(r"\b(?:command|exec|cmd|argv)\s*[:=](?!=)"),
    re.compile(r"\bexecDetached\s*\("),
    re.compile(r"\bSpawn\s*\.\s*run\s*\("),
    re.compile(r"\.\s*exec\s*\("),
]


def _find_arrays_after(text: str, view: str, pos: int, limit: int = 600) -> list[int]:
    """Offsets of array literals in the expression that starts at pos.

    Covers `[..]`, `cond ? [..] : [..]`, `[..].concat(x)` and a wrapped
    `({command: [..]})`. The walk stops where the expression does.
    """
    n = len(view)
    i = pos
    stack: list[str] = []
    found: list[int] = []
    end = min(n, pos + limit)
    while i < end:
        c = view[i]
        # An array is a command when it stands alone or is a property of an object
        # literal; inside a call's parentheses it is an argument to a helper.
        if c == "[" and (not stack or stack[-1] == "{"):
            found.append(i)
            _, j = jslex.parse_array(text, i)
            i = j
            continue
        if c in "({":
            stack.append(c)
        elif c in ")}":
            if not stack:
                break
            stack.pop()
        elif c == ";" and not stack:
            break
        elif c == "\n" and not stack:
            k = i + 1
            while k < n and view[k] in " \t\r\n":
                k += 1
            if k >= n or view[k] not in "?:.+&|":
                break
        elif c == "," and not stack:
            break
        i += 1
    return found


_CONST_RES = [
    re.compile(r"\bproperty\s+(?:string|var|url)\s+(\w+)\s*:\s*(\"(?:[^\"\\\n]|\\.)*\"|'(?:[^'\\\n]|\\.)*')"),
    re.compile(r"\b(?:const|var|let)\s+(\w+)\s*=\s*(\"(?:[^\"\\\n]|\\.)*\"|'(?:[^'\\\n]|\\.)*')"),
]


def _constants(text: str) -> dict[str, str | None]:
    """String constants a file defines; a name defined twice resolves to nothing."""
    consts: dict[str, str | None] = {}
    for rx in _CONST_RES:
        for m in rx.finditer(text):
            val = jslex.unescape(m.group(2)[1:-1])
            name = m.group(1)
            consts[name] = val if name not in consts else None
    return consts


_CHAIN = re.compile(r"\s*(?:[A-Za-z_]\w*\.)*([A-Za-z_]\w*)\s*(?:[,\]\)]|$)")
_SHELLPATH_RE = re.compile(r"""^\s*Quickshell\s*\.\s*shellPath\(\s*(["'])(.*?)\1\s*\)\s*$""")
INSTALLED_SHELL = "/usr/share/zenith/shell/"


def scan_qml(rel: str, text: str, out: ScanResult) -> None:
    view = jslex.code_view(text)
    starts = jslex.line_starts(text)
    consts = _constants(text)
    seen: set[int] = set()
    for rx in _TRIGGERS:
        for m in rx.finditer(view):
            for apos in _find_arrays_after(text, view, m.end()):
                if apos in seen:
                    continue
                seen.add(apos)
                elems, _ = jslex.parse_array(text, apos)
                if not elems:
                    continue
                line = jslex.line_of(starts, apos)
                words = _words_from_elems(elems, starts)
                head = elems[0]
                src = ""
                if head.kind == "dyn":
                    expr = text[head.pos:head.pos + 120].split("]")[0].strip()
                    expr = re.split(r",\s*(?=[\"'`\w.]+\s*[,\]]|$)", expr)[0].strip()
                    src = expr
                    sp = _SHELLPATH_RE.match(expr)
                    chain = _CHAIN.match(text[head.pos:head.pos + 200])
                    if sp:
                        words[0] = Word(INSTALLED_SHELL + sp.group(2), False, line)
                    elif chain and consts.get(chain.group(1)) is not None:
                        words[0] = Word(consts[chain.group(1)], False, line)
                elif head.kind == "mixed":
                    src = head.text.replace(DYN, "{}")
                if _plausible_command(words):
                    unwrap(words, out, rel, line, "qml", False, src)
    _collect_probes(text, rel, out)


def _plausible_command(words: list[Word]) -> bool:
    """A command array starts with a program name, not with data."""
    if not words:
        return False
    h = words[0]
    if h.dyn:
        return True
    t = h.text
    if not t or any(ch in t for ch in " \t\n{}()<>;|&'\"=") or t.startswith(("-", "#", "@", ":")):
        return False
    return bool(re.search(r"[A-Za-z]", t))


_PROBE_RES = [
    re.compile(r"\bcommand\s+-v\s+['\"]?([A-Za-z0-9_.+-]+)"),
    re.compile(r"\bwhich\s+['\"]?([A-Za-z0-9_.+-]+)"),
    re.compile(r"\btype\s+(?:-[a-zA-Z]+\s+)?['\"]?([A-Za-z0-9_.+-]+)\s*(?:>|&&|\|\||;|\)|$)"),
    re.compile(r"\bhash\s+['\"]?([A-Za-z0-9_.+-]+)\s*(?:>|&&|\|\||;|\)|$)"),
    re.compile(r"\bexec\.LookPath\(\s*\"([^\"]+)\""),
    re.compile(r"""["']which["']\s*,\s*["']([A-Za-z0-9_.+-]+)["']"""),
    re.compile(r"""["']command["']\s*,\s*["']-v["']\s*,\s*["']([A-Za-z0-9_.+-]+)["']"""),
    re.compile(r"""(?:\[|\btest)\s+-[xef]\s+["']?(?:/usr(?:/local)?/bin/|/bin/)([A-Za-z0-9_.+-]+)"""),
    re.compile(r"""["']test["']\s*,\s*["']-[xef]["']\s*,\s*["'](?:/usr(?:/local)?/bin/|/bin/)([A-Za-z0-9_.+-]+)["']"""),
]


def _collect_probes(text: str, rel: str, out: ScanResult) -> None:
    names = out.probes.setdefault(rel, set())
    for rx in _PROBE_RES:
        for m in rx.finditer(text):
            names.add(m.group(1))


# ------------------------------------------------------------------ shell files


def scan_script(rel: str, text: str, out: ScanResult) -> None:
    collect_shell(shparse.parse_shell(text), out, rel, "sh")
    _collect_probes(text, rel, out)


# ----------------------------------------------------------------------- confs

_EXEC_KEYS = {
    "exec", "exec-once", "execr", "execr-once", "lock_cmd", "unlock_cmd",
    "before_sleep_cmd", "after_sleep_cmd", "on-timeout", "on-resume",
}
_BIND_KEY = re.compile(r"^bind[a-z]*$")


def _strip_conf_comment(line: str) -> str:
    """hyprlang: "#" starts a comment, "##" is a literal "#"."""
    out = []
    i = 0
    n = len(line)
    while i < n:
        c = line[i]
        if c == "#":
            if i + 1 < n and line[i + 1] == "#":
                out.append("#")
                i += 2
                continue
            break
        out.append(c)
        i += 1
    return "".join(out).strip()


def _expand_vars(value: str, variables: dict[str, str]) -> str:
    for _ in range(4):
        new = re.sub(r"\$([A-Za-z_][A-Za-z0-9_]*)", lambda m: variables.get(m.group(1), DYN), value)
        if new == value:
            break
        value = new
    return value


def read_conf_variables(texts: list[str]) -> dict[str, str]:
    """hyprlang `$name = value` definitions; they are global across sourced files."""
    variables: dict[str, str] = {}
    for text in texts:
        for raw in text.splitlines():
            m = re.match(r"\$([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$", _strip_conf_comment(raw))
            if m:
                variables[m.group(1)] = m.group(2).strip()
    return variables


def scan_conf(rel: str, text: str, out: ScanResult, variables: dict[str, str] | None = None) -> None:
    variables = variables if variables is not None else read_conf_variables([text])
    for lineno, raw in enumerate(text.splitlines(), 1):
        line = _strip_conf_comment(raw)
        if not line or line.startswith("$"):
            continue
        m = re.match(r"([A-Za-z0-9_.:-]+)\s*=\s*(.*)$", line)
        if not m:
            continue
        key, value = m.group(1), m.group(2).strip()
        cmd = None
        if key in _EXEC_KEYS:
            cmd = value
        elif _BIND_KEY.match(key):
            # MODS, KEY, DISPATCHER, ARGS... ("bindd" carries a description first)
            cmd_at = 4 if "d" in key[4:] else 3
            parts = [p.strip() for p in value.split(",", cmd_at)]
            if len(parts) > cmd_at and parts[cmd_at - 1] in ("exec", "execr"):
                cmd = parts[cmd_at]
        elif "cmd[" in value:
            cm = re.search(r"cmd\[[^\]]*\]\s*(.*)$", value)
            cmd = cm.group(1) if cm else None
        if not cmd or not cmd.strip():
            continue
        cmd = re.sub(r"^\[[^\]]*\]\s*", "", cmd)   # a window-rule prefix: `[workspace 1] app`
        cmd = _expand_vars(cmd, variables)
        collect_shell(shparse.parse_shell(cmd, lineno), out, rel, "conf")


# --------------------------------------------------------------------------- Go

_GO_STR = r'(?:"(?:[^"\\\n]|\\.)*"|`[^`]*`)'
_GO_STR_RE = re.compile(_GO_STR)
_GO_EXEC = re.compile(
    r"\bexec\.(?:Command\(\s*|CommandContext\(\s*[A-Za-z_][\w.()]*\s*,\s*)(" + _GO_STR + r")"
)


def _go_unquote(lit: str) -> str:
    if lit.startswith("`"):
        return lit[1:-1]
    try:
        return bytes(lit[1:-1], "utf-8").decode("unicode_escape")
    except UnicodeDecodeError:
        return lit[1:-1]


def _go_args(text: str, pos: int) -> list[Word]:
    """The arguments after the program name: literal strings, or one DYN word each."""
    words: list[Word] = []
    i = pos
    n = len(text)
    while i < n:
        while i < n and text[i] in " \t\r\n":
            i += 1
        if i >= n or text[i] != ",":
            break
        i += 1
        while i < n and text[i] in " \t\r\n":
            i += 1
        m = _GO_STR_RE.match(text, i)
        if m:
            words.append(Word(_go_unquote(m.group(0)), False, 0))
            i = m.end()
            continue
        j = i
        d = 0
        while j < n:
            c = text[j]
            if c in "([{":
                d += 1
            elif c in ")]}":
                if d == 0:
                    break
                d -= 1
            elif c == "," and d == 0:
                break
            j += 1
        words.append(Word(DYN, True, 0))
        i = j
    return words


def scan_go(rel: str, text: str, out: ScanResult) -> None:
    starts = jslex.line_starts(text)
    # Comments carry `exec.Command("foo")` examples; blank them first.
    cleaned = re.sub(r"//[^\n]*", lambda m: " " * len(m.group(0)), text)
    for m in _GO_EXEC.finditer(cleaned):
        line = jslex.line_of(starts, m.start())
        words = [Word(_go_unquote(m.group(1)), False, line)] + _go_args(cleaned, m.end())
        unwrap(words, out, rel, line, "go", False)
    names = out.probes.setdefault(rel, set())
    for m in re.finditer(r"\bexec\.LookPath\(\s*\"([^\"]+)\"", cleaned):
        names.add(m.group(1))


# ------------------------------------------------------------------- .desktop


def scan_desktop(rel: str, text: str, out: ScanResult) -> None:
    for lineno, raw in enumerate(text.splitlines(), 1):
        m = re.match(r"(Exec|TryExec)\s*=\s*(.*)$", raw)
        if not m:
            continue
        cmd = re.sub(r"%[a-zA-Z%]", "", m.group(2)).strip()
        if cmd:
            collect_shell(shparse.parse_shell(cmd, lineno), out, rel, "desktop")


# ------------------------------------------------------------------ the walker


def excluded(rel: str, excludes: list[str]) -> bool:
    for pat in excludes:
        if fnmatch.fnmatchcase(rel, pat):
            return True
        if pat.startswith("**/") and fnmatch.fnmatchcase(rel, pat[3:]):
            return True
        if pat.endswith("/**") and rel == pat[:-3]:
            return True
    return False


def classify(rel: str, head: str) -> str | None:
    """How a file should be scanned: qml, conf, sh, go, desktop; None to skip."""
    base = os.path.basename(rel)
    ext = os.path.splitext(base)[1]
    if ext in (".qml", ".js"):
        return "qml"
    if ext == ".go":
        return "go"
    if ext == ".desktop":
        return "desktop"
    if ext == ".conf" and rel.startswith("session/hyprland/"):
        return "conf"
    if ext in (".sh", ".bash"):
        return "sh"
    first = head.split("\n", 1)[0]
    if first.startswith("#!") and re.search(r"\b(sh|bash|dash|zsh)\b", first):
        return "sh"
    if rel.startswith("release/mkdeb/") and base in ("postinst", "prerm", "postrm", "preinst"):
        return "sh"
    return None


def iter_files(root: str, roots: list[str], excludes: list[str]):
    for r in roots:
        top = os.path.join(root, r)
        if os.path.isfile(top):
            yield top, os.path.relpath(top, root)
            continue
        for d, dirs, files in os.walk(top):
            reld = os.path.relpath(d, root)
            dirs[:] = sorted(
                x for x in dirs
                if not x.startswith(".") and not excluded(os.path.join(reld, x), excludes)
            )
            for f in sorted(files):
                rel = os.path.join(reld, f)
                if not excluded(rel, excludes):
                    yield os.path.join(root, rel), rel


def scan_tree(root: str, roots: list[str] | None = None, excludes: list[str] | None = None) -> ScanResult:
    roots = roots or SCAN_ROOTS
    excludes = DEFAULT_EXCLUDES if excludes is None else excludes
    out = ScanResult()
    confs: list[tuple[str, str]] = []
    for full, rel in iter_files(root, roots, excludes):
        try:
            with open(full, "r", encoding="utf-8", errors="replace") as fh:
                text = fh.read()
        except OSError as exc:
            out.errors.append(f"{rel}: {exc}")
            continue
        kind = classify(rel, text[:200])
        if kind is None:
            continue
        out.files_scanned += 1
        try:
            if kind == "qml":
                scan_qml(rel, text, out)
            elif kind == "sh":
                scan_script(rel, text, out)
            elif kind == "go":
                scan_go(rel, text, out)
            elif kind == "desktop":
                scan_desktop(rel, text, out)
            elif kind == "conf":
                confs.append((rel, text))
        except Exception as exc:  # a half-written file from a parallel lane must not stop the run
            out.errors.append(f"{rel}: {type(exc).__name__}: {exc}")
    variables = read_conf_variables([t for _, t in confs])
    for rel, text in confs:
        try:
            scan_conf(rel, text, out, variables)
        except Exception as exc:
            out.errors.append(f"{rel}: {type(exc).__name__}: {exc}")
    return out
