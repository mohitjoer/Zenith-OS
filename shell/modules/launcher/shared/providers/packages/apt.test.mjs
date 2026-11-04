import { createRequire } from "node:module";
import { execFileSync } from "node:child_process";
const require = createRequire(import.meta.url);
const { parseSearch, parseInstalled, words, rank } = require("./apt.js");

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a); }
}

const search = [
    "ripgrep-doc - docs for ripgrep",
    "elpa-ag - Emacs frontend to The Silver Searcher (ag)",
    "ripgrep - Recursively searches directories for a regex pattern",
    "-oAPT::Evil - an option, not a package",
    "libripgrep-dev - rust crate",
    "",
].join("\n");
const rows = parseSearch(search);
eq(rows.map(r => r.name), ["ripgrep-doc", "elpa-ag", "ripgrep", "libripgrep-dev"], "option-looking and blank lines are dropped");
eq(rows[2].description, "Recursively searches directories for a regex pattern", "description follows the first ' - '");
eq(rank(rows, "ripgrep", 3).map(r => r.name), ["ripgrep", "ripgrep-doc", "libripgrep-dev"], "exact, then prefix, then substring, capped");

eq(parseInstalled("bash\tii \t5.2\ngcc-15-base:amd64\tii \t15.2\ngcc-15-doc\tun \t\nfoo\trc \t1\n"),
    { bash: "5.2", "gcc-15-base": "15.2" }, "only 'ii' is installed, arch qualifier dropped");

eq(words("c++  foo.bar"), ["c\\+\\+", "foo\\.bar"], "typed words are escaped for apt-cache's regex");

// the real tool agrees with the escaping: a literal "c++" must not make apt-cache fail
try {
    execFileSync("apt-cache", ["search", "--", ...words("c++")], { stdio: "pipe" });
    console.log("PASS apt-cache accepts the escaped words");
} catch (e) {
    if (e.code === "ENOENT") console.log("SKIP apt-cache not installed");
    else { failed++; console.log("FAIL apt-cache rejected the escaped words"); }
}

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");
