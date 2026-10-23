#!/usr/bin/env python3
"""Daemon verbs and IPC functions the tree calls must exist: `zenith|zenith-shell|zenith-hubd|
zenith-wallpaper VERB` needs VERB as a quoted word ("clip-copy " is a prefix match) in desktop/cmd/NAME/*.go;
`ipc call TARGET FN` needs an IpcHandler in shell/ declaring both. First word only. --report: exit 0."""
import glob, os, re, sys
sys.dont_write_bytecode = True
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import extract, jslex
ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
read = lambda f: open(f, errors="replace").read()
verbs = {n: set(re.findall(r'"([a-z][a-z0-9_-]*) ?"', "".join(read(f) for f in glob.glob(f"{ROOT}/desktop/cmd/{n}/*.go")
                                                          if not f.endswith("_test.go"))))
         for n in ("zenith", "zenith-shell", "zenith-hubd", "zenith-wallpaper")}
ipc = {}  # IpcHandler target -> its functions
for f in glob.glob(f"{ROOT}/shell/**/*.qml", recursive=True):
    if extract.excluded(os.path.relpath(f, ROOT), extract.DEFAULT_EXCLUDES):
        continue
    t = read(f); view = jslex.code_view(t)
    for m in re.finditer(r"IpcHandler\s*\{", view):
        end, depth = m.end(), 1
        while depth and end < len(view):
            depth += {"{": 1, "}": -1}.get(view[end], 0); end += 1
        tg = re.search(r'target:\s*"([^"]+)"', t[m.end():end])
        ipc.setdefault(tg and tg.group(1), set()).update(re.findall(r"function\s+(\w+)\s*\(", view[m.end():end]))
bad = [f"no Go source for {n} yet, its verbs are unchecked" for n, v in verbs.items() if not v]
for i in extract.scan_tree(ROOT).invocations:
    a = [w.text for w in i.args]
    if verbs.get(i.name) and a and re.fullmatch(r"[a-z][a-z0-9_-]*", a[0]) and a[0] not in verbs[i.name]:
        bad.append(f"{i.file}:{i.line}: {i.name} {a[0]}: not a verb in desktop/cmd/{i.name}")
    if "ipc" in a and "call" in a[a.index("ipc"):]:
        k = a.index("call", a.index("ipc"))
        if len(a) > k + 2 and jslex.DYN not in a[k + 1] + a[k + 2] and a[k + 2] not in ipc.get(a[k + 1], ()):
            bad.append(f"{i.file}:{i.line}: ipc call {a[k + 1]} {a[k + 2]}: no such IpcHandler function")
print("\n".join(bad) or "daemon: ok")
sys.exit(0 if "--report" in sys.argv or not bad else 1)
