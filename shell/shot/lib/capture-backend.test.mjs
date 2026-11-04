import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";

const require = createRequire(import.meta.url);
const { shouldUseGrim } = require("./capture-backend.js");

assert.equal(shouldUseGrim("1"), true, "explicit fallback works");
assert.equal(shouldUseGrim("0"), false, "disabled fallback");
assert.equal(shouldUseGrim(""), false, "default retains Quickshell capture");

const qml = readFileSync(new URL("../Overlay.qml", import.meta.url), "utf8");
assert.match(qml, /active:\s*!overlay\.forceFallback/, "screencopy respects fallback");
assert.match(qml, /running:\s*!overlay\.forceFallback/, "screencopy respects fallback");
assert.match(qml, /sourceComponent:\s*Component\s*\{\s*ScreencopyView/, "capture object is lazy");
assert.match(qml, /shotLoader\.item/, "normal capture uses the dynamically loaded object");
assert.equal((qml.match(/ScreencopyView\s*\{/g) || []).length, 1,
    "no second eager screencopy instance remains");
console.log("PASS asura-shot capture backend policy and lazy Wayland source guard");
