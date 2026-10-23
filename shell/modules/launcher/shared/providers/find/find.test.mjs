import { createRequire } from "node:module";
import { execFileSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, chmodSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
const require = createRequire(import.meta.url);
const { args } = require("./find.js");

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a); }
}

// plocate is not installed everywhere this runs, so a stand-in prints a fixed
// list; what is under test is the script that narrows the list.
const root = mkdtempSync(join(tmpdir(), "find-test-"));
const home = join(root, "home");
const bin = join(root, "bin");
mkdirSync(bin);
const make = (p, dir) => { if (dir) mkdirSync(p, { recursive: true }); else { mkdirSync(join(p, ".."), { recursive: true }); writeFileSync(p, ""); } };

const listed = [
    [join(root, "usr/share/report.txt"), false],
    [join(home, "Documents/report.pdf"), false],
    [join(home, "Documents/reports"), true],
    [join(home, "My Documents/report one.pdf"), false],
    [join(home, ".config/report.conf"), false],
    [join(home, "proj/node_modules/x/report.js"), false],
    [join(home, "Pictures/report.png"), false],
    [join(home, "Videos/report.MP4"), false],
];
for (const [p, dir] of listed) make(p, dir);
writeFileSync(join(bin, "plocate"), "#!/bin/sh\ncat <<'EOF2'\n" + listed.map(l => l[0]).join("\n") + "\nEOF2\n");
chmodSync(join(bin, "plocate"), 0o755);

const run = (mode) => {
    const [cmd, ...rest] = args(mode, "report");
    return execFileSync(cmd, rest, { env: { PATH: bin + ":/usr/bin:/bin", HOME: home }, encoding: "utf8" })
        .split("\n").filter(Boolean).map(p => p.replace(home, "~"));
};

eq(run("file"), ["~/Documents/report.pdf", "~/My Documents/report one.pdf", "~/Pictures/report.png", "~/Videos/report.MP4"],
    "file mode: files under home only, no dot-directories, no node_modules, spaces kept");
eq(run("folder"), ["~/Documents/reports"], "folder mode: directories only");
eq(run("image"), ["~/Pictures/report.png"], "image mode: image extensions");
eq(run("video"), ["~/Videos/report.MP4"], "video mode: extension match ignores case");

for (let i = 0; i < 40; i++) make(join(home, "many", "f" + i), false);
writeFileSync(join(bin, "plocate"), "#!/bin/sh\nfor i in $(seq 0 39); do echo \"" + home + "/many/f$i\"; done\n");
eq(run("file").length, 25, "results are capped at 25");

rmSync(root, { recursive: true, force: true });

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");
