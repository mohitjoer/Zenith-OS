#!/usr/bin/env python3
"""port.py SRC DST [--exclude GLOB]... [--rename SRCREL=DSTREL]...

Copy text files from the Ryoku reference tree into the Zenith tree and apply the
port's rebranding rules to content and file names. Never overwrites an existing
file. Binary assets are skipped (the port does not take Ryoku's first-party
art, fonts or audio). Prints what it created and every 'ryoku' mention left.

Go files get the identifier/string rules only; fix their imports by hand.
"""
import fnmatch, os, re, stat, sys

BINARY = {'.png','.jpg','.jpeg','.gif','.webp','.ttf','.otf','.woff','.woff2','.mp3',
          '.ogg','.wav','.mp4','.webm','.qsb','.so','.ico','.icns','.svgz','.pdf','.zip','.gz','.xz'}
SKIPDIR = {'.git','vendor','node_modules','__pycache__'}
# Lines that carry attribution or upstream references are never rewritten.
PROTECT = re.compile(r'github\.com/ryoku|ryoku\.dev|Omarchy|Caelestia|SPDX-|Copyright|\u00a9|end-4|Credits? ')
RULES = [
    (r'\bRyoku\.Ui\.Singletons\b', 'qs.Ui.Singletons'),
    (r'\bRyoku\.Ui\b', 'qs.Ui'),
    (r'\bRyoku\.FrameBars\b', 'qs.FrameBars'),
    (r'\bRyoku\.PluginKit\.Singletons\b', 'qs.PluginKit.Singletons'),
    (r'\bRyoku\.PluginKit\b', 'qs.PluginKit'),
    (r'\bRyoku\.Blobs\b', 'qs.Blobs'),
    (r'ryoku-shell', 'zenith-shell'),
    (r'ryoku-hub\b', 'zenith-hubd'),
    (r'ryogami', 'zenith-wallpaper'),
    (r'ryoshot', 'zenith-shot'),
    (r'ryostage', 'zenith-stage'),
    (r'ryopin', 'zenith-pin'),
    (r'ryostore', 'zenith-store'),
    (r'ryovm', 'zenith-vm'),
    (r'ryotunes', 'zenith-tunes'),
    (r'RYOKU', 'ZENITH'),
    (r'Ryoku', 'Zenith'),
    (r'ryoku', 'zenith'),
]
COMPILED = [(re.compile(a), b) for a, b in RULES]

def rebrand_line(line):
    if PROTECT.search(line):
        return line
    for rx, rep in COMPILED:
        line = rx.sub(rep, line)
    return line

def rebrand_name(name):
    for rx, rep in COMPILED[6:]:
        name = rx.sub(rep, name)
    return name

def main():
    a = sys.argv[1:]
    if len(a) < 2:
        print(__doc__); sys.exit(2)
    src, dst = a[0], a[1]
    excl, ren = [], {}
    i = 2
    while i < len(a):
        if a[i] == '--exclude': excl.append(a[i+1]); i += 2
        elif a[i] == '--rename':
            s, d = a[i+1].split('=', 1); ren[s] = d; i += 2
        else:
            print('bad arg', a[i]); sys.exit(2)
    created, skipped, left = [], [], []
    for d, ds, fs in os.walk(src):
        ds[:] = [x for x in ds if x not in SKIPDIR and not any(fnmatch.fnmatch(os.path.relpath(os.path.join(d, x), src), e) for e in excl)]
        for f in fs:
            full = os.path.join(d, f)
            rel = os.path.relpath(full, src)
            if any(fnmatch.fnmatch(rel, e) or fnmatch.fnmatch(f, e) for e in excl):
                continue
            if os.path.splitext(f)[1].lower() in BINARY:
                skipped.append(rel); continue
            outrel = ren.get(rel) or os.path.join(os.path.dirname(rel), rebrand_name(f))
            out = os.path.join(dst, outrel)
            if os.path.exists(out):
                skipped.append(rel + '  (exists)'); continue
            try:
                with open(full, encoding='utf-8') as fh: text = fh.read()
            except UnicodeDecodeError:
                skipped.append(rel + '  (not utf-8)'); continue
            new = ''.join(rebrand_line(l) for l in text.splitlines(keepends=True))
            os.makedirs(os.path.dirname(out), exist_ok=True)
            with open(out, 'w', encoding='utf-8') as fh: fh.write(new)
            os.chmod(out, stat.S_IMODE(os.stat(full).st_mode))
            created.append(outrel)
            for n, l in enumerate(new.splitlines(), 1):
                if re.search(r'ryoku', l, re.I) and not PROTECT.search(l):
                    left.append(f'{outrel}:{n}: {l.strip()[:110]}')
    print(f'created {len(created)} files under {dst}')
    print(f'skipped {len(skipped)} (binary assets or already present)')
    if left:
        print(f'{len(left)} unprotected mentions of ryoku remain:')
        for x in left[:40]: print('  ', x)
if __name__ == '__main__': main()
